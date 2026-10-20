package expr

import (
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	cases := []struct{ input, want string }{
		{"x", "x"},
		{"1", "1"},
		{"2.5", "2.5"},
		{".5", "0.5"},
		{"7.", "7"},
		{"pi", "pi"},
		{"e", "e"},
		{"-x", "-x"},
		{"+x", "x"},
		{"2+3", "(2+3)"},
		{"2-3-4", "((2-3)-4)"},
		{"2+3*4", "(2+(3*4))"},
		{"(2+3)*4", "((2+3)*4)"},
		{"2*3/4", "((2*3)/4)"},
		{"-x^2", "-(x^2)"},
		{"(-x)^2", "((-x)^2)"},
		{"2^3^2", "(2^(3^2))"},
		{"2^-3", "(2^-3)"},
		{"2^+3", "(2^3)"},
		{"sin(x)", "sin(x)"},
		{"sin(x)^2", "(sin(x)^2)"},
		{"sin(cos(x))", "sin(cos(x))"},
		{"abs(-3)", "abs(-3)"},
		{"ln(x+1)", "ln((x+1))"},
		{"sqrt(x)", "sqrt(x)"},
		{"exp(x)", "exp(x)"},
		{"pi*x", "(pi*x)"},
		{"2*e", "(2*e)"},
		{" x + 1 ", "(x+1)"},
		{"SIN(X)", "sin(x)"},
		{"1/(x-0.5)", "(1/(x-0.5))"},
	}
	for _, c := range cases {
		n, err := Parse(c.input)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", c.input, err)
			continue
		}
		if got := String(n); got != c.want {
			t.Errorf("Parse(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		input string
		pos   int // expected error position, 0 = whole input
	}{
		{"", 0},
		{"   ", 0},
		{"2x", 2},     // implicit multiplication
		{"2(x+1)", 2}, // implicit multiplication
		{"x pi", 3},   // implicit multiplication
		{"sin x", 5},  // function without parens
		{"sin(x", 1},  // unclosed call paren
		{"(x+1", 1},   // unclosed paren
		{"x)", 2},     // trailing junk
		{"1 2", 3},    // two numbers
		{"2..3", 3},   // double dot
		{".", 1},      // bare dot
		{"y", 1},      // unknown identifier
		{"tan(x)", 1}, // function outside the allowed list
		{"x!", 2},     // bad char
		{"x @ y", 3},  // bad char
		{"*", 1},      // dangling operator
		{"1+", 0},     // trailing operator
		{"()", 2},     // empty parens
		{"e5", 2},     // implicit multiplication, not scientific notation
		{"1e5", 2},    // scientific notation is not the contract
	}
	for _, c := range cases {
		n, err := Parse(c.input)
		if err == nil {
			t.Errorf("Parse(%q) = %q, want error", c.input, String(n))
			continue
		}
		ee, ok := err.(*ExprError)
		if !ok {
			t.Errorf("Parse(%q) error type = %T, want *ExprError", c.input, err)
			continue
		}
		if ee.Pos != c.pos {
			t.Errorf("Parse(%q) error pos = %d (%s), want %d", c.input, ee.Pos, ee.Msg, c.pos)
		}
	}
}

func TestParseLimits(t *testing.T) {
	long := strings.Repeat("1+", 60) + "1" // 121 chars, 121 nodes
	if _, err := Parse(long); err == nil {
		t.Error("Parse(long) = nil error, want node/length limit error")
	}

	// A chain of ^ nests unary recursion: 20 operators = depth 21.
	deep := "2" + strings.Repeat("^2", 20)
	if _, err := Parse(deep); err == nil {
		t.Error("Parse(deep ^-chain) = nil error, want depth limit error")
	}

	// Parentheses alone do not grow the AST: deep parens around x parse fine.
	parens := strings.Repeat("(", 20) + "x" + strings.Repeat(")", 20)
	if _, err := Parse(parens); err != nil {
		t.Errorf("Parse(deep parens) error = %v, want ok (AST depth is 1)", err)
	}

	// A flat chain has AST depth = node count, so depth ≤16 caps it:
	// 15 nodes parse, 17 do not.
	flat15 := strings.Repeat("1+", 14) + "1" // 15 nodes, depth 15
	if _, err := Parse(flat15); err != nil {
		t.Errorf("Parse(flat 15) error = %v, want ok", err)
	}
	flat17 := strings.Repeat("1+", 16) + "1" // 17 nodes, depth 17
	if _, err := Parse(flat17); err == nil {
		t.Error("Parse(flat 17) = nil error, want depth limit error")
	}

	// 49 nodes with depth ~6 fit all limits together: balanced tree of
	// 25 terms (nodes include atoms) is accepted.
	ok49 := balanced(25)
	if _, err := Parse(ok49); err != nil {
		t.Errorf("Parse(balanced 49 nodes, %d chars) error = %v, want ok", len(ok49), err)
	}

	// Just over the input-length bound is rejected without lexing.
	if _, err := Parse(strings.Repeat("x", MaxInputLen+1)); err == nil {
		t.Error("Parse(101 x) = nil error, want length error")
	}
	// Exactly at the bound: 100 chars, 15 nodes, trailing spaces allowed.
	atBound := strings.Repeat("1.55+", 7) + "1.55" + strings.Repeat(" ", 61)
	if len(atBound) != MaxInputLen {
		t.Fatalf("atBound length = %d, want %d", len(atBound), MaxInputLen)
	}
	if _, err := Parse(atBound); err != nil {
		t.Errorf("Parse(100 chars) error = %v, want ok", err)
	}
}

// balanced builds a balanced sum of n terms "(a+b)+(c+d)...".
func balanced(n int) string {
	if n == 1 {
		return "1"
	}
	return "(" + balanced(n/2) + "+" + balanced(n-n/2) + ")"
}

// TestParseNoPanics throws deterministic garbage at the parser: no input
// may panic, only return an error.
func TestParseNoPanics(t *testing.T) {
	alphabet := []rune("0123456789.xpie+-*/^() sin cos ть Ω ")
	seed := uint64(20261020)
	next := func() uint64 { // xorshift
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		return seed
	}
	for i := 0; i < 2000; i++ {
		n := int(next()%40) + 1
		runes := make([]rune, n)
		for j := range runes {
			runes[j] = alphabet[next()%uint64(len(alphabet))]
		}
		input := string(runes)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parse(%q) panicked: %v", input, r)
				}
			}()
			_, _ = Parse(input)
		}()
	}
}
