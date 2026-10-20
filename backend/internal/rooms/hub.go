// Package rooms holds the in-memory lobby and match driver: rooms with
// two players, the event stream with seq numbers, and the managed turn
// clock. Match rules themselves live in the pure game package.
package rooms

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/game"
)

// Errors of the room operations; the HTTP layer maps them to codes.
var (
	ErrInvalidInvite = errors.New("приглашение не найдено")
	ErrRoomFull      = errors.New("в комнате уже два игрока")
	ErrRoomExpired   = errors.New("комната закрыта")
	ErrNotInRoom     = errors.New("игрок не в этой комнате")
	ErrNotLobby      = errors.New("матч уже начался")
)

// DefaultGrace is how long a disconnected player keeps the seat (contract §8).
const DefaultGrace = 60 * time.Second

// Config of a Hub. Zero values pick production defaults.
type Config struct {
	Clock  Clock
	Logger *slog.Logger
	Grace  time.Duration
	// Seed returns the next map seed; First picks who shoots first.
	Seed  func() uint64
	First func() game.Side
}

// RoomInfo is what the creator learns about a new room.
type RoomInfo struct {
	RoomID   string
	InviteID string
}

type player struct {
	guestID string
	name    string
	side    game.Side
	ready   bool
	left    bool // pressed "leave"; the seat stays in the roster of a played match
}

type room struct {
	id, invite string
	players    []*player
	streamID   string
	seq        int
	match      *game.Match
	seedString string
	conns      map[string]Conn
	grace      map[string]Timer
	seen       map[string]map[string]string // guest → requestId → content hash
	turnTimer  Timer
	resolveTmr Timer
}

// Hub owns all rooms. One mutex guards everything: rooms are tiny and
// every critical section is short and non-blocking (Conn.Send never waits).
type Hub struct {
	mu     sync.Mutex
	cfg    Config
	rooms  map[string]*room
	member map[string]*room // guestID → the room they currently sit in
}

// NewHub builds an empty hub.
func NewHub(cfg Config) *Hub {
	if cfg.Clock == nil {
		cfg.Clock = RealClock{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Grace <= 0 {
		cfg.Grace = DefaultGrace
	}
	if cfg.Seed == nil {
		cfg.Seed = randomUint64
	}
	if cfg.First == nil {
		cfg.First = func() game.Side {
			if randomUint64()&1 == 0 {
				return game.SideA
			}
			return game.SideB
		}
	}
	return &Hub{cfg: cfg, rooms: map[string]*room{}, member: map[string]*room{}}
}

func randomUint64() uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint64(b[:])
}

func randomID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomInvite() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// CreateRoom opens a room with the caller as side A. A guest sits in one
// room at a time: any previous seat is released first.
func (h *Hub) CreateRoom(guestID, name string) RoomInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.leaveLocked(guestID)

	r := &room{
		id:       randomID(8),
		invite:   randomInvite(),
		streamID: "",
		conns:    map[string]Conn{},
		grace:    map[string]Timer{},
		seen:     map[string]map[string]string{},
		players:  []*player{{guestID: guestID, name: name, side: game.SideA}},
	}
	r.streamID = "room:" + r.id
	h.rooms[r.id] = r
	h.member[guestID] = r
	return RoomInfo{RoomID: r.id, InviteID: r.invite}
}

// Join seats the guest as side B. Joining your own room again is a no-op,
// so a retried request is safe.
func (h *Hub) Join(guestID, name, roomID, invite string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.rooms[roomID]
	if !ok {
		return ErrRoomExpired
	}
	if invite != r.invite {
		return ErrInvalidInvite
	}
	if h.member[guestID] == r {
		return nil
	}
	if r.match != nil {
		return ErrRoomExpired
	}
	if len(r.players) >= 2 {
		return ErrRoomFull
	}
	h.leaveLocked(guestID)
	r.players = append(r.players, &player{guestID: guestID, name: name, side: game.SideB})
	h.member[guestID] = r
	h.broadcast(r, "room.changed", roomChangedWire{Players: playersWire(r)})
	return nil
}

