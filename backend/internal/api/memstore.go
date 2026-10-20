package api

import (
	"errors"
	"strconv"
	"sync"
)

// MemGuestStore keeps guests in memory: the server works without
// PostgreSQL (LAN play); identities vanish on restart.
type MemGuestStore struct {
	mu     sync.Mutex
	nextID int
	guests map[string]Guest // string(tokenHash) → guest
}

func NewMemGuestStore() *MemGuestStore {
	return &MemGuestStore{guests: map[string]Guest{}}
}

func (s *MemGuestStore) CreateGuest(tokenHash []byte, displayName string) (Guest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	g := Guest{ID: "g" + strconv.Itoa(s.nextID) + "-" + randomSuffix(), DisplayName: displayName}
	s.guests[string(tokenHash)] = g
	return g, nil
}

func (s *MemGuestStore) GuestByTokenHash(tokenHash []byte) (Guest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.guests[string(tokenHash)]
	if !ok {
		return Guest{}, errors.New("guest not found")
	}
	return g, nil
}

func (s *MemGuestStore) TouchGuest(string) error { return nil }

func randomSuffix() string {
	_, hash, err := newToken()
	if err != nil {
		return "x"
	}
	const hexdigits = "0123456789abcdef"
	out := make([]byte, 8)
	for i := range out {
		out[i] = hexdigits[hash[i]&0x0f]
	}
	return string(out)
}
