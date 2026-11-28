package variable

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// parsePythonLiteral evaluates s as a Python literal: a string (with
// Python's escape rules, prefixes and implicit concatenation), an int, a
// float, True, False, None, or a list, tuple or dict of literals. Python
// TTP loads <vars> with exec, so a value is whatever Python makes of it;
// this covers the literals <vars> values are written as. Tuples become
// []interface{}. It reports false if s is not such a literal.
func parsePythonLiteral(s string) (interface{}, bool) {
	p := &pyLiteralParser{s: s}
	p.skipSpace()
	v, err := p.value()
	if err != nil {
		return nil, false
	}
	p.skipSpace()
	if p.i != len(p.s) {
		return nil, false
	}
	return v, true
}

// PythonLiteral evaluates s as a Python literal (see parsePythonLiteral).
// It reports false if s is not one.
func PythonLiteral(s string) (interface{}, bool) {
	return parsePythonLiteral(s)
}

// PythonStringLiteral evaluates s as a Python string literal (quotes,
// escapes, an r or u prefix, adjacent literals). It reports false if s is
// not one.
func PythonStringLiteral(s string) (string, bool) {
	v, ok := parsePythonLiteral(s)
	if !ok {
		return "", false
	}
	str, ok := v.(string)
	return str, ok
}

type pyLiteralParser struct {
	s string
	i int
	// names reads an identifier the way TTP's get_attributes evaluates it
	// (_UndefSubst): true, false and none in any case are True, False and
	// None, and any other name is the name as a string.
	names bool
	// depth is the nesting depth of lists, tuples and dicts, bounded by
	// maxLiteralDepth.
	depth int
	// tuple is set when the last list or tuple read was a tuple.
	tuple bool
}

// maxLiteralDepth bounds how deeply lists, tuples and dicts nest, so a
// template can't exhaust the stack.
const maxLiteralDepth = 100

// PythonValueAt reads one Python value from s at byte offset i and returns
// it with the offset just after it. With names, an identifier is read the
// way TTP's get_attributes evaluates call arguments (_UndefSubst): true,
// false and none in any case are True, False and None, and any other name
// is its own text. Without names an identifier other than True, False and
// None is an error.
func PythonValueAt(s string, i int, names bool) (interface{}, int, error) {
	v, next, _, err := PythonValueTupleAt(s, i, names)
	return v, next, err
}

// PythonValueTupleAt is PythonValueAt that also reports whether the value,
// when it is a []interface{}, is a tuple rather than a list: both read as
// []interface{}, but some Python functions take only one of them.
func PythonValueTupleAt(s string, i int, names bool) (v interface{}, next int, tuple bool, err error) {
	p := &pyLiteralParser{s: s, i: i, names: names}
	p.skipSpace()
	v, err = p.value()
	_, seq := v.([]interface{})
	return v, p.i, seq && p.tuple, err
}

func (p *pyLiteralParser) skipSpace() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		case '#':
			for p.i < len(p.s) && p.s[p.i] != '\n' {
				p.i++
			}
		case '\\':
			// An explicit line join.
			if strings.HasPrefix(p.s[p.i:], "\\\n") {
				p.i += 2
				continue
			}
			return
		default:
			return
		}
	}
}

func (p *pyLiteralParser) value() (interface{}, error) {
	if p.i >= len(p.s) {
		return nil, fmt.Errorf("unexpected end")
	}
	switch c := p.s[p.i]; {
	case c == '[' || c == '(' || c == '{':
		if p.depth >= maxLiteralDepth {
			return nil, fmt.Errorf("literal nested more than %d deep", maxLiteralDepth)
		}
		p.depth++
		defer func() { p.depth-- }()
		p.i++
		switch c {
		case '[':
			return p.sequence(']')
		case '(':
			return p.sequence(')')
		}
		return p.dict()
	case c == '"' || c == '\'':
		return p.stringLiterals()
	case IdentStartAt(p.s, p.i):
		start := p.i
		p.i = IdentEnd(p.s, p.i)
		word := p.s[start:p.i]
		if p.i < len(p.s) && (p.s[p.i] == '"' || p.s[p.i] == '\'') && isStringPrefix(word) {
			p.i = start
			return p.stringLiterals()
		}
		switch word {
		case "True":
			return true, nil
		case "False":
			return false, nil
		case "None":
			return nil, nil
		}
		if p.names {
			switch strings.ToLower(word) {
			case "true":
				return true, nil
			case "false":
				return false, nil
			case "none":
				return nil, nil
			}
			return word, nil
		}
		return nil, fmt.Errorf("name %q is not a literal", word)
	case c >= utf8.RuneSelf:
		r, _ := utf8.DecodeRuneInString(p.s[p.i:])
		return nil, fmt.Errorf("unexpected %q", r)
	default:
		return p.number()
	}
}

