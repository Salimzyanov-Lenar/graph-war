package storage

import (
	"context"
	"os"
	"testing"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/db"
)

// TestMatchRoundtrip is an integration test: it needs a real PostgreSQL
// (no SQLite stand-in). Run `make test-integration`; without
// GRAPHWAR_TEST_DSN the test is skipped.
func TestMatchRoundtrip(t *testing.T) {
	dsn := os.Getenv("GRAPHWAR_TEST_DSN")
	if dsn == "" {
		t.Skip("GRAPHWAR_TEST_DSN is not set; run make test-integration")
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer pool.Close()
	store := New(pool)

	match, err := store.Queries.CreateMatch(ctx, "private")
	if err != nil {
		t.Fatalf("CreateMatch: %v", err)
	}
	if match.Status != "active" {
		t.Errorf("new match status = %q, want active", match.Status)
	}

	for _, side := range []string{"a", "b"} {
		p, err := store.Queries.CreateParticipant(ctx, db.CreateParticipantParams{
			MatchID:     match.ID,
			Side:        side,
			DisplayName: "guest-" + side,
			GuestID:     "opaque-" + side,
		})
		if err != nil {
			t.Fatalf("CreateParticipant(%s): %v", side, err)
		}
		if p.GuestID != "opaque-"+side {
			t.Errorf("participant guest_id = %q, want opaque-%s", p.GuestID, side)
		}
	}

	got, err := store.Queries.GetMatch(ctx, match.ID)
	if err != nil {
		t.Fatalf("GetMatch: %v", err)
	}
	if got.ID != match.ID {
		t.Errorf("GetMatch id = %v, want %v", got.ID, match.ID)
	}

	parts, err := store.Queries.ListParticipantsByMatch(ctx, match.ID)
	if err != nil {
		t.Fatalf("ListParticipantsByMatch: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("participants = %d, want 2", len(parts))
	}
	if parts[0].Side != "a" || parts[1].Side != "b" {
		t.Errorf("participant sides = [%s %s], want [a b]", parts[0].Side, parts[1].Side)
	}

	// One side per match is unique: a second 'a' must be rejected.
	_, err = store.Queries.CreateParticipant(ctx, db.CreateParticipantParams{
		MatchID:     match.ID,
		Side:        "a",
		DisplayName: "intruder",
		GuestID:     "opaque-x",
	})
	if err == nil {
		t.Fatal("duplicate side accepted, want unique violation")
	}
}
