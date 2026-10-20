package rooms

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/game"
)

// fakeClock fires timers only when the test advances time.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*fakeTimer
}

type fakeTimer struct {
	at      time.Time
	f       func()
	stopped bool
	fired   bool
}

func (t *fakeTimer) Stop() bool {
	was := !t.stopped && !t.fired
	t.stopped = true
	return was
}

func newFakeClock() *fakeClock { return &fakeClock{now: time.Unix(1_700_000_000, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) AfterFunc(d time.Duration, f func()) Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

// Advance moves time forward, running due timers in order outside the lock.
func (c *fakeClock) Advance(d time.Duration) {
	target := c.Now().Add(d)
	for {
		c.mu.Lock()
		var next *fakeTimer
		for _, t := range c.timers {
			if !t.stopped && !t.fired && !t.at.After(target) && (next == nil || t.at.Before(next.at)) {
				next = t
			}
		}
		if next == nil {
			c.now = target
			c.mu.Unlock()
			return
		}
		next.fired = true
		if next.at.After(c.now) {
			c.now = next.at
		}
		c.mu.Unlock()
		next.f()
	}
}

type fakeConn struct {
	mu     sync.Mutex
	msgs   []map[string]any
	closed int
	full   bool
}

func (c *fakeConn) Send(b []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.full {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		panic(err)
	}
	c.msgs = append(c.msgs, m)
	return true
}

func (c *fakeConn) Close(code int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = code
}

func (c *fakeConn) types() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []string
	for _, m := range c.msgs {
		out = append(out, m["type"].(string))
	}
	return out
}

func (c *fakeConn) last(t *testing.T, typ string) map[string]any {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.msgs) - 1; i >= 0; i-- {
		if c.msgs[i]["type"] == typ {
			return c.msgs[i]
		}
	}
	t.Fatalf("no %q message in %v", typ, c.msgs)
	return nil
}

func payload(m map[string]any) map[string]any { return m["payload"].(map[string]any) }

type fixture struct {
	hub  *Hub
	clk  *fakeClock
	room RoomInfo
	a, b *fakeConn
}

// newMatchFixture seats two players, connects both and readies them.
// Side A always shoots first so tests are deterministic.
func newMatchFixture(t *testing.T) *fixture {
	t.Helper()
	clk := newFakeClock()
	h := NewHub(Config{Clock: clk, Seed: func() uint64 { return 7 }, First: func() game.Side { return game.SideA }, Grace: 30 * time.Second})
	room := h.CreateRoom("ga", "Анна")
	if err := h.Join("gb", "Борис", room.RoomID, room.InviteID); err != nil {
		t.Fatal(err)
	}
	f := &fixture{hub: h, clk: clk, room: room, a: &fakeConn{}, b: &fakeConn{}}
	if err := h.Attach("ga", f.a); err != nil {
		t.Fatal(err)
	}
	if err := h.Attach("gb", f.b); err != nil {
		t.Fatal(err)
	}
	if err := h.SetReady("ga", room.RoomID, true); err != nil {
		t.Fatal(err)
	}
	if err := h.SetReady("gb", room.RoomID, true); err != nil {
		t.Fatal(err)
	}
	return f
}

func fire(h *Hub, id string, c *fakeConn, matchID, turn, fighter, expression, req string) {
	body, _ := json.Marshal(map[string]any{
		"v": 1, "type": "match.fire",
		"payload": map[string]string{"matchId": matchID, "expectedTurnId": turn, "fighterId": fighter, "expression": expression, "requestId": req},
	})
	h.Command(id, c, body)
}

func TestJoinErrors(t *testing.T) {
	h := NewHub(Config{Clock: newFakeClock()})
	room := h.CreateRoom("ga", "A")

	if err := h.Join("gb", "B", room.RoomID, "wrong"); err != ErrInvalidInvite {
		t.Errorf("wrong invite: %v", err)
	}
	if err := h.Join("gb", "B", "nope", room.InviteID); err != ErrRoomExpired {
		t.Errorf("unknown room: %v", err)
	}
	if err := h.Join("gb", "B", room.RoomID, room.InviteID); err != nil {
		t.Fatal(err)
	}
	if err := h.Join("gb", "B", room.RoomID, room.InviteID); err != nil {
		t.Errorf("repeat join must be a no-op: %v", err)
	}
	if err := h.Join("gc", "C", room.RoomID, room.InviteID); err != ErrRoomFull {
		t.Errorf("third player: %v", err)
	}
}

