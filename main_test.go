package main

import (
	"bufio"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

func TestServerProcess(t *testing.T) {
	if os.Getenv("SSHBOX_PROCESS_TEST") != "1" {
		return
	}
	os.Args = []string{"sshbox", "--listen", os.Getenv("SSHBOX_TEST_ADDR"), "--runtime", os.Getenv("SSHBOX_TEST_RUNTIME"), "--host-key", os.Getenv("SSHBOX_TEST_HOSTKEY")}
	main()
	os.Exit(0)
}

func TestSIGTERMWaitsForContainerRemoval(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("SIGTERM test requires Unix")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "runtime")
	script := `#!/bin/sh
state=${0%/*}
case "$1" in
 run)
  shift
  while [ "$1" != "--name" ]; do shift; done
  name=$2
  echo ready
  while [ ! -f "$state/removed" ]; do sleep 0.02; done
  ;;
 rm) sleep 0.2; touch "$state/removed" ;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestServerProcess$")
	cmd.Env = append(os.Environ(), "SSHBOX_PROCESS_TEST=1", "SSHBOX_TEST_ADDR="+addr, "SSHBOX_TEST_RUNTIME="+binary, "SSHBOX_TEST_HOSTKEY="+filepath.Join(dir, "host_key"))
	cmd.Dir = dir
	logFile, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	var client *gossh.Client
	deadline := time.Now().Add(5 * time.Second)
	for {
		client, err = gossh.Dial("tcp", addr, &gossh.ClientConfig{User: "test", Auth: []gossh.AuthMethod{gossh.Password("")}, HostKeyCallback: gossh.InsecureIgnoreHostKey(), Timeout: time.Second})
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			data, _ := os.ReadFile(logFile.Name())
			t.Fatalf("server did not start: %v\n%s", err, data)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	out, err := sess.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	sess.Stderr = io.Discard
	if err = sess.Shell(); err != nil {
		t.Fatal(err)
	}
	if line, err := bufio.NewReader(out).ReadString('\n'); err != nil || strings.TrimSpace(line) != "ready" {
		t.Fatalf("start: %q %v", line, err)
	}
	if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
	if _, err = os.Stat(filepath.Join(dir, "removed")); err != nil {
		t.Fatal("server exited before container removal finished")
	}
}
