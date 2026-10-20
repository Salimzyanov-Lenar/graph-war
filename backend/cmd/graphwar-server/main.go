// Command graphwar-server starts the Graphwar backend HTTP server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/config"
	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/db"
	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/server"
	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/storage"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "graphwar-server:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var store *storage.Store
	if cfg.DBDSN != "" {
		pool, err := db.Open(ctx, cfg.DBDSN)
		if err != nil {
			return err
		}
		defer pool.Close()
		store = storage.New(pool)
		logger.Info("database enabled")
	} else {
		logger.Info("database disabled: GRAPHWAR_DB_DSN is not set")
	}

	srv := server.New(cfg.Addr, logger, store, server.Options{CookieSecure: cfg.CookieSecure, WSOrigins: cfg.WSOrigins})
	logger.Info("starting", "addr", cfg.Addr)
	return srv.Listen(ctx)
}
