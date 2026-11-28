package pymethod

import (
	"strings"
	"unicode"

	"github.com/roc-ops/gottp/internal/pyunicode"
)

//go:generate sh -c "python3.12 gen_unicode.py > unicode_tables.go"

// runeRanges is a sorted list of inclusive code point ranges.
type runeRanges [][2]rune

func (t runeRanges) has(r rune) bool {
	lo, hi := 0, len(t)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		switch {
		case r < t[m][0]:
			hi = m
		case r > t[m][1]:
			lo = m + 1
		default:
			return true
		}
	}
	return false
}

// The character classes of Python's str methods (Objects/unicodectype.c).
func isAlpha(r rune) bool { return r < 0x80 && asciiAlpha(r) || r >= 0x80 && alphaTable.has(r) }
func isDecimal(r rune) bool {
	return r < 0x80 && '0' <= r && r <= '9' || r >= 0x80 && decimalTable.has(r)
}
func isDigit(r rune) bool { return r < 0x80 && '0' <= r && r <= '9' || r >= 0x80 && digitTable.has(r) }
func isNumeric(r rune) bool {
	return r < 0x80 && '0' <= r && r <= '9' || r >= 0x80 && numericTable.has(r)
}
func isSpace(r rune) bool     { return spaceTable.has(r) }
func isPrintable(r rune) bool { return pyunicode.IsPrintable(r) }
func isLower(r rune) bool     { return lowerTable.has(r) }
func isUpper(r rune) bool     { return upperTable.has(r) }
func isTitle(r rune) bool     { return titleTable.has(r) }
func isCased(r rune) bool     { return isLower(r) || isUpper(r) || isTitle(r) }
func isAlnum(r rune) bool     { return isAlpha(r) || isDecimal(r) || isDigit(r) || isNumeric(r) }

func asciiAlpha(r rune) bool { return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' }

func isCaseIgnorable(r rune) bool { return caseIgnorableTable.has(r) }

// Python's full case mappings of one character.
func upperOf(r rune) string {
	if s, ok := upperFullMap[r]; ok {
		return s
	}
	return string(unicode.ToUpper(r))
}

func lowerOf(r rune) string {
	if s, ok := lowerFullMap[r]; ok {
		return s
	}
	return string(unicode.ToLower(r))
}

func titleOf(r rune) string {
	if s, ok := titleFullMap[r]; ok {
		return s
	}
	return string(unicode.ToTitle(r))
}

func foldOf(r rune) string {
	if s, ok := foldMap[r]; ok {
		return s
	}
	return lowerOf(r)
}

// lowerAt is Python's lower_ucs4: the lowercase of rs[i], with a capital
// sigma in the final position (after a cased letter, not before one,
// skipping case-ignorable characters) as "ς".
func lowerAt(rs []rune, i int) string {
	if rs[i] != 'Σ' {
		return lowerOf(rs[i])
	}
	j := i - 1
	for j >= 0 && isCaseIgnorable(rs[j]) {
		j--
	}
	final := j >= 0 && isCased(rs[j])
	if final && i+1 < len(rs) {
		j = i + 1
		for j < len(rs) && isCaseIgnorable(rs[j]) {
			j++
		}
		final = j == len(rs) || !isCased(rs[j])
	}
	if final {
		return "ς"
	}
	return "σ"
}

func pyUpper(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		b.WriteString(upperOf(r))
	}
	return b.String()
}

func pyLower(s string) string {
	rs := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i := range rs {
		b.WriteString(lowerAt(rs, i))
	}
	return b.String()
}

func pyCasefold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		b.WriteString(foldOf(r))
	}
	return b.String()
}

func pySwapcase(s string) string {
	rs := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range rs {
		switch {
		case isUpper(r):
			b.WriteString(lowerAt(rs, i))
		case isLower(r):
			b.WriteString(upperOf(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pyTitle is Python's str.title: a character after a cased one is
// lowercased, any other titlecased.
func pyTitle(s string) string {
	rs := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	prevCased := false
	for i, r := range rs {
		if prevCased {
			b.WriteString(lowerAt(rs, i))
		} else {
			b.WriteString(titleOf(r))
		}
		prevCased = isCased(r)
	}
	return b.String()
}

// pyCapitalize is Python's str.capitalize: the first character titlecased
// (since 3.8), the rest lowercased.
func pyCapitalize(s string) string {
	rs := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range rs {
		if i == 0 {
			b.WriteString(titleOf(r))
		} else {
			b.WriteString(lowerAt(rs, i))
		}
	}
	return b.String()
}

// IsSpace reports whether r is whitespace as Python's str.isspace() and the
// \s of a str regex read it.
func IsSpace(r rune) bool { return isSpace(r) }
