// sshbox runs a disposable Docker container per SSH connection.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"sshbox/internal/config"
	"sshbox/internal/runtime"
)

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "sshbox:", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	rt := runtime.New("docker")
	if err := rt.Check(); err != nil {
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
		IdleTimeout: cfg.IdleDuration,
		Handler:     newHandler(cfg, rt, logger),
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
		"idle_timeout", cfg.IdleDuration.String(),
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

func newHandler(cfg config.Config, rt runtime.Runtime, logger *slog.Logger) ssh.Handler {
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
		args := rt.RunArgs(runtime.Spec{
			Image:   cfg.Image,
			Shell:   cfg.Shell,
			Network: cfg.Network,
			Memory:  cfg.Memory,
			CPUs:    cfg.CPUs,
			IsPty:   isPty,
			Term:    ptyReq.Term,
			Name:    name,
		})
		cmd := exec.Command(rt.Binary, args...)

		// A dropped connection or an idle timeout cancels s.Context(), but
		// killing our local runtime client process would NOT stop the
		// container -- the daemon owns its lifecycle independently of the
		// CLI process that started it. So we explicitly kill the named
		// container; combined with --rm that also removes it.
		stopWatch := make(chan struct{})
		defer close(stopWatch)
		go func() {
			select {
			case <-s.Context().Done():
				if err := rt.Kill(name); err != nil {
					logger.Debug("container kill on session end", "id", id, "container", name, "err", err)
				}
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
