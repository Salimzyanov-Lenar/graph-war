package rooms

import (
	"encoding/json"
	"strconv"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/game"
)

// Wire shapes of contract §8 (field names are what the frontend schemas expect).

type eventEnvelope struct {
	V        int    `json:"v"`
	Type     string `json:"type"`
	StreamID string `json:"streamId"`
	Seq      int    `json:"seq"`
	Payload  any    `json:"payload"`
}

// Errors go only to the sender and are not part of any stream.
type errorEnvelope struct {
	V       int          `json:"v"`
	Type    string       `json:"type"`
	Payload errorPayload `json:"payload"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type playerWire struct {
	GuestID     string `json:"guestId"`
	DisplayName string `json:"displayName"`
	Side        string `json:"side,omitempty"`
	Ready       bool   `json:"ready"`
}

type roomChangedWire struct {
	Players []playerWire `json:"players"`
}

type fighterWire struct {
	ID    string  `json:"id"`
	Side  string  `json:"side"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Alive bool    `json:"alive"`
}

type obstacleWire struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	R float64 `json:"r"`
}

type mapWire struct {
	Width            float64        `json:"width"`
	Height           float64        `json:"height"`
	Obstacles        []obstacleWire `json:"obstacles"`
	MapSeed          string         `json:"mapSeed"`
	GeneratorVersion int            `json:"generatorVersion"`
}

type matchWire struct {
	MatchID  string        `json:"matchId"`
	YourSide string        `json:"yourSide"`
	Map      mapWire       `json:"map"`
	Fighters []fighterWire `json:"fighters"`
}

type turnWire struct {
	TurnID     string `json:"turnId"`
	Side       string `json:"side"`
	DeadlineAt int64  `json:"deadlineAt"`
	ServerTime int64  `json:"serverTime"`
}

type shotWire struct {
	ShotID             string       `json:"shotId"`
	TurnID             string       `json:"turnId"`
	Points             [][2]float64 `json:"points"`
	StopReason         string       `json:"stopReason"`
	DestroyedFighterID string       `json:"destroyedFighterId,omitempty"`
}

type resultWire struct {
	WinnerSide string `json:"winnerSide,omitempty"`
	Reason     string `json:"reason"`
}

type snapshotWire struct {
	Phase    string       `json:"phase"`
	Players  []playerWire `json:"players"`
	Match    *matchWire   `json:"match,omitempty"`
	Turn     *turnWire    `json:"turn,omitempty"`
	LastShot *shotWire    `json:"lastShot,omitempty"`
	Result   *resultWire  `json:"result,omitempty"`
}

func encodeEvent(typ, stream string, seq int, payload any) []byte {
	b, err := json.Marshal(eventEnvelope{V: 1, Type: typ, StreamID: stream, Seq: seq, Payload: payload})
	if err != nil {
		// Payloads are plain structs of finite numbers; a failure is a bug.
		panic("rooms: encode " + typ + ": " + err.Error())
	}
	return b
}

func encodeError(code, message string) []byte {
	b, _ := json.Marshal(errorEnvelope{V: 1, Type: "error", Payload: errorPayload{Code: code, Message: message}})
	return b
}

func turnIDString(id int) string { return strconv.Itoa(id) }

func shotFromRecord(matchID string, rec game.ShotRecord) shotWire {
	points := make([][2]float64, len(rec.Points))
	for i, p := range rec.Points {
		points[i] = [2]float64{p.X, p.Y}
	}
	return shotWire{
		ShotID:             matchID + ":" + turnIDString(rec.TurnID),
		TurnID:             turnIDString(rec.TurnID),
		Points:             points,
		StopReason:         string(rec.Stop),
		DestroyedFighterID: rec.DestroyedFighterID,
	}
}

func resultFromMatch(r *game.MatchResult) *resultWire {
	if r == nil {
		return nil
	}
	w := &resultWire{Reason: string(r.Reason)}
	if r.WinnerIsValid {
		w.WinnerSide = string(r.Winner)
	}
	return w
}
