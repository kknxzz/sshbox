// sshbox runs a disposable Docker container per SSH connection.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/gliderlabs/ssh"

	"sshbox/internal/config"
	"sshbox/internal/hostkey"
	"sshbox/internal/runtime"
	"sshbox/internal/session"
)

// version is set at build time via -ldflags "-X main.version=...";
// left as "dev" for plain `go build`/`go run` so local builds still work.
var version = "dev"

func newLogger(w io.Writer, format string) *slog.Logger {
	if format == "json" {
		return slog.New(slog.NewJSONHandler(w, nil))
	}
	return slog.New(slog.NewTextHandler(w, nil))
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--version" {
			fmt.Println("sshbox", version)
			return
		}
	}

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "sshbox:", err)
		os.Exit(1)
	}

	logger := newLogger(os.Stdout, cfg.LogFormat)

	var auditLogger *slog.Logger
	if cfg.AuditLog != "" {
		f, err := os.OpenFile(cfg.AuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Error("failed to open audit log", "path", cfg.AuditLog, "err", err)
			os.Exit(1)
		}
		defer f.Close()
		auditLogger = newLogger(f, cfg.LogFormat)
	}

	if cfg.SessionLog != "" {
		if err := os.MkdirAll(cfg.SessionLog, 0700); err != nil {
			logger.Error("failed to create session log directory", "path", cfg.SessionLog, "err", err)
			os.Exit(1)
		}
	}

	rt := runtime.New(cfg.Runtime)
	if err := rt.Check(); err != nil {
		logger.Error(cfg.Runtime+" not available", "err", err)
		os.Exit(1)
	}

	hostKey, err := hostkey.LoadOrCreate(cfg.HostKeyPath)
	if err != nil {
		logger.Error("failed to load or create host key", "path", cfg.HostKeyPath, "err", err)
		os.Exit(1)
	}

	if cfg.Check {
		logger.Info("check ok", "runtime", cfg.Runtime, "image", cfg.Image, "host_key_path", cfg.HostKeyPath)
		return
	}

	sessions := session.NewHandler(cfg, rt, logger, auditLogger)
	srv := &ssh.Server{
		Addr:    cfg.ListenAddr,
		Handler: sessions.Handle,
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return true
		},
	}
	srv.AddHostKey(hostKey)

	var shuttingDown atomic.Bool
	shutdownDone := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down")
		shuttingDown.Store(true)
		srv.Close()
		sessions.Close()
		close(shutdownDone)
	}()

	logger.Info("listening",
		"addr", cfg.ListenAddr,
		"runtime", cfg.Runtime,
		"image", cfg.Image,
		"network", cfg.Network,
		"memory", cfg.Memory,
		"cpus", cfg.CPUs,
		"idle_timeout", cfg.IdleDuration.String(),
	)
	if err := srv.ListenAndServe(); err != nil && !shuttingDown.Load() {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
	if shuttingDown.Load() {
		<-shutdownDone
	}
}
