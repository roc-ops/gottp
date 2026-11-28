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
	re, err := syntax.Parse(cp.Regex.String(), syntax.Perl)
	if err != nil {
		return true
	}
	prog, err := syntax.Compile(re.Simplify())
	if err != nil {
		return true
	}
	word := defaultWordCaptures(cp, re.MaxCap())

	// Reachability of a match over the alphabet {' ', '\t'}. The position
	// in the line is not tracked, so every empty-width assertion is assumed
	// to hold: that over-approximates, which is safe.
	seen := make([]bool, len(prog.Inst))
	stack := []uint32{uint32(prog.Start)}
	for len(stack) > 0 {
		pc := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[pc] {
			continue
		}
		seen[pc] = true
		inst := &prog.Inst[pc]
		switch inst.Op {
		case syntax.InstMatch:
			return true
		case syntax.InstFail:
		case syntax.InstAlt, syntax.InstAltMatch:
			stack = append(stack, inst.Out, inst.Arg)
		case syntax.InstCapture:
			// Arg is 2*n at the start of capture n.
			if inst.Arg%2 == 0 && word[int(inst.Arg/2)] {
				continue
			}
			stack = append(stack, inst.Out)
		case syntax.InstEmptyWidth, syntax.InstNop:
			stack = append(stack, inst.Out)
		case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
			if inst.MatchRune(' ') || inst.MatchRune('\t') {
				stack = append(stack, inst.Out)
			}
		default:
			return true
		}
	}
	return false
}

// defaultWordCaptures returns the capture numbers of the default() variables
// of cp whose pattern is the engine's .*? fallback. Captures are numbered as
// pattern.Engine.GenerateRegexFromVariables emits them: one per variable in
// VariableOrder, except _exact_ and _exact_space_, plus the captures inside
// the variable's own pattern. If the count doesn't add up to maxCap the
// numbering is unknown and no capture is returned (the safe answer).
func defaultWordCaptures(cp *pattern.CompiledPattern, maxCap int) map[int]bool {
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
