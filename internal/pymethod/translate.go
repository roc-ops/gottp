package pymethod

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// str.maketrans and str.translate. A translation table is a dict from code
// points to code points, strings or None. gottp reads a dict literal's keys
// as strings, so the key 97 is "97"; a table's keys are code points written
// in decimal (a str key such as "a" is not, and Python ignores it too).

func strMaketrans(self interface{}, a *args) (interface{}, error) {
	if err := a.positional(1, 3); err != nil {
		return nil, err
	}
	out := map[string]interface{}{}
	if len(a.pos) == 1 {
		m, ok := a.pos[0].(map[string]interface{})
		if !ok {
			return nil, raise("TypeError", "if you give only one argument to maketrans it must be a dict")
		}
		for k, v := range m {
			key := k
			// A one-character str key is its code point; an int key (read
			// as its decimal text) stays.
			if _, err := strconv.Atoi(k); err != nil {
				if utf8.RuneCountInString(k) != 1 {
					return nil, raise("ValueError", "string keys in translate table must be of length 1")
				}
				r, _ := utf8.DecodeRuneInString(k)
				key = strconv.Itoa(int(r))
			}
			out[key] = v
		}
		return out, nil
	}
	x, ok := a.pos[0].(string)
	if !ok {
		return nil, raise("TypeError", "maketrans() argument 1 must be str, not %s", typeName(a.pos[0]))
	}
	y, ok := a.pos[1].(string)
	if !ok {
		return nil, raise("TypeError", "maketrans() argument 2 must be str, not %s", typeName(a.pos[1]))
	}
	xs, ys := []rune(x), []rune(y)
	if len(xs) != len(ys) {
		return nil, raise("ValueError", "the first two maketrans arguments must have equal length")
	}
	for i := range xs {
		out[strconv.Itoa(int(xs[i]))] = int(ys[i])
	}
	if len(a.pos) == 3 {
		z, ok := a.pos[2].(string)
		if !ok {
			return nil, raise("TypeError", "maketrans() argument 3 must be str, not %s", typeName(a.pos[2]))
		}
		for _, r := range z {
			out[strconv.Itoa(int(r))] = nil
		}
	}
	return out, nil
}

func strTranslate(self interface{}, a *args) (interface{}, error) {
	if err := a.positional(1, 1); err != nil {
		return nil, err
	}
	table := a.pos[0]
	// lookup is table[ord(c)]; ok is false on a LookupError, which keeps
	// the character.
	var lookup func(r rune) (interface{}, bool, error)
	switch t := table.(type) {
	case map[string]interface{}:
		lookup = func(r rune) (interface{}, bool, error) {
			v, ok := t[strconv.Itoa(int(r))]
			return v, ok, nil
		}
	case []interface{}:
		lookup = func(r rune) (interface{}, bool, error) {
			if int(r) < len(t) {
				return t[r], true, nil
			}
			return nil, false, nil
		}
	case string:
		rs := []rune(t)
		lookup = func(r rune) (interface{}, bool, error) {
			if int(r) < len(rs) {
				return string(rs[r]), true, nil
			}
			return nil, false, nil
		}
	default:
		return nil, raise("TypeError", "'%s' object is not subscriptable", typeName(table))
	}
	var b strings.Builder
	for _, r := range self.(string) {
		v, ok, err := lookup(r)
		if err != nil {
			return nil, err
		}
		if !ok {
			b.WriteRune(r)
			continue
		}
		switch x := v.(type) {
		case nil:
		case string:
			b.WriteString(x)
		default:
			n, isInt := toInt(x)
			if !isInt {
				return nil, raise("TypeError", "character mapping must return integer, None or str")
			}
			if !n.IsInt64() || n.Int64() < 0 || n.Int64() > 0x10ffff {
				return nil, raise("ValueError", "character mapping must be in range(0x110000)")
			}
			b.WriteRune(rune(n.Int64()))
		}
	}
	return b.String(), nil
}
