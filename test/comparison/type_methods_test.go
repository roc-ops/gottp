package comparison

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	gottp "github.com/roc-ops/gottp"
	"github.com/roc-ops/gottp/internal/pymethod"
)

// Refactor F2 (#36): a match variable function that isn't a TTP function
// runs the value's Python 3.12 method of that name, as Python TTP's
// getattr fallback does (ttp.py:2464). These cases run through both
// engines end to end.

// typeMethodCase is one chain on one value. The value is captured with
// ORPHRASE, so it may hold single spaces.
type typeMethodCase struct {
	chain string
	value string
}

// typeMethodCases cover every ported method that a match value can reach:
// str methods on ASCII and non-ASCII values, the True/False filters, and
// the int, float, list and dict methods after to_int, to_float, split and
// ip_info. Left out: the dict views keys, values and items (Python TTP saves
// a dict_keys object, which has no JSON form; see KNOWN_DIFFERENCES), and
// count, isdigit, join and copy, which are TTP match functions, so they
// never reach the fallback.
var typeMethodCases = []typeMethodCase{
	{`capitalize`, `hello wORLD`}, {`capitalize`, `ßtraße`},
	{`casefold`, `Straße`}, {`casefold`, `ΣΑΣ`},
	{`lower`, `ÀÉÎ ΣΑΣ`}, {`lower`, `İstanbul`},
	{`upper`, `straße ﬁ`}, {`upper`, `abc`},
	{`swapcase`, `Hello ßΣ`},
	{`title`, `hello-world they're`}, {`title`, `ǆa ßa`},
	{`isalpha`, `abc`}, {`isalpha`, `ab1`}, {`isalpha`, `héllo`},
	{`isalnum`, `abc123`}, {`isalnum`, `a-b`},
	{`isdecimal`, `123`}, {`isdecimal`, `²`}, {`isdecimal`, `١٢٣`},
	{`isnumeric`, `½`}, {`isnumeric`, `1a`},
	{`isspace`, `abc`},
	{`isascii`, `abc`}, {`isascii`, `é`},
	{`isprintable`, `abc def`},
	{`isidentifier`, `_foo1`}, {`isidentifier`, `1foo`}, {`isidentifier`, `héllo`},
	{`islower`, `abc1`}, {`islower`, `aBc`}, {`islower`, `ß`},
	{`isupper`, `ABC`}, {`isupper`, `ÀBc`},
	{`istitle`, `Hello World`}, {`istitle`, `Hello world`},
	{`center(12, "*")`, `abc`}, {`center(7, "é")`, `ñ`}, {`center(2)`, `abc`},
	{`ljust(6, "-")`, `ab`}, {`rjust(6, "0")`, `ñb`}, {`zfill(6)`, `-42`}, {`zfill(9)`, `123`},
	{`find("2")`, `12345`}, {`find("a", 2)`, `ñaña`}, {`find("z")`, `abc`},
	{`rfind("b")`, `abcabc`}, {`index("c")`, `abc`}, {`rindex("a")`, `abcabc`},
	{`startswith("he")`, `hello`}, {`startswith(("x", "hel"))`, `hello`}, {`startswith("x")`, `hello`},
	{`endswith("lo")`, `hello`}, {`endswith("x")`, `hello`}, {`endswith("dú")`, `ñandú`},
	{`strip("x")`, `xxhixx`}, {`strip("ab")`, `abcba`}, {`lstrip("x")`, `xxhi`}, {`rstrip(",")`, `1,2,3,`}, {`strip`, `hi`},
	{`removeprefix("pre-")`, `pre-x`}, {`removesuffix(",")`, `1,2,3,`}, {`removesuffix("dú")`, `ñandú`},
	{`replace("a", "b")`, `aXa`}, {`replace("a", "b", 2)`, `aaaa`}, {`replace("ñ", "n")`, `ñaña`},
	{`split(",")`, `a, b,c`}, {`split`, `a b c`}, {`split(None, 1)`, `a b c`}, {`split(",", maxsplit=1)`, `a,b,c`},
	{`rsplit(",", 1)`, `a,b,c`}, {`rsplit`, `a b c`},
	{`splitlines`, `a b`},
	{`partition(",")`, `a,b,c`}, {`partition(",")`, `abc`}, {`rpartition(",")`, `a,b,c`},
	{`expandtabs`, `a b`},
	{`format("x", "y")`, `{}-{}`}, {`format("ñ", 3)`, `[{:>4}|{:05d}]`},
	// repr escapes what str.isprintable rejects: NBSP, NEL, a line separator
	{`format("a\xa0b")`, `{!r}`}, {`format("\x85\u2028\U000e0001")`, `{!r}`},
	{`format_map({"a": 1})`, `<{a}>`},
	{`maketrans("ab", "xy")`, `abc`},
	{`translate({97: "x", 98: None})`, `abc`},
	// chained: the result of one method is the next one's value
	{`upper | center(9, "*") | lower`, `ñab`},
	{`split(",") | index("b")`, `a,b,c`},
	{`split(",") | index("b", 1)`, `a,b,c`},
	{`to_int | bit_length`, `255`}, {`to_int | bit_count`, `255`}, {`to_int | conjugate`, `-5`},
	{`to_int | as_integer_ratio`, `-6`}, {`to_int | is_integer`, `5`},
	{`to_int | real`, `5`}, {`to_int | imag`, `5`}, {`to_int | numerator`, `5`}, {`to_int | denominator`, `5`},
	{`to_float | is_integer`, `2.0`}, {`to_float | is_integer`, `2.5`}, {`to_float | conjugate`, `2.5`},
	{`to_float | as_integer_ratio`, `0.75`}, {`to_float | hex`, `1.5`}, {`to_float | real`, `2.5`}, {`to_float | imag`, `2.5`},
	{`find("b") | bit_length`, `abc`},
	// a name the value's type doesn't have: the value is kept
	{`to_int | upper`, `5`}, {`bit_length`, `abc`}, {`split(",") | upper`, `a,b`}, {`nosuchfn`, `abc`},
}

