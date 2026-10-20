// Package server assembles the HTTP server: routes, logging, lifecycle.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/api"
	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/rooms"
	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/storage"
)

// Server wraps http.Server with the routes and shutdown policy of the app.
type Server struct {
	http   *http.Server
	logger *slog.Logger
	store  *storage.Store // nil = server runs without a database
}

// Options are the non-database settings of the HTTP layer.
type Options struct {
	CookieSecure bool
	WSOrigins    []string
	Clock        rooms.Clock // nil = wall clock
}

// New wires routes onto an http.Server. Dependencies grow here, not in main.
func New(addr string, logger *slog.Logger, store *storage.Store, opts Options) *Server {
	s := &Server{
		http: &http.Server{
			Addr:              addr,
			Handler:           newMux(store, opts, logger),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
		},
		logger: logger,
		store:  store,
	}
	return s
}

func newMux(store *storage.Store, opts Options, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /readyz", handleReady(store))

	// Guests live in PostgreSQL when it is configured, otherwise in memory.
	var guestStore api.GuestStore = api.NewMemGuestStore()
	if store != nil {
		guestStore = api.NewPgGuestStore(store.Queries)
	}
	mux.Handle("POST /api/guest", &api.GuestHandler{
		Store:   guestStore,
		Secure:  opts.CookieSecure,
		Limiter: api.NewRateLimiter(20, time.Minute, 10_000),
	})

	hub := rooms.NewHub(rooms.Config{Clock: opts.Clock, Logger: logger})
	auth := &api.Authenticator{Store: guestStore}
	(&api.RoomsHandler{Auth: auth, Hub: hub, Limiter: api.NewRateLimiter(120, time.Minute, 10_000)}).Register(mux)
	mux.Handle("GET /ws", &api.WSHandler{Auth: auth, Hub: hub, AllowedOrigins: opts.WSOrigins})
	return mux
}

// Listen blocks until the process is asked to stop.
func (s *Server) Listen(ctx context.Context) error {
	// Guest janitor: drop identities idle for over 7 days (bounded
	// memory). Guests in live matches are touched on every request,
	// so they are never idle.
	janitorStop := make(chan struct{})
	go s.runGuestJanitor(ctx, janitorStop)
	defer close(janitorStop)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.http.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			return err
		}
		// ListenAndServe exits with ErrServerClosed once Shutdown is called.
		if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// runGuestJanitor deletes guests idle for over 7 days once an hour
// until the server stops. Never deletes a guest seen recently.
func (s *Server) runGuestJanitor(ctx context.Context, stop <-chan struct{}) {
	if s.store == nil {
		return
	}
	guests := api.NewPgGuestStore(s.store.Queries)
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			n, err := guests.DeleteIdle(time.Now().Add(-7 * 24 * time.Hour))
			if err != nil {
				s.logger.Warn("guest janitor failed", "err", err)
				continue
			}
			if n > 0 {
				s.logger.Info("guest janitor removed idle guests", "count", n)
			}
		}
	}
}

// handleReady reports whether the process can serve now. Liveness (healthz)
// says "the process runs"; readiness says "its dependencies work".
func handleReady(store *storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "disabled"})
			return
		}
		if err := store.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, code int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}
