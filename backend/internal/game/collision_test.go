package game

import (
	"context"
	"testing"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/expr"
)

// Horizontal shot along Y=30: f(x)=0 keeps every sample at the height
// of the shooter. Points land on X = 10, 10.5, 11, …
func shot(t *testing.T, bg Battleground, shooterID string) ShotResult {
	t.Helper()
	ast, err := expr.Parse("0")
	if err != nil {
		t.Fatal(err)
	}
	res, err := ResolveShot(context.Background(), ast, SideA, Point{X: 10, Y: 30}, shooterID, bg)
	if err != nil {
		t.Fatalf("ResolveShot: %v", err)
	}
	return res
}

func TestResolveShotMiss(t *testing.T) {
	res := shot(t, Battleground{}, "a1")
	if res.Stop != StopOutOfField {
		t.Errorf("stop = %q, want out_of_field", res.Stop)
	}
	if res.DestroyedFighterID != "" {
		t.Errorf("destroyed = %q, want none", res.DestroyedFighterID)
	}
}

func TestResolveShotObstacleOnPath(t *testing.T) {
	bg := Battleground{Obstacles: []Obstacle{{X: 50, Y: 30, R: 3}}}
	res := shot(t, bg, "a1")
	if res.Stop != StopHit {
		t.Fatalf("stop = %q, want hit", res.Stop)
	}
	if res.DestroyedFighterID != "" {
		t.Errorf("destroyed = %q, want none (obstacle)", res.DestroyedFighterID)
	}
	last := res.Points[len(res.Points)-1]
	if last.X != 47 || last.Y != 30 {
		t.Errorf("hit point = %v, want {47 30} (obstacle surface)", last)
	}
}

func TestResolveShotTangentCounts(t *testing.T) {
	// Circle touching the flight line from above at exactly (50, 30).
	bg := Battleground{Obstacles: []Obstacle{{X: 50, Y: 34, R: 4}}}
	res := shot(t, bg, "a1")
	if res.Stop != StopHit {
		t.Errorf("stop = %q, want hit (touch counts)", res.Stop)
	}
	last := res.Points[len(res.Points)-1]
	if last.X != 50 || last.Y != 30 {
		t.Errorf("hit point = %v, want {50 30}", last)
	}
}

func TestResolveShotSmallObstacleBetweenSamples(t *testing.T) {
	// Circle spans X 45.05..45.45; samples land at 45.0 and 45.5 —
	// only a segment test can see it.
	bg := Battleground{Obstacles: []Obstacle{{X: 45.25, Y: 30, R: 0.2}}}
	res := shot(t, bg, "a1")
	if res.Stop != StopHit {
		t.Fatalf("stop = %q, want hit", res.Stop)
	}
	last := res.Points[len(res.Points)-1]
	if last.X != 45.05 {
		t.Errorf("hit X = %v, want 45.05", last.X)
	}
}

func TestResolveShotTwoObstaclesEarliestWins(t *testing.T) {
	bg := Battleground{Obstacles: []Obstacle{
		{X: 70, Y: 30, R: 2},
		{X: 40, Y: 30, R: 2},
	}}
	res := shot(t, bg, "a1")
	last := res.Points[len(res.Points)-1]
	if last.X != 38 {
		t.Errorf("hit X = %v, want 38 (first obstacle)", last.X)
	}
}

func TestResolveShotFriendlyFireBeforeEnemy(t *testing.T) {
	bg := Battleground{Fighters: []Fighter{
		{ID: "b1", Side: SideB, X: 60, Y: 30, Alive: true},
		{ID: "a2", Side: SideA, X: 30, Y: 30, Alive: true},
	}}
	res := shot(t, bg, "a1")
	if res.Stop != StopHit || res.DestroyedFighterID != "a2" {
		t.Errorf("result = %q/%q, want hit/a2 (friendly fire)", res.Stop, res.DestroyedFighterID)
	}
}

func TestResolveShotShooterDoesNotCollide(t *testing.T) {
	// The shooter himself sits in the fighter list at the start point;
	// the shot must ignore him and fly on.
	bg := Battleground{
		Fighters: []Fighter{
			{ID: "a1", Side: SideA, X: 10, Y: 30, Alive: true}, // shooter
			{ID: "b1", Side: SideB, X: 60, Y: 30, Alive: true},
		},
	}
	res := shot(t, bg, "a1")
	if res.DestroyedFighterID != "b1" {
		t.Errorf("destroyed = %q, want b1 (shooter must be excluded)", res.DestroyedFighterID)
	}
}

