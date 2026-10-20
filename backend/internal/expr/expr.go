// Package expr parses the limited formula language of Graphwar shots
// into an AST. It never evaluates anything and never executes user code.
// Grammar and limits are fixed by docs/contract.md §2.
package expr

import "fmt"

// ExprError points at the offending rune in the input.
type ExprError struct {
	Pos int // 1-based rune index; 0 = whole input
	Msg string
}

func (e *ExprError) Error() string {
	if e.Pos == 0 {
		return e.Msg
	}
	return fmt.Sprintf("%s (позиция %d)", e.Msg, e.Pos)
}

// Node is a parsed formula. Only these forms exist; the parser rejects
// anything else, so evaluation later can trust the shape.
type Node interface{ node() }

// Num is a numeric literal.
type Num struct{ Value float64 }

// Var is the variable x.
type Var struct{}

// Const is a named constant: pi or e.
type Const struct{ Name string }

// Unary is a sign: +e or -e.
type Unary struct {
	Op string // "+" | "-"
	X  Node
}

// Binary is an arithmetic operation: + - * / ^.
type Binary struct {
	Op   string
	L, R Node
}

// Call is a function application: sin cos sqrt abs ln exp.
type Call struct {
	Name string
	Arg  Node
}

func (*Num) node()    {}
func (*Var) node()    {}
func (*Const) node()  {}
func (*Unary) node()  {}
func (*Binary) node() {}
func (*Call) node()   {}

// Limits from contract §2; enforced during parsing, before any evaluation.
const (
	MaxInputLen = 100
	MaxNodes    = 64
	MaxDepth    = 16
)

// Functions allowed by the contract. Trigonometric arguments are radians.
var functions = map[string]bool{
	"sin": true, "cos": true, "sqrt": true, "abs": true, "ln": true, "exp": true,
}

var constants = map[string]bool{"pi": true, "e": true}

// String renders the AST back to a canonical form; used in tests and logs.
func String(n Node) string {
	switch v := n.(type) {
	case *Num:
		return formatNum(v.Value)
	case *Var:
		return "x"
	case *Const:
		return v.Name
	case *Unary:
		return v.Op + String(v.X)
	case *Binary:
		return "(" + renderOperand(v.L) + v.Op + String(v.R) + ")"
	case *Call:
		return v.Name + "(" + String(v.Arg) + ")"
	default:
		return "<?>"
	}
}

func formatNum(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%g", v)
}

// renderOperand parenthesizes a unary child: (-x)^2 must not render as
// the ambiguous "-x^2".
func renderOperand(n Node) string {
	if _, ok := n.(*Unary); ok {
		return "(" + String(n) + ")"
	}
	return String(n)
}
