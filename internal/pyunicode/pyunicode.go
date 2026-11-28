// Package pyunicode holds the Unicode character classes of Python 3.12 that
// more than one gottp package needs: str.isprintable, which repr() uses to
// decide which characters to escape (package call) and which is a str
// method itself (package pymethod).
package pyunicode

//go:generate sh -c "python3.12 ../pymethod/gen_unicode.py printable > printable_table.go"

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

// IsPrintable is Python 3.12's str.isprintable for one character: false
// for the characters of the Unicode categories Other and Separator, except
// the ASCII space.
func IsPrintable(r rune) bool { return printableTable.has(r) }
