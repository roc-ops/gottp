package compiler

import (
	"testing"

	"github.com/roc-ops/gottp/internal/pattern"
)

// TestRegexMatchesBlank checks which template lines may match a blank data
// line (#162). The answer must never be false for a regex that matches one,
// except for a line whose only way to match is a default() variable without
// a regex, which TTP gives WORD (\S+).
func TestRegexMatchesBlank(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"interface {{ name }}", false},
		{"{{ x }}", false},
		{"{{ x | _line_ }}", true},
		{" {{ x | _line_ }}", true},
		{` {{ d | _line_ | joinmatches }}`, true},
		{`{{ x | re(".*") }}`, true},
		{`{{ x | re("[ ]+") }}`, true},
		{`{{ x | re("[ ]{3}") }}`, true},
		{`{{ x | re("\\s*") }}`, true},
		{`{{ x | re(".+") }}`, true},
		{`{{ x | re("\\S+") }}`, false},
		{`{{ x | re(".*") | default("n") }}`, true},
		{`{{ x | default("none") }}`, false},
		{`  {{ x | default("none") }}`, false},
		{`{{ a | default("x") }} {{ b | re("[ ]*") }}`, false},
		{`{{ a | default("x") }} {{ b | default("y") }}`, false},
		{`description {{ d | default("") }}`, false},
		{"{{ _start_ }}", true},
		{"  {{ _start_ }}", true},
		{"{{ _end_ }}", true},
		{"exit {{ _end_ }}", false},
		{"{{ ignore }}", false},
	}
	e := pattern.NewEngine()
	for _, tc := range cases {
		cp, err := e.CompilePattern(tc.line, false, false)
		if err != nil {
			t.Fatalf("CompilePattern(%q): %v", tc.line, err)
		}
		if got := regexMatchesBlank(cp); got != tc.want {
			t.Errorf("%q (regex %s): regexMatchesBlank = %v, want %v", tc.line, cp.Regex, got, tc.want)
		}
		// Soundness: a regex that matches a probe blank line must be reported.
		for _, probe := range []string{"", " ", "\t", "   ", " \t "} {
			if cp.Regex.MatchString(probe) && !regexMatchesBlank(cp) && !onlyDefaultBlank(cp) {
				t.Errorf("%q (regex %s) matches %q but regexMatchesBlank is false", tc.line, cp.Regex, probe)
			}
		}
	}
}

// onlyDefaultBlank reports whether cp has a default() variable with the .*?
// fallback, the one case regexMatchesBlank deliberately answers false for.
func onlyDefaultBlank(cp *pattern.CompiledPattern) bool {
	for _, v := range cp.Variables {
		if v.Pattern == lazyAny && hasFunction(v, "default") {
			return true
		}
	}
	return false
}
