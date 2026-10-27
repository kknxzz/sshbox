// Package session wires an incoming ssh.Session to a container: build the
// run arguments, start it, pipe stdio (or a PTY) through, and kill the
// container when the session ends.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
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

func (a *activitySession) Write(p []byte) (int, error) {
	n, err := a.Session.Write(p)
	if n > 0 {
		a.lastActivity.Store(time.Now().UnixNano())
	}
	return n, err
}

type activityWriter struct {
	io.Writer
	lastActivity *atomic.Int64
}

func (w activityWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if n > 0 {
		w.lastActivity.Store(time.Now().UnixNano())
	}
	return n, err
}

func (a *activitySession) Stderr() io.ReadWriter {
	return struct {
		io.Reader
		io.Writer
	}{
		a.Session.Stderr(), activityWriter{a.Session.Stderr(), a.lastActivity},
	}
}

// Server-generated warnings bypass activity tracking. Otherwise the warning
// would extend the very deadline it announces.
func watchIdle(s ssh.Session, lastActivity *atomic.Int64, idle, warning time.Duration, stop <-chan struct{}, expire func()) {
	if idle <= 0 {
		return
	}
	interval := min(time.Second, idle/20)
	if warning > 0 {
		interval = min(interval, warning/5)
	}
	interval = max(interval, time.Millisecond)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var warnedAt int64
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			last := lastActivity.Load()
			remaining := idle - time.Since(time.Unix(0, last))
			if remaining <= 0 {
				expire()
				return
			}
			if warning > 0 && idle > warning && remaining <= warning && warnedAt != last {
				warnedAt = last
				fmt.Fprintf(s, "\r\n[sshbox] idle timeout in %s -- send any input to stay connected\r\n", remaining.Round(time.Second))
			}
		}
	}
}

// syncWriter serializes writes from multiple goroutines onto one
// underlying writer. Both directions of a session (client -> container
// and container -> client) run in separate goroutines and both write to
// the same recording file, so a plain *os.File isn't safe here on its
// own -- concurrent Write calls could interleave mid-write.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// openRecording creates a raw transcript file for one session under dir,
// named by start time and session id. Returns nil if dir is empty
// (recording disabled) or the file can't be created -- a recording
// failure logs a warning but never blocks the session itself.
func openRecording(dir, id string, start time.Time, logger *slog.Logger) (*os.File, io.Writer) {
	if dir == "" {
		return nil, nil
	}
	path := filepath.Join(dir, start.Format("20060102T150405")+"_"+id+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		logger.Error("failed to create session recording", "id", id, "path", path, "err", err)
		return nil, nil
	}
	return f, &syncWriter{w: f}
}

// Handler tracks sessions so shutdown can wait for container cleanup.
type Handler struct {
	mu      sync.Mutex
	closing bool
	wg      sync.WaitGroup
	handle  ssh.Handler
}

func (h *Handler) Handle(s ssh.Session) {
	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		s.Close()
		return
	}
	h.wg.Add(1)
	h.mu.Unlock()
	defer h.wg.Done()
	h.handle(s)
}

// Close stops admitting sessions and waits for handlers to finish.
// Close the SSH server first so active connections are canceled.
func (h *Handler) Close() {
	h.mu.Lock()
	h.closing = true
	h.mu.Unlock()
	h.wg.Wait()
}

