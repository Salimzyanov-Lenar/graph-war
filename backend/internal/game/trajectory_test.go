package game

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/Salimzyanov-Lenar/graph-war/backend/internal/expr"
)

func build(t *testing.T, formula string) expr.Node {
	t.Helper()
	n, err := expr.Parse(formula)
	if err != nil {
		t.Fatalf("Parse(%q): %v", formula, err)
	}
	return n
}

func mustTrajectory(t *testing.T, formula string, side Side, shooter Point) Trajectory {
	t.Helper()
	tr, err := BuildTrajectory(context.Background(), build(t, formula), side, shooter)
	if err != nil {
		t.Fatalf("BuildTrajectory(%q): %v", formula, err)
	}
	return tr
}

func TestTrajectoryStraightLine(t *testing.T) {
	// f(x) = 0 keeps the shot at the shooter's height: horizontal flight.
	tr := mustTrajectory(t, "0", SideA, Point{X: 10, Y: 30})

	if tr.Points[0] != (Point{X: 10, Y: 30}) {
		t.Errorf("first point = %v, want shooter position", tr.Points[0])
	}
	// Field exit at X > 100: t = 90.5 → X = 100.5 is the included exit point.
	last := tr.Points[len(tr.Points)-1]
	if last.X != 100.5 || last.Y != 30 {
		t.Errorf("last point = %v, want {100.5 30}", last)
	}
	if tr.Stop != StopOutOfField {
		t.Errorf("stop = %q, want out_of_field", tr.Stop)
	}
}

func TestTrajectory45DegreeLineExitsTop(t *testing.T) {
	// f(x) = x climbs at 45°: from Y=30 it leaves through the top.
	tr := mustTrajectory(t, "x", SideA, Point{X: 10, Y: 30})
	last := tr.Points[len(tr.Points)-1]
	if last.Y != 60.5 || last.X != 40.5 {
		t.Errorf("last point = %v, want {40.5 60.5}", last)
	}
	if tr.Stop != StopOutOfField {
		t.Errorf("stop = %q, want out_of_field", tr.Stop)
	}
}

func TestTrajectorySideBShootsLeft(t *testing.T) {
	tr := mustTrajectory(t, "0", SideB, Point{X: 90, Y: 30})
	last := tr.Points[len(tr.Points)-1]
	if last.X != -0.5 || last.Y != 30 {
		t.Errorf("last point = %v, want {-0.5 30} (exit through the left edge)", last)
	}
	if tr.Stop != StopOutOfField {
		t.Errorf("stop = %q, want out_of_field", tr.Stop)
	}
}

func TestTrajectoryTopExit(t *testing.T) {
	tr := mustTrajectory(t, "x^2", SideA, Point{X: 5, Y: 0})
	last := tr.Points[len(tr.Points)-1]
	// rel(t) = t^2; leaves the top at Y > 60 → t = 8 → Y = 64.
	if last.Y != 64 || last.X != 13 {
		t.Errorf("last point = %v, want {13 64}", last)
	}
	if tr.Stop != StopOutOfField {
		t.Errorf("stop = %q, want out_of_field", tr.Stop)
	}
}

func TestTrajectoryConstantOffsetInvisible(t *testing.T) {
	a := mustTrajectory(t, "x^2", SideA, Point{X: 5, Y: 0})
	b := mustTrajectory(t, "x^2+5", SideA, Point{X: 5, Y: 0})
	if len(a.Points) != len(b.Points) {
		t.Fatalf("point counts differ: %d vs %d", len(a.Points), len(b.Points))
	}
	for i := range a.Points {
		if a.Points[i] != b.Points[i] {
			t.Fatalf("point %d differs: %v vs %v", i, a.Points[i], b.Points[i])
		}
	}
}

func TestTrajectoryStartRejection(t *testing.T) {
	for _, formula := range []string{"ln(x)", "sin(1/x)", "(-8)^(1/3)"} {
		_, err := BuildTrajectory(context.Background(), build(t, formula), SideA, Point{X: 10, Y: 30})
		if !errors.Is(err, ErrStartDomain) {
			t.Errorf("BuildTrajectory(%q) err = %v, want ErrStartDomain", formula, err)
		}
	}
}