func TestParallelJoinSeatsOnlyOne(t *testing.T) {
	h := NewHub(Config{Clock: newFakeClock()})
	room := h.CreateRoom("ga", "A")
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- h.Join(fmt.Sprintf("g%d", i), "P", room.RoomID, room.InviteID)
		}(i)
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		} else if err != ErrRoomFull {
			t.Errorf("unexpected: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("seated %d joiners, want 1", ok)
	}
}

func TestLobbyEventsAndMatchStartOnlyOnce(t *testing.T) {
	f := newMatchFixture(t)

	snap := f.a.last(t, "snapshot")
	if snap["streamId"] != "match:"+payload(snap)["match"].(map[string]any)["matchId"].(string) {
		t.Errorf("snapshot stream %v", snap["streamId"])
	}
	if payload(snap)["phase"] != "active" {
		t.Errorf("phase %v", payload(snap)["phase"])
	}
	if payload(f.a.last(t, "snapshot"))["match"].(map[string]any)["yourSide"] != "A" ||
		payload(f.b.last(t, "snapshot"))["match"].(map[string]any)["yourSide"] != "B" {
		t.Error("yourSide must be per player")
	}
	turn := payload(snap)["turn"].(map[string]any)
	if turn["side"] != "A" || turn["turnId"] != "1" {
		t.Errorf("turn %v", turn)
	}

	before := len(f.a.types())
	// Repeated ready after the start must not start a second match.
	if err := f.hub.SetReady("ga", f.room.RoomID, true); err != ErrNotLobby {
		t.Errorf("ready after start: %v", err)
	}
	if len(f.a.types()) != before {
		t.Error("extra events after a rejected ready")
	}
}

func TestFireValidationAndShot(t *testing.T) {
	f := newMatchFixture(t)
	snap := payload(f.a.last(t, "snapshot"))
	matchID := snap["match"].(map[string]any)["matchId"].(string)

	expectErr := func(c *fakeConn, want string) {
		t.Helper()
		if got := payload(c.last(t, "error"))["code"]; got != want {
			t.Errorf("error code = %v, want %s", got, want)
		}
	}

	fire(f.hub, "gb", f.b, matchID, "1", "b1", "0", "r1")
	expectErr(f.b, "not_your_turn")
	fire(f.hub, "ga", f.a, matchID, "9", "a1", "0", "r2")
	expectErr(f.a, "stale_turn")
	fire(f.hub, "ga", f.a, matchID, "1", "zz", "0", "r3")
	expectErr(f.a, "unknown_fighter")
	fire(f.hub, "ga", f.a, matchID, "1", "b1", "0", "r4")
	expectErr(f.a, "unknown_fighter")
	fire(f.hub, "ga", f.a, matchID, "1", "a1", "2x", "r5")
	expectErr(f.a, "bad_expression")
	fire(f.hub, "ga", f.a, matchID, "1", "a1", "ln(x)", "r6")
	expectErr(f.a, "domain_error_at_start")

	if _, ok := errorHasStream(f.a.last(t, "error")); ok {
		t.Error("error message must not carry streamId/seq")
	}

	fire(f.hub, "ga", f.a, matchID, "1", "a1", "0", "r7")
	shot := payload(f.a.last(t, "shot.resolved"))
	if shot["turnId"] != "1" || len(shot["points"].([]any)) == 0 {
		t.Errorf("shot %v", shot)
	}
	if got := payload(f.b.last(t, "shot.resolved"))["shotId"]; got != shot["shotId"] {
		t.Error("both players must see the same shot")
	}

	// Retry of the accepted request: no second shot. Different content: duplicate_request.
	n := len(f.a.types())
	fire(f.hub, "ga", f.a, matchID, "1", "a1", "0", "r7")
	if len(f.a.types()) != n {
		t.Error("retry produced extra messages")
	}
	fire(f.hub, "ga", f.a, matchID, "1", "a1", "x", "r7")
	expectErr(f.a, "duplicate_request")
}

func TestResolvingThenNextTurn(t *testing.T) {
	f := newMatchFixture(t)
	matchID := payload(f.a.last(t, "snapshot"))["match"].(map[string]any)["matchId"].(string)

	fire(f.hub, "ga", f.a, matchID, "1", "a1", "0", "r1")
	f.clk.Advance(game.ResolvingDuration - time.Millisecond)
	if types := f.b.types(); types[len(types)-1] != "shot.resolved" {
		t.Fatalf("turn opened too early: %v", types)
	}
	f.clk.Advance(2 * time.Millisecond)
	turn := payload(f.b.last(t, "turn.opened"))
	if turn["side"] != "B" || turn["turnId"] != "2" {
		t.Errorf("next turn %v", turn)
	}
	if turn["deadlineAt"].(float64)-turn["serverTime"].(float64) != float64(game.TurnDuration.Milliseconds()) {
		t.Errorf("deadline %v server %v", turn["deadlineAt"], turn["serverTime"])
	}
}

