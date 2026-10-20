package game

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/expr"
)

// Match timing policy (contract §6).
const (
	TurnDuration      = 60 * time.Second
	ResolvingDuration = 1200 * time.Millisecond
	TimeoutStreakMax  = 3
)

// Phase of the match (contract §6): lobby → active → (resolving ↔ active)*
// → finished; from any state → aborted.
type Phase string

const (
	PhaseLobby     Phase = "lobby"
	PhaseActive    Phase = "active"
	PhaseResolving Phase = "resolving"
	PhaseFinished  Phase = "finished"
	PhaseAborted   Phase = "aborted"
)

// End reason of a finished match (contract §6).
type EndReason string

const (
	EndTeamWiped         EndReason = "team_wiped"
	EndSurrender         EndReason = "surrender"
	EndTimeoutStreak     EndReason = "timeout_streak"
	EndForfeitDisconnect EndReason = "forfeit_disconnect"
)

// Rule violations surface as typed errors: the WS layer maps them to
// error codes, the state itself never changes (contract §8).
var (
	ErrNotYourTurn     = errors.New("не ваш ход")
	ErrUnknownFighter  = errors.New("бойец не найден")
	ErrDeadFighter     = errors.New("бойец уничтожен")
	ErrWrongSide       = errors.New("бойец другой стороны")
	ErrWrongPhase      = errors.New("матч не в фазе active")
	ErrStaleTurn       = errors.New("ход уже сменился (stale_turn)")
	ErrAlreadyFinished = errors.New("матч уже завершён")
	ErrNotParticipant  = errors.New("сторона не участвует в матче")
)

// MatchResult is immutable once set: whoever finishes a match second
// gets ErrAlreadyFinished, not a changed winner.
type MatchResult struct {
	Winner        Side // "" only for aborted
	WinnerIsValid bool
	Reason        EndReason
}

// ShotRecord is the last accepted shot, kept for snapshots
// (reconnecting clients must be able to replay the animation).
type ShotRecord struct {
	TurnID             int
	Side               Side
	Points             []Point
	Stop               StopReason
	DestroyedFighterID string
}

// Match is the pure state machine of one match. No network, no DB, no
// wall clock: every method takes `now` so tests are deterministic and
// the WS driver (B10–B12) feeds real time in.
type Match struct {
	ID    string
	Phase Phase

	Fighters  []Fighter  // positions never change; only Alive flips
	obstacles []Obstacle // copied from the GameMap at construction

	current      Side // side whose turn it is in active phase
	turnID       int  // monotonic, expected by match.fire
	deadline     time.Time
	resolvingEnd time.Time
	lastShot     *ShotRecord
	result       *MatchResult

	timeoutStreak map[Side]int
}

// NewMatch starts an active match: the server chose the first side
// (contract §4, who exactly — решает B09/B10). turnID starts at 1.
func NewMatch(id string, gm GameMap, first Side, now time.Time) (*Match, error) {
	if !first.Valid() {
		return nil, fmt.Errorf("неизвестная первая сторона %q", first)
	}
	fighters := make([]Fighter, len(gm.Fighters))
	copy(fighters, gm.Fighters)
	return &Match{
		ID:            id,
		Phase:         PhaseActive,
		Fighters:      fighters,
		obstacles:     append([]Obstacle(nil), gm.Obstacles...),
		current:       first,
		turnID:        1,
		deadline:      now.Add(TurnDuration),
		timeoutStreak: map[Side]int{SideA: 0, SideB: 0},
	}, nil
}

// Phase snapshot helpers ------------------------------------------------

// CurrentTurn returns the active side and the turn ID its fire must
// carry (expectedTurnId).
func (m *Match) CurrentTurn() (Side, int) { return m.current, m.turnID }

// Deadline is when the current turn expires (active phase only).
func (m *Match) Deadline() time.Time { return m.deadline }

// Result is non-nil once the match reached a terminal state.
func (m *Match) Result() *MatchResult { return m.result }

// LastShot is the most recent accepted shot (nil before the first).
func (m *Match) LastShot() *ShotRecord { return m.lastShot }

// Actions ----------------------------------------------------------------

