package test

// Pins for refactor F1 (#36): one call module for match, group, input and
// output functions (internal/call). The results these templates give are
// pinned against Python TTP 0.10.1 in test/ttpsuite/testdata/gottp-repros
// (issue36_*, issue23_*, issue151_*).

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

// compileWarnings compiles tmpl, parses one line through it (TTP skips an
// unknown name at run time and keeps going) and returns the compile warnings.
func compileWarnings(t *testing.T, tmpl string) []string {
	t.Helper()
	c, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := c.Parse(gottp.Inputs{"Default_Input": "x 1\n"}, nil, nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return c.GetWarnings()
}

// TestUnknownFunctionNamesWarn: TTP logs an unknown function name in every
// scope and, for a close match, the names its utils.guess suggests
// (difflib, cutoff 0.8). gottp gives no warning in any scope today. The
// suggestions below are the ones TTP 0.10.1 logs for these templates.
func TestUnknownFunctionNamesWarn(t *testing.T) {
	cases := []struct {
		name, tmpl string
		want       []string // substrings of one warning
	}{
		{"match", "x {{ v | nosuchfn }}\n", []string{"nosuchfn"}},
		{"match close", "x {{ v | to_itn }}\n", []string{"to_itn", "to_int"}},
		{"group", "<group name=\"g\" functions=\"nosuchgrp()\">\nx {{ v }}\n</group>\n", []string{"nosuchgrp"}},
		{"group close", "<group name=\"g\" functions=\"contians('v')\">\nx {{ v }}\n</group>\n", []string{"contians", "contains"}},
		{"input", "<input load=\"text\" functions=\"nosuchin()\">\nx 1\n</input>\n<group name=\"g\">\nx {{ v }}\n</group>\n", []string{"nosuchin"}},
		{"input close", "<input load=\"text\" functions=\"extract_comands('x')\">\nx 1\n</input>\n<group name=\"g\">\nx {{ v }}\n</group>\n", []string{"extract_comands", "extract_commands"}},
		{"output", "<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"nosuchout()\"/>\n", []string{"nosuchout"}},
		{"output close", "<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"is_eqal('x')\"/>\n", []string{"is_eqal", "is_equal"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warnings := compileWarnings(t, tc.tmpl)
			for _, w := range warnings {
				if containsAll(w, tc.want) {
					return
				}
			}
			t.Errorf("want a warning containing %q, got %q", tc.want, warnings)
		})
	}
}

// TestPipeInQuotedArgumentWarns (#23, decision 2026-11-27): gottp keeps
// splitting the chain on "|" as TTP does, so the result doesn't change
// (gottp-repros/issue23_re_pipe_splits_chain), but it warns. The warning
// names the variable, the broken-off text and the <vars> workaround.
func TestPipeInQuotedArgumentWarns(t *testing.T) {
	warnings := compileWarnings(t, "x {{ direction | re(\"UP|DOWN\") }}\n")
	want := []string{"direction", `DOWN")`, "<vars>"}
	for _, w := range warnings {
		if containsAll(w, want) {
			return
		}
	}
	t.Errorf("want a warning containing %q, got %q", want, warnings)
}

// TestMalformedMacroCallIsCompileError (#146 item 2): TTP raises TypeError
// for both templates; the agreed behaviour is a compile error. gottp skips
// the call silently today.
func TestMalformedMacroCallIsCompileError(t *testing.T) {
	cases := map[string]string{
		// TTP: macro() missing 1 required positional argument: 'macro_name'
		"empty macro attribute": "<group name=\"g\">\nx {{ v }}\n</group>\n<output macro=\"\"/>\n",
		// TTP: macro() got an unexpected keyword argument 'name'
		"keyword argument": "<macro>\ndef m(data):\n    return data\n</macro>\n<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"macro(name='m')\"/>\n",
	}
	for name, tmpl := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := gottp.CompileTemplate(tmpl); err == nil {
				t.Error("want a compile error, got none")
			}
		})
	}
}

// issue144Template hangs CompileTemplate with unbounded memory today (#144).
const issue144Template = "<group name=\"items\">\nitem {{ name }}\n</group>\n<output functions=\"macro(\\\"m\\\",\\\"n\\\")\"/>\n"

