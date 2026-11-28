package compiler

import (
	"regexp/syntax"

	"github.com/roc-ops/gottp/internal/pattern"
)

// lazyAny is the pattern the engine gives a default() variable that has no
// regex of its own (pattern.Engine's fallback for default, set, _start_ and
// _end_).
const lazyAny = `.*?`

// regexMatchesBlank reports whether the line regex of cp may match a blank
// data line: an empty line, or one of only spaces and tabs. The runtime skips
// blank lines for patterns that can't match them (#162). Python TTP runs its
// regexes over the whole text, so a pattern such as {{ x | _line_ }},
// re(".*") or re("[ ]+") matches blank lines there.
//
// The answer may be a false positive (the runtime then runs the regex on the
// line, which gives the right result, only slower) but never a false
// negative, with one deliberate exception: a default() variable without a
// regex of its own uses .*? in gottp (so it can match an empty or missing last
// column, #64) where TTP uses WORD (\S+), which can't match a blank line.
// Such a variable is treated as WORD here, so a line made only of default()
// variables doesn't match blank lines, as in TTP.
func regexMatchesBlank(cp *pattern.CompiledPattern) bool {
	if cp.Regex == nil {
		return false
	}
	// Fast path: a literal prefix with a character other than a space or
	// tab can't match a blank line.
	if prefix, _ := cp.Regex.LiteralPrefix(); !isBlankText(prefix) {
		return false
	}
	re, err := syntax.Parse(cp.Regex.String(), syntax.Perl)
	if err != nil {
		return true
	}
	return canMatchBlank(re, defaultWordCaptures(cp, re.MaxCap()))
}

// canMatchBlank reports whether re can match a string of only spaces and
// tabs (the empty string included), with the captures in word treated as
// matching no such string. Empty-width assertions are assumed to hold, which
// over-approximates and so is safe.
func canMatchBlank(re *syntax.Regexp, word map[int]bool) bool {
	switch re.Op {
	case syntax.OpNoMatch:
		return false
	case syntax.OpLiteral:
		for _, r := range re.Rune {
			if r != ' ' && r != '\t' {
				return false
			}
		}
		return true
	case syntax.OpCharClass:
		for i := 0; i+1 < len(re.Rune); i += 2 {
			lo, hi := re.Rune[i], re.Rune[i+1]
			if (lo <= ' ' && ' ' <= hi) || (lo <= '\t' && '\t' <= hi) {
				return true
			}
		}
		return false
	case syntax.OpCapture:
		if word[re.Cap] {
			return false
		}
		return canMatchBlank(re.Sub[0], word)
	case syntax.OpStar, syntax.OpQuest:
		return true
	case syntax.OpPlus:
		return canMatchBlank(re.Sub[0], word)
	case syntax.OpRepeat:
		return re.Min == 0 || canMatchBlank(re.Sub[0], word)
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			if !canMatchBlank(sub, word) {
				return false
			}
		}
		return true
	case syntax.OpAlternate:
		for _, sub := range re.Sub {
			if canMatchBlank(sub, word) {
				return true
			}
		}
		return false
	default:
		// Empty match, the any-character ops and the empty-width
		// assertions.
		return true
	}
}

func isBlankText(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return false
		}
	}
	return true
}

// defaultWordCaptures returns the capture numbers of the default() variables
// of cp whose pattern is the engine's .*? fallback. Captures are numbered as
// pattern.Engine.GenerateRegexFromVariables emits them: one per variable in
// VariableOrder, except _exact_ and _exact_space_, plus the captures inside
// the variable's own pattern. If the count doesn't add up to maxCap the
// numbering is unknown and no capture is returned (the safe answer).
func defaultWordCaptures(cp *pattern.CompiledPattern, maxCap int) map[int]bool {
	found := false
	for _, v := range cp.Variables {
		if v.Pattern == lazyAny && hasFunction(v, "default") && !pattern.IsNonCapturingVar(v.Name) {
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	var word map[int]bool
	n := 0
	for _, name := range cp.VariableOrder {
		v := cp.Variables[name]
		if v == nil || v.Name == pattern.IndicatorExact || v.Name == pattern.IndicatorExactSpace {
			continue
		}
		n++
		if v.Pattern == lazyAny && hasFunction(v, "default") && !pattern.IsNonCapturingVar(v.Name) {
			if word == nil {
				word = make(map[int]bool)
			}
			word[n] = true
		}
		if inner, err := syntax.Parse(v.Pattern, syntax.Perl); err == nil {
			n += inner.MaxCap()
		} else {
			return nil
		}
	}
	if n != maxCap {
		return nil
	}
	return word
}

func hasFunction(v *pattern.MatchVariable, name string) bool {
	for _, f := range v.Functions {
		if pattern.FunctionName(f) == name {
			return true
		}
	}
	return false
}