// typeMethodRaises are the calls Python raises on (ValueError, TypeError,
// ...). TTP doesn't catch the exception, so the parse fails; gottp's Parse
// fails too.
var typeMethodRaises = []typeMethodCase{
	{`index("z")`, `abc`},
	{`rindex("z")`, `abc`},
	{`split(",") | index("z")`, `a,b`},
	{`center("a")`, `abc`},
	{`center(9, "**")`, `abc`},
	{`partition("")`, `abc`},
	{`startswith(["a"])`, `abc`},
	{`endswith(["c"])`, `abc`},
	{`upper(1)`, `abc`},
	{`format()`, `{}`},
	{`format("x")`, `{}-{}`},
	{`to_float | as_integer_ratio | index(9)`, `0.5`},
}

func typeMethodTemplate(cases []typeMethodCase) (template, data string) {
	var tb, db strings.Builder
	for i, c := range cases {
		name := caseName(i)
		fmt.Fprintf(&tb, "<group name=\"%s\">\n%s {{ v | ORPHRASE | %s }}\n</group>\n", name, name, c.chain)
		fmt.Fprintf(&db, "%s %s\n", name, c.value)
	}
	return tb.String(), db.String()
}

// caseName names case i with letters only: TTP turns the digits of a
// line's text into \d+, so "c1" would also match "c2" lines.
func caseName(i int) string {
	return "case" + string(rune('a'+i/26)) + string(rune('a'+i%26))
}

// TestTypeMethodsMatchPythonTTP runs the cases in batches of one template
// each, one group per case.
func TestTypeMethodsMatchPythonTTP(t *testing.T) {
	const batch = 40
	for start := 0; start < len(typeMethodCases); start += batch {
		end := min(start+batch, len(typeMethodCases))
		cases := typeMethodCases[start:end]
		template, data := typeMethodTemplate(cases)
		t.Run(fmt.Sprintf("cases_%d_%d", start, end-1), func(t *testing.T) {
			RunComparison(t, fmt.Sprintf("type_methods_%d", start), template, data, nil, nil)
			if t.Failed() {
				for i, c := range cases {
					t.Logf("%s: %s on %q", caseName(i), c.chain, c.value)
				}
			}
		})
	}
}