const issue144ChildEnv = "GOTTP_TEST_ISSUE144_CHILD"

// TestBackslashQuotedAttributeCompilesQuickly (#144): the template must
// compile or return an error quickly. The compile runs in a child process
// (this test binary, re-run for this test only) that is killed after 2s:
// a goroutine that never returns would keep allocating inside the shared
// test binary after the test failed.
func TestBackslashQuotedAttributeCompilesQuickly(t *testing.T) {
	if os.Getenv(issue144ChildEnv) == "1" {
		_, err := gottp.CompileTemplate(issue144Template)
		t.Logf("compile returned, err=%v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBackslashQuotedAttributeCompilesQuickly$", "-test.count=1")
	cmd.Env = append(os.Environ(), issue144ChildEnv+"=1")
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("CompileTemplate did not return within 2s")
	}
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func parseOne(t *testing.T, tmpl, data string) interface{} {
	t.Helper()
	c, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, err := c.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return res
}

// TestMalformedCallsAreCompileErrors: calls with broken syntax (an
// unterminated quote, unbalanced brackets) and the macro() calls Python
// TTP fails with a TypeError on are compile errors in every scope. An
// argument that is only not a literal compiles (TestRawTextArguments).
func TestMalformedCallsAreCompileErrors(t *testing.T) {
	cases := map[string]string{
		"unterminated quote":   "x {{ v | contains(\"a) }}\n",
		"unbalanced brackets":  "x {{ v | contains([1, 2) }}\n",
		"positional after kw":  "x {{ v | lookup(add_field='a', t) }}\n",
		"group attribute":      "<group name=\"g\" contains=\"'-VM-\">\nx {{ v }}\n</group>\n",
		"empty argument":       "x {{ v | contains(a,,b) }}\n",
		"output macro attr":    "<group name=\"g\">\nx {{ v }}\n</group>\n<output macro=\"\"/>\n",
		"group functions":      "<group name=\"g\" functions=\"contains('a'), record('b')\">\nx {{ v }}\n</group>\n",
		"input functions":      "<input load=\"text\" functions=\"test('a)\">\nx 1\n</input>\n<group name=\"g\">\nx {{ v }}\n</group>\n",
		"output functions":     "<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"traverse('a)\"/>\n",
		"group macro keyword":  "<group name=\"g\" functions=\"macro(name='m')\">\nx {{ v }}\n</group>\n",
		"output macro no name": "<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"macro()\"/>\n",
	}
	for name, tmpl := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := gottp.CompileTemplate(tmpl); err == nil {
				t.Error("want a compile error, got none")
			}
		})
	}
}

