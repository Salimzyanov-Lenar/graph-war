package rooms

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/expr"
	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/game"
)

const (
	maxRequestIDLen = 64
	maxSeenRequests = 128
	fireBudget      = 2 * time.Second
)

// startMatchLocked turns a ready room into a match: map from the seed,
// a new stream `match:<id>`, a snapshot to every connected player.
func (h *Hub) startMatchLocked(r *room) {
	seed := h.cfg.Seed()
	gm := game.GenerateMap(seed, game.DefaultMapParams())
	matchID := randomID(8)
	m, err := game.NewMatch(matchID, gm, h.cfg.First(), h.cfg.Clock.Now())
	if err != nil {
		h.cfg.Logger.Error("start match", "err", err)
		return
	}
	r.match = m
	r.seedString = strconv.FormatUint(seed, 10)
	r.streamID = "match:" + matchID
	r.seq = 0
	r.seen = map[string]map[string]string{}
	for id, c := range r.conns {
		if !c.Send(h.snapshotFor(r, id)) {
			h.dropConnLocked(r, id, c, CloseSlow)
		}
	}
	h.scheduleTurnLocked(r)
}

// scheduleTurnLocked arms the turn deadline timer for the current turn (B12):
// one timer per turn, no per-frame ticker.
func (h *Hub) scheduleTurnLocked(r *room) {
	stopTimer(r.turnTimer)
	m := r.match
	if m == nil || m.Phase != game.PhaseActive {
		return
	}
	_, turnID := m.CurrentTurn()
	d := m.Deadline().Sub(h.cfg.Clock.Now())
	r.turnTimer = h.cfg.Clock.AfterFunc(d, func() { h.onTurnTimeout(r, m, turnID) })
}

func (h *Hub) onTurnTimeout(r *room, m *game.Match, turnID int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r.match != m || m.Phase != game.PhaseActive {
		return
	}
	if _, cur := m.CurrentTurn(); cur != turnID {
		return
	}
	m.OnTurnTimeout(h.cfg.Clock.Now())
	h.afterAdvanceLocked(r)
}

func (h *Hub) onResolved(r *room, m *game.Match) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r.match != m {
		return
	}
	if m.AdvancePhase(h.cfg.Clock.Now()) {
		h.afterAdvanceLocked(r)
	}
}

// afterAdvanceLocked publishes the outcome of a timer-driven transition.
func (h *Hub) afterAdvanceLocked(r *room) {
	m := r.match
	if m.Result() != nil {
		h.finishedLocked(r)
		return
	}
	h.broadcast(r, "turn.opened", turnBody(m, h.cfg.Clock.Now()))
	h.scheduleTurnLocked(r)
}

func (h *Hub) finishedLocked(r *room) {
	stopTimer(r.turnTimer)
	stopTimer(r.resolveTmr)
	if res := resultFromMatch(r.match.Result()); res != nil {
		h.broadcast(r, "match.finished", res)
	}
}