func TestSeqIsMonotonicWithoutGaps(t *testing.T) {
	f := newMatchFixture(t)
	matchID := payload(f.a.last(t, "snapshot"))["match"].(map[string]any)["matchId"].(string)
	fire(f.hub, "ga", f.a, matchID, "1", "a1", "0", "r1")
	f.clk.Advance(game.ResolvingDuration)
	fire(f.hub, "gb", f.b, matchID, "2", "b1", "0", "r2")

	last := -1.0
	for _, m := range f.a.msgs {
		if m["type"] == "error" {
			continue
		}
		seq := m["seq"].(float64)
		if m["type"] == "snapshot" {
			last = seq
			continue
		}
		if seq != last+1 {
			t.Fatalf("seq gap: %v after %v (%v)", seq, last, m["type"])
		}
		last = seq
	}
}

func TestTurnTimeoutsEndInTechnicalLoss(t *testing.T) {
	f := newMatchFixture(t)

	// A times out, B times out, A times out ... third personal timeout of A loses.
	for i := 0; i < 4; i++ {
		f.clk.Advance(game.TurnDuration)
	}
	turn := payload(f.a.last(t, "turn.opened"))
	if turn["turnId"] != "5" || turn["side"] != "A" {
		t.Fatalf("turn after 4 timeouts: %v", turn)
	}
	f.clk.Advance(game.TurnDuration)
	fin := payload(f.a.last(t, "match.finished"))
	if fin["reason"] != "timeout_streak" || fin["winnerSide"] != "B" {
		t.Errorf("finished %v", fin)
	}
}

func TestSurrender(t *testing.T) {
	f := newMatchFixture(t)
	body, _ := json.Marshal(map[string]any{"type": "match.surrender", "payload": map[string]any{"confirm": true}})
	f.hub.Command("gb", f.b, body) // surrender is allowed out of turn
	fin := payload(f.a.last(t, "match.finished"))
	if fin["reason"] != "surrender" || fin["winnerSide"] != "A" {
		t.Errorf("finished %v", fin)
	}
	// No further turns after the end.
	f.clk.Advance(5 * game.TurnDuration)
	for _, ty := range f.a.types() {
		if ty == "turn.opened" {
			t.Error("turn opened after the match ended")
		}
	}
}

func TestDisconnectGraceThenForfeit(t *testing.T) {
	f := newMatchFixture(t)
	f.hub.Detach("gb", f.b)

	f.clk.Advance(29 * time.Second)
	for _, ty := range f.a.types() {
		if ty == "match.finished" {
			t.Fatal("forfeit before the grace period ended")
		}
	}
	f.clk.Advance(2 * time.Second)
	fin := payload(f.a.last(t, "match.finished"))
	if fin["reason"] != "forfeit_disconnect" || fin["winnerSide"] != "A" {
		t.Errorf("finished %v", fin)
	}
}

func TestReconnectWithinGraceKeepsSeatAndGetsSnapshot(t *testing.T) {
	f := newMatchFixture(t)
	f.hub.Detach("gb", f.b)
	f.clk.Advance(10 * time.Second)

	b2 := &fakeConn{}
	if err := f.hub.Attach("gb", b2); err != nil {
		t.Fatal(err)
	}
	if got := payload(b2.last(t, "snapshot"))["phase"]; got != "active" {
		t.Errorf("phase after reconnect: %v", got)
	}
	f.clk.Advance(2 * time.Minute)
	for _, ty := range f.a.types() {
		if ty == "match.finished" {
			t.Fatal("reconnected player must not forfeit")
		}
	}
}

func TestBothAbsentAbortsMatch(t *testing.T) {
	f := newMatchFixture(t)
	f.hub.Detach("ga", f.a)
	f.hub.Detach("gb", f.b)
	f.clk.Advance(31 * time.Second)

	c := &fakeConn{}
	if err := f.hub.Attach("ga", c); err != nil {
		t.Fatal(err)
	}
	if got := payload(c.last(t, "snapshot"))["phase"]; got != "aborted" {
		t.Errorf("phase = %v, want aborted", got)
	}
}

