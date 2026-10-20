package expr

import (
	"fmt"
	"strings"
	"unicode"
)

type tokenKind int

const (
	tokNumber tokenKind = iota
	tokIdent
	tokOp
	tokLParen
	tokRParen
)

type token struct {
	kind tokenKind
	text string // identifier name or operator sign
	pos  int    // 1-based rune position
	num  float64
}

// lex splits the input into tokens. It accepts only what the contract
// allows: decimal digits with a dot, identifiers, single-char operators.
func lex(input string) ([]token, error) {
	runes := []rune(input)
	var toks []token
	i := 0
	for i < len(runes) {
		r := runes[i]
		pos := i + 1
		switch {
		case r == ' ' || r == '\t':
			i++
		case r == '(':
			toks = append(toks, token{tokLParen, "(", pos, 0})
			i++
		case r == ')':
			toks = append(toks, token{tokRParen, ")", pos, 0})
			i++
		case r == '+' || r == '-' || r == '*' || r == '/' || r == '^':
			toks = append(toks, token{tokOp, string(r), pos, 0})
			i++
		case unicode.IsDigit(r) || r == '.':
			t, next, err := lexNumber(runes, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, t)
			i = next
		case unicode.IsLetter(r):
			j := i
			for j < len(runes) && unicode.IsLetter(runes[j]) {
				j++
			}
			name := strings.ToLower(string(runes[i:j]))
			toks = append(toks, token{tokIdent, name, pos, 0})
			i = j
		default:
			return nil, &ExprError{pos, fmt.Sprintf("недопустимый символ %q", r)}
		}
	}
	return toks, nil
}

// lexNumber reads digits with an optional single dot. ".5", "1.", "1.5"
// are accepted; "1.2.3" and a bare dot are errors.
func lexNumber(runes []rune, start int) (token, int, error) {
	i := start
	dotSeen := false
	for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
		if runes[i] == '.' {
			if dotSeen {
				return token{}, 0, &ExprError{i + 1, "лишняя точка в числе"}
			}
			dotSeen = true
		}
		i++
	}
	text := string(runes[start:i])
	if text == "." {
		return token{}, 0, &ExprError{start + 1, "точка без цифр"}
	}
	var v float64
	if _, err := fmt.Sscanf(text, "%g", &v); err != nil {
		return token{}, 0, &ExprError{start + 1, "не удалось прочитать число " + text}
	}
	return token{tokNumber, text, start + 1, v}, i, nil
}
