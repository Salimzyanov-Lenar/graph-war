// PG-backed implementation of api.GuestStore on top of sqlc queries.
package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/db"
)

// PgGuestStore adapts sqlc queries to the GuestStore interface.
type PgGuestStore struct {
	Q *db.Queries
}

// NewPgGuestStore builds the adapter.
func NewPgGuestStore(q *db.Queries) *PgGuestStore { return &PgGuestStore{Q: q} }

// CreateGuest inserts a new guest row.
func (s *PgGuestStore) CreateGuest(tokenHash []byte, displayName string) (Guest, error) {
	g, err := s.Q.CreateGuest(context.Background(), db.CreateGuestParams{
		TokenHash:   tokenHash,
		DisplayName: displayName,
	})
	if err != nil {
		return Guest{}, fmt.Errorf("create guest: %w", err)
	}
	return Guest{ID: g.ID.String(), DisplayName: g.DisplayName}, nil
}

// GuestByTokenHash finds a guest by SHA-256 of the cookie token.
func (s *PgGuestStore) GuestByTokenHash(tokenHash []byte) (Guest, error) {
	g, err := s.Q.GuestByTokenHash(context.Background(), tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, err
	}
	if err != nil {
		return Guest{}, fmt.Errorf("guest by token: %w", err)
	}
	return Guest{ID: g.ID.String(), DisplayName: g.DisplayName}, nil
}

// TouchGuest refreshes last_seen_at (sliding session renewal).
func (s *PgGuestStore) TouchGuest(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("guest id %q: %w", id, err)
	}
	return s.Q.TouchGuest(context.Background(), pgtype.UUID{Bytes: uid, Valid: true})
}

// DeleteIdle removes guests not seen since `before`; returns the count.
func (s *PgGuestStore) DeleteIdle(before time.Time) (int64, error) {
	n, err := s.Q.DeleteGuestsIdleBefore(context.Background(), pgtype.Timestamptz{
		Time:  before,
		Valid: true,
	})
	if err != nil {
		return 0, fmt.Errorf("delete idle guests: %w", err)
	}
	return n, nil
}
