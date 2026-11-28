package pattern

import "testing"

// A line's regex ends at a {{ _start_ }} or {{ _end_ }} variable, as Python
// TTP's regex_deleteVar cuts it (#136). The expected TTP regexes are noted
// for each case; gottp anchors with ^...$ where TTP uses \n...(?=\n|\r\n).
func TestGenerateRegex_CutAtStartEnd(t *testing.T) {
	e := NewEngine()
	tests := []struct {
		line, want string
		exactSpace bool
	}{
		// \ninterface[\t ]*(?=\n|\r\n)
		{line: "interface {{ _start_ }}", want: `^interface[\t ]*$`},
		// one [ \t]+ is removed, so two spaces are no different
		{line: "interface  {{ _start_ }}", want: `^interface[\t ]*$`},
		// \ninterface[ \t]+(?P<name>\S+)[\t ]*(?=\n|\r\n)
		{line: "interface {{ name }} {{ _start_ }}", want: `^interface[ \t]+(\S+)[\t ]*$`},
		// the text and variables after the indicator are dropped
		{line: "exit {{ _end_ }} now", want: `^exit[\t ]*$`},
		{line: "interface {{ _start_ }} {{ name }}", want: `^interface[\t ]*$`},
		{line: "{{ _start_ }} interface {{ name }}", want: `^[\t ]*$`},
		{line: "  {{ _end_ }}", want: `^  [\t ]*$`},
		// _exact_space_ keeps the space before the indicator: \ninterface\ [\t ]*
		{line: "interface {{ _start_ }}", exactSpace: true, want: `^interface [\t ]*$`},
		// a run of mixed spaces and tabs is one [ \t]+, so it is cut whole
		{line: "interface \t{{ _start_ }}", want: `^interface[\t ]*$`},
		{line: "interface \t {{ _start_ }}", exactSpace: true, want: "^interface \t [\\t ]*$"},
		// _start_ as a function is not cut
		{line: "interface {{ name | _start_ }} x", want: `^interface[ \t]+(\S+)[ \t]+x[\t ]*$`},
		// {{ _start_ | re(...) }} keeps its regex (gottp extension)
		{line: `interface {{ _start_ | re("Gi\\S+") }}`, want: `^interface[ \t]*(Gi\S+)[\t ]*$`},
	}
	for _, tt := range tests {
		got, _, err := e.GenerateRegex(tt.line, tt.exactSpace, false)
		if err != nil {
			t.Fatalf("%q: %v", tt.line, err)
		}
		if got != tt.want {
			t.Errorf("%q: got %s, want %s", tt.line, got, tt.want)
		}
	}
}

func TestCutVariables(t *testing.T) {
	e := NewEngine()
	tests := []struct {
		line      string
		indicator string
		names     []string
	}{
		{"interface {{ _start_ }}", "_start_", nil},
		{"interface {{ name }} {{ _start_ }}", "_start_", nil},
		{"interface {{ _start_ }} {{ name }} {{ ignore }} {{ x | set(1) }} {{ _exact_ }}", "_start_", []string{"name"}},
		{"exit {{ _end_ }} {{ a }} {{ b }}", "_end_", []string{"a", "b"}},
		// a name repeated after the cut is listed once
		{"interface {{ _start_ }} {{ a }} {{ b }} {{ a }}", "_start_", []string{"a", "b"}},
		{`x {{ _start_ | re("a") }} {{ name }}`, "", nil},
		{"interface {{ name | _start_ }}", "", nil},
	}
	for _, tt := range tests {
		cp, err := e.CompilePattern(tt.line, false, false)
		if err != nil {
			t.Fatalf("%q: %v", tt.line, err)
		}
		indicator, names := CutVariables(cp)
		if indicator != tt.indicator || len(names) != len(tt.names) {
			t.Errorf("%q: got %q %q, want %q %q", tt.line, indicator, names, tt.indicator, tt.names)
			continue
		}
		for i := range names {
			if names[i] != tt.names[i] {
				t.Errorf("%q: got %q, want %q", tt.line, names, tt.names)
			}
		}
	}
}

// TTP's regex_deleteVar leaves no named group on a line cut at _end_ with no
// capture before it, and check_matches then gives every variable the whole
// match: WholeMatch marks such a line.
func TestWholeMatch(t *testing.T) {
	e := NewEngine()
	tests := []struct {
		line string
		want bool
	}{
		{"exit {{ _end_ }} {{ w }}", true},
		// nothing after the cut: an end line, which keeps no values
		{"exit {{ _end_ }}", false},
		{`exit {{ x | set("y") }} {{ _end_ }} {{ w }}`, true},
		{"exit {{ x }} {{ _end_ }} {{ w }}", false},
		{"interface {{ _start_ }} {{ w }}", true},
		{"exit {{ w | _end_ }}", false},
		{`exit {{ _end_ | re("x") }} {{ w }}`, false},
	}
	for _, tt := range tests {
		cp, err := e.CompilePattern(tt.line, false, false)
		if err != nil {
			t.Fatalf("%q: %v", tt.line, err)
		}
		if cp.WholeMatch != tt.want {
			t.Errorf("%q: WholeMatch = %v, want %v", tt.line, cp.WholeMatch, tt.want)
		}
	}
}

// Python TTP replaces any run of escaped spaces and tabs with one [ \t]+
// (re.sub(r"(\\ |\\\t)+", ...)), so a mixed run is not two runs.
func TestGenerateRegex_MixedBlankRun(t *testing.T) {
	e := NewEngine()
	tests := []struct {
		line, want string
		exactSpace bool
	}{
		{line: "interface \t{{ name }}", want: `^interface[ \t]+(\S+)[\t ]*$`},
		{line: "a\t \tb {{ x }} \t c", want: `^a[ \t]+b[ \t]+(\S+)[ \t]+c[\t ]*$`},
		{line: "interface \t{{ name }}", exactSpace: true, want: "^interface \t(\\S+)[\\t ]*$"},
	}
	for _, tt := range tests {
		got, _, err := e.GenerateRegex(tt.line, tt.exactSpace, false)
		if err != nil {
			t.Fatalf("%q: %v", tt.line, err)
		}
		if got != tt.want {
			t.Errorf("%q: got %s, want %s", tt.line, got, tt.want)
		}
	}
}