// sequence parses list or tuple items up to end. A parenthesized value
// without a comma, (x), is the value itself, as in Python.
func (p *pyLiteralParser) sequence(end byte) (interface{}, error) {
	items := []interface{}{}
	comma := false
	for {
		p.skipSpace()
		if p.i < len(p.s) && p.s[p.i] == end {
			p.i++
			if end == ')' && len(items) == 1 && !comma {
				return items[0], nil
			}
			p.tuple = end == ')'
			return items, nil
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
		p.skipSpace()
		if p.i >= len(p.s) {
			return nil, fmt.Errorf("unterminated sequence")
		}
		if p.s[p.i] == ',' {
			p.i++
			comma = true
			continue
		}
		if p.s[p.i] != end {
			return nil, fmt.Errorf("expected %q", end)
		}
	}
}

func (p *pyLiteralParser) dict() (interface{}, error) {
	out := map[string]interface{}{}
	for {
		p.skipSpace()
		if p.i < len(p.s) && p.s[p.i] == '}' {
			p.i++
			return out, nil
		}
		k, err := p.value()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			return nil, fmt.Errorf("expected ':'")
		}
		p.i++
		p.skipSpace()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		out[fmt.Sprint(k)] = v
		p.skipSpace()
		if p.i >= len(p.s) {
			return nil, fmt.Errorf("unterminated dict")
		}
		if p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.s[p.i] != '}' {
			return nil, fmt.Errorf("expected '}'")
		}
	}
}

func (p *pyLiteralParser) number() (interface{}, error) {
	start := p.i
	if p.i < len(p.s) && (p.s[p.i] == '-' || p.s[p.i] == '+') {
		p.i++
	}
	for p.i < len(p.s) && strings.IndexByte("0123456789._eExXoObBaAcCdDfF+-", p.s[p.i]) >= 0 {
		// A sign is part of the number only right after an exponent.
		if (p.s[p.i] == '+' || p.s[p.i] == '-') && !(p.s[p.i-1] == 'e' || p.s[p.i-1] == 'E') {
			break
		}
		p.i++
	}
	text := strings.ReplaceAll(p.s[start:p.i], "_", "")
	if text == "" {
		r, _ := utf8.DecodeRuneInString(p.s[start:])
		return nil, fmt.Errorf("unexpected %q", r)
	}
	if n, err := strconv.ParseInt(text, 0, 64); err == nil && !isLeadingZeroInt(text) {
		return int(n), nil
	}
	if f, err := strconv.ParseFloat(text, 64); err == nil && !strings.ContainsAny(text, "xXoObB") {
		return f, nil
	}
	return nil, fmt.Errorf("bad number %q", text)
}

// isLeadingZeroInt reports a decimal like "012", which Python rejects but
// strconv base 0 reads as octal.
func isLeadingZeroInt(text string) bool {
	t := strings.TrimLeft(text, "+-")
	return len(t) > 1 && t[0] == '0' && t[1] >= '0' && t[1] <= '9'
}

// stringLiterals parses one or more adjacent string literals, which Python
// concatenates.
func (p *pyLiteralParser) stringLiterals() (interface{}, error) {
	var b strings.Builder
	n := 0
	for {
		save := p.i
		if n > 0 {
			p.skipSpace()
		}
		start := p.i
		p.i = IdentEnd(p.s, p.i)
		prefix := p.s[start:p.i]
		if p.i >= len(p.s) || (p.s[p.i] != '"' && p.s[p.i] != '\'') || !isStringPrefix(prefix) {
			p.i = save
			break
		}
		s, err := p.stringBody(strings.ContainsAny(prefix, "rR"))
		if err != nil {
			return nil, err
		}
		b.WriteString(s)
		n++
	}
	if n == 0 {
		return nil, fmt.Errorf("expected a string")
	}
	return b.String(), nil
}

