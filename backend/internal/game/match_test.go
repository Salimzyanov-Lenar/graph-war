package game

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/expr"
)

// Empty field, six fighters on the standard spawn layout.
func newTestMatch(t *testing.T, first Side) (*Match, time.Time) {
	t.Helper()
	gm := GameMap{Fighters: spawnFighters()}
	start := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	mm, err := NewMatch("match-test", gm, first, start)
	if err != nil {
		t.Fatal(err)
	}
	return mm, start
}

// fire is a helper: parse+fire from the given side at the given time.
func fire(t *testing.T, mm *Match, now time.Time, side Side, fighterID string, turnID int, formula string) (ShotRecord, error) {
	t.Helper()
	ast, err := expr.Parse(formula)
	if err != nil {
		t.Fatalf("parse %q: %v", formula, err)
	}
	return mm.Fire(context.Background(), now, side, fighterID, turnID, ast)
}

func alive(mm *Match, id string) bool {
	for _, f := range mm.Fighters {
		if f.ID == id {
			return f.Alive
		}
	}
	return false
}

func TestMatchFullScenarioTeamWiped(t *testing.T) {
	mm, start := newTestMatch(t, SideA)
	adv := func(n int) { mm.AdvancePhase(start.Add(time.Duration(n) * ResolvingDuration)) }

	// Turn 1: a1 fires horizontally (Y=12) → destroys b3 (92,12).
	if _, err := fire(t, mm, start, SideA, "a1", 1, "0"); err != nil {
		t.Fatal(err)
	}
	if alive(mm, "b3") {
		t.Fatal("b3 should be destroyed")
	}
	if mm.Phase != PhaseResolving {
		t.Fatalf("phase = %q, want resolving", mm.Phase)
	}
	// Premature advance is ignored.
	if mm.AdvancePhase(start.Add(ResolvingDuration - time.Millisecond)) {
		t.Fatal("phase advanced before resolving ended")
	}
	adv(1) // turn 2: B

	// Turn 2: B dives out of the field — a legal miss.
	if _, err := fire(t, mm, start, SideB, "b2", 2, "-x*2"); err != nil {
		t.Fatal(err)
	}
	adv(2) // turn 3: A

	// Turn 3: a2 fires horizontally (Y=30) → destroys b2 (88,30).
	if _, err := fire(t, mm, start, SideA, "a2", 3, "0"); err != nil {
		t.Fatal(err)
	}
	adv(3) // turn 4: B

	// Turn 4: B misses again.
	if _, err := fire(t, mm, start, SideB, "b1", 4, "-x*2"); err != nil {
		t.Fatal(err)
	}
	adv(4) // turn 5: A

	// Turn 5: a3 fires horizontally (Y=48) → destroys b1 (92,48)
	// → team wiped → finished, winner A.
	if _, err := fire(t, mm, start, SideA, "a3", 5, "0"); err != nil {
		t.Fatal(err)
	}
	if mm.Phase != PhaseFinished {
		t.Fatalf("phase = %q, want finished", mm.Phase)
	}
	res := mm.Result()
	if res == nil || res.Winner != SideA || res.Reason != EndTeamWiped {
		t.Fatalf("result = %+v, want A/team_wiped", res)
	}
}

func TestMatchFriendlyFireReducesOwnTeam(t *testing.T) {
	mm, start := newTestMatch(t, SideA)
	// a2 sits at (12,30); the horizontal shot from a1 (8,12) does not
	// reach it, so aim through: formula "0" from a2 hits a1? Use a
	// fighter whose line crosses a teammate: a1 → (12,30) needs a slope:
	// from (8,12) to (12,30): slope (30-12)/(12-8) = 4.5 → f = 4.5*x.
	if _, err := fire(t, mm, start, SideA, "a1", 1, "4.5*x"); err != nil {
		t.Fatal(err)
	}
	if alive(mm, "a2") {
		t.Fatal("a2 should be destroyed by friendly fire")
	}
	if mm.Result() != nil {
		t.Fatal("one friendly kill must not finish the match")
	}
}

