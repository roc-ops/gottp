package comparison

import (
	"fmt"
	"strings"
	"testing"

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