// NewHandler builds the ssh.Handler that spawns one container per
// session and tears it down when the session ends.
func NewHandler(cfg config.Config, rt runtime.Runtime, logger, auditLogger *slog.Logger) *Handler {
	var slots chan struct{}
	if cfg.MaxSessions > 0 {
		slots = make(chan struct{}, cfg.MaxSessions)
	}

	// logBoth writes to the main logger, and to the audit log too if one
	// is configured -- a dedicated, connection-only paper trail separate
	// from the rest of sshbox's diagnostic output.
	logBoth := func(msg string, args ...any) {
		logger.Info(msg, args...)
		if auditLogger != nil {
			auditLogger.Info(msg, args...)
		}
	}

	h := &Handler{}
	h.handle = func(s ssh.Session) {
		var token [16]byte
		if _, err := rand.Read(token[:]); err != nil {
			logger.Error("failed to create session id", "err", err)
			s.Exit(1)
			return
		}
		id := hex.EncodeToString(token[:])
		defer s.Close()
		user, remote := s.User(), s.RemoteAddr().String()
		start := time.Now()

		if slots != nil {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			default:
				logBoth("session rejected, at capacity", "id", id, "user", user, "remote", remote, "max_sessions", cfg.MaxSessions)
				fmt.Fprintf(s, "sshbox: too many active sessions (max %d), try again shortly\r\n", cfg.MaxSessions)
				s.Exit(1)
				return
			}
		}

		recFile, rec := openRecording(cfg.SessionLog, id, start, logger)
		if recFile != nil {
			defer recFile.Close()
			logBoth("session opened", "id", id, "user", user, "remote", remote, "recording", recFile.Name())
		} else {
			logBoth("session opened", "id", id, "user", user, "remote", remote)
		}
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
		ctx, cancel := context.WithCancel(s.Context())
		cmd := exec.CommandContext(ctx, rt.Binary, args...)
		cmd.WaitDelay = 2 * time.Second

		stopWatch := make(chan struct{})
		watchDone := make(chan struct{})
		go func() {
			defer close(watchDone)
			select {
			case <-ctx.Done():
				if err := rt.Kill(name); err != nil {
					logger.Debug("container kill on session end", "id", id, "container", name, "err", err)
				}
			case <-stopWatch:
			}
		}()
		defer func() {
			close(stopWatch)
			cancel()
			<-watchDone
			// A disconnect can arrive before the runtime has created the
			// container. Retry removal after the run process has finished.
			if err := rt.Remove(name); err != nil {
				logger.Warn("container cleanup failed", "id", id, "container", name, "err", err)
			}
		}()

		lastActivity := &atomic.Int64{}
		lastActivity.Store(time.Now().UnixNano())
		wrapped := &activitySession{Session: s, lastActivity: lastActivity}
		warning := time.Duration(0)
		if isPty {
			warning = idleWarnBefore
		}
		go watchIdle(s, lastActivity, cfg.IdleDuration, warning, stopWatch, func() {
			cancel()
			s.Close()
		})

		if isPty {
			runPTYSession(cmd, wrapped, ptyReq, winCh, logger, id, rec, cancel)
			return
		}
		runPipeSession(cmd, wrapped, logger, id, rec)
	}
	return h
}

func runPTYSession(cmd *exec.Cmd, s ssh.Session, ptyReq ssh.Pty, winCh <-chan ssh.Window, logger *slog.Logger, id string, rec io.Writer, cancel context.CancelFunc) {
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
	resizeDone := make(chan struct{})
	resizeExited := make(chan struct{})
	defer func() { close(resizeDone); <-resizeExited }()

	go func() {
		defer close(resizeExited)
		for {
			select {
			case <-resizeDone:
				return
			case win, ok := <-winCh:
				if !ok {
					return
				}
				pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(win.Height), Cols: uint16(win.Width)})
			}
		}
	}()

	clientIn := io.Reader(s)
	containerOut := io.Writer(s)
	if rec != nil {
		clientIn = io.TeeReader(s, rec)
		containerOut = io.MultiWriter(s, rec)
	}

	inputDone := make(chan struct{})
	go func() {
		defer close(inputDone)
		io.Copy(ptmx, clientIn)
		cancel()
	}()
	io.Copy(containerOut, ptmx)
	cancel()
	s.Close()
	cmd.Wait()
	<-inputDone
}

func runPipeSession(cmd *exec.Cmd, s ssh.Session, logger *slog.Logger, id string, rec io.Writer) {
	stdin := io.Reader(s)
	stdout := io.Writer(s)
	stderr := io.Writer(s.Stderr())
	if rec != nil {
		stdin = io.TeeReader(s, rec)
		stdout = io.MultiWriter(s, rec)
		stderr = io.MultiWriter(s.Stderr(), rec)
	}

	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		logger.Error("container exited with error", "id", id, "err", err)
	}
}