func TestMatchRejectionsLeaveStateUntouched(t *testing.T) {
	mm, start := newTestMatch(t, SideA)

	if _, err := fire(t, mm, start, SideB, "b1", 1, "0"); !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("other side: %v, want ErrNotYourTurn", err)
	}
	if _, err := fire(t, mm, start, SideA, "zz", 1, "0"); !errors.Is(err, ErrUnknownFighter) {
		t.Errorf("unknown fighter: %v, want ErrUnknownFighter", err)
	}
	if _, err := fire(t, mm, start, SideA, "b1", 1, "0"); !errors.Is(err, ErrWrongSide) {
		t.Errorf("enemy fighter: %v, want ErrWrongSide", err)
	}
	if _, err := fire(t, mm, start, SideA, "a1", 2, "0"); !errors.Is(err, ErrStaleTurn) {
		t.Errorf("stale turn: %v, want ErrStaleTurn", err)
	}

	// Accepted shot: a1 destroys b3 → resolving → advance → B's turn.
	if _, err := fire(t, mm, start, SideA, "a1", 1, "0"); err != nil {
		t.Fatal(err)
	}
	mm.AdvancePhase(start.Add(ResolvingDuration))
	if _, err := fire(t, mm, start, SideB, "b3", 2, "0"); !errors.Is(err, ErrDeadFighter) {
		t.Errorf("dead fighter: %v, want ErrDeadFighter", err)
	}

	// The rejection left the state consistent.
	if mm.Phase != PhaseActive {
		t.Errorf("phase = %q, want active", mm.Phase)
	}
	if mm.turnID != 2 || mm.current != SideB {
		t.Errorf("turn state = %d/%s, want 2/B", mm.turnID, mm.current)
	}
}

func TestMatchFireAfterFinishRejected(t *testing.T) {
	mm, start := newTestMatch(t, SideA)
	// B surrenders → winner is the OTHER side (A).
	if err := mm.Surrender(SideB); err != nil {
		t.Fatal(err)
	}
	if _, err := fire(t, mm, start, SideA, "a1", 1, "0"); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("fire after finish: %v, want ErrWrongPhase", err)
	}
	// Double finish: result immutable.
	if err := mm.Surrender(SideA); !errors.Is(err, ErrAlreadyFinished) {
		t.Errorf("second finish: %v, want ErrAlreadyFinished", err)
	}
	res := mm.Result()
	if res.Winner != SideA || res.Reason != EndSurrender {
		t.Errorf("result changed: %+v", res)
	}
}

func TestMatchNoArtificialDrawOnMisses(t *testing.T) {
	mm, start := newTestMatch(t, SideA)
	now := start
	turn := 1
	sides := []Side{SideA, SideB}
	shooters := map[Side]string{SideA: "a1", SideB: "b1"}
	for i := 0; i < 100; i++ {
		side := sides[i%2]
		if _, err := fire(t, mm, now, side, shooters[side], turn, "-x*2"); err != nil {
			t.Fatalf("legal miss %d rejected: %v", i, err)
		}
		now = now.Add(time.Minute)
		if !mm.AdvancePhase(now) {
			t.Fatalf("resolving did not end at turn %d", turn)
		}
		turn++
	}
	if mm.Phase != PhaseActive {
		t.Fatalf("phase = %q after 100 misses, want active", mm.Phase)
	}
	if mm.Result() != nil {
		t.Fatal("misses must never end the match")
	}
}

