// Package session wires an incoming ssh.Session to a container: build the
// run arguments, start it, pipe stdio (or a PTY) through, and kill the
// container when the session ends.
package session

import (
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"

	"sshbox/internal/config"
	"sshbox/internal/runtime"
)

// idleWarnBefore is how long before the configured idle timeout fires
// that a client gets a heads-up warning. Not user-configurable -- it's a
// small UX nicety, not worth another setting.
const idleWarnBefore = 10 * time.Second

// activitySession wraps an ssh.Session to track the last time the client
// sent data, so the idle-timeout warning can tell how close a session
// actually is to timing out.
type activitySession struct {
	ssh.Session
	lastActivity *atomic.Int64 // unix nano
}

func (a *activitySession) Read(p []byte) (int, error) {
	n, err := a.Session.Read(p)
	if n > 0 {
		a.lastActivity.Store(time.Now().UnixNano())
	}
	return n, err
}

// warnBeforeIdleTimeout periodically checks how long it's been since the
// client last sent data and, once within idleWarnBefore of the
// configured idle timeout, writes a one-time warning to the session.
// gliderlabs/ssh enforces the actual disconnect independently of this --
// it just gives the person using the session a heads up first.
func warnBeforeIdleTimeout(s ssh.Session, lastActivity *atomic.Int64, idleDuration time.Duration, stop <-chan struct{}) {
	if idleDuration <= idleWarnBefore {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	warned := false
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			remaining := idleDuration - time.Since(time.Unix(0, lastActivity.Load()))
			switch {
			case remaining <= 0:
				return // the real timeout should be firing any moment now
			case remaining <= idleWarnBefore:
				if !warned {
					fmt.Fprintf(s, "\r\n[sshbox] idle timeout in %s -- send any input to stay connected\r\n", remaining.Round(time.Second))
					warned = true
				}
			default:
				warned = false
			}
		}
	}
}

// NewHandler builds the ssh.Handler that spawns one container per
// session and tears it down when the session ends.
func NewHandler(cfg config.Config, rt runtime.Runtime, logger, auditLogger *slog.Logger) ssh.Handler {
	var active atomic.Int64

	// logBoth writes to the main logger, and to the audit log too if one
	// is configured -- a dedicated, connection-only paper trail separate
	// from the rest of sshbox's diagnostic output.
	logBoth := func(msg string, args ...any) {
		logger.Info(msg, args...)
		if auditLogger != nil {
			auditLogger.Info(msg, args...)
		}
	}

	return func(s ssh.Session) {
		id := s.Context().SessionID()
		if len(id) > 8 {
			id = id[:8]
		}
		user, remote := s.User(), s.RemoteAddr().String()
		start := time.Now()

		if cfg.MaxSessions > 0 && active.Load() >= int64(cfg.MaxSessions) {
			logger.Info("session rejected, at capacity", "id", id, "user", user, "remote", remote, "max_sessions", cfg.MaxSessions)
			fmt.Fprintf(s, "sshbox: too many active sessions (max %d), try again shortly\r\n", cfg.MaxSessions)
			s.Exit(1)
			return
		}
		active.Add(1)
		defer active.Add(-1)

		logBoth("session opened", "id", id, "user", user, "remote", remote)
		defer func() {
			logBoth("session closed", "id", id, "user", user, "remote", remote,
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

		lastActivity := &atomic.Int64{}
		lastActivity.Store(time.Now().UnixNano())
		wrapped := &activitySession{Session: s, lastActivity: lastActivity}
		go warnBeforeIdleTimeout(wrapped, lastActivity, cfg.IdleDuration, stopWatch)

		if isPty {
			runPTYSession(cmd, wrapped, ptyReq, winCh, logger, id)
			return
		}
		runPipeSession(cmd, wrapped, logger, id)
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