// Fire validates and applies a shot (contract §8 match.fire).
// Rejections leave the state untouched. An accepted shot is recorded,
// applies at most one destruction, and either finishes the match
// (team wiped) or switches to resolving.
func (m *Match) Fire(ctx context.Context, now time.Time, side Side, fighterID string, expectedTurnID int, ast expr.Node) (ShotRecord, error) {
	if m.Phase != PhaseActive {
		return ShotRecord{}, ErrWrongPhase
	}
	if m.current != side {
		return ShotRecord{}, ErrNotYourTurn
	}
	if expectedTurnID != m.turnID {
		return ShotRecord{}, ErrStaleTurn
	}
	idx := -1
	for i, f := range m.Fighters {
		if f.ID == fighterID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ShotRecord{}, ErrUnknownFighter
	}
	if m.Fighters[idx].Side != side {
		return ShotRecord{}, ErrWrongSide
	}
	if !m.Fighters[idx].Alive {
		return ShotRecord{}, ErrDeadFighter
	}

	shooter := m.Fighters[idx]
	res, err := ResolveShot(ctx, ast, side, Point{X: shooter.X, Y: shooter.Y}, shooter.ID, Battleground{
		Obstacles: m.Obstacles(),
		Fighters:  m.Fighters,
	})
	if err != nil {
		return ShotRecord{}, fmt.Errorf("internal error: trajectory failed: %w", err)
	}

	rec := ShotRecord{
		TurnID:             m.turnID,
		Side:               side,
		Points:             res.Points,
		Stop:               res.Stop,
		DestroyedFighterID: res.DestroyedFighterID,
	}
	if rec.DestroyedFighterID != "" {
		for i := range m.Fighters {
			if m.Fighters[i].ID == rec.DestroyedFighterID {
				m.Fighters[i].Alive = false
			}
		}
	}
	m.lastShot = &rec
	m.timeoutStreak[side] = 0

	if wiped, winner := m.teamWiped(); wiped {
		m.finish(winner, EndTeamWiped)
		return rec, nil
	}

	// Accepted shot: show the animation, then the other side moves.
	m.Phase = PhaseResolving
	m.resolvingEnd = now.Add(ResolvingDuration)
	return rec, nil
}

// AdvancePhase moves resolving → active (opening the next turn) when
// the resolving window is over. Premature calls are ignored: the driver
// may poll freely. Returns true when a new turn opened.
func (m *Match) AdvancePhase(now time.Time) bool {
	if m.Phase != PhaseResolving || now.Before(m.resolvingEnd) {
		return false
	}
	m.openTurn(otherSide(m.current), now)
	return true
}

// OnTurnTimeout handles an expired turn deadline (driver decides when
// to call this, B12). Counts a personal timeout; the third consecutive
// one is a technical loss. A timeout passes the turn.
func (m *Match) OnTurnTimeout(now time.Time) {
	if m.Phase != PhaseActive {
		return
	}
	m.timeoutStreak[m.current]++
	if m.timeoutStreak[m.current] >= TimeoutStreakMax {
		m.finish(otherSide(m.current), EndTimeoutStreak)
		return
	}
	m.openTurn(otherSide(m.current), now)
}

// Surrender is an explicit, confirmed action (contract §8).
func (m *Match) Surrender(side Side) error {
	if m.result != nil {
		return ErrAlreadyFinished
	}
	if m.Phase != PhaseActive && m.Phase != PhaseResolving {
		return ErrWrongPhase
	}
	if side != SideA && side != SideB {
		return ErrNotParticipant
	}
	m.finish(otherSide(side), EndSurrender)
	return nil
}

// Forfeit ends the match with a technical loss for `loser` (left the
// room or stayed disconnected past the grace period).
func (m *Match) Forfeit(loser Side) error {
	if m.result != nil {
		return ErrAlreadyFinished
	}
	if m.Phase != PhaseActive && m.Phase != PhaseResolving {
		return ErrWrongPhase
	}
	if !loser.Valid() {
		return ErrNotParticipant
	}
	m.finish(otherSide(loser), EndForfeitDisconnect)
	return nil
}

// Abort marks the match aborted (server restart, both offline).
func (m *Match) Abort() {
	if m.result != nil {
		return
	}
	m.Phase = PhaseAborted
	m.result = &MatchResult{Reason: EndForfeitDisconnect}
}

// Internals --------------------------------------------------------------

func (m *Match) Obstacles() []Obstacle { return m.obstacles }

func (m *Match) openTurn(side Side, now time.Time) {
	m.Phase = PhaseActive
	m.current = side
	m.turnID++
	m.deadline = now.Add(TurnDuration)
}

func (m *Match) finish(winner Side, reason EndReason) {
	m.Phase = PhaseFinished
	m.result = &MatchResult{Winner: winner, WinnerIsValid: true, Reason: reason}
}

func (m *Match) teamWiped() (bool, Side) {
	aAlive, bAlive := false, false
	for _, f := range m.Fighters {
		if f.Alive {
			if f.Side == SideA {
				aAlive = true
			} else {
				bAlive = true
			}
		}
	}
	switch {
	case !aAlive:
		return true, SideB
	case !bAlive:
		return true, SideA
	}
	return false, ""
}

func otherSide(s Side) Side {
	if s == SideA {
		return SideB
	}
	return SideA
}
