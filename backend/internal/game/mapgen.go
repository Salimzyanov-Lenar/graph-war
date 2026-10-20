package game

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// GeneratorVersion bumps whenever generation logic changes: two clients
// with different versions must not compare seeds across versions.
const GeneratorVersion = 1

// Spawn layout (fixed, not random): three fighters per side, mirrored
// by the 180° rotation (x,y) → (W-x, H-y). Matches the frontend fixture.
var spawnA = []Point{{X: 8, Y: 12}, {X: 12, Y: 30}, {X: 8, Y: 48}}

// MapParams is the tunable policy of map generation (experiment D02).
type MapParams struct {
	MinObstacles, MaxObstacles int     // total, including mirrored pairs
	MinRadius, MaxRadius       float64 // obstacle radius range
	SpawnClearance             float64 // obstacle surface must be this far from any spawn center
	ObstacleGap                float64 // min gap between obstacle surfaces
	PassabilityCell            float64 // BFS grid step for the passability check
	MaxAttempts                int     // attempts before the fallback map
}

// DefaultMapParams is the D02 recommendation: 4–6 obstacles in mirrored
// pairs, radii 2..6, comfortable clearances.
func DefaultMapParams() MapParams {
	return MapParams{
		MinObstacles:    4,
		MaxObstacles:    6,
		MinRadius:       2,
		MaxRadius:       6,
		SpawnClearance:  3,
		ObstacleGap:     1,
		PassabilityCell: 1,
		MaxAttempts:     200,
	}
}

// GameMap is the complete geometry the server sends to clients.
// Clients never generate maps from the seed themselves (contract §3).
type GameMap struct {
	Seed             uint64
	GeneratorVersion int
	Obstacles        []Obstacle
	Fighters         []Fighter
}

// GenerateMap builds a random map for the seed. It never fails: if
// MaxAttempts attempts fail validation, the fixed fallback map is
// returned (still labeled with the seed).
func GenerateMap(seed uint64, p MapParams) GameMap {
	m := GameMap{Seed: seed, GeneratorVersion: GeneratorVersion}

	if p.MaxAttempts <= 0 {
		p.MaxAttempts = DefaultMapParams().MaxAttempts
	}
	rng := rand.New(rand.NewPCG(seed, 0))
	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		cand := generateAttempt(rng, p)
		if cand != nil && ValidateMap(cand, p) == nil {
			m.Obstacles = cand
			m.Fighters = spawnFighters()
			return m
		}
	}
	m.Obstacles = fallbackObstacles()
	m.Fighters = spawnFighters()
	return m
}

// generateAttempt samples mirrored obstacle pairs until the requested
// count is reached or an obstacle cannot be placed (nil → retry).
func generateAttempt(rng *rand.Rand, p MapParams) []Obstacle {
	var obs []Obstacle
	total := p.MinObstacles + rng.IntN(p.MaxObstacles-p.MinObstacles+1)

	for len(obs) < total {
		var o Obstacle
		if len(obs) == 0 && rng.IntN(2) == 0 {
			// A self-mirrored obstacle must sit exactly at the field
			// center (50,30): the 180° rotation maps (x,y) → (100-x,60-y),
			// so only the exact center maps to itself.
			o = Obstacle{X: 50, Y: 30, R: p.MinRadius + rng.Float64()*(p.MaxRadius-p.MinRadius)}
			if !fits(o, obs, p) {
				return nil
			}
			obs = append(obs, o)
			continue
		}
		o = sampleObstacle(rng, p, 10, 44)
		if o == (Obstacle{}) || !fits(o, obs, p) {
			return nil
		}
		obs = append(obs, o, Obstacle{X: FieldWidth - o.X, Y: FieldHeight - o.Y, R: o.R})
	}
	return obs
}

func sampleObstacle(rng *rand.Rand, p MapParams, xMin, xMax float64) Obstacle {
	x := xMin + rng.Float64()*(xMax-xMin)
	y := p.MinRadius + rng.Float64()*(FieldHeight-2*p.MinRadius)
	r := p.MinRadius + rng.Float64()*(p.MaxRadius-p.MinRadius)
	return Obstacle{X: x, Y: y, R: r}
}

func fits(o Obstacle, obs []Obstacle, p MapParams) bool {
	if o.X-o.R < 0.5 || o.X+o.R > FieldWidth-0.5 || o.Y-o.R < 0.5 || o.Y+o.R > FieldHeight-0.5 {
		return false
	}
	for _, other := range obs {
		if math.Hypot(o.X-other.X, o.Y-other.Y) < o.R+other.R+p.ObstacleGap {
			return false
		}
	}
	for _, s := range spawnA {
		if math.Hypot(o.X-s.X, o.Y-s.Y) < o.R+p.SpawnClearance {
			return false
		}
		for _, sm := range spawnA {
			if math.Hypot(o.X-(FieldWidth-sm.X), o.Y-(FieldHeight-sm.Y)) < o.R+p.SpawnClearance {
				return false
			}
		}
	}
	return true
}

