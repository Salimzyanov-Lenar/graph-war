// Package api holds the HTTP handlers of the game: guest sessions now,
// rooms and the WebSocket later (B09–B12).
package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode"
)

// SessionCookie is the name of the guest identity cookie.
const SessionCookie = "graphwar_session"

// sessionMaxAge: a guest identity lives 30 days, refreshed on use.
const sessionMaxAge = 30 * 24 * time.Hour

// Guest is the identity the handlers see (DB row minus secrets).
type Guest struct {
	ID          string
	DisplayName string
}

// GuestStore is the persistence the guest handlers need.
type GuestStore interface {
	CreateGuest(tokenHash []byte, displayName string) (Guest, error)
	GuestByTokenHash(tokenHash []byte) (Guest, error)
	TouchGuest(id string) error
}

// GuestHandler serves POST /api/guest.
type GuestHandler struct {
	Store   GuestStore
	Secure  bool // Secure cookie attribute (true behind HTTPS)
	Limiter *RateLimiter
}

type guestRequest struct {
	DisplayName string `json:"displayName"`
}

type guestResponse struct {
	GuestID     string `json:"guestId"`
	DisplayName string `json:"displayName"`
	// Token is returned only in tab mode (header X-Session-Mode: tab): the
	// client keeps it per browser tab and sends it as a bearer token.
	Token string `json:"token,omitempty"`
}

// SessionModeHeader switches POST /api/guest to a per-tab identity: no cookie
// is set, the token comes back in the body.
const SessionModeHeader = "X-Session-Mode"

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, code int, c, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorResponse{Code: c, Message: msg})
}

func writeOK(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(body)
}

// ServeHTTP implements POST /api/guest (contract §8):
//   - request without a valid cookie → new guest, new cookie, 201;
//   - request with a valid cookie → the same guest, 200, cookie refreshed;
//   - forged JSON guestId never grants an identity (it is not read).
func (h *GuestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "используйте POST")
		return
	}

	var req guestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_name", "некорректный JSON")
		return
	}
	name, nerr := validateDisplayName(req.DisplayName)
	if nerr != nil {
		writeError(w, http.StatusBadRequest, nerr.code, nerr.msg)
		return
	}

	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && !h.Limiter.Allow(ip) {
		writeError(w, http.StatusTooManyRequests, "rate_limited", "слишком много гостей с одного адреса, попробуйте позже")
		return
	}

	if r.Header.Get(SessionModeHeader) == "tab" {
		h.serveTab(w, r, name)
		return
	}

	// Existing identity: the cookie decides, never the request body.
	if c, err := r.Cookie(SessionCookie); err == nil {
		if g, ok := h.lookup(w, r, c.Value); ok {
			writeOK(w, http.StatusOK, guestResponse{GuestID: g.ID, DisplayName: g.DisplayName})
			return
		}
		// Invalid/expired cookie: fall through and issue a new identity.
	}

	token, tokenHash, err := newToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "не удалось создать сессию")
		return
	}
	g, err := h.Store.CreateGuest(tokenHash, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "не удалось создать гостя")
		return
	}
	setSessionCookie(w, token, h.Secure)
	writeOK(w, http.StatusCreated, guestResponse{GuestID: g.ID, DisplayName: g.DisplayName})
}

func (h *GuestHandler) serveTab(w http.ResponseWriter, r *http.Request, name string) {
	if token := bearerToken(r); token != "" {
		if sum := hashToken(token); sum != nil {
			if g, err := h.Store.GuestByTokenHash(sum); err == nil {
				_ = h.Store.TouchGuest(g.ID)
				writeOK(w, http.StatusOK, guestResponse{GuestID: g.ID, DisplayName: g.DisplayName, Token: token})
				return
			}
		}
	}
	token, tokenHash, err := newToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "не удалось создать сессию")
		return
	}
	g, err := h.Store.CreateGuest(tokenHash, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", "не удалось создать гостя")
		return
	}
	writeOK(w, http.StatusCreated, guestResponse{GuestID: g.ID, DisplayName: g.DisplayName, Token: token})
}

// lookup finds a guest by cookie token (hash → DB) and touches
// last_seen for the sliding renewal of long matches.
func (h *GuestHandler) lookup(w http.ResponseWriter, r *http.Request, token string) (Guest, bool) {
	sum := hashToken(token)
	if sum == nil {
		return Guest{}, false
	}
	g, err := h.Store.GuestByTokenHash(sum)
	if err != nil {
		return Guest{}, false
	}
	_ = h.Store.TouchGuest(g.ID)
	setSessionCookie(w, token, h.Secure) // refresh the sliding window
	return g, true
}

// nameError carries the REST error code (contract §8).
type nameError struct {
	code string
	msg  string
}

// validateDisplayName enforces the contract limit: 1..24 characters
// after trimming; control characters are rejected so a hostile name
// can never smuggle markup or escapes.
func validateDisplayName(s string) (string, *nameError) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", &nameError{"invalid_name", "имя не может быть пустым"}
	}
	if len([]rune(s)) > 24 {
		return "", &nameError{"name_too_long", "имя длиннее 24 символов"}
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", &nameError{"invalid_name", "имя содержит недопустимые символы"}
		}
	}
	return s, nil
}

// newToken returns (token, sha256(token)). The token goes to the cookie
// only; only the hash is stored, so a DB leak does not hijack sessions.
func newToken() (string, []byte, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("rand: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256(buf)
	return token, sum[:], nil
}

func hashToken(token string) []byte {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil
	}
	sum := sha256.Sum256(raw)
	return sum[:]
}

func setSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
}
