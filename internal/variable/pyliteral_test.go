package variable

import (
	"reflect"
	"testing"
)

// The expected values are what Python 3 evaluates each literal to.
func TestPythonStringLiteral(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"\s+\S.*"`, `\s+\S.*`},
		{`"\\s+\\S.*"`, `\s+\S.*`},
		{`'\d{2,}'`, `\d{2,}`},
		{`r"\d\\"`, `\d\\`},
		{`"a\tb"`, "a\tb"},
		{`"\x41\101é"`, "AAé"},
		{`"a" "b"`, "ab"},
		{`"""x\"y"""`, `x"y`},
		{`"\\\\d"`, `\\d`},
		{`u"\s"`, `\s`},
		{`"q\'s"`, "q's"},
		{"\"\\\nx\"", "x"},
		{`"UP|DOWN"  # states`, "UP|DOWN"},
	}
	for _, tc := range cases {
		got, ok := PythonStringLiteral(tc.in)
		if !ok || got != tc.want {
			t.Errorf("PythonStringLiteral(%s) = %q, %v; want %q", tc.in, got, ok, tc.want)
		}
	}
	for _, in := range []string{`abc`, `"abc`, `"a" b`, `f"x"`, `12`} {
		if got, ok := PythonStringLiteral(in); ok {
			t.Errorf("PythonStringLiteral(%s) = %q, want not a string literal", in, got)
		}
	}
}

func TestLoadPythonRegexVars(t *testing.T) {
	got, err := NewLoader().LoadPython(`
single = "\s+\S.*"
double = "\\s{2,}\\S.*"
raw = r"Channel\d+"
dict = {
    "re": "\d+",
    "n": 1,
    "none": None,
}
`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"single": `\s+\S.*`,
		"double": `\s{2,}\S.*`,
		"raw":    `Channel\d+`,
		"dict":   map[string]interface{}{"re": `\d+`, "n": 1, "none": nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LoadPython = %#v\nwant %#v", got, want)
	}
}