type command struct {
	V       *int            `json:"v"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type fireCommand struct {
	MatchID        string `json:"matchId"`
	ExpectedTurnID string `json:"expectedTurnId"`
	FighterID      string `json:"fighterId"`
	Expression     string `json:"expression"`
	RequestID      string `json:"requestId"`
}

// Command executes one client command. Failures are reported only to the
// sender as an `error` message and never change the match state.
func (h *Hub) Command(guestID string, c Conn, raw []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	fail := func(code, msg string) { c.Send(encodeError(code, msg)) }

	r, ok := h.member[guestID]
	if !ok || r.conns[guestID] != c {
		fail("room_expired", "вы не в комнате")
		return
	}
	var cmd command
	if err := json.Unmarshal(raw, &cmd); err != nil {
		fail("bad_request", "не удалось разобрать команду")
		return
	}
	if cmd.V != nil && *cmd.V != 1 {
		fail("bad_request", "неподдерживаемая версия протокола")
		return
	}
	p := findPlayer(r, guestID)

	switch cmd.Type {
	case "room.ready":
		var body struct {
			Ready bool `json:"ready"`
		}
		if err := json.Unmarshal(cmd.Payload, &body); err != nil {
			fail("bad_request", "ожидается {ready: bool}")
			return
		}
		if err := h.setReadyLocked(r, p, body.Ready); err != nil {
			fail("bad_request", err.Error())
		}

	case "match.fire":
		var body fireCommand
		if err := json.Unmarshal(cmd.Payload, &body); err != nil {
			fail("bad_request", "некорректный match.fire")
			return
		}
		if code, msg := h.fireLocked(r, p, body); code != "" {
			fail(code, msg)
		}

	case "match.surrender":
		var body struct {
			Confirm bool `json:"confirm"`
		}
		if err := json.Unmarshal(cmd.Payload, &body); err != nil || !body.Confirm {
			fail("bad_request", "сдача требует confirm: true")
			return
		}
		if r.match == nil {
			fail("not_your_turn", "матч ещё не начался")
			return
		}
		if err := r.match.Surrender(p.side); err != nil {
			fail("not_your_turn", err.Error())
			return
		}
		h.finishedLocked(r)

	default:
		fail("bad_request", "неизвестная команда "+strconv.Quote(cmd.Type))
	}
}

// fireLocked returns an error code ("" on success).
func (h *Hub) fireLocked(r *room, p *player, body fireCommand) (code, msg string) {
	m := r.match
	if m == nil || body.MatchID != m.ID {
		return "stale_turn", "матч не найден или уже другой"
	}
	if body.RequestID == "" || len(body.RequestID) > maxRequestIDLen {
		return "bad_request", "requestId обязателен (до 64 символов)"
	}

	hash := body.MatchID + "|" + body.ExpectedTurnID + "|" + body.FighterID + "|" + body.Expression
	seen := r.seen[p.guestID]
	if seen == nil {
		seen = map[string]string{}
		r.seen[p.guestID] = seen
	}
	if prev, dup := seen[body.RequestID]; dup {
		if prev == hash {
			return "", "" // retry of an accepted shot: the result was already broadcast
		}
		return "duplicate_request", "requestId уже использован с другим содержимым"
	}

	turnID, err := strconv.Atoi(body.ExpectedTurnID)
	if err != nil {
		return "stale_turn", "некорректный expectedTurnId"
	}
	ast, err := expr.Parse(body.Expression)
	if err != nil {
		return "bad_expression", err.Error()
	}

	ctx, cancel := context.WithTimeout(context.Background(), fireBudget)
	defer cancel()
	rec, err := m.Fire(ctx, h.cfg.Clock.Now(), p.side, body.FighterID, turnID, ast)
	switch {
	case err == nil:
	case errors.Is(err, game.ErrStartDomain):
		return "domain_error_at_start", err.Error()
	case errors.Is(err, game.ErrNotYourTurn), errors.Is(err, game.ErrWrongPhase):
		return "not_your_turn", err.Error()
	case errors.Is(err, game.ErrStaleTurn):
		return "stale_turn", err.Error()
	case errors.Is(err, game.ErrUnknownFighter), errors.Is(err, game.ErrWrongSide):
		return "unknown_fighter", err.Error()
	case errors.Is(err, game.ErrDeadFighter):
		return "dead_fighter", err.Error()
	default:
		h.cfg.Logger.Error("fire failed", "err", err)
		return "server_error", "внутренняя ошибка"
	}

	if len(seen) >= maxSeenRequests {
		clear(seen)
	}
	seen[body.RequestID] = hash

	stopTimer(r.turnTimer)
	h.broadcast(r, "shot.resolved", shotFromRecord(m.ID, rec))
	if m.Result() != nil {
		h.finishedLocked(r)
		return "", ""
	}
	r.resolveTmr = h.cfg.Clock.AfterFunc(game.ResolvingDuration, func() { h.onResolved(r, m) })
	return "", ""
}
