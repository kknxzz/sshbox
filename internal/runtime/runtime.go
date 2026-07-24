// Package runtime wraps the container runtime CLI (docker, or later
// podman) that sshbox shells out to. Both speak a compatible
// `run`/`kill`/`info` command line, so one implementation parameterized
// by binary name covers both -- there's nothing runtime-specific to
// abstract beyond that.
package runtime

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runtime is the container CLI sshbox drives, e.g. "docker".
type Runtime struct {
	Binary string
}

// New returns a Runtime for the given binary name.
func New(binary string) Runtime {
	return Runtime{Binary: binary}
}

// Check distinguishes "the binary isn't installed" from "it's installed
// but the daemon isn't responding", since the fix differs and neither
// error is obvious to someone new to the tool.
func (r Runtime) Check() error {
	if _, err := exec.LookPath(r.Binary); err != nil {
		return fmt.Errorf("%s not found in PATH -- install it first", r.Binary)
	}

	var stderr bytes.Buffer
	cmd := exec.Command(r.Binary, "info")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		return fmt.Errorf("%s is installed but not responding -- is the daemon running? (%s)", r.Binary, msg)
	}
	return nil
}

// Spec describes the container a session needs.
type Spec struct {
	Image, Shell, Network, Memory, CPUs string
	IsPty                               bool
	Term, Name                          string
}

// RunArgs turns a Spec into a `run` argument list. -t is only added when
// the client actually requested a PTY; TERM has to be passed with -e
// since neither runtime forwards the host environment. Every container
// gets a fixed name so it can be killed by name later -- exec.Cmd only
// gives us a handle to the local CLI process, not the container itself.
func (r Runtime) RunArgs(s Spec) []string {
	args := []string{"run", "--rm", "-i", "--name", s.Name}
	if s.IsPty {
		args = append(args, "-t", "-e", "TERM="+s.Term)
	}
	args = append(args,
		"--network", s.Network,
		"--memory", s.Memory,
		"--cpus", s.CPUs,
		s.Image, s.Shell,
	)
	return args
}

// Kill stops the named container. Combined with --rm on run, that also
// removes it.
func (r Runtime) Kill(name string) error {
	return exec.Command(r.Binary, "kill", name).Run()
}
