// sshbox runs a disposable Docker container per SSH connection.
package main

import (
	"fmt"
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

func main() {
	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "sshbox:", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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

	srv := &ssh.Server{
		Addr:        cfg.ListenAddr,
		IdleTimeout: cfg.IdleDuration,
		Handler:     session.NewHandler(cfg, rt, logger),
		PasswordHandler: func(ctx ssh.Context, password string) bool {
			return true
		},
	}
	srv.AddHostKey(hostKey)

	var shuttingDown atomic.Bool
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down")
		shuttingDown.Store(true)
		srv.Close()
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
}
