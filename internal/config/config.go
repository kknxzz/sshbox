// Package config loads sshbox's settings from a TOML file and command
// line flags, with flags taking precedence.
package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	MaxSessions int `toml:"max_sessions"` // 0 means unlimited

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
		MaxSessions: 0,
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
	maxSessions := fs.Int("max-sessions", 0, "reject new connections once this many sessions are active, 0 for unlimited")

	flagVals := make(map[string]*string)
	targets := make(map[string]*string)
	for _, f := range stringFields(&cfg) {
		flagVals[f.flag] = fs.String(f.flag, "", f.usage)
		targets[f.flag] = f.target
	}

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	path := resolveConfigPath(fs, *configPath)
	if data, err := os.ReadFile(path); err == nil {
		if _, err := toml.Decode(string(data), &cfg); err != nil {
			return Config{}, fmt.Errorf("parsing %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}

	// Env vars sit between the file and flags: SSHBOX_IDLE_TIMEOUT
	// overrides idle_timeout from the file, but --idle-timeout still
	// wins over both. Useful for systemd units and containerized
	// deployments of sshbox itself, where env vars are the natural knob.
	for flagName, target := range targets {
		envName := "SSHBOX_" + strings.ToUpper(strings.ReplaceAll(flagName, "-", "_"))
		if v, ok := os.LookupEnv(envName); ok {
			*target = v
		}
	}
	if v, ok := os.LookupEnv("SSHBOX_MAX_SESSIONS"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SSHBOX_MAX_SESSIONS %q: %w", v, err)
		}
		cfg.MaxSessions = n
	}

	// Flags win over whatever the config file (or env) said. fs.Visit
	// only calls back for flags actually passed on the command line, so
	// a flag left at its zero-value default never clobbers a value set
	// some other way. max-sessions needs its own check since 0 is both
	// its default and a meaningful value, so the string-sentinel trick
	// the other fields use doesn't apply.
	fs.Visit(func(fl *flag.Flag) {
		if v, ok := flagVals[fl.Name]; ok {
			*targets[fl.Name] = *v
		}
		if fl.Name == "max-sessions" {
			cfg.MaxSessions = *maxSessions
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

// resolveConfigPath decides which config file to read. An explicit
// --config always wins. Otherwise, "config.toml" in the current
// directory takes priority if it exists; failing that, it falls back to
// the OS-standard config location (~/.config/sshbox/config.toml on
// Linux, the platform equivalent elsewhere). If neither exists, the
// default path is returned as-is -- a missing config file isn't an
// error, defaults apply.
func resolveConfigPath(fs *flag.FlagSet, defaultPath string) string {
	explicit := false
	fs.Visit(func(fl *flag.Flag) {
		if fl.Name == "config" {
			explicit = true
		}
	})
	if explicit {
		return defaultPath
	}
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}
	if dir, err := os.UserConfigDir(); err == nil {
		candidate := filepath.Join(dir, "sshbox", "config.toml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return defaultPath
}