// TestTypeMethodsRaiseLikePythonTTP checks that each raising call fails
// both engines' parse.
func TestTypeMethodsRaiseLikePythonTTP(t *testing.T) {
	requirePythonTTP(t)
	for _, c := range typeMethodRaises {
		t.Run(c.chain, func(t *testing.T) {
			template, data := typeMethodTemplate([]typeMethodCase{c})
			if _, err := runPythonTTP(template, data, nil, nil); err == nil {
				t.Fatalf("Python TTP doesn't fail on %s", c.chain)
			}
			if _, err := runGoTTP(template, data, nil, nil); err == nil {
				t.Errorf("gottp doesn't fail on %s, as Python TTP does", c.chain)
			} else if !strings.Contains(err.Error(), "Python TTP's parse fails") {
				t.Errorf("gottp fails on %s with an unexpected error: %v", c.chain, err)
			}
		})
	}
}

// typeMethodsNotCompared are the ported methods the comparison cases leave
// out, and why.
var typeMethodsNotCompared = map[string]string{
	"count":   "a TTP match function: str.count and list.count are never reached",
	"join":    "a TTP match function: str.join is never reached",
	"copy":    "a TTP match function: list.copy and dict.copy are never reached",
	"isdigit": "a TTP match function: str.isdigit is never reached",
	"get":     "TTP reads get(x) as re(x) (its pattern lookup finds patterns.get); see KNOWN_DIFFERENCES",
	"keys":    "TTP saves a dict_keys object, which has no JSON form; see KNOWN_DIFFERENCES",
	"values":  "TTP saves a dict_values object, which has no JSON form",
	"items":   "TTP saves a dict_items object, which has no JSON form",
}

// TestTypeMethodCasesCoverEveryMethod checks that the comparison cases
// call every ported method, except those listed in typeMethodsNotCompared.
func TestTypeMethodCasesCoverEveryMethod(t *testing.T) {
	called := map[string]bool{}
	for _, cases := range [][]typeMethodCase{typeMethodCases, typeMethodRaises} {
		for _, c := range cases {
			for _, part := range strings.Split(c.chain, "|") {
				name, _, _ := strings.Cut(strings.TrimSpace(part), "(")
				called[name] = true
			}
		}
	}
	for _, typ := range []string{"str", "int", "float", "list", "dict"} {
		for _, name := range pymethod.Names(typ) {
			if !called[name] && typeMethodsNotCompared[name] == "" {
				t.Errorf("no comparison case calls %s.%s", typ, name)
			}
		}
	}
}

// raiseScopeCase is a template whose type method raises on some input
// lines. TTP's run_re runs a group's regexes, and so their functions, only
// on part of the input: start patterns everywhere, the others from the
// first start match on, ordinary ones only up to the line of the last end
// match, and a child group's on its own part of its parent's region. A
// raise fails the parse only on a line TTP evaluates.
type raiseScopeCase struct {
	name, template, data string
	raises               bool // TTP's parse fails
}

const raiseScopeFlat = `<group name="i">
interface {{ name }}
 description {{ d | rindex("-") }}
</group>
`

const raiseScopeFlatEnd = `<group name="i">
interface {{ name }}
 description {{ d | rindex("-") }}
 exit {{ e | rindex("-") }} {{ _end_ }}
</group>
`

const raiseScopeEndLine = `<group name="i">
interface {{ name }}
 {{ x | rindex("-") }} {{ y }}
 exit {{ e }} {{ _end_ }}
</group>
`

const raiseScopeChild = `<group name="i">
interface {{ name }}
 <group name="ip">
 ip address {{ ip }}
 ip helper {{ h | rindex("-") }}
 </group>
</group>
`

const raiseScopeChildEnd = `<group name="i">
interface {{ name }}
 <group name="ip">
 ip address {{ ip }}
 {{ x | rindex("-") }} {{ y }}
 exit {{ e }} {{ _end_ }}
 </group>
</group>
`