func TestResolveShotDeadFighterNoCollision(t *testing.T) {
	bg := Battleground{
		Obstacles: []Obstacle{{X: 40, Y: 30, R: 2}},
		Fighters:  []Fighter{{ID: "b1", Side: SideB, X: 30, Y: 30, Alive: false}},
	}
	res := shot(t, bg, "a1")
	if res.DestroyedFighterID != "" {
		t.Errorf("destroyed = %q, want none (dead fighter)", res.DestroyedFighterID)
	}
	last := res.Points[len(res.Points)-1]
	if last.X != 38 {
		t.Errorf("hit X = %v, want 38 (flew past the corpse)", last.X)
	}
}

func TestResolveShotFighterRadiusSurfaceHit(t *testing.T) {
	// Fighter 0.9 below the line: within FighterRadius=1 → hit on the
	// circle surface before reaching his X.
	bg := Battleground{Fighters: []Fighter{
		{ID: "b1", Side: SideB, X: 30, Y: 30.9, Alive: true},
	}}
	res := shot(t, bg, "a1")
	if res.DestroyedFighterID != "b1" {
		t.Fatalf("destroyed = %q, want b1", res.DestroyedFighterID)
	}
	last := res.Points[len(res.Points)-1]
	if last.X >= 30 {
		t.Errorf("hit X = %v, want < 30 (surface, not center)", last.X)
	}
	if diff := last.Y - 30; diff*diff > 1e-12 {
		t.Errorf("hit Y = %v, want on the flight line", last.Y)
	}
}

func TestResolveShotOnlyOneVictim(t *testing.T) {
	bg := Battleground{Fighters: []Fighter{
		{ID: "b2", Side: SideB, X: 32, Y: 30, Alive: true},
		{ID: "b1", Side: SideB, X: 30, Y: 30, Alive: true},
	}}
	res := shot(t, bg, "a1")
	if res.DestroyedFighterID != "b1" {
		t.Errorf("destroyed = %q, want b1", res.DestroyedFighterID)
	}
	if res.Stop != StopHit {
		t.Errorf("stop = %q, want hit (shot ends at first victim)", res.Stop)
	}
}

func TestResolveShotOrderIndependent(t *testing.T) {
	makeBG := func(flip bool) Battleground {
		obs := []Obstacle{{X: 40, Y: 30, R: 2}, {X: 70, Y: 30, R: 2}}
		ftr := []Fighter{
			{ID: "b1", Side: SideB, X: 60, Y: 30, Alive: true},
			{ID: "b2", Side: SideB, X: 55, Y: 30, Alive: true},
		}
		if flip {
			obs[0], obs[1] = obs[1], obs[0]
			ftr[0], ftr[1] = ftr[1], ftr[0]
		}
		return Battleground{Obstacles: obs, Fighters: ftr}
	}
	a := shot(t, makeBG(false), "a1")
	b := shot(t, makeBG(true), "a1")
	if a.DestroyedFighterID != b.DestroyedFighterID || a.Stop != b.Stop {
		t.Fatalf("order changed outcome: %q/%q vs %q/%q", a.DestroyedFighterID, a.Stop, b.DestroyedFighterID, b.Stop)
	}
	if len(a.Points) != len(b.Points) || a.Points[len(a.Points)-1] != b.Points[len(b.Points)-1] {
		t.Fatalf("order changed hit point: %v vs %v", a.Points[len(a.Points)-1], b.Points[len(b.Points)-1])
	}
}

func TestResolveShotObstacleBeatsFighterOnlyIfEarlier(t *testing.T) {
	// Obstacle in front of the enemy: shot stops at the obstacle.
	bg := Battleground{
		Obstacles: []Obstacle{{X: 40, Y: 30, R: 2}},
		Fighters:  []Fighter{{ID: "b1", Side: SideB, X: 60, Y: 30, Alive: true}},
	}
	res := shot(t, bg, "a1")
	if res.DestroyedFighterID != "" {
		t.Errorf("destroyed = %q, want none", res.DestroyedFighterID)
	}
	last := res.Points[len(res.Points)-1]
	if last.X != 38 {
		t.Errorf("hit X = %v, want 38", last.X)
	}
}

func TestResolveShotStartsInsideObstacleIsImmediate(t *testing.T) {
	// A malformed spawn (inside an obstacle) still resolves
	// deterministically: immediate hit, no panic, no negative t.
	bg := Battleground{Obstacles: []Obstacle{{X: 10, Y: 30, R: 3}}}
	res := shot(t, bg, "a1")
	if res.Stop != StopHit {
		t.Errorf("stop = %q, want hit", res.Stop)
	}
	if len(res.Points) != 2 {
		t.Errorf("points = %d, want 2 (start + surface)", len(res.Points))
	}
}
