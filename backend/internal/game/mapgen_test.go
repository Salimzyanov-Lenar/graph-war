package game

import (
	"fmt"
	"math"
	"testing"
)

func TestGenerateMapReproducible(t *testing.T) {
	for _, seed := range []uint64{1, 42, 12345, 987654321} {
		a := GenerateMap(seed, DefaultMapParams())
		b := GenerateMap(seed, DefaultMapParams())
		if len(a.Obstacles) != len(b.Obstacles) {
			t.Fatalf("seed %d: obstacle count differs between runs", seed)
		}
		for i := range a.Obstacles {
			if a.Obstacles[i] != b.Obstacles[i] {
				t.Fatalf("seed %d: obstacle %d differs: %+v vs %+v", seed, i, a.Obstacles[i], b.Obstacles[i])
			}
		}
	}
}

func TestGenerateMapValid(t *testing.T) {
	p := DefaultMapParams()
	for seed := uint64(1); seed <= 500; seed++ {
		m := GenerateMap(seed, p)
		if err := ValidateMap(m.Obstacles, p); err != nil {
			t.Fatalf("seed %d: invalid map: %v", seed, err)
		}
		if len(m.Fighters) != 6 {
			t.Fatalf("seed %d: %d fighters, want 6", seed, len(m.Fighters))
		}
		// 180° rotational symmetry: every obstacle mirrors to another
		// (compare with tolerance: 100-x is not bit-exact in float64).
		for _, o := range m.Obstacles {
			found := false
			for _, o2 := range m.Obstacles {
				if math.Abs(o2.X-(FieldWidth-o.X)) < 1e-9 &&
					math.Abs(o2.Y-(FieldHeight-o.Y)) < 1e-9 &&
					o2.R == o.R {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("seed %d: obstacle %+v has no mirror", seed, o)
			}
		}
	}
}

func TestGenerateMapFallbackOnImpossibleParams(t *testing.T) {
	// Radii so big nothing fits: generator must fall back, not hang.
	p := MapParams{
		MinObstacles: 4, MaxObstacles: 6,
		MinRadius: 25, MaxRadius: 30,
		SpawnClearance: 3, ObstacleGap: 1, PassabilityCell: 1, MaxAttempts: 50,
	}
	m := GenerateMap(1, p)
	fb := fallbackObstacles()
	if len(m.Obstacles) != len(fb) {
		t.Fatalf("expected fallback with %d obstacles, got %d", len(fb), len(m.Obstacles))
	}
	for i := range fb {
		if m.Obstacles[i] != fb[i] {
			t.Fatalf("fallback obstacle %d differs", i)
		}
	}
	if err := ValidateMap(m.Obstacles, DefaultMapParams()); err != nil {
		t.Fatalf("fallback map invalid: %v", err)
	}
}

func TestValidateMapRejectsBadMaps(t *testing.T) {
	p := DefaultMapParams()
	cases := map[string][]Obstacle{
		"out of field": {{X: 99, Y: 30, R: 5}, {X: 50, Y: 30, R: 2}},
		"overlapping":  {{X: 30, Y: 30, R: 5}, {X: 33, Y: 30, R: 5}},
		"on spawn":     {{X: 10, Y: 12, R: 4}, {X: 50, Y: 30, R: 2}},
		"zero radius":  {{X: 30, Y: 30, R: 0}, {X: 70, Y: 30, R: 2}},
	}
	for name, obs := range cases {
		if err := ValidateMap(obs, p); err == nil {
			t.Errorf("%s: accepted, want error", name)
		}
	}
}

func TestSpawnFightersMirrored(t *testing.T) {
	fs := spawnFighters()
	if len(fs) != 6 {
		t.Fatalf("fighters = %d, want 6", len(fs))
	}
	alive := 0
	for _, f := range fs {
		if f.Alive {
			alive++
		}
		if f.Side == SideA && (f.X > FieldWidth/2) {
			t.Errorf("side A fighter at x=%v is not on the left half", f.X)
		}
		if f.Side == SideB && (f.X < FieldWidth/2) {
			t.Errorf("side B fighter at x=%v is not on the right half", f.X)
		}
	}
	if alive != 6 {
		t.Errorf("alive = %d, want 6", alive)
	}
	fmt.Println("spawn layout:", fs)
}
