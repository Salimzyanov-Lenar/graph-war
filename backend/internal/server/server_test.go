package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/storage"
)

func TestHealthz(t *testing.T) {
	ts := httptest.NewServer(newMux(nil))
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestReadyzWithoutDatabase(t *testing.T) {
	ts := httptest.NewServer(newMux(nil))
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestReadyzWithUnreachableDatabase(t *testing.T) {
	// Nothing listens on port 1; pool creation is lazy, ping must fail.
	pool, err := pgxpool.New(context.Background(), "postgres://graphwar:graphwar@127.0.0.1:1/graphwar")
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	ts := httptest.NewServer(newMux(storage.New(pool)))
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 503 {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

func TestListenShutsDownOnContextCancel(t *testing.T) {
	// Port 0 lets the OS pick a free port; we only verify clean shutdown.
	s := New("127.0.0.1:0", slog.Default(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Listen(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Listen() after cancel = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Listen() did not return after context cancel")
	}
}
