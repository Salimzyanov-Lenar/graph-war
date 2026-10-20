package config

import (
	"log/slog"
	"testing"
)

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("GRAPHWAR_ADDR", "")
	t.Setenv("GRAPHWAR_LOG_LEVEL", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestFromEnvInvalidAddr(t *testing.T) {
	t.Setenv("GRAPHWAR_ADDR", "no-port-here")

	if _, err := FromEnv(); err == nil {
		t.Fatal("FromEnv() with portless addr = nil error, want error")
	}
}

func TestFromEnvInvalidLogLevel(t *testing.T) {
	t.Setenv("GRAPHWAR_LOG_LEVEL", "loud")

	if _, err := FromEnv(); err == nil {
		t.Fatal("FromEnv() with unknown level = nil error, want error")
	}
}
