// Package server assembles the HTTP server: routes, logging, lifecycle.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/storage"
)

// Server wraps http.Server with the routes and shutdown policy of the app.
type Server struct {
	http   *http.Server
	logger *slog.Logger
	store  *storage.Store // nil = server runs without a database
}

// New wires routes onto an http.Server. Dependencies grow here, not in main.
func New(addr string, logger *slog.Logger, store *storage.Store) *Server {
	return &Server{
		http: &http.Server{
			Addr:              addr,
			Handler:           newMux(store),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
		},
		logger: logger,
		store:  store,
	}
}

func newMux(store *storage.Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /readyz", handleReady(store))
	return mux
}

// Listen blocks until the process is asked to stop.
func (s *Server) Listen(ctx context.Context) error {
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
