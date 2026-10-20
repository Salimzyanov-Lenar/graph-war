// Package storage wraps generated queries with the small API the server needs.
package storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/db"
)

// Store owns the pool and the typed queries on top of it.
type Store struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool, Queries: db.New(pool)}
}

// Ping proves the database answers right now; used by /readyz.
func (s *Store) Ping(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.Pool.Ping(pingCtx)
}

func (s *Store) Close() {
	s.Pool.Close()
}