var raiseScopeCases = []raiseScopeCase{
	// not evaluated by TTP: no raise
	{"flat_before_first_start", raiseScopeFlat, "hostname r1\nbanner\n description none\ninterface e0\n description to-core\n", false},
	{"flat_no_start_line", raiseScopeFlat, "hostname r1\n description core-uplink\n description none\n", false},
	{"flat_after_last_end", raiseScopeFlatEnd, "interface e0\n description to-core\n exit a-b\n description none\n", false},
	{"flat_end_line_before_first_start", raiseScopeFlatEnd, " exit z\ninterface e0\n description a-b\n", false},
	{"flat_end_line_not_ordinary", raiseScopeEndLine, "interface e0\n exit a\n", false},
	{"child_before_child_start", raiseScopeChild, " ip helper none\ninterface e0\n ip helper none\n ip address 1.1.1.1\n ip helper a-b\n", false},
	{"child_after_child_end", raiseScopeChildEnd, "interface e0\n ip address 1.1.1.1\n fil-ler a\n fil-ler b\n exit a\n bad c\n", false},
	{"child_no_parent_start", raiseScopeChild, "hostname x\n ip address 1.1.1.1\n ip helper none\n", false},
	// evaluated by TTP: the parse fails
	{"flat_second_record", raiseScopeFlat, "interface e0\n description a-b\ninterface e1\n description none\n", true},
	{"flat_start_line", `<group name="i">
interface {{ name | rindex("-") }}
</group>
`, "hostname r1\ninterface e0\n", true},
	{"flat_end_line", raiseScopeFlatEnd, "interface e0\n description a-b\n exit z\n", true},
	{"flat_between_end_lines", raiseScopeFlatEnd, "interface e0\n description a-b\n exit a-b\n description none\n exit c-d\n", true},
	{"flat_dropped_by_group_filter", `<group name="i" contains="zzz">
interface {{ name }}
 description {{ d | rindex("-") }}
</group>
`, "interface e0\n description none\n", true},
	{"flat_argument_error", `<group name="i">
interface {{ name }}
 description {{ d | center("a") }}
</group>
`, "interface e0\n description to-core\n", true},
	{"child_in_second_parent_record", raiseScopeChild, "interface e0\n ip address 1.1.1.1\n ip helper a-b\ninterface e1\n ip helper none\n", true},
	// TTP compares the end of the last start match with a relative offset
	// (run_re's se), so here the region isn't narrowed and the end line is
	// evaluated.
	{"child_end_line_not_narrowed", raiseScopeChildEnd, "interface e0\n ip address 1.1.1.1\n exit a\n", true},
}

// TestTypeMethodRaiseScopeLikePythonTTP runs each raiseScopeCase through
// both engines: both fail, or both give the same result. ParseStream fails
// exactly where Parse does.
func TestTypeMethodRaiseScopeLikePythonTTP(t *testing.T) {
	requirePythonTTP(t)
	for _, c := range raiseScopeCases {
		t.Run(c.name, func(t *testing.T) {
			_, pyErr := runPythonTTP(c.template, c.data, nil, nil)
			_, goErr := runGoTTP(c.template, c.data, nil, nil)
			if (pyErr != nil) != c.raises {
				t.Fatalf("Python TTP's error: %v; want a failing parse: %v", pyErr, c.raises)
			}
			switch {
			case pyErr != nil && goErr == nil:
				t.Fatalf("Python TTP fails (%v), gottp doesn't", pyErr)
			case pyErr != nil:
				if !strings.Contains(goErr.Error(), "Python TTP's parse fails") {
					t.Fatalf("gottp fails with an unexpected error: %v", goErr)
				}
			case goErr != nil:
				t.Fatalf("gottp fails (%v), Python TTP doesn't", goErr)
			default:
				RunComparison(t, "raise_scope_"+c.name, c.template, c.data, nil, nil)
			}

			ct, err := gottp.CompileTemplate(c.template)
			if err != nil {
				t.Fatal(err)
			}
			inputs := gottp.Inputs{"Default_Input": c.data}
			_, parseErr := ct.Parse(inputs, nil, nil)
			streamErr := ct.ParseStream(inputs, nil, nil, func(map[string]interface{}, [2]int, string) error { return nil })
			if errors.Is(streamErr, gottp.ErrTemplateNotStreamable) {
				return
			}
			if fmt.Sprint(streamErr) != fmt.Sprint(parseErr) {
				t.Errorf("ParseStream error %v, Parse error %v", streamErr, parseErr)
			}
		})
	}
}
