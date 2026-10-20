package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeStore records what the handler did; safe for parallel requests.
type fakeStore struct {
	mu      sync.Mutex
	guests  map[string]Guest // tokenHash → guest
	nextID  int
	created int
	touched map[string]int
}

func newFakeStore() *fakeStore {
	return &fakeStore{guests: map[string]Guest{}, touched: map[string]int{}}
}

func (s *fakeStore) CreateGuest(tokenHash []byte, displayName string) (Guest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created++
	s.nextID++
	g := Guest{ID: string(rune('a' + s.nextID - 1)) + "-id", DisplayName: displayName}
	s.guests[string(tokenHash)] = g
	return g, nil
}

func (s *fakeStore) GuestByTokenHash(tokenHash []byte) (Guest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.guests[string(tokenHash)]
	if !ok {
		return Guest{}, errors.New("no rows")
	}
	return g, nil
}

func (s *fakeStore) TouchGuest(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.touched[id]++
	return nil
}

func newTestHandler() (*GuestHandler, *fakeStore) {
	st := newFakeStore()
	return &GuestHandler{Store: st, Secure: false, Limiter: NewRateLimiter(1000, 1e9, 100)}, st
}

func postGuest(t *testing.T, h *GuestHandler, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/guest", strings.NewReader(body))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.RemoteAddr = "203.0.113.7:12345"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decodeGuest(t *testing.T, w *httptest.ResponseRecorder) guestResponse {
	t.Helper()
	var resp guestResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return resp
}

func TestGuestCreateAndRepeat(t *testing.T) {
	h, st := newTestHandler()

	w := postGuest(t, h, `{"displayName":"Ленар"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create: %d %s", w.Code, w.Body.String())
	}
	first := decodeGuest(t, w)
	if first.GuestID == "" || first.DisplayName != "Ленар" {
		t.Fatalf("first response: %+v", first)
	}

	// The cookie must be HttpOnly, SameSite=Lax, Path=/.
	cookies := w.Result().Cookies()
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookie {
			session = c
		}
	}
	if session == nil {
		t.Fatal("no session cookie set")
	}
	if !session.HttpOnly || session.SameSite != http.SameSiteLaxMode || session.Path != "/" {
		t.Fatalf("cookie attributes: %+v", session)
	}

	// Repeat WITH the cookie: same identity, 200, regardless of the
	// name in the body (the cookie decides, not the payload).
	w = postGuest(t, h, `{"displayName":"Другое имя"}`, []*http.Cookie{session})
	if w.Code != http.StatusOK {
		t.Fatalf("repeat: %d %s", w.Code, w.Body.String())
	}
	again := decodeGuest(t, w)
	if again.GuestID != first.GuestID {
		t.Fatalf("repeat identity = %q, want %q", again.GuestID, first.GuestID)
	}
	if again.DisplayName != "Ленар" {
		t.Fatalf("display name changed: %q", again.DisplayName)
	}
	if st.created != 1 {
		t.Fatalf("created = %d, want 1", st.created)
	}
	if st.touched[first.GuestID] != 1 {
		t.Fatalf("TouchGuest calls = %d, want 1", st.touched[first.GuestID])
	}
}

func TestGuestWithoutCookieIsAnotherGuest(t *testing.T) {
	h, _ := newTestHandler()
	w1 := postGuest(t, h, `{"displayName":"A"}`, nil)
	w2 := postGuest(t, h, `{"displayName":"A"}`, nil)
	if decodeGuest(t, w1).GuestID == decodeGuest(t, w2).GuestID {
		t.Fatal("two requests without cookies got one identity")
	}
}

func TestGuestForgedCookieGetsNewIdentity(t *testing.T) {
	h, st := newTestHandler()
	forged := &http.Cookie{Name: SessionCookie, Value: "not-a-real-token"}
	w := postGuest(t, h, `{"displayName":"X"}`, []*http.Cookie{forged})
	if w.Code != http.StatusCreated {
		t.Fatalf("forged cookie: %d, want 201 (new identity)", w.Code)
	}
	if st.created != 1 {
		t.Fatalf("created = %d, want 1", st.created)
	}
}

func TestGuestBodyGuestIdIgnored(t *testing.T) {
	h, _ := newTestHandler()
	// A hostile client sends someone else's guestId in the body:
	// it must not grant that identity — the response carries a fresh id.
	w := postGuest(t, h, `{"displayName":"Eve","guestId":"stolen-id"}`, nil)
	resp := decodeGuest(t, w)
	if resp.GuestID == "stolen-id" {
		t.Fatal("request body guestId was honored")
	}
}

func TestGuestNameValidation(t *testing.T) {
	h, _ := newTestHandler()
	cases := map[string]struct {
		body string
		want int
		code string
	}{
		"empty":      {`{"displayName":"   "}`, 400, "invalid_name"},
		"too long":   {`{"displayName":"` + strings.Repeat("ж", 25) + `"}`, 400, "name_too_long"},
		"no field":   {`{}`, 400, "invalid_name"},
		"bad json":   {`{not json`, 400, "invalid_name"},
		"24 ok":      {`{"displayName":"` + strings.Repeat("ж", 24) + `"}`, 201, ""},
		"trim ok":    {`{"displayName":"  Ленар  "}`, 201, ""},
	}
	for name, tc := range cases {
		w := postGuest(t, h, tc.body, nil)
		if w.Code != tc.want {
			t.Errorf("%s: status %d, want %d (%s)", name, w.Code, tc.want, w.Body.String())
			continue
		}
		if tc.code == "" {
			continue
		}
		var resp errorResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil || resp.Code != tc.code {
			t.Errorf("%s: error body %+v, want code %q", name, resp, tc.code)
		}
	}
}

func TestGuestCookieSecureFlag(t *testing.T) {
	st := newFakeStore()
	h := &GuestHandler{Store: st, Secure: true, Limiter: NewRateLimiter(1000, 1e9, 100)}
	w := postGuest(t, h, `{"displayName":"A"}`, nil)
	var session *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookie {
			session = c
		}
	}
	if session == nil || !session.Secure {
		t.Fatal("Secure=false behind HTTPS configuration")
	}
}

func TestGuestRateLimited(t *testing.T) {
	st := newFakeStore()
	h := &GuestHandler{Store: st, Limiter: NewRateLimiter(2, 1e9, 100)}
	for i := 0; i < 2; i++ {
		if w := postGuest(t, h, `{"displayName":"A"}`, nil); w.Code != http.StatusCreated {
			t.Fatalf("request %d: %d, want 201", i, w.Code)
		}
	}
	w := postGuest(t, h, `{"displayName":"A"}`, nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("third request: %d, want 429", w.Code)
	}
	var resp errorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != "rate_limited" {
		t.Fatalf("code = %q, want rate_limited", resp.Code)
	}
}

func TestRateLimiterBoundedMemory(t *testing.T) {
	l := NewRateLimiter(5, 1e9, 100)
	for i := 0; i < 10_000; i++ {
		l.Allow(string(rune(i%3000))) // many distinct keys, bounded map
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hits) > 100 {
		t.Fatalf("limiter map grew to %d keys", len(l.hits))
	}
}

func TestTokenHashRoundTrip(t *testing.T) {
	token, hash, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(hashToken(token), hash) {
		t.Fatal("hashToken(newToken()) does not round-trip")
	}
	if bytes.Contains(hash, []byte(token)) {
		t.Fatal("hash contains the raw token")
	}
	// The token must not be derivable from the hash (sanity: hash of
	// another token differs).
	_, hash2, _ := newToken()
	if bytes.Equal(hash, hash2) {
		t.Fatal("two tokens share a hash")
	}
}
