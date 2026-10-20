package game

import (
	"context"
	"math"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/expr"
)

// Collision policy (contract §3, added with B05):
//   - the shot is a ray with zero thickness;
//   - fighters are circles of FighterRadius; obstacles are circles from
//     the map;
//   - any intersection counts, touching (tangent) included;
//   - earliest intersection along the flight wins; a fighter and an
//     obstacle reached within tieEps of each other → the fighter wins;
//   - dead fighters and the shooter do not collide.

const (
	// FighterRadius is the collision radius of a fighter in world units.
	FighterRadius = 1.0
	// tieEps: two intersections closer than this are a tie.
	tieEps = 1e-9
)

// Obstacle is an indestructible circle on the map.
type Obstacle struct {
	X, Y, R float64
}

// Fighter is a unit on the field. Collision only cares about position
// and liveness; the rest is match state.
type Fighter struct {
	ID    string
	Side  Side
	X, Y  float64
	Alive bool
}

// Battleground is everything a shot can collide with.
type Battleground struct {
	Obstacles []Obstacle
	Fighters  []Fighter
}

// ShotResult is a trajectory with collision applied. When Stop == hit,
// Points end exactly on the collision surface and DestroyedFighterID
// names the fighter hit (empty for obstacles).
type ShotResult struct {
	Trajectory
	DestroyedFighterID string
}

// ResolveShot builds the trajectory (B04) and truncates it at the first
// collision (B05). Results never depend on the order of objects in the
// Battleground: every candidate intersection is computed, the earliest
// wins deterministically.
func ResolveShot(ctx context.Context, ast expr.Node, side Side, shooter Point, shooterID string, bg Battleground) (ShotResult, error) {
	tr, err := BuildTrajectory(ctx, ast, side, shooter)
	if err != nil {
		return ShotResult{}, err
	}
	res := ShotResult{Trajectory: tr}

	bestSeg := -1 // segment index
	bestS := 0.0  // position along that segment, [0,1]
	bestFighter := ""

	consider := func(seg int, s float64, isFighter bool, fighterID string) {
		if bestSeg != -1 {
			if seg > bestSeg {
				return // an earlier segment always wins
			}
			if seg == bestSeg {
				if s > bestS+tieEps {
					return
				}
				if s > bestS && !isFighter {
					return // tie: only a fighter displaces an obstacle
				}
			}
		}
		bestSeg, bestS = seg, s
		bestFighter = ""
		if isFighter {
			bestFighter = fighterID
		}
	}

	for seg := 0; seg+1 < len(tr.Points); seg++ {
		p0, p1 := tr.Points[seg], tr.Points[seg+1]
		for _, ob := range bg.Obstacles {
			if s, ok := segmentCircle(p0, p1, ob.X, ob.Y, ob.R); ok {
				consider(seg, s, false, "")
			}
		}
		for _, f := range bg.Fighters {
			if !f.Alive || f.ID == shooterID {
				continue
			}
			if s, ok := segmentCircle(p0, p1, f.X, f.Y, FighterRadius); ok {
				consider(seg, s, true, f.ID)
			}
		}
	}

	if bestSeg == -1 {
		return res, nil
	}

	p0, p1 := tr.Points[bestSeg], tr.Points[bestSeg+1]
	hit := Point{
		X: p0.X + (p1.X-p0.X)*bestS,
		Y: p0.Y + (p1.Y-p0.Y)*bestS,
	}
	res.Points = append(tr.Points[:bestSeg+1], hit)
	res.Stop = StopHit
	res.DestroyedFighterID = bestFighter
	return res, nil
}

// segmentCircle returns the smallest s in [0,1] where the segment
// P0+s·(P1−P0) touches the circle, or ok=false. A segment starting
// inside the circle counts as s=0.
func segmentCircle(p0, p1 Point, cx, cy, r float64) (float64, bool) {
	dx, dy := p1.X-p0.X, p1.Y-p0.Y
	fx, fy := p0.X-cx, p0.Y-cy

	a := dx*dx + dy*dy
	if a == 0 {
		return 0, false // degenerate segment
	}
	b := 2 * (fx*dx + fy*dy)
	c := fx*fx + fy*fy - r*r
	if c <= 0 {
		return 0, true // start inside (or on the surface)
	}
	disc := b*b - 4*a*c
	if disc < 0 {
		return 0, false
	}
	root := math.Sqrt(disc)
	s := (-b - root) / (2 * a)
	if s < 0 || s > 1 {
		return 0, false
	}
	return s, true
}
