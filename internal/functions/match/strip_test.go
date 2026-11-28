package match

import "testing"

// Issue #35: strip/lstrip/rstrip("chars") follow Python's str.strip(chars):
// the argument is a set of characters, not a substring, and no argument
// strips whitespace.
var stripCases = []struct {
	fn    string
	value string
	args  []string
	want  string
}{
	{"strip", "  (00:01)  ", nil, "(00:01)"},
	{"strip", "(00:01)", []string{"()"}, "00:01"},
	{"strip", "((a) (b))", []string{")("}, "a) (b"},
	{"strip", "xyabcyx", []string{"xy"}, "abc"},
	{"strip", " (a) ", []string{"()"}, " (a) "},
	{"strip", "abc", []string{""}, "abc"},
	{"lstrip", "  a  ", nil, "a  "},
	{"lstrip", "((a)", []string{"("}, "a)"},
	{"lstrip", "xyabcyx", []string{"yx"}, "abcyx"},
	{"rstrip", "  a  ", nil, "  a"},
	{"rstrip", "(b))", []string{")"}, "(b"},
	{"rstrip", "xyabcyx", []string{"yx"}, "xyabc"},
}

func TestStripCharsArgument(t *testing.T) {
	r := NewRegistry()
	for _, tc := range stripCases {
		fn, ok := r.Get(tc.fn)
		if !ok {
			t.Fatalf("%s is not registered", tc.fn)
		}
		got, err := fn(tc.value, tc.args, nil)
		if err != nil {
			t.Fatalf("%s(%q, %q): %v", tc.fn, tc.value, tc.args, err)
		}
		if got != tc.want {
			t.Errorf("%s(%q, %q) = %q, want %q", tc.fn, tc.value, tc.args, got, tc.want)
		}
	}
}
