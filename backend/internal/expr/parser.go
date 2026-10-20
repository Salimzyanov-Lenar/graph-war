package expr

import "fmt"

// Parse turns a formula string into an AST or returns an ExprError with
// the offending position. Input longer than MaxInputLen is rejected
// before any lexing work.
func Parse(input string) (Node, error) {
	if len(input) > MaxInputLen {
		return nil, &ExprError{0, fmt.Sprintf("формула длиннее %d символов", MaxInputLen)}
	}
	toks, err := lex(input)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	if len(toks) == 0 {
		return nil, &ExprError{0, "пустая формула"}
	}
	n, err := p.expr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.toks) {
		return nil, &ExprError{p.toks[p.pos].pos, "лишний текст после формулы"}
	}
	if p.nodes > MaxNodes {
		return nil, &ExprError{0, fmt.Sprintf("слишком сложная формула: более %d узлов", MaxNodes)}
	}
	if astDepth(n) > MaxDepth {
		return nil, &ExprError{0, fmt.Sprintf("слишком глубокая вложенность: более %d", MaxDepth)}
	}
	return n, nil
}

type parser struct {
	toks  []token
	pos   int
	nodes int
}

func (p *parser) next() *token {
	if p.pos >= len(p.toks) {
		return nil
	}
	t := &p.toks[p.pos]
	p.pos++
	return t
}

func (p *parser) peek() *token {
	if p.pos >= len(p.toks) {
		return nil
	}
	return &p.toks[p.pos]
}

// addNode registers one AST node, enforcing MaxNodes. Parentheses do not
// create nodes; nesting depth is checked on the finished AST (astDepth).
func (p *parser) addNode() error {
	p.nodes++
	if p.nodes > MaxNodes {
		return &ExprError{0, fmt.Sprintf("слишком сложная формула: более %d узлов", MaxNodes)}
	}
	return nil
}

// astDepth returns the nesting level of the AST: a lone atom is 1.
// Parentheses do not grow the AST.
func astDepth(n Node) int {
	switch v := n.(type) {
	case *Num, *Var, *Const:
		return 1
	case *Unary:
		return 1 + astDepth(v.X)
	case *Binary:
		l, r := astDepth(v.L), astDepth(v.R)
		if r > l {
			l = r
		}
		return 1 + l
	case *Call:
		return 1 + astDepth(v.Arg)
	default:
		return 0
	}
}

// expr := term (('+'|'-') term)*
func (p *parser) expr() (Node, error) {
	left, err := p.term()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t == nil || t.kind != tokOp || (t.text != "+" && t.text != "-") {
			return left, nil
		}
		p.next()
		right, err := p.term()
		if err != nil {
			return nil, err
		}
		if err := p.addNode(); err != nil {
			return nil, err
		}
		left = &Binary{Op: t.text, L: left, R: right}
	}
}

// term := unary (('*'|'/') unary)*
func (p *parser) term() (Node, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t == nil || t.kind != tokOp || (t.text != "*" && t.text != "/") {
			return left, nil
		}
		p.next()
		right, err := p.unary()
		if err != nil {
			return nil, err
		}
		if err := p.addNode(); err != nil {
			return nil, err
		}
		left = &Binary{Op: t.text, L: left, R: right}
	}
}

// unary := ('+'|'-') unary | power
// Unary minus binds looser than ^: -x^2 == -(x^2).
func (p *parser) unary() (Node, error) {
	t := p.peek()
	if t != nil && t.kind == tokOp && (t.text == "+" || t.text == "-") {
		p.next()
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		if t.text == "+" {
			return x, nil
		}
		if err := p.addNode(); err != nil {
			return nil, err
		}
		return &Unary{Op: "-", X: x}, nil
	}
	return p.power()
}

// power := atom ('^' unary)?   — right-associative via unary recursion,
// so 2^3^2 parses as 2^(3^2); the exponent may carry a sign (2^-3).
func (p *parser) power() (Node, error) {
	base, err := p.atom()
	if err != nil {
		return nil, err
	}
	t := p.peek()
	if t != nil && t.kind == tokOp && t.text == "^" {
		p.next()
		exp, err := p.unary()
		if err != nil {
			return nil, err
		}
		if err := p.addNode(); err != nil {
			return nil, err
		}
		return &Binary{Op: "^", L: base, R: exp}, nil
	}
	return base, nil
}

// atom := number | 'x' | constant | func '(' expr ')' | '(' expr ')'
// Implicit multiplication is rejected here: "2x" and "sin x" are errors.
func (p *parser) atom() (Node, error) {
	t := p.next()
	if t == nil {
		return nil, &ExprError{0, "неожиданный конец формулы"}
	}

	switch t.kind {
	case tokNumber:
		if err := p.rejectAdjacent(); err != nil {
			return nil, err
		}
		if err := p.addNode(); err != nil {
			return nil, err
		}
		return &Num{Value: t.num}, nil

	case tokIdent:
		if functions[t.text] {
			if lp := p.peek(); lp == nil || lp.kind != tokLParen {
				return nil, &ExprError{offenderPos(p, t), fmt.Sprintf("%s требует скобки: %s(x)", t.text, t.text)}
			}
			p.next()
			arg, err := p.expr()
			if err != nil {
				return nil, err
			}
			if rp := p.peek(); rp == nil || rp.kind != tokRParen {
				return nil, &ExprError{t.pos, "не закрыта скобка функции " + t.text}
			}
			p.next()
			if err := p.addNode(); err != nil {
				return nil, err
			}
			return &Call{Name: t.text, Arg: arg}, nil
		}
		if constants[t.text] {
			if err := p.rejectAdjacent(); err != nil {
				return nil, err
			}
			if err := p.addNode(); err != nil {
				return nil, err
			}
			return &Const{Name: t.text}, nil
		}
		if t.text == "x" {
			if err := p.rejectAdjacent(); err != nil {
				return nil, err
			}
			if err := p.addNode(); err != nil {
				return nil, err
			}
			return &Var{}, nil
		}
		return nil, &ExprError{t.pos, fmt.Sprintf("неизвестное имя %q", t.text)}

	case tokLParen:
		inner, err := p.expr()
		if err != nil {
			return nil, err
		}
		if rp := p.peek(); rp == nil || rp.kind != tokRParen {
			return nil, &ExprError{t.pos, "не закрыта скобка"}
		}
		p.next()
		if err := p.rejectAdjacent(); err != nil {
			return nil, err
		}
		return inner, nil

	default:
		return nil, &ExprError{t.pos, fmt.Sprintf("неожиданный символ %q", t.text)}
	}
}

// offenderPos points at the token that broke the expectation (the
// operand where '(' was required), or at end-of-input.
func offenderPos(p *parser, fallback *token) int {
	if nxt := p.peek(); nxt != nil {
		return nxt.pos
	}
	return fallback.pos
}

// rejectAdjacent fires when a value atom is directly followed by another
// value atom or '(' — the contract's ban on implicit multiplication
// ("2x", "2(x+1)", "x pi"), which silently changes meaning elsewhere.
func (p *parser) rejectAdjacent() error {
	nxt := p.peek()
	if nxt == nil {
		return nil
	}
	if nxt.kind == tokIdent || nxt.kind == tokNumber || nxt.kind == tokLParen {
		return &ExprError{nxt.pos, "пропущен знак операции (неявное умножение запрещено)"}
	}
	return nil
}
