package session

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"sshbox/internal/config"
	"sshbox/internal/runtime"
)

// This CLI keeps a session running until sshbox explicitly kills its container.
// It exercises the real SSH transport without requiring a Docker daemon.
func testServer(t *testing.T, cfg config.Config) (string, *ssh.Server, string) {
	addr, srv, dir, _ := testServerWithHandler(t, cfg)
	return addr, srv, dir
}

func testServerWithHandler(t *testing.T, cfg config.Config) (string, *ssh.Server, string, *Handler) {
	t.Helper()
	dir := t.TempDir()
	binary := filepath.Join(dir, "runtime")
	script := `#!/bin/sh
state=${0%/*}
case "$1" in
 run)
  shift
  while [ "$1" != "--name" ]; do shift; done
  name=$2
  if [ -f "$state/delay" ]; then sleep 0.2; fi
  touch "$state/$name.started"
  echo ready
  while [ ! -f "$state/$name.killed" ]; do sleep 0.02; done
  ;;
 kill) touch "$state/$2.killed" ;;
 rm) touch "$state/$3.killed" ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(cfg, runtime.New(binary), logger, nil)
	srv := &ssh.Server{Handler: handler.Handle}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(listener)
	t.Cleanup(func() { srv.Close(); handler.Close() })
	return listener.Addr().String(), srv, dir, handler
}

func connect(t *testing.T, addr string) (*gossh.Client, *gossh.Session, *bufio.Reader) {
	return connectSession(t, addr, false)
}

func connectSession(t *testing.T, addr string, terminal bool) (*gossh.Client, *gossh.Session, *bufio.Reader) {
	t.Helper()
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User: "test", HostKeyCallback: gossh.InsecureIgnoreHostKey(), Timeout: 3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sess.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	out, err := sess.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if terminal {
		if err := sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := sess.Shell(); err != nil {
		t.Fatal(err)
	}
	return client, sess, bufio.NewReader(out)
}

func TestConcurrentSessionLimit(t *testing.T) {
	const limit, attempts = 3, 32
	addr, _, dir := testServer(t, config.Config{MaxSessions: limit})
	var wg sync.WaitGroup
	results := make(chan string, attempts)
	clients := make(chan *gossh.Client, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, _, output := connect(t, addr)
			clients <- client
			line, err := output.ReadString('\n')
			if err != nil {
				t.Error(err)
				return
			}
			results <- strings.TrimSpace(line)
		}()
	}
	wg.Wait()
	close(results)
	admitted := 0
	for line := range results {
		if line == "ready" {
			admitted++
		} else if !strings.Contains(line, "too many active sessions") {
			t.Error(line)
		}
	}
	if admitted != limit {
		t.Fatalf("admitted %d sessions, want %d", admitted, limit)
	}
	close(clients)
	for client := range clients {
		client.Close()
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		files, _ := filepath.Glob(filepath.Join(dir, "*.killed"))
		if len(files) == limit {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("containers were not killed after disconnect")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// A freed slot must accept the next connection.
	time.Sleep(50 * time.Millisecond)
	_, _, output := connect(t, addr)
	line, err := output.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatal(fmt.Sprint("slot was not released: ", line, err))
	}
}

type warningSession struct {
	ssh.Session
	warnings chan struct{}
}

func (s warningSession) Write(p []byte) (int, error) {
	select {
	case s.warnings <- struct{}{}:
	default:
	}
	return len(p), nil
}

func TestIdleWarningDoesNotExtendDeadline(t *testing.T) {
	s := warningSession{warnings: make(chan struct{}, 4)}
	last := &atomic.Int64{}
	start := time.Now()
	last.Store(start.UnixNano())
	expired := make(chan struct{})
	go watchIdle(s, last, 400*time.Millisecond, 200*time.Millisecond, make(chan struct{}), func() { close(expired) })
	select {
	case <-s.warnings:
	case <-time.After(time.Second):
		t.Fatal("no idle warning")
	}
	select {
	case <-expired:
	case <-time.After(time.Second):
		t.Fatal("idle timeout never fired")
	}
	if elapsed := time.Since(start); elapsed > 550*time.Millisecond {
		t.Fatalf("warning extended deadline: %s", elapsed)
	}
}

func TestActivityResetsIdleDeadline(t *testing.T) {
	s := warningSession{warnings: make(chan struct{}, 4)}
	last := &atomic.Int64{}
	last.Store(time.Now().UnixNano())
	expired := make(chan struct{})
	go watchIdle(s, last, 400*time.Millisecond, 200*time.Millisecond, make(chan struct{}), func() { close(expired) })
	select {
	case <-s.warnings:
	case <-time.After(time.Second):
		t.Fatal("no first warning")
	}
	reset := time.Now()
	last.Store(reset.UnixNano())
	select {
	case <-expired:
		t.Fatal("activity did not reset deadline")
	case <-time.After(300 * time.Millisecond):
	}
	select {
	case <-expired:
	case <-time.After(time.Second):
		t.Fatal("idle timeout never fired after reset")
	}
	if time.Since(reset) < 400*time.Millisecond {
		t.Fatal("disconnected too early")
	}
}

func TestPipeSessionTimesOutWithoutWarning(t *testing.T) {
	addr, _, dir := testServer(t, config.Config{IdleDuration: 200 * time.Millisecond})
	_, _, output := connect(t, addr)
	line, err := output.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("start: %q %v", line, err)
	}
	rest, err := io.ReadAll(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 0 {
		t.Fatalf("idle warning corrupted pipe output: %q", rest)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		files, _ := filepath.Glob(filepath.Join(dir, "*.killed"))
		if len(files) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("idle container was not killed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestServerShutdownWaitsForCleanup(t *testing.T) {
	addr, srv, dir, handler := testServerWithHandler(t, config.Config{})
	_, _, output := connect(t, addr)
	if line, err := output.ReadString('\n'); err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("start: %q %v", line, err)
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { handler.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.killed"))
	if len(files) != 1 {
		t.Fatal("shutdown returned before container cleanup")
	}
}

func TestDisconnectDuringStartup(t *testing.T) {
	addr, srv, dir, handler := testServerWithHandler(t, config.Config{})
	if err := os.WriteFile(filepath.Join(dir, "delay"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	client, _, _ := connect(t, addr)
	client.Close()
	srv.Close()
	done := make(chan struct{})
	go func() { handler.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("startup disconnect hung")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.killed"))
	if len(files) != 1 {
		t.Fatal("no final cleanup attempt after interrupted startup")
	}
}

func TestMultiplexedSessionsHaveSeparateContainers(t *testing.T) {
	addr, _, dir, handler := testServerWithHandler(t, config.Config{MaxSessions: 2})
	client, _, output := connect(t, addr)
	if _, err := output.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	second, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	out, err := second.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = second.Shell(); err != nil {
		t.Fatal(err)
	}
	if _, err = bufio.NewReader(out).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.started"))
	if len(files) != 2 {
		t.Fatalf("two channels shared a container name: %v", files)
	}
	client.Close()
	handler.Close()
	files, _ = filepath.Glob(filepath.Join(dir, "*.killed"))
	if len(files) != 2 {
		t.Fatal("multiplexed containers were not both cleaned up")
	}
}

func TestPTYDisconnectCleansUpContainer(t *testing.T) {
	addr, _, dir, handler := testServerWithHandler(t, config.Config{})
	client, _, output := connectSession(t, addr, true)
	if line, err := output.ReadString('\n'); err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("start: %q %v", line, err)
	}
	client.Close()
	done := make(chan struct{})
	go func() { handler.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("PTY disconnect hung")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.killed"))
	if len(files) != 1 {
		t.Fatal("PTY container was not cleaned up")
	}
}
