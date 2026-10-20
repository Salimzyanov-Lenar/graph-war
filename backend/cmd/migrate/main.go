// Command migrate applies or inspects database migrations.
// Usage: migrate -up | -down | -status, DSN from GRAPHWAR_DB_DSN.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // register pgx driver for database/sql
	"github.com/pressly/goose/v3"

	"github.com/Salimzyanov-Lenar/graph-war/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	down := flag.Bool("down", false, "roll back one migration")
	status := flag.Bool("status", false, "show migration status")
	flag.Parse()

	dsn := os.Getenv("GRAPHWAR_DB_DSN")
	if dsn == "" {
		return fmt.Errorf("GRAPHWAR_DB_DSN is not set")
	}

	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	goose.SetBaseFS(migrations.FS)

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close()

	ctx := context.Background()
	switch {
	case *status:
		return goose.Status(sqlDB, ".")
	case *down:
		return goose.DownContext(ctx, sqlDB, ".")
	default:
		return goose.UpContext(ctx, sqlDB, ".")
	}
}
