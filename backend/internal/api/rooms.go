package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/rooms"
)

// Authenticator resolves the session cookie to a guest.
type Authenticator struct {
	Store GuestStore
}

// Guest returns the guest of the request, or false for a missing/unknown cookie.
func (a *Authenticator) Guest(r *http.Request) (Guest, bool) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return Guest{}, false
	}
	sum := hashToken(c.Value)
	if sum == nil {
		return Guest{}, false
	}
	g, err := a.Store.GuestByTokenHash(sum)
	if err != nil {
		return Guest{}, false
	}
	_ = a.Store.TouchGuest(g.ID)
	return g, true
}

// RoomsHandler serves the REST part of rooms (contract §8).
type RoomsHandler struct {
	Auth    *Authenticator
	Hub     *rooms.Hub
	Limiter *RateLimiter
}

// Register mounts the routes on the mux.
func (h *RoomsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/rooms", h.create)
	mux.HandleFunc("POST /api/rooms/{id}/join", h.join)
	mux.HandleFunc("POST /api/rooms/{id}/ready", h.ready)
	mux.HandleFunc("DELETE /api/rooms/{id}", h.leave)
}

func (h *RoomsHandler) authorize(w http.ResponseWriter, r *http.Request) (Guest, bool) {
	g, ok := h.Auth.Guest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "сначала войдите: POST /api/guest")
		return Guest{}, false
	}
	if h.Limiter != nil {
		if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && !h.Limiter.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "слишком часто, попробуйте позже")
			return Guest{}, false
		}
	}
	return g, true
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "некорректный JSON")
		return false
	}
	return true
}

type roomResponse struct {
	RoomID   string `json:"roomId"`
	InviteID string `json:"inviteId"`
}

func (h *RoomsHandler) create(w http.ResponseWriter, r *http.Request) {
	g, ok := h.authorize(w, r)
	if !ok {
		return
	}
	info := h.Hub.CreateRoom(g.ID, g.DisplayName)
	writeOK(w, http.StatusCreated, roomResponse{RoomID: info.RoomID, InviteID: info.InviteID})
}

func (h *RoomsHandler) join(w http.ResponseWriter, r *http.Request) {
	g, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var body struct {
		InviteID string `json:"inviteId"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.Hub.Join(g.ID, g.DisplayName, r.PathValue("id"), body.InviteID); err != nil {
		writeRoomError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RoomsHandler) ready(w http.ResponseWriter, r *http.Request) {
	g, ok := h.authorize(w, r)
	if !ok {
		return
	}
	var body struct {
		Ready bool `json:"ready"`
	}
	if !decodeBody(w, r, &body) {
		return
	}
	if err := h.Hub.SetReady(g.ID, r.PathValue("id"), body.Ready); err != nil {
		writeRoomError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RoomsHandler) leave(w http.ResponseWriter, r *http.Request) {
	g, ok := h.authorize(w, r)
	if !ok {
		return
	}
	if err := h.Hub.Leave(g.ID, r.PathValue("id")); err != nil {
		writeRoomError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeRoomError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rooms.ErrInvalidInvite):
		writeError(w, http.StatusNotFound, "invalid_invite", "приглашение не найдено")
	case errors.Is(err, rooms.ErrRoomFull):
		writeError(w, http.StatusConflict, "room_full", "в комнате уже два игрока")
	case errors.Is(err, rooms.ErrRoomExpired), errors.Is(err, rooms.ErrNotInRoom):
		writeError(w, http.StatusNotFound, "room_expired", "комната закрыта или вы в ней не сидите")
	case errors.Is(err, rooms.ErrNotLobby):
		writeError(w, http.StatusConflict, "not_in_lobby", "матч уже начался")
	default:
		writeError(w, http.StatusInternalServerError, "server_error", "внутренняя ошибка")
	}
}