// SetReady marks the readiness; when both seats are ready the match starts.
func (h *Hub) SetReady(guestID, roomID string, ready bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, p, err := h.seatLocked(guestID, roomID)
	if err != nil {
		return err
	}
	return h.setReadyLocked(r, p, ready)
}

func (h *Hub) setReadyLocked(r *room, p *player, ready bool) error {
	if r.match != nil {
		return ErrNotLobby
	}
	if p.ready == ready {
		return nil
	}
	p.ready = ready
	h.broadcast(r, "room.changed", roomChangedWire{Players: playersWire(r)})
	if len(r.players) == 2 && r.players[0].ready && r.players[1].ready {
		h.startMatchLocked(r)
	}
	return nil
}

// Leave releases the seat. In a running match it is a technical loss.
func (h *Hub) Leave(guestID, roomID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, _, err := h.seatLocked(guestID, roomID); err != nil {
		return err
	}
	h.leaveLocked(guestID)
	return nil
}

func (h *Hub) seatLocked(guestID, roomID string) (*room, *player, error) {
	r, ok := h.member[guestID]
	if !ok || r.id != roomID {
		return nil, nil, ErrNotInRoom
	}
	return r, findPlayer(r, guestID), nil
}

func findPlayer(r *room, guestID string) *player {
	for _, p := range r.players {
		if p.guestID == guestID {
			return p
		}
	}
	return nil
}

func (h *Hub) leaveLocked(guestID string) {
	r, ok := h.member[guestID]
	if !ok {
		return
	}
	delete(h.member, guestID)
	delete(r.conns, guestID)
	stopTimer(r.grace[guestID])
	delete(r.grace, guestID)

	p := findPlayer(r, guestID)
	if r.match == nil {
		// Lobby: free the seat; the remaining player becomes side A again.
		kept := r.players[:0]
		for _, q := range r.players {
			if q != p {
				kept = append(kept, q)
			}
		}
		r.players = kept
		for _, q := range r.players {
			q.side, q.ready = game.SideA, false
		}
		if len(r.players) == 0 {
			delete(h.rooms, r.id)
			return
		}
		h.broadcast(r, "room.changed", roomChangedWire{Players: playersWire(r)})
		return
	}

	p.left = true
	if r.match.Result() == nil && r.match.Phase != game.PhaseAborted {
		if err := r.match.Forfeit(p.side); err == nil {
			h.finishedLocked(r)
		}
	}
	if allLeft(r) {
		h.dropRoomLocked(r)
	}
}

func allLeft(r *room) bool {
	for _, p := range r.players {
		if !p.left {
			return false
		}
	}
	return true
}

func (h *Hub) dropRoomLocked(r *room) {
	stopTimer(r.turnTimer)
	stopTimer(r.resolveTmr)
	for _, t := range r.grace {
		stopTimer(t)
	}
	delete(h.rooms, r.id)
}

func stopTimer(t Timer) {
	if t != nil {
		t.Stop()
	}
}

func playersWire(r *room) []playerWire {
	out := make([]playerWire, 0, len(r.players))
	for _, p := range r.players {
		out = append(out, playerWire{GuestID: p.guestID, DisplayName: p.name, Side: string(p.side), Ready: p.ready})
	}
	return out
}

// Attach registers the live connection of a seated guest and sends the
// snapshot first. A previous connection of the same guest is replaced.
func (h *Hub) Attach(guestID string, c Conn) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.member[guestID]
	if !ok {
		return ErrNotInRoom
	}
	if old, ok := r.conns[guestID]; ok && old != c {
		old.Close(CloseReplaced, "replaced")
	}
	r.conns[guestID] = c
	stopTimer(r.grace[guestID])
	delete(r.grace, guestID)
	if !c.Send(h.snapshotFor(r, guestID)) {
		h.dropConnLocked(r, guestID, c, CloseSlow)
	}
	return nil
}

// Detach is called when a connection ends. The seat is kept for the
// grace period; then the absent player forfeits (or the match aborts).
func (h *Hub) Detach(guestID string, c Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.member[guestID]
	if !ok || r.conns[guestID] != c {
		return
	}
	h.detachLocked(r, guestID)
}

