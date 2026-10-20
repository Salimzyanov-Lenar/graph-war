// Package game holds the authoritative game model: trajectories,
// collisions, maps and match state. The server owns all of it; clients
// only render what comes out of here (contract §1).
package game

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/expr"
)

// Field dimensions from contract §1.
const (
	FieldWidth  = 100.0
	FieldHeight = 60.0
)

// Side of a player. Lowercase is not allowed anywhere in the protocol.
type Side string

const (
	SideA Side = "A"
	SideB Side = "B"
)

func (s Side) Valid() bool { return s == SideA || s == SideB }

// dir returns the horizontal direction of fire: A shoots right, B left.
func (s Side) dir() float64 {
	if s == SideA {
		return +1
	}
	return -1
}

// Point in world coordinates.
type Point struct {
	X, Y float64
}

// StopReason explains why a trajectory ended (contract §3).
type StopReason string

const (
	StopHit          StopReason = "hit" // set by the B05 collision layer, not here
	StopOutOfField   StopReason = "out_of_field"
	StopDiscontinuit StopReason = "discontinuity"
	StopNonFinite    StopReason = "non_finite"
	StopMaxPoints    StopReason = "max_points"
)

// Numeric policy from contract §3 (D01).
const (
	StepUnit    = 0.5 // t step in world units
	MaxPoints   = 400 // sample budget per shot
	MagnitudeCe = 1e6 // |f(t)-f(0)| beyond this = asymptote/overflow
)

// ErrStartDomain marks a formula whose f(0) does not exist: the shot is
// rejected and the turn is not spent (contract §3).
var ErrStartDomain = errors.New("выстрел не может начаться: f(0) не определён")

// Trajectory is the sampled flight path. Points[0] is the shooter
// position; the last point is where the flight stopped.
type Trajectory struct {
	Points []Point
	Stop   StopReason
}

// BuildTrajectory samples the flight path of a shot: t runs from 0 with
// a fixed step, X = X0 + dir*t, Y = Y0 + f(t) - f(0). Infrastructure
// failures (budget, cancellation) come back as an error; domain
// failures shape the trajectory itself (contract §3).
func BuildTrajectory(ctx context.Context, ast expr.Node, side Side, shooter Point) (Trajectory, error) {
	if !side.Valid() {
		return Trajectory{}, fmt.Errorf("неизвестная сторона %q", side)
	}

	f0, err := expr.Eval(ctx, ast, 0)
	if err != nil {
		return Trajectory{}, fmt.Errorf("%w: %v", ErrStartDomain, err)
	}

	tr := Trajectory{Points: []Point{shooter}, Stop: StopMaxPoints}
	prevT := 0.0

	for i := 1; i < MaxPoints; i++ {
		t := float64(i) * StepUnit

		// Subdivision probes inside the interval: a domain error or an
		// absurd magnitude here means an asymptote between samples.
		for _, frac := range []float64{1.0 / 3.0, 2.0 / 3.0} {
			if _, err := evalRel(ctx, ast, f0, prevT+(t-prevT)*frac); err != nil {
				tr.Stop = StopDiscontinuit
				return tr, nil
			}
		}

		rel, err := evalRel(ctx, ast, f0, t)
		if err != nil {
			tr.Stop = StopNonFinite
			return tr, nil
		}
		p := Point{X: shooter.X + side.dir()*t, Y: shooter.Y + rel}

		// Outside the field: include the exit point so the renderer sees
		// the direction of departure, then stop.
		if p.X < 0 || p.X > FieldWidth || p.Y < 0 || p.Y > FieldHeight {
			tr.Points = append(tr.Points, p)
			tr.Stop = StopOutOfField
			return tr, nil
		}

		tr.Points = append(tr.Points, p)
		prevT = t
	}
	return tr, nil
}

// evalRel computes f(t)-f(0) and rejects absurd magnitudes.
func evalRel(ctx context.Context, ast expr.Node, f0, t float64) (float64, error) {
	v, err := expr.Eval(ctx, ast, t)
	if err != nil {
		return 0, err
	}
	rel := v - f0
	if math.Abs(rel) > MagnitudeCe {
		return 0, fmt.Errorf("%w: |f(t)-f(0)| > %g", expr.ErrDomain, MagnitudeCe)
	}
	return rel, nil
}
