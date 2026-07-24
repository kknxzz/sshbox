// Package config loads sshbox's settings from a TOML file and command
// line flags, with flags taking precedence.
package config

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds every tunable in one place.
type Config struct {
	ListenAddr  string `toml:"listen_addr"`
	Image       string `toml:"image"`
	Shell       string `toml:"shell"`
	Network     string `toml:"network"`
	Memory      string `toml:"memory"`
	CPUs        string `toml:"cpus"`
	IdleTimeout string `toml:"idle_timeout"`
	HostKeyPath string `toml:"host_key_path"`

	IdleDuration time.Duration // parsed form of IdleTimeout
}

func defaultConfig() Config {
	return Config{
		ListenAddr:  ":2222",
		Image:       "alpine:latest",
		Shell:       "/bin/sh",
		Network:     "none",
		Memory:      "256m",
		CPUs:        "0.5",
		IdleTimeout: "10m",
		HostKeyPath: "host_key",
	}
}

// Load reads configPath if it exists (missing file is not an error --
// you get the defaults), then applies any flags the user explicitly set
// on top of it.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("sshbox", flag.ContinueOnError)

	configPath := fs.String("config", "config.toml", "path to config file")
	listenAddr := fs.String("listen", "", "address to listen on, e.g. :2222")
	image := fs.String("image", "", "docker image to run per session")
	network := fs.String("network", "", "docker --network mode")
	memory := fs.String("memory", "", "docker --memory limit")
	cpus := fs.String("cpus", "", "docker --cpus limit")
	shell := fs.String("shell", "", "command to run inside the container")
	idleTimeout := fs.String("idle-timeout", "", "disconnect idle sessions after this long, e.g. 10m")
	hostKeyPath := fs.String("host-key", "", "path to persist the ssh host key")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := defaultConfig()

	if data, err := os.ReadFile(*configPath); err == nil {
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", *configPath, err)
		}
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("reading %s: %w", *configPath, err)
	}

	// Flags win over whatever the config file said.
	overrideIfSet(fs, "listen", listenAddr, &cfg.ListenAddr)
	overrideIfSet(fs, "image", image, &cfg.Image)
	overrideIfSet(fs, "network", network, &cfg.Network)
	overrideIfSet(fs, "memory", memory, &cfg.Memory)
	overrideIfSet(fs, "cpus", cpus, &cfg.CPUs)
	overrideIfSet(fs, "shell", shell, &cfg.Shell)
	overrideIfSet(fs, "idle-timeout", idleTimeout, &cfg.IdleTimeout)
	overrideIfSet(fs, "host-key", hostKeyPath, &cfg.HostKeyPath)

	d, err := time.ParseDuration(cfg.IdleTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("invalid idle_timeout %q: %w", cfg.IdleTimeout, err)
	}
	cfg.IdleDuration = d

	return cfg, nil
}

// overrideIfSet copies *val into *dst only if the flag named name was
// actually passed on the command line, so an empty default doesn't clobber
// a value that came from the config file.
func overrideIfSet(fs *flag.FlagSet, name string, val *string, dst *string) {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	if set {
		*dst = *val
	}
}