func spawnFighters() []Fighter {
	fs := make([]Fighter, 0, 2*len(spawnA))
	for i, s := range spawnA {
		fs = append(fs,
			Fighter{ID: fmt.Sprintf("a%d", i+1), Side: SideA, X: s.X, Y: s.Y, Alive: true},
			Fighter{ID: fmt.Sprintf("b%d", i+1), Side: SideB, X: FieldWidth - s.X, Y: FieldHeight - s.Y, Alive: true},
		)
	}
	return fs
}

// fallbackObstacles is the verified spare map used when generation
// exhausts its attempts. Known good: central obstacle + one mirrored pair.
func fallbackObstacles() []Obstacle {
	return []Obstacle{
		{X: 50, Y: 30, R: 6},
		{X: 34, Y: 16, R: 3},
		{X: 66, Y: 44, R: 3},
	}
}

// ValidateMap enforces every map invariant. Used on generated maps, on
// the fallback and in the mass-generation tests.
func ValidateMap(obs []Obstacle, p MapParams) error {
	if len(obs) < 2 {
		return fmt.Errorf("слишком мало препятствий: %d", len(obs))
	}
	if len(obs) > 64 {
		return fmt.Errorf("слишком много препятствий: %d > 64", len(obs))
	}
	for i, o := range obs {
		if o.R <= 0 {
			return fmt.Errorf("препятствие %d: радиус %v ≤ 0", i, o.R)
		}
		if o.X-o.R < 0.5 || o.X+o.R > FieldWidth-0.5 || o.Y-o.R < 0.5 || o.Y+o.R > FieldHeight-0.5 {
			return fmt.Errorf("препятствие %d (%v,%v,r=%v) выходит за поле", i, o.X, o.Y, o.R)
		}
		for _, s := range spawnA {
			if math.Hypot(o.X-s.X, o.Y-s.Y) < o.R+p.SpawnClearance {
				return fmt.Errorf("препятствие %d слишком близко к спавну", i)
			}
			if math.Hypot(o.X-(FieldWidth-s.X), o.Y-(FieldHeight-s.Y)) < o.R+p.SpawnClearance {
				return fmt.Errorf("препятствие %d слишком близко к зеркальному спавну", i)
			}
		}
		for j := i + 1; j < len(obs); j++ {
			o2 := obs[j]
			if math.Hypot(o.X-o2.X, o.Y-o2.Y) < o.R+o2.R+p.ObstacleGap {
				return fmt.Errorf("препятствия %d и %d перекрываются", i, j)
			}
		}
	}
	// Passability is deliberately NOT required (D02): a wall that cuts
	// the field in two is a legal map, as in the original Graphwar.
	return nil
}

// passable reports whether the two sides are connected on a coarse
// grid (cells blocked within r+FighterRadius of an obstacle center).
// Kept for diagnostics and future map-quality experiments; NOT part of
// ValidateMap since D02 decided walls are legal.
func passable(obs []Obstacle, p MapParams) bool {
	step := p.PassabilityCell
	cols := int(FieldWidth / step)
	rows := int(FieldHeight / step)
	blocked := make([]bool, cols*rows)
	for c := 0; c < cols; c++ {
		for r := 0; r < rows; r++ {
			x, y := (float64(c)+0.5)*step, (float64(r)+0.5)*step
			for _, o := range obs {
				if math.Hypot(x-o.X, y-o.Y) < o.R+FighterRadius {
					blocked[r*cols+c] = true
					break
				}
			}
		}
	}

	bfs := func(startCell, goalCol func(c, r int) bool) bool {
		seen := make([]bool, cols*rows)
		queue := []int{}
		for c := 0; c < cols; c++ {
			for r := 0; r < rows; r++ {
				if blocked[r*cols+c] {
					continue
				}
				if startCell(c, r) {
					queue = append(queue, r*cols+c)
					seen[r*cols+c] = true
				}
			}
		}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			cc, cr := cur%cols, cur/cols
			if goalCol(cc, cr) {
				return true
			}
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nc, nr := cc+d[0], cr+d[1]
				if nc < 0 || nc >= cols || nr < 0 || nr >= rows {
					continue
				}
				idx := nr*cols + nc
				if !seen[idx] && !blocked[idx] {
					seen[idx] = true
					queue = append(queue, idx)
				}
			}
		}
		return false
	}

	return bfs(
		func(c, r int) bool { return c < 2 },
		func(c, r int) bool { return c >= cols-2 },
	)
}
