package expr

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestEvalValues(t *testing.T) {
	cases := []struct {
		input string
		x     float64
		want  float64
	}{
		{"2+3*4", 0, 14},
		{"(2+3)*4", 0, 20},
		{"2^3^2", 0, 512},
		{"-x^2", 2, -4},
		{"(-x)^2", 2, 4},
		{"2^-3", 0, 0.125},
		{"x", 2.5, 2.5},
		{".5+x", 1, 1.5},
		{"pi", 0, math.Pi},
		{"e", 0, math.E},
		{"sin(pi)", 0, 0}, // ~1.2e-16, tolerance below
		{"cos(0)", 0, 1},
		{"abs(-3)", 0, 3},
		{"ln(e)", 0, 1},
		{"sqrt(4)", 0, 2},
		{"exp(0)", 0, 1},
		{"ln(x+1)", 0, 0},
		{"sqrt(x)", 9, 3},
		{"0^0", 0, 1},
		{"(-2)^3", 0, -8},
		{"2^x", 10, 1024},
		{"x/2", 1, 0.5},
		{"1/(x-0.5)", 2, 1.0 / 1.5},
	}
	for _, c := range cases {
		n, err := Parse(c.input)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.input, err)
			continue
		}
		got, err := Eval(context.Background(), n, c.x)
		if err != nil {
			t.Errorf("Eval(%q, x=%v) error = %v", c.input, c.x, err)
			continue
		}
		if math.Abs(got-c.want) > 1e-9 && math.Abs(got-c.want) > 1e-9*math.Abs(c.want) {
			t.Errorf("Eval(%q, x=%v) = %v, want %v", c.input, c.x, got, c.want)
		}
	}
}

func TestEvalDomainErrors(t *testing.T) {
	cases := []struct {
		input string
		x     float64
	}{
		{"1/0", 0},          // division by zero
		{"1/(x-x)", 5},      // 1/0 through x
		{"ln(x)", 0},        // ln(0) -> -Inf
		{"ln(x)", -1},       // ln(negative) -> NaN
		{"sqrt(x)", -1},     // sqrt(negative) -> NaN
		{"(-8)^(1/3)", 0},   // fractional power of negative base
		{"(-2)^x", 0.5},     // NaN even though (-2)^3 works
		{"10^1000", 0},      // overflow to +Inf
		{"exp(x)", 10000},   // overflow to +Inf
		{"sin(1/x)", 0},     // 1/0 -> +Inf -> sin(+Inf) -> NaN
		{"0^(-1)", 0},       // +Inf
		{"x^x^x^x^x^x", 10}, // towers overflow
	}
	for _, c := range cases {
		n, err := Parse(c.input)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.input, err)
			continue
		}
		v, err := Eval(context.Background(), n, c.x)
		if !errors.Is(err, ErrDomain) {
			t.Errorf("Eval(%q, x=%v) = (%v, %v), want ErrDomain", c.input, c.x, v, err)
		}
		if v != 0 {
			t.Errorf("Eval(%q, x=%v) value = %v on error, want 0", c.input, c.x, v)
		}
	}
}

func TestEvalDomainIsErrorNotSentinelOnly(t *testing.T) {
	n, _ := Parse("ln(x)")
	_, err := Eval(context.Background(), n, 0)
	if err == nil || err.Error() == "" {
		t.Fatal("Eval(ln(x), 0) error message is empty")
	}
}

// TestEvalBudgetUnreachableViaParse proves the op budget exists but
// cannot be tripped by parsed input (nodes cap at 64).
func TestEvalBudgetUnreachableViaParse(t *testing.T) {
	// Hand-built 2000-node left chain: parser would reject it, the
	// evaluator's budget must catch it anyway.
	var root Node = &Num{Value: 1}
	for i := 0; i < 2000; i++ {
		root = &Binary{Op: "+", L: root, R: &Num{Value: 1}}
	}
	if _, err := Eval(context.Background(), root, 1); !errors.Is(err, ErrBudget) {
		t.Errorf("Eval(oversized AST) err = %v, want ErrBudget", err)
	}
}

func TestEvalContextCancel(t *testing.T) {
	n, _ := Parse("x+1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Eval(ctx, n, 1); !errors.Is(err, ErrBudget) {
		t.Errorf("Eval(canceled ctx) err = %v, want ErrBudget wrapping cancellation", err)
	}
}

func TestEvalDeterministic(t *testing.T) {
	n, _ := Parse("sin(x)^2 + x")
	a, err1 := Eval(context.Background(), n, 1.7)
	b, err2 := Eval(context.Background(), n, 1.7)
	if err1 != nil || err2 != nil {
		t.Fatalf("eval errors: %v %v", err1, err2)
	}
	if a != b {
		t.Errorf("same input, different results: %v vs %v", a, b)
	}
}
