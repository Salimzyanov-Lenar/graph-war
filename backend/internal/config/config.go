// Package config loads server configuration from the environment.
// Invalid configuration must abort startup: a half-configured server
// is worse than no server.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// Config carries everything main needs to start the server.
type Config struct {
	Addr     string
	LogLevel slog.Level
	// DBDSN is optional: without it the server runs without PostgreSQL
	// and /readyz reports the database as disabled.
	DBDSN string
}

// FromEnv builds a Config from environment variables:
//
//	GRAPHWAR_ADDR      listen address, default ":8080"
//	GRAPHWAR_LOG_LEVEL debug|info|warn|error, default "info"
//	GRAPHWAR_DB_DSN    PostgreSQL DSN, default empty (database disabled)
func FromEnv() (Config, error) {
	addr := getenv("GRAPHWAR_ADDR", ":8080")
	if err := checkAddr(addr); err != nil {
		return Config{}, fmt.Errorf("GRAPHWAR_ADDR: %w", err)
	}

	level, err := parseLevel(getenv("GRAPHWAR_LOG_LEVEL", "info"))
	if err != nil {
		return Config{}, fmt.Errorf("GRAPHWAR_LOG_LEVEL: %w", err)
	}

	return Config{Addr: addr, LogLevel: level, DBDSN: os.Getenv("GRAPHWAR_DB_DSN")}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// checkAddr rejects values net.Listen would only fail on at serve time.
func checkAddr(addr string) error {
	if addr == "" {
		return fmt.Errorf("must not be empty")
	}
	if addr[0] != ':' && !strings.Contains(addr, ":") {
		return fmt.Errorf("%q: missing port, expected host:port or :port", addr)
	}
	return nil
}

func parseLevel(s string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToLower(s))); err != nil {
		return 0, fmt.Errorf("%q: unknown level, want debug|info|warn|error", s)
	}
	return level, nil
}
