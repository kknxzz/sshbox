// sshbox runs a disposable Docker container per SSH connection.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
)

func main() {
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "sshbox:", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := checkDocker(); err != nil {
		logger.Error("docker not available", "err", err)
		os.Exit(1)
	}

	hostKey, err := loadOrCreateHostKey(cfg.HostKeyPath)
	if err != nil {
		logger.Error("failed to load or create host key", "path", cfg.HostKeyPath, "err", err)
		os.Exit(1)
	}

	srv := &ssh.Server{
		Addr:        cfg.ListenAddr,
		IdleTimeout: cfg.idleTimeout,
		Handler:     newHandler(cfg, logger),
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return true
		},
	}
	srv.AddHostKey(hostKey)

	var shuttingDown atomic.Bool
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down")
		shuttingDown.Store(true)
		srv.Close()
	}()

	logger.Info("listening",
		"addr", cfg.ListenAddr,
		"image", cfg.Image,
		"network", cfg.Network,
		"memory", cfg.Memory,
		"cpus", cfg.CPUs,
		"idle_timeout", cfg.idleTimeout.String(),
	)
	if err := srv.ListenAndServe(); err != nil && !shuttingDown.Load() {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// loadOrCreateHostKey reads the ssh host key from path, generating and
// saving a new one if it doesn't exist yet. Without this, gliderlabs/ssh
// generates a fresh throwaway key every time the server starts, which
// makes every restart look like a different host to any client that
// already has the old key in known_hosts.
func loadOrCreateHostKey(path string) (gossh.Signer, error) {
	if data, err := os.ReadFile(path); err == nil {
		return gossh.ParsePrivateKey(data)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	block, err := gossh.MarshalPrivateKey(priv, "sshbox host key")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		return nil, err
	}

	return gossh.NewSignerFromKey(priv)
}

// checkDocker distinguishes "docker isn't installed" from "docker is
// installed but the daemon isn't running", since the fix is different and
// the raw error from either case is not obvious to someone new to Docker.
func checkDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker CLI not found in PATH -- install Docker first")
	}

	var stderr bytes.Buffer
	cmd := exec.Command("docker", "info")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		return fmt.Errorf("docker is installed but not responding -- is the Docker daemon running? (%s)", msg)
	}
	return nil
}

// buildDockerArgs turns config plus PTY info into a `docker run` argument
// list. -t is only added when the client actually requested a PTY; TERM has
// to be passed with -e since Docker doesn't forward the host environment.
// Every container gets a fixed --name so it can be killed by name later --
// exec.Cmd only gives us a handle to the local `docker` client process, not
// the container itself.
func buildDockerArgs(cfg Config, isPty bool, term, name string) []string {
	args := []string{"run", "--rm", "-i", "--name", name}
	if isPty {
		args = append(args, "-t", "-e", "TERM="+term)
	}
	args = append(args,
		"--network", cfg.Network,
		"--memory", cfg.Memory,
		"--cpus", cfg.CPUs,
		cfg.Image, cfg.Shell,
	)
	return args
}

func newHandler(cfg Config, logger *slog.Logger) ssh.Handler {
	return func(s ssh.Session) {
		id := s.Context().SessionID()
		if len(id) > 8 {
			id = id[:8]
		}
		user, remote := s.User(), s.RemoteAddr().String()
		start := time.Now()

		logger.Info("session opened", "id", id, "user", user, "remote", remote)
		defer func() {
			logger.Info("session closed", "id", id, "user", user, "remote", remote,
				"duration", time.Since(start).Round(time.Second).String())
		}()

		ptyReq, winCh, isPty := s.Pty()
		name := "sshbox-" + id
		cmd := exec.Command("docker", buildDockerArgs(cfg, isPty, ptyReq.Term, name)...)

		// A dropped connection or an idle timeout cancels s.Context(), but
		// killing our local `docker` client process would NOT stop the
		// container -- the daemon owns its lifecycle independently of the
		// CLI process that started it. So we explicitly `docker kill` the
		// named container; combined with --rm that also removes it.
		stopWatch := make(chan struct{})
		defer close(stopWatch)
		go func() {
			select {
			case <-s.Context().Done():
				killContainer(name, logger, id)
			case <-stopWatch:
			}
		}()

		if isPty {
			runPTYSession(cmd, s, ptyReq, winCh, logger, id)
			return
		}
		runPipeSession(cmd, s, logger, id)
	}
}

func killContainer(name string, logger *slog.Logger, id string) {
	if err := exec.Command("docker", "kill", name).Run(); err != nil {
		logger.Debug("container kill on session end", "id", id, "container", name, "err", err)
	}
}

func runPTYSession(cmd *exec.Cmd, s ssh.Session, ptyReq ssh.Pty, winCh <-chan ssh.Window, logger *slog.Logger, id string) {
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(ptyReq.Window.Height),
		Cols: uint16(ptyReq.Window.Width),
	})
	if err != nil {
		logger.Error("failed to start container", "id", id, "err", err)
		fmt.Fprintln(s, "sshbox: failed to start container:", err)
		s.Exit(1)
		return
	}
	defer ptmx.Close()

	go func() {
		for win := range winCh {
			pty.Setsize(ptmx, &pty.Winsize{
				Rows: uint16(win.Height),
				Cols: uint16(win.Width),
			})
		}
	}()

	go io.Copy(ptmx, s) // client keystrokes -> container
	io.Copy(s, ptmx)    // container output -> client
	cmd.Wait()
}

func runPipeSession(cmd *exec.Cmd, s ssh.Session, logger *slog.Logger, id string) {
	cmd.Stdin = s
	cmd.Stdout = s
	cmd.Stderr = s.Stderr()
	if err := cmd.Run(); err != nil {
		logger.Error("container exited with error", "id", id, "err", err)
	}
}
