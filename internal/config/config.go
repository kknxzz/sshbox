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
	Runtime     string `toml:"runtime"`
	LogFormat   string `toml:"log_format"`

	IdleDuration time.Duration `toml:"-"` // parsed form of IdleTimeout
	Check        bool          `toml:"-"` // --check: validate and exit, not a persisted setting
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
		Runtime:     "docker",
		LogFormat:   "text",
	}
}

// stringFields lists every string setting once: its flag name, help text,
// and a pointer to where it lives in Config. Adding a new string setting
// means adding one line here -- nothing else needs touching.
func stringFields(cfg *Config) []struct {
	flag, usage string
	target      *string
} {
	return []struct {
		flag, usage string
		target      *string
	}{
		{"listen", "address to listen on, e.g. :2222", &cfg.ListenAddr},
		{"image", "docker image to run per session", &cfg.Image},
		{"shell", "command to run inside the container", &cfg.Shell},
		{"network", "docker --network mode", &cfg.Network},
		{"memory", "docker --memory limit", &cfg.Memory},
		{"cpus", "docker --cpus limit", &cfg.CPUs},
		{"idle-timeout", "disconnect idle sessions after this long, e.g. 10m", &cfg.IdleTimeout},
		{"host-key", "path to persist the ssh host key", &cfg.HostKeyPath},
		{"runtime", "container runtime binary, e.g. docker or podman", &cfg.Runtime},
		{"log-format", "log output format: text or json", &cfg.LogFormat},
	}
}

// Load reads configPath if it exists (missing file is not an error --
// you get the defaults), then applies any flags the user explicitly set
// on top of it.
func Load(args []string) (Config, error) {
	cfg := defaultConfig()
	fs := flag.NewFlagSet("sshbox", flag.ContinueOnError)

	configPath := fs.String("config", "config.toml", "path to config file")
	check := fs.Bool("check", false, "validate config and runtime availability, then exit")

	flagVals := make(map[string]*string)
	targets := make(map[string]*string)
	for _, f := range stringFields(&cfg) {
		flagVals[f.flag] = fs.String(f.flag, "", f.usage)
		targets[f.flag] = f.target
	}

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if data, err := os.ReadFile(*configPath); err == nil {
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", *configPath, err)
		}
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("reading %s: %w", *configPath, err)
	}

	// Flags win over whatever the config file said. fs.Visit only calls
	// back for flags actually passed on the command line, so a flag left
	// at its zero-value default never clobbers a value from the file.
	fs.Visit(func(fl *flag.Flag) {
		if v, ok := flagVals[fl.Name]; ok {
			*targets[fl.Name] = *v
		}
	})

	cfg.Check = *check

	d, err := time.ParseDuration(cfg.IdleTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("invalid idle_timeout %q: %w", cfg.IdleTimeout, err)
	}
	cfg.IdleDuration = d

	return cfg, nil
}
