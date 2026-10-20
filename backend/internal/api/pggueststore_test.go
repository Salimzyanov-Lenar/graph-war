package api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/storage"
)

// Integration test against a real PostgreSQL, same pattern as B15:
// GRAPHWAR_TEST_DSN must point to a disposable database.
func TestPgGuestStoreRoundTrip(t *testing.T) {
	dsn := os.Getenv("GRAPHWAR_TEST_DSN")
	if dsn == "" {
		t.Skip("GRAPHWAR_TEST_DSN is not set; skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()
	guests := NewPgGuestStore(storage.New(pool).Queries)

	token, hash, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	_ = token

	g, err := guests.CreateGuest(hash, "Интеграционный")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if g.ID == "" || g.DisplayName != "Интеграционный" {
		t.Fatalf("created: %+v", g)
	}

	// Same token hash → same identity (cookie round trip).
	got, err := guests.GuestByTokenHash(hash)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.ID != g.ID {
		t.Fatalf("lookup id = %q, want %q", got.ID, g.ID)
	}
	// Different token → no rows.
	_, other, _ := newToken()
	if _, err := guests.GuestByTokenHash(other); err == nil {
		t.Fatal("foreign token hash resolved to a guest")
	}

	if err := guests.TouchGuest(g.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}

	// Idle cleanup: a freshly touched guest survives the 7-day sweep.
	if _, err := guests.DeleteIdle(time.Now().Add(-7 * 24 * time.Hour)); err != nil {
		t.Fatalf("delete old idle: %v", err)
	}
	if _, err := guests.GuestByTokenHash(hash); err != nil {
		t.Fatalf("fresh guest was deleted: %v", err)
	}
	// A sweep that considers everything idle removes it.
	if _, err := guests.DeleteIdle(time.Now().Add(7 * 24 * time.Hour)); err != nil {
		t.Fatalf("delete all idle: %v", err)
	}
	if _, err := guests.GuestByTokenHash(hash); err == nil {
		t.Fatal("guest survived the full idle sweep")
	}
}