func (h *Hub) dropConnLocked(r *room, guestID string, c Conn, code int) {
	if r.conns[guestID] != c {
		return
	}
	c.Close(code, "closed")
	h.detachLocked(r, guestID)
}

func (h *Hub) detachLocked(r *room, guestID string) {
	delete(r.conns, guestID)
	m := r.match
	if m == nil || m.Result() != nil || m.Phase == game.PhaseAborted {
		return
	}
	stopTimer(r.grace[guestID])
	r.grace[guestID] = h.cfg.Clock.AfterFunc(h.cfg.Grace, func() { h.onGraceExpired(r, guestID, m) })
}

func (h *Hub) onGraceExpired(r *room, guestID string, m *game.Match) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r.match != m || m.Result() != nil || m.Phase == game.PhaseAborted {
		return
	}
	if _, back := r.conns[guestID]; back {
		return
	}
	delete(r.grace, guestID)
	p := findPlayer(r, guestID)
	opponentOnline := false
	for _, q := range r.players {
		if q != p && !q.left {
			if _, ok := r.conns[q.guestID]; ok {
				opponentOnline = true
			}
		}
	}
	if opponentOnline {
		if err := m.Forfeit(p.side); err == nil {
			h.finishedLocked(r)
		}
		return
	}
	m.Abort()
	stopTimer(r.turnTimer)
	stopTimer(r.resolveTmr)
	for _, q := range r.players {
		if c, ok := r.conns[q.guestID]; ok {
			c.Send(h.snapshotFor(r, q.guestID))
		}
	}
}

// broadcast appends an event to the room stream and fans it out. A client
// whose queue is full is dropped instead of slowing everyone down.
func (h *Hub) broadcast(r *room, typ string, payload any) {
	r.seq++
	msg := encodeEvent(typ, r.streamID, r.seq, payload)
	for id, c := range r.conns {
		if !c.Send(msg) {
			h.dropConnLocked(r, id, c, CloseSlow)
		}
	}
}

func (h *Hub) snapshotFor(r *room, guestID string) []byte {
	snap := snapshotWire{Phase: string(game.PhaseLobby), Players: playersWire(r)}
	if m := r.match; m != nil {
		snap.Phase = string(m.Phase)
		snap.Match = matchBody(r, m, guestID)
		if m.Phase == game.PhaseActive {
			snap.Turn = turnBody(m, h.cfg.Clock.Now())
		}
		if ls := m.LastShot(); ls != nil {
			s := shotFromRecord(m.ID, *ls)
			snap.LastShot = &s
		}
		snap.Result = resultFromMatch(m.Result())
		if m.Phase == game.PhaseAborted {
			snap.Result = nil
		}
	}
	return encodeEvent("snapshot", r.streamID, r.seq, snap)
}

func matchBody(r *room, m *game.Match, guestID string) *matchWire {
	side := game.SideA
	if p := findPlayer(r, guestID); p != nil {
		side = p.side
	}
	obstacles := make([]obstacleWire, 0, len(m.Obstacles()))
	for _, o := range m.Obstacles() {
		obstacles = append(obstacles, obstacleWire{X: o.X, Y: o.Y, R: o.R})
	}
	fighters := make([]fighterWire, 0, len(m.Fighters))
	for _, f := range m.Fighters {
		fighters = append(fighters, fighterWire{ID: f.ID, Side: string(f.Side), X: f.X, Y: f.Y, Alive: f.Alive})
	}
	return &matchWire{
		MatchID:  m.ID,
		YourSide: string(side),
		Map: mapWire{
			Width:            game.FieldWidth,
			Height:           game.FieldHeight,
			Obstacles:        obstacles,
			MapSeed:          r.seedString,
			GeneratorVersion: game.GeneratorVersion,
		},
		Fighters: fighters,
	}
}

func turnBody(m *game.Match, now time.Time) *turnWire {
	side, id := m.CurrentTurn()
	return &turnWire{
		TurnID:     turnIDString(id),
		Side:       string(side),
		DeadlineAt: m.Deadline().UnixMilli(),
		ServerTime: now.UnixMilli(),
	}
}
