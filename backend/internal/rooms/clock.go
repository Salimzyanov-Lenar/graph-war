package rooms

import "time"

// Timer is the part of *time.Timer the hub needs.
type Timer interface{ Stop() bool }

// Clock lets tests drive match time without sleeping (B12).
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// RealClock is the wall clock.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

func (RealClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

// Conn is one live WebSocket from the hub's point of view. Both methods
// must never block: the hub calls them with its lock held.
type Conn interface {
	// Send queues an already-encoded message; false means the queue is full
	// (slow client) and the hub drops the connection.
	Send(msg []byte) bool
	// Close asks the transport to close with a WebSocket status code.
	Close(code int, reason string)
}

// WebSocket close codes used by the hub.
const (
	CloseReplaced = 4001 // a newer connection of the same guest took over
	CloseSlow     = 4002 // outbound queue overflow
	CloseNoRoom   = 4003 // guest is not in any room
)