// TestQuotedPipeViaVars: the workaround the #23 warning gives, a regex with
// "|" defined in <vars> and passed by name, matches only its alternatives.
func TestQuotedPipeViaVars(t *testing.T) {
	tmpl := "<vars>\nUD = \"UP|DOWN\"\n</vars>\n<group name=\"r\">\n{{ id }} {{ direction | re(UD) }}\n</group>\n"
	c, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	if w := c.GetWarnings(); len(w) != 0 {
		t.Errorf("unexpected warnings %q", w)
	}
	res, err := c.Parse(gottp.Inputs{"Default_Input": "1 UP\n2 --\n3 DOWN\n"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprint(res)
	if strings.Contains(got, "--") || !strings.Contains(got, "DOWN") {
		t.Errorf("result %s", got)
	}
}

// TestPythonLiteralArguments: arguments are Python values.
func TestPythonLiteralArguments(t *testing.T) {
	res := parseOne(t, "<group name=\"g\">\nx {{ v | set(True) }} {{ w | sformat(\"w={}\") }}\n {{ n | set(5) }}\n</group>\n", "x 1 2\n")
	want := "[map[g:map[n:5 v:true w:w=2]]]"
	if got := fmt.Sprint(res); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if b, ok := res.([]interface{})[0].(map[string]interface{})["g"].(map[string]interface{})["v"].(bool); !ok || !b {
		t.Errorf("set(True) gave %#v, want the bool true", res)
	}
}

// TestOutputKeywordArguments: an output function gets its keyword
// arguments as Python values in kwargs.
func TestOutputKeywordArguments(t *testing.T) {
	var gotKwargs map[string]interface{}
	var gotArgs []string
	tmpl := "<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"probe('a', strict=True, n=2)\"/>\n"
	c, err := gottp.CompileTemplateWithOptions(tmpl, &gottp.CompileOptions{Functions: &gottp.Functions{
		Output: map[string]gottp.OutputFunc{"probe": func(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
			gotArgs, gotKwargs = args, kwargs
			return data, nil
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if w := c.GetWarnings(); len(w) != 0 {
		t.Errorf("a compile-time function must not be warned about: %q", w)
	}
	if _, err := c.Parse(gottp.Inputs{"Default_Input": "x 1\n"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(gotArgs) != "[a strict=True n=2]" {
		t.Errorf("args %q", gotArgs)
	}
	if gotKwargs["strict"] != true || gotKwargs["n"] != 2 {
		t.Errorf("kwargs %#v", gotKwargs)
	}
}

// TestSiblingsOfFirstGroupAreKept: a template that starts with <group>
// keeps the elements after it, as Python TTP does (only the first was kept).
func TestSiblingsOfFirstGroupAreKept(t *testing.T) {
	res := parseOne(t, "<group name=\"a\">\nx {{ v }}\n</group>\n<group name=\"b\">\ny {{ w }}\n</group>\n", "x 1\ny 2\n")
	if got, want := fmt.Sprint(res), "[map[a:map[v:1] b:map[w:2]]]"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// TestCustomGroupAttributeIsReadAtCompileTime: a group attribute naming a
// CompileOptions group function is read as name(value) when the template
// compiles, so a value that can't be read is a compile error.
func TestCustomGroupAttributeIsReadAtCompileTime(t *testing.T) {
	var got []string
	fns := &gottp.Functions{Group: map[string]gottp.GroupFunc{"tag": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		got = args
		return data, true, nil
	}}}
	if _, err := gottp.CompileTemplateWithOptions("<group name=\"g\" tag=\"'Gig Eth\">\nx {{ v }}\n</group>\n", &gottp.CompileOptions{Functions: fns}); err == nil {
		t.Error("want a compile error for tag=\"'Gig Eth\"")
	}
	// A bare word is passed as its text, with a warning.
	c, err := gottp.CompileTemplateWithOptions("<group name=\"g\" tag=\"Gig-Eth\">\nx {{ v }}\n</group>\n", &gottp.CompileOptions{Functions: fns})
	if err != nil {
		t.Fatal(err)
	}
	if w := c.GetWarnings(); len(w) != 1 || !strings.Contains(w[0], `tag("Gig-Eth")`) {
		t.Errorf("warnings %q, want one suggesting tag(\"Gig-Eth\")", w)
	}
	if _, err := c.Parse(gottp.Inputs{"Default_Input": "x 1\n"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[Gig-Eth]" {
		t.Errorf("args %q", got)
	}
	c, err = gottp.CompileTemplateWithOptions("<group name=\"g\" tag=\"'a b', k=1\">\nx {{ v }}\n</group>\n", &gottp.CompileOptions{Functions: fns})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Parse(gottp.Inputs{"Default_Input": "x 1\n"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != "[a b k=1]" {
		t.Errorf("args %q", got)
	}
}

// TestRawTextArguments: an unquoted word Python TTP can't evaluate
// (set(auto-decision) is "auto" - "decision" in Python, and TTP exits) and
// an argument that is a Python expression rather than a literal (D1) both
// compile. gottp passes the argument's raw text and warns: for the word,
// that Python TTP fails on it, with the quoted call to use instead; for
// the expression, that Python TTP evaluates it (#157 review, a deliberate
// gottp extension; see test/comparison/KNOWN_DIFFERENCES.md).
func TestRawTextArguments(t *testing.T) {
	cases := []struct {
		name, tmpl, data, want string
		warn                   []string
	}{
		{
			name: "set bare word",
			tmpl: "<group name=\"g\">\nx {{ v }} {{ mode | set(auto-decision) }}\n</group>\n",
			data: "x 1\n",
			want: "[map[g:map[mode:auto-decision v:1]]]",
			warn: []string{`match variable "mode"`, "Python TTP evaluates it as a Python expression and fails", `set("auto-decision")`},
		},
		{
			name: "set dots and dashes",
			tmpl: "<group name=\"g\">\nx {{ v }} {{ mode | set(...-...) }}\n</group>\n",
			data: "x 1\n",
			want: "[map[g:map[mode:...-... v:1]]]",
			warn: []string{`set("...-...")`},
		},
		{
			name: "containsall attribute",
			tmpl: "<group name=\"g\" containsall=\"af-marker\">\nx {{ v }}\nm {{ af-marker }}\n</group>\n",
			data: "x 1\nm 2\n",
			want: "[map[g:map[af-marker:2 v:1]]]",
			warn: []string{`attribute containsall`, `containsall("af-marker")`},
		},
		{
			name: "expression",
			tmpl: "<group name=\"g\">\nx {{ v | set(1+2) }} {{ w | sformat(\"a\"+\"{}\") }}\n</group>\n",
			data: "x 1 2\n",
			want: "[map[g:map[v:1+2 w:\"a\"+\"2\"]]]",
			warn: []string{"is a Python expression, not a literal: Python TTP evaluates it; gottp passes its text \"1+2\" unevaluated"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := gottp.CompileTemplate(tc.tmpl)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if w := strings.Join(c.GetWarnings(), "\n"); !containsAll(w, tc.warn) {
				t.Errorf("warnings %q, want %q", w, tc.warn)
			}
			res, err := c.Parse(gottp.Inputs{"Default_Input": tc.data}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprint(res); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestTypeMethodWarnings (refactor F2, #36): a name that is a ported Python
// method of some value type isn't unknown, so it doesn't warn; a Python
// method gottp doesn't port (one that changes the value in place, or whose
// result has no JSON form) warns that the value is kept; a name that is a
// method of no type still warns as unknown. A custom function of the name
// drops the not-ported warning.
func TestTypeMethodWarnings(t *testing.T) {
	cases := []struct {
		fn   string
		want []string // substrings of one warning; nil: no warning
	}{
		{"title", nil},
		{"zfill(9)", nil},
		{"to_int | bit_length", nil},
		{"to_int | real", nil},
		{"split(',') | index('1')", nil},
		{"upper", nil},
		{"split(',') | sort", []string{`"sort"`, "changes the value in place", "None"}},
		{"split(',') | reverse", []string{`"reverse"`, "changes the value in place"}},
		{"encode", []string{`"encode"`, "bytes"}},
		{"nosuchfn", []string{"unknown match variable function", "nosuchfn"}},
	}
	for _, tc := range cases {
		t.Run(tc.fn, func(t *testing.T) {
			warnings := compileWarnings(t, "x {{ v | "+tc.fn+" }}\n")
			if tc.want == nil {
				if len(warnings) > 0 {
					t.Errorf("want no warning, got %q", warnings)
				}
				return
			}
			for _, w := range warnings {
				if containsAll(w, tc.want) {
					return
				}
			}
			t.Errorf("want a warning containing %q, got %q", tc.want, warnings)
		})
	}

	c, err := gottp.CompileTemplateWithOptions("x {{ v | sort }}\n", &gottp.CompileOptions{
		Functions: &gottp.Functions{Match: map[string]gottp.MatchFunc{
			"sort": func(v interface{}, _ []string, _ map[string]interface{}) (interface{}, error) { return v, nil },
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if w := c.GetWarnings(); len(w) > 0 {
		t.Errorf("custom sort: want no warning, got %q", w)
	}
}

// TestTypeMethodRaiseFailsParse: Python TTP catches only AttributeError
// from a type method; any other exception fails its parse, and gottp's.
func TestTypeMethodRaiseFailsParse(t *testing.T) {
	c, err := gottp.CompileTemplate("x {{ v | index(\"z\") }}\n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Parse(gottp.Inputs{"Default_Input": "x abc\n"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "ValueError: substring not found") {
		t.Fatalf("want the ValueError, got %v", err)
	}
	// A value whose type has no such method (AttributeError) is kept.
	c, err = gottp.CompileTemplate("x {{ v | to_int | index(\"z\") }}\n")
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Parse(gottp.Inputs{"Default_Input": "x 5\n"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s := fmt.Sprint(got); !strings.Contains(s, "v:5") {
		t.Errorf("want v kept as 5, got %s", s)
	}
}