func TestMatchTimeoutStreak(t *testing.T) {
	mm, start := newTestMatch(t, SideA)
	now := start
	// Alternating timeouts pass the turn each time: A, B, A, B —
	// nobody reaches three in a row.
	for i := 0; i < 4; i++ {
		mm.OnTurnTimeout(now)
		if mm.Phase != PhaseActive {
			t.Fatalf("timeout %d: phase = %q, want active", i, mm.Phase)
		}
	}
	if mm.current != SideA {
		t.Fatalf("current = %s after 4 alternating timeouts, want A", mm.current)
	}
	// A's third personal timeout → technical loss, winner B.
	mm.OnTurnTimeout(now)
	if mm.Phase != PhaseFinished {
		t.Fatalf("phase = %q, want finished after third timeout", mm.Phase)
	}
	res := mm.Result()
	if res.Winner != SideB || res.Reason != EndTimeoutStreak {
		t.Fatalf("result = %+v, want B/timeout_streak", res)
	}
}

func TestMatchAcceptedShotResetsStreak(t *testing.T) {
	mm, start := newTestMatch(t, SideA)
	now := start
	adv := func() { now = now.Add(ResolvingDuration); mm.AdvancePhase(now) }

	// A times out (A streak 1) → B, turn 2.
	mm.OnTurnTimeout(now)
	// B answers, A answers: A's accepted shot resets A's streak.
	if _, err := fire(t, mm, now, SideB, "b1", 2, "-x*2"); err != nil {
		t.Fatal(err)
	}
	adv() // A, turn 3
	if _, err := fire(t, mm, now, SideA, "a1", 3, "-x*2"); err != nil {
		t.Fatal(err)
	}
	adv() // B, turn 4
	if _, err := fire(t, mm, now, SideB, "b1", 4, "-x*2"); err != nil {
		t.Fatal(err)
	}
	adv() // A, turn 5
	// A's streak starts over: timeouts 1 and 2 pass the turn each time.
	mm.OnTurnTimeout(now) // A streak 1 → B, turn 6
	if _, err := fire(t, mm, now, SideB, "b1", 6, "-x*2"); err != nil {
		t.Fatal(err)
	}
	adv()                 // A, turn 7
	mm.OnTurnTimeout(now) // A streak 2 → B, turn 8
	if _, err := fire(t, mm, now, SideB, "b1", 8, "-x*2"); err != nil {
		t.Fatal(err)
	}
	adv() // A, turn 9
	if mm.Result() != nil {
		t.Fatalf("A fired in between: streak must have reset, but match finished: %+v", mm.Result())
	}
	// A's third timeout in THIS streak → technical loss.
	mm.OnTurnTimeout(now)
	res := mm.Result()
	if res == nil || res.Winner != SideB || res.Reason != EndTimeoutStreak {
		t.Fatalf("result = %+v, want B/timeout_streak", res)
	}
}

func TestMatchAbort(t *testing.T) {
	mm, start := newTestMatch(t, SideA)
	mm.Abort()
	if mm.Phase != PhaseAborted || mm.Result().Winner != "" {
		t.Fatalf("abort: %q/%+v", mm.Phase, mm.Result())
	}
	// Aborted is terminal: nothing else changes it.
	mm.Abort()
	if _, err := fire(t, mm, start, SideA, "a1", 1, "0"); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("fire after abort: %v", err)
	}
}

func TestMatchFirstSideValidation(t *testing.T) {
	gm := GameMap{Fighters: spawnFighters()}
	if _, err := NewMatch("m", gm, Side("a"), time.Now()); err == nil {
		t.Error("lowercase first side accepted")
	}
}

func TestMatchPositionsNeverChange(t *testing.T) {
	mm, start := newTestMatch(t, SideA)
	before := append([]Fighter(nil), mm.Fighters...)
	if _, err := fire(t, mm, start, SideA, "a1", 1, "sin(x)"); err != nil {
		t.Fatal(err)
	}
	for i, f := range mm.Fighters {
		if f.X != before[i].X || f.Y != before[i].Y {
			t.Fatalf("fighter %s moved: (%v,%v) → (%v,%v)", f.ID, before[i].X, before[i].Y, f.X, f.Y)
		}
	}
}