// stringBody parses a quoted string at p.i, decoding escapes unless raw.
func (p *pyLiteralParser) stringBody(raw bool) (string, error) {
	q := p.s[p.i]
	quote := string(q)
	if strings.HasPrefix(p.s[p.i:], strings.Repeat(quote, 3)) {
		quote = strings.Repeat(quote, 3)
	}
	p.i += len(quote)
	start := p.i
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == '\\':
			p.i += 2
		case strings.HasPrefix(p.s[p.i:], quote):
			body := p.s[start:p.i]
			p.i += len(quote)
			if raw {
				return body, nil
			}
			if hasNamedEscape(body) {
				// A named Unicode escape needs Python's name table.
				return "", fmt.Errorf(`\N{...} escape is not supported`)
			}
			return decodePythonEscapes(body), nil
		case c == '\n' && len(quote) == 1:
			return "", fmt.Errorf("newline in string")
		default:
			p.i++
		}
	}
	return "", fmt.Errorf("unterminated string")
}

// hasNamedEscape reports a \N{...} escape in a string body.
func hasNamedEscape(body string) bool {
	for i := 0; i+2 < len(body); i++ {
		if body[i] == '\\' {
			if body[i+1] == 'N' && body[i+2] == '{' {
				return true
			}
			i++
		}
	}
	return false
}

// decodePythonEscapes applies Python's string escapes to body. Unknown
// escapes such as \s or \d stay as they are, so a regex written with
// single backslashes keeps them, and \\ becomes \.
func decodePythonEscapes(body string) string {
	if !strings.Contains(body, "\\") {
		return body
	}
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' || i+1 >= len(body) {
			b.WriteByte(c)
			continue
		}
		e := body[i+1]
		switch e {
		case '\n':
			i++
		case '\\', '\'', '"':
			b.WriteByte(e)
			i++
		case 'a':
			b.WriteByte('\a')
			i++
		case 'b':
			b.WriteByte('\b')
			i++
		case 'f':
			b.WriteByte('\f')
			i++
		case 'n':
			b.WriteByte('\n')
			i++
		case 'r':
			b.WriteByte('\r')
			i++
		case 't':
			b.WriteByte('\t')
			i++
		case 'v':
			b.WriteByte('\v')
			i++
		case '0', '1', '2', '3', '4', '5', '6', '7':
			j := i + 1
			for j < len(body) && j < i+4 && body[j] >= '0' && body[j] <= '7' {
				j++
			}
			n, _ := strconv.ParseUint(body[i+1:j], 8, 32)
			b.WriteRune(rune(n))
			i = j - 1
		case 'x', 'u', 'U':
			width := map[byte]int{'x': 2, 'u': 4, 'U': 8}[e]
			if i+2+width <= len(body) {
				if n, err := strconv.ParseUint(body[i+2:i+2+width], 16, 32); err == nil && utf8.ValidRune(rune(n)) {
					b.WriteRune(rune(n))
					i += 1 + width
					continue
				}
			}
			// Python rejects a bad \x, \u or \U; keep it as written.
			b.WriteByte(c)
		default:
			// Unknown escape (\s, \d, \S, \w, \., ...): Python keeps it.
			b.WriteByte(c)
		}
	}
	return b.String()
}

// IsIdentStartRune approximates Python's XID_Start, the characters an
// identifier may start with: a letter (categories L and Nl) or "_".
func IsIdentStartRune(r rune) bool {
	if r < utf8.RuneSelf {
		return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	return unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_ID_Start, r)
}

// IsIdentRune approximates Python's XID_Continue, the characters after the
// first: XID_Start, digits (Nd), combining marks (Mn, Mc) and connector
// punctuation (Pc).
func IsIdentRune(r rune) bool {
	if r < utf8.RuneSelf {
		return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
	}
	return IsIdentStartRune(r) || unicode.In(r, unicode.Nd, unicode.Mn, unicode.Mc, unicode.Pc) ||
		unicode.Is(unicode.Other_ID_Continue, r)
}

// IdentStartAt reports whether a Python identifier starts at byte offset i
// of s.
func IdentStartAt(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	if c := s[i]; c < utf8.RuneSelf {
		return IsIdentStartRune(rune(c))
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return IsIdentStartRune(r)
}

// IdentEnd returns the offset just after the identifier characters of s
// starting at byte offset i (i itself if there are none).
func IdentEnd(s string, i int) int {
	for i < len(s) {
		if c := s[i]; c < utf8.RuneSelf {
			if !IsIdentRune(rune(c)) {
				break
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if !IsIdentRune(r) {
			break
		}
		i += size
	}
	return i
}

// isStringPrefix reports a string prefix gottp evaluates: none, r and u
// in either case. Byte and f-strings are left to the old loader.
func isStringPrefix(prefix string) bool {
	switch strings.ToLower(prefix) {
	case "", "r", "u":
		return true
	}
	return false
}