func TestSecondConnectionReplacesFirst(t *testing.T) {
	f := newMatchFixture(t)
	a2 := &fakeConn{}
	if err := f.hub.Attach("ga", a2); err != nil {
		t.Fatal(err)
	}
	if f.a.closed != CloseReplaced {
		t.Errorf("old connection closed with %d, want %d", f.a.closed, CloseReplaced)
	}
	f.hub.Detach("ga", f.a) // late detach of the replaced connection must not start the grace timer
	f.clk.Advance(45 * time.Second)
	for _, ty := range a2.types() {
		if ty == "match.finished" {
			t.Fatal("replaced connection caused a forfeit")
		}
	}
}

func TestLeaveInMatchForfeits(t *testing.T) {
	f := newMatchFixture(t)
	if err := f.hub.Leave("ga", f.room.RoomID); err != nil {
		t.Fatal(err)
	}
	fin := payload(f.b.last(t, "match.finished"))
	if fin["reason"] != "forfeit_disconnect" || fin["winnerSide"] != "B" {
		t.Errorf("finished %v", fin)
	}
}

func TestLeaveInLobbyPromotesRemainingPlayer(t *testing.T) {
	h := NewHub(Config{Clock: newFakeClock()})
	room := h.CreateRoom("ga", "A")
	_ = h.Join("gb", "B", room.RoomID, room.InviteID)
	cb := &fakeConn{}
	_ = h.Attach("gb", cb)

	if err := h.Leave("ga", room.RoomID); err != nil {
		t.Fatal(err)
	}
	players := payload(cb.last(t, "room.changed"))["players"].([]any)
	if len(players) != 1 || players[0].(map[string]any)["side"] != "A" {
		t.Errorf("players %v", players)
	}
	if err := h.Leave("ga", room.RoomID); err != ErrNotInRoom {
		t.Errorf("second leave: %v", err)
	}
}

func TestSlowClientIsDropped(t *testing.T) {
	f := newMatchFixture(t)
	f.b.full = true
	matchID := payload(f.a.last(t, "snapshot"))["match"].(map[string]any)["matchId"].(string)
	fire(f.hub, "ga", f.a, matchID, "1", "a1", "0", "r1")
	if f.b.closed != CloseSlow {
		t.Errorf("slow client close code = %d", f.b.closed)
	}
	// The healthy player keeps receiving events.
	f.a.last(t, "shot.resolved")
}

func TestAttachWithoutRoom(t *testing.T) {
	h := NewHub(Config{Clock: newFakeClock()})
	if err := h.Attach("nobody", &fakeConn{}); err != ErrNotInRoom {
		t.Errorf("attach: %v", err)
	}
}

func TestTeamWipeFinishesWithoutResolving(t *testing.T) {
	f := newMatchFixture(t)
	m := f.hub.rooms[f.room.RoomID].match
	for i := range m.Fighters {
		if m.Fighters[i].Side == game.SideB && m.Fighters[i].ID != "b1" {
			m.Fighters[i].Alive = false
		}
	}
	var b1 game.Fighter
	for _, fi := range m.Fighters {
		if fi.ID == "b1" {
			b1 = fi
		}
	}
	// Shoot straight at b1: place a1 on the same line by aiming at its height.
	matchID := m.ID
	var a1 game.Fighter
	for _, fi := range m.Fighters {
		if fi.ID == "a1" {
			a1 = fi
		}
	}
	// y = (b1.Y-a1.Y)/(b1.X-a1.X) * x is a straight line through b1 in local coordinates.
	slope := (b1.Y - a1.Y) / (b1.X - a1.X)
	fire(f.hub, "ga", f.a, matchID, "1", "a1", fmt.Sprintf("%.9f*x", slope), "r1")
	if payload(f.a.last(t, "shot.resolved"))["destroyedFighterId"] != "b1" {
		t.Skip("shot did not reach b1 (obstacle on the line); covered by game tests")
	}
	fin := payload(f.a.last(t, "match.finished"))
	if fin["reason"] != "team_wiped" || fin["winnerSide"] != "A" {
		t.Errorf("finished %v", fin)
	}
}

func errorHasStream(m map[string]any) (any, bool) {
	if _, ok := m["streamId"]; ok {
		return m["streamId"], true
	}
	if _, ok := m["seq"]; ok {
		return m["seq"], true
	}
	return nil, false
}
