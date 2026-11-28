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
