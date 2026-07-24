// Package session wires an incoming ssh.Session to a container: build the
// run arguments, start it, pipe stdio (or a PTY) through, and kill the
// container when the session ends.
package session

import (
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"

	"sshbox/internal/config"
	"sshbox/internal/runtime"
)

// NewHandler builds the ssh.Handler that spawns one container per
// session and tears it down when the session ends.
func NewHandler(cfg config.Config, rt runtime.Runtime, logger *slog.Logger) ssh.Handler {
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
