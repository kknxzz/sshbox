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
  touch "$state/$name.started"
  echo ready
  while [ ! -f "$state/$name.killed" ]; do sleep 0.02; done
  ;;
 kill) touch "$state/$2.killed" ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := &ssh.Server{Handler: NewHandler(cfg, runtime.New(binary), logger, nil)}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go srv.Serve(listener)
	t.Cleanup(func() { srv.Close() })
	return listener.Addr().String(), srv, dir
}

func connect(t *testing.T, addr string) (*gossh.Client, *gossh.Session, *bufio.Reader) {
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
	out, err := sess.StdoutPipe()
	if err != nil {
		t.Fatal(err)
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
