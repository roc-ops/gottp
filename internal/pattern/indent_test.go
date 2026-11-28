package pattern

import "testing"

// Issue #30: Python TTP rebuilds a template line's leading whitespace
// verbatim ("\n" + indent + ...), so an indented template line matches
// data lines with exactly that indent: not deeper, not shallower, and a
// tab only where the template has a tab.
func TestIndentMatchesExactly(t *testing.T) {
	cases := []struct {
		tmpl  string
		data  string
		match bool
	}{
		{"  {{ v }}", "  a", true},
		{"  {{ v }}", "    a", false},
		{"  {{ v }}", " a", false},
		{"  {{ v }}", "\ta", false},
		{"  {{ v }}", "a", false},
		{"\t{{ v }}", "\ta", true},
		{"\t{{ v }}", "  a", false},
		{"  x  {{ y }} z", "  x y   z", true},
		{"  x  {{ y }} z", "   x y z", false},
		{"  {{ x | _start_ }}", "  a", true},
		{"  {{ x | _start_ }}", "    a", false},
		{"  {{ x | _line_ }}", "  a b", true},
		{"  {{ x | _line_ }}", "a b", false},
		{"   literal {{ ignore }}", "   literal a", true},
		{"   literal {{ ignore }}", "    literal a", false},
		// A standalone {{ _start_ }}/{{ _end_ }} still matches any blank line:
		// the runtime right-trims data lines, so a whitespace-only line
		// arrives as "" and has no indent to check.
		{"    {{ _start_ }}", "", true},
		// Unindented lines are unchanged.
		{"{{ v }}", "a", true},
		{"H {{ _start_ }}", "H", true},
	}
	e := NewEngine()
	for _, tc := range cases {
		cp, err := e.CompilePattern(tc.tmpl, false, false)
		if err != nil {
			t.Fatalf("CompilePattern(%q): %v", tc.tmpl, err)
		}
		if got := cp.Regex.MatchString(tc.data); got != tc.match {
			t.Errorf("template %q (regex %s) on %q: match = %v, want %v", tc.tmpl, cp.Regex, tc.data, got, tc.match)
		}
	}
}
