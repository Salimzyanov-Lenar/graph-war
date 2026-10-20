package expr

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// ErrDomain marks a value outside the formula's domain at this x
// (division by zero, ln/sqrt of the wrong sign, NaN, infinity).
// The trajectory layer decides whether that rejects the shot at start
// or breaks an accepted one (contract §3).
var ErrDomain = errors.New("значение вне области определения")

// ErrBudget marks a formula that exceeded its computation budget. Only
// reachable with hand-built oversized ASTs: Parse caps every input.
var ErrBudget = errors.New("превышен бюджет вычислений")

// maxOps bounds the work of one evaluation. Parse caps nodes at 64, so
// honest formulas never come close; the budget is defense in depth
// against hand-built or future ASTs.
const maxOps = 1024

// Eval computes the node at the given x on float64. Non-finite results
// and domain violations are errors, never +Inf/NaN return values:
// Go's floats would otherwise silently produce 1/0 == +Inf.
// A cancelled context aborts the walk (checked at every node).
func Eval(ctx context.Context, n Node, x float64) (float64, error) {
	e := &eval{ctx: ctx}
	v, err := e.eval(n, x)
	if err != nil {
		return 0, err
	}
	if !finite(v) {
		return 0, fmt.Errorf("%w: не-конечный результат %v", ErrDomain, v)
	}
	return v, nil
}

type eval struct {
	ctx context.Context
	ops int
}

var errCanceled = errors.New("вычисление отменено")

func (e *eval) eval(n Node, x float64) (float64, error) {
	if err := e.ctx.Err(); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrBudget, errCanceled)
	}
	e.ops++
	if e.ops > maxOps {
		return 0, fmt.Errorf("%w: более %d операций", ErrBudget, maxOps)
	}

	switch v := n.(type) {
	case *Num:
		return v.Value, nil
	case *Var:
		return x, nil
	case *Const:
		if v.Name == "pi" {
			return math.Pi, nil
		}
		return math.E, nil

	case *Unary:
		a, err := e.eval(v.X, x)
		if err != nil {
			return 0, err
		}
		return -a, nil

	case *Binary:
		l, err := e.eval(v.L, x)
		if err != nil {
			return 0, err
		}
		r, err := e.eval(v.R, x)
		if err != nil {
			return 0, err
		}
		return e.binary(v.Op, l, r)

	case *Call:
		a, err := e.eval(v.Arg, x)
		if err != nil {
			return 0, err
		}
		return e.call(v.Name, a)

	default:
		return 0, fmt.Errorf("неизвестный узел AST")
	}
}

// binary applies one operation with the contract's domain rules:
// 0^0 == 1; negative base with a fractional exponent is a domain error;
// a non-finite intermediate (overflow, 0/0) is a domain error too.
func (e *eval) binary(op string, l, r float64) (float64, error) {
	var res float64
	switch op {
	case "+":
		res = l + r
	case "-":
		res = l - r
	case "*":
		res = l * r
	case "/":
		if r == 0 {
			return 0, fmt.Errorf("%w: деление на ноль", ErrDomain)
		}
		res = l / r
	case "^":
		if l < 0 && r != math.Trunc(r) {
			return 0, fmt.Errorf("%w: отрицательное основание в дробной степени", ErrDomain)
		}
		res = math.Pow(l, r)
	default:
		return 0, fmt.Errorf("неизвестная операция %q", op)
	}
	if !finite(res) {
		return 0, fmt.Errorf("%w: %s %v %v переполнилось", ErrDomain, op, l, r)
	}
	return res, nil
}

func (e *eval) call(name string, a float64) (float64, error) {
	var res float64
	switch name {
	case "sin":
		res = math.Sin(a)
	case "cos":
		res = math.Cos(a)
	case "sqrt":
		if a < 0 {
			return 0, fmt.Errorf("%w: sqrt от отрицательного", ErrDomain)
		}
		res = math.Sqrt(a)
	case "abs":
		res = math.Abs(a)
	case "ln":
		if a <= 0 {
			return 0, fmt.Errorf("%w: ln от неположительного", ErrDomain)
		}
		res = math.Log(a)
	case "exp":
		res = math.Exp(a)
	default:
		return 0, fmt.Errorf("неизвестная функция %q", name)
	}
	if !finite(res) {
		return 0, fmt.Errorf("%w: %s(%v) не-конечно", ErrDomain, name, a)
	}
	return res, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