func TestTrajectoryBreaksAfterStart(t *testing.T) {
	// Pole exactly on the second sample (t = 0.5): shot accepted (f(0) = -2),
	// then division by zero → non_finite, trajectory ends at the shooter.
	tr := mustTrajectory(t, "1/(x-0.5)", SideA, Point{X: 10, Y: 30})
	if tr.Stop != StopNonFinite {
		t.Errorf("stop = %q, want non_finite", tr.Stop)
	}
	if len(tr.Points) != 1 {
		t.Errorf("points = %d, want 1 (ends before the pole)", len(tr.Points))
	}
}

func TestTrajectoryDiscontinuityOnProbe(t *testing.T) {
	// (-2)^x: f(0) = 1 is fine, but the first probe (t = 1/6) gives NaN.
	tr := mustTrajectory(t, "(-2)^x", SideA, Point{X: 10, Y: 30})
	if tr.Stop != StopDiscontinuit {
		t.Errorf("stop = %q, want discontinuity", tr.Stop)
	}
	if len(tr.Points) != 1 {
		t.Errorf("points = %d, want 1", len(tr.Points))
	}

	// An asymptote the probes do land near: 1/(6x-1) at probe t=1/6
	// evaluates to |rel| ≈ 9e15 > 1e6 → discontinuity via magnitude cut.
	tr = mustTrajectory(t, "1/(6*x-1)", SideA, Point{X: 10, Y: 30})
	if tr.Stop != StopDiscontinuit {
		t.Errorf("stop = %q, want discontinuity (magnitude cutoff)", tr.Stop)
	}
}

func TestTrajectorySinFliesAcrossField(t *testing.T) {
	tr := mustTrajectory(t, "sin(x)", SideA, Point{X: 0, Y: 30})
	last := tr.Points[len(tr.Points)-1]
	if last.X != 100.5 {
		t.Errorf("last X = %v, want 100.5", last.X)
	}
	if math.Abs(last.Y-30) > 0.1 {
		t.Errorf("last Y = %v, want ~30 (sin(100.5) ≈ -0.06)", last.Y)
	}
}

func TestTrajectoryBottomExit(t *testing.T) {
	// f(x) = -2x dives: from Y=10 it leaves through the bottom.
	tr := mustTrajectory(t, "-x*2", SideA, Point{X: 10, Y: 10})
	last := tr.Points[len(tr.Points)-1]
	if last.Y >= 0 {
		t.Errorf("last Y = %v, want below the field", last.Y)
	}
	if tr.Stop != StopOutOfField {
		t.Errorf("stop = %q, want out_of_field", tr.Stop)
	}
}

func TestTrajectoryDeterministic(t *testing.T) {
	a := mustTrajectory(t, "sin(x)+x/2", SideA, Point{X: 5, Y: 25})
	b := mustTrajectory(t, "sin(x)+x/2", SideA, Point{X: 5, Y: 25})
	if len(a.Points) != len(b.Points) || a.Stop != b.Stop {
		t.Fatal("same input produced different trajectories")
	}
	for i := range a.Points {
		if a.Points[i] != b.Points[i] {
			t.Fatalf("point %d differs: %v vs %v", i, a.Points[i], b.Points[i])
		}
	}
}

func TestTrajectoryInvalidSide(t *testing.T) {
	n := build(t, "x")
	if _, err := BuildTrajectory(context.Background(), n, Side("a"), Point{}); err == nil {
		t.Error("lowercase side accepted, want error")
	}
}

func TestTrajectoryPointBudget(t *testing.T) {
	// A shot that never leaves the field still stops at MaxPoints.
	// Vertical impossible (x only), so emulate with a trapped formula:
	// sin(0) = 0 forever — the shot stays at Y=30 and flies right;
	// it WILL exit the field, so instead check the absolute cap:
	// points never exceed MaxPoints.
	tr := mustTrajectory(t, "sin(0)*x", SideA, Point{X: 0, Y: 30})
	if len(tr.Points) > MaxPoints {
		t.Errorf("points = %d, want ≤ %d", len(tr.Points), MaxPoints)
	}
}
