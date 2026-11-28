package pymethod

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/roc-ops/gottp/internal/call"
	"github.com/roc-ops/gottp/internal/variable"
)

// pythonBin is GOTTP_PYTHON: a Python 3.12 with TTP 0.10.1. The
// differential tests skip without it; the method-table tests
// (methods_test.go) don't need it.
func pythonBin(t *testing.T) string {
	t.Helper()
	p := os.Getenv("GOTTP_PYTHON")
	if p == "" {
		t.Skip("GOTTP_PYTHON is not set: the differential tests need Python 3.12 with TTP 0.10.1")
	}
	return p
}

// diffCase is a method call on a value, both written as Python source.
type diffCase struct {
	Self string `json:"self"`
	Call string `json:"call"`
}

// diffCases cover every ported method, with ASCII and non-ASCII input,
// default and explicit arguments, and the calls that raise.
var diffCases = []diffCase{
	// case mapping
	{`"hello wORLD"`, `capitalize`}, {`"ßtraße"`, `capitalize`}, {`"ǆemal"`, `capitalize`}, {`""`, `capitalize`},
	{`"Straße ΣΑΣ"`, `casefold`}, {`"ﬁ İ"`, `casefold`},
	{`"ÀÉÎ ΣΑΣ ΣΑΣ."`, `lower`}, {`"İstanbul"`, `lower`}, {`"aΣ"`, `lower`}, {`"Σ"`, `lower`}, {`"aΣb"`, `lower`}, {`"a'Σ"`, `lower`},
	{`"straße ﬁ ŉ"`, `upper`}, {`"ǰ ΐ ᾳ"`, `upper`},
	{`"Hello ßΣ"`, `swapcase`}, {`"aBΣ"`, `swapcase`},
	{`"hello-world they're 3rd"`, `title`}, {`"ǆa ßa ﬁx"`, `title`}, {`"ΣΑΣ σας"`, `title`},
	// is* tests
	{`"abc"`, `isalpha`}, {`"abç"`, `isalpha`}, {`"ab1"`, `isalpha`}, {`""`, `isalpha`},
	{`"abc123"`, `isalnum`}, {`"½"`, `isalnum`}, {`"a b"`, `isalnum`},
	{`"123"`, `isdecimal`}, {`"١٢٣"`, `isdecimal`}, {`"²"`, `isdecimal`},
	{`"²"`, `isdigit`}, {`"½"`, `isdigit`}, {`"12"`, `isdigit`},
	{`"½Ⅻ"`, `isnumeric`}, {`"1a"`, `isnumeric`},
	{`" \t\n\x1c\u2003"`, `isspace`}, {`"\u200b"`, `isspace`}, {`""`, `isspace`},
	{`"abc"`, `isascii`}, {`"é"`, `isascii`}, {`""`, `isascii`},
	{`"abc def"`, `isprintable`}, {`"a\tb"`, `isprintable`}, {`"\u00a0"`, `isprintable`}, {`""`, `isprintable`},
	{`"_foo1"`, `isidentifier`}, {`"1foo"`, `isidentifier`}, {`"héllo"`, `isidentifier`}, {`"℘x"`, `isidentifier`}, {`""`, `isidentifier`},
	{`"abc1"`, `islower`}, {`"aBc"`, `islower`}, {`"123"`, `islower`}, {`"ß"`, `islower`},
	{`"ABC1"`, `isupper`}, {`"ǅ"`, `isupper`}, {`"ÀB"`, `isupper`},
	{`"Hello World"`, `istitle`}, {`"Hello world"`, `istitle`}, {`"ǅungla"`, `istitle`}, {`"123"`, `istitle`},
	// padding
	{`"abc"`, `center(12, "*")`}, {`"abc"`, `center(10)`}, {`"ab"`, `center(5, "é")`}, {`"abc"`, `center(2)`},
	{`"abc"`, `center("a")`}, {`"abc"`, `center(9, "**")`}, {`"abc"`, `center(7.0)`},
	{`"ab"`, `ljust(5, "-")`}, {`"ñ"`, `ljust(3)`}, {`"ab"`, `rjust(5, "0")`}, {`"ab"`, `rjust(True)`},
	{`"123"`, `zfill(9)`}, {`"-42"`, `zfill(6)`}, {`"+x"`, `zfill(4)`}, {`"abc"`, `zfill(1)`}, {`""`, `zfill(3)`},
	// searching
	{`"banana"`, `count("a")`}, {`"banana"`, `count("an", 2)`}, {`"banana"`, `count("")`}, {`"banana"`, `count("", 10)`}, {`"ñaña"`, `count("ñ", -2)`},
	{`"12345"`, `find("2")`}, {`"ñaña"`, `find("a", 2)`}, {`"abc"`, `find("z")`}, {`"abc"`, `find("", 3)`}, {`"abc"`, `find("", 4)`}, {`"abc"`, `find("b", None, 1)`},
	{`"abcabc"`, `rfind("b")`}, {`"abcabc"`, `rfind("b", 0, 4)`}, {`"abc"`, `rfind("")`},
	{`"abc"`, `index("c")`}, {`"abc"`, `index("z")`}, {`"abcabc"`, `rindex("a")`}, {`"abc"`, `rindex("z")`}, {`"abc"`, `find(1)`},
	{`"hello"`, `startswith("he")`}, {`"hello"`, `startswith(("x", "hel"))`}, {`"hello"`, `startswith("l", 2)`}, {`"hello"`, `startswith("")`}, {`"hello"`, `startswith("", 9)`}, {`"héllo"`, `startswith("hé")`}, {`"hello"`, `startswith(1)`},
	{`"hello"`, `endswith("lo")`}, {`"hello"`, `endswith("l", 0, 4)`}, {`"hello"`, `endswith(("x", "y"))`}, {`"hello"`, `endswith`},
	// stripping
	{`"  hi \t\n"`, `strip`}, {`"xxhixx"`, `strip("x")`}, {`"\u3000hi\u2003"`, `strip`}, {`"\x1chi\x1f"`, `strip`}, {`"abcba"`, `strip("ab")`}, {`"  hi"`, `strip(None)`}, {`"hi"`, `strip(1)`},
	{`"  hi  "`, `lstrip`}, {`"xxhi"`, `lstrip("x")`}, {`"  hi  "`, `rstrip`}, {`"1,2,3,"`, `rstrip(",")`},
	{`"prefix-x"`, `removeprefix("prefix-")`}, {`"x"`, `removeprefix("y")`}, {`"1,2,3,"`, `removesuffix(",")`}, {`"ñandú"`, `removesuffix("dú")`},
	// replace and split
	{`"aXa"`, `replace("a", "b")`}, {`"aaaa"`, `replace("a", "b", 2)`}, {`"abc"`, `replace("", "-")`}, {`"abc"`, `replace("", "-", 2)`}, {`"ñaña"`, `replace("ñ", "n")`}, {`"aaa"`, `replace("a", "b", -1)`}, {`"a"`, `replace("a")`},
	{`"a,b,c"`, `split(",")`}, {`"a, b"`, `split(",")`}, {`"  a  b c  "`, `split`}, {`"a b c"`, `split(None, 1)`}, {`"a,b,c"`, `split(",", maxsplit=1)`}, {`"a,b,c"`, `split(sep=",")`},
	{`"a\u3000b\x1cc"`, `split`}, {`""`, `split`}, {`""`, `split(",")`}, {`"abc"`, `split("")`}, {`"a b"`, `split(maxsplit=0)`}, {`"  a b  "`, `split(None, 0)`}, {`"a"`, `split(x=1)`},
	{`"a,b,c"`, `rsplit(",", 1)`}, {`"  a  b c  "`, `rsplit(None, 1)`}, {`"a b c"`, `rsplit`}, {`"a,,b"`, `rsplit(",")`},
	{`"a\nb\r\nc\rd\x0be\x1cf\u2028g"`, `splitlines`}, {`"a\nb\r\n"`, `splitlines(True)`}, {`"a\n\nb"`, `splitlines(keepends=False)`}, {`""`, `splitlines`},
	{`"a,b,c"`, `partition(",")`}, {`"abc"`, `partition(",")`}, {`"a,b,c"`, `rpartition(",")`}, {`"abc"`, `rpartition(",")`}, {`"abc"`, `partition("")`},
	{`"a\tb\tc"`, `expandtabs`}, {`"ab\tc\nxy\tz"`, `expandtabs(4)`}, {`"ñ\tx"`, `expandtabs(tabsize=3)`}, {`"a\tb"`, `expandtabs(0)`},
	{`"-"`, `join(["a", "b"])`}, {`"-"`, `join("abc")`}, {`"-"`, `join([1, 2])`},
	// format
	{`"{}-{}"`, `format("a", 1)`}, {`"{1}{0}"`, `format("a", "b")`}, {`"{x}!"`, `format(x=3)`}, {`"{:>6}"`, `format("ab")`}, {`"{:*^7}"`, `format("ñ")`},
	{`"{:05d}"`, `format(42)`}, {`"{:+,}"`, `format(1234567)`}, {`"{:x} {:#X} {:#b} {:o}"`, `format(255, 255, 5, 8)`}, {`"{:010,}"`, `format(1234)`}, {`"{:08,}"`, `format(1234)`}, {`"{:_x}"`, `format(65535)`},
	{`"{:.2f}"`, `format(3.14159)`}, {`"{:e}"`, `format(12345.678)`}, {`"{:g}"`, `format(0.0001234)`}, {`"{:g}"`, `format(1234567.0)`}, {`"{:.3}"`, `format(123.0)`}, {`"{:.4}"`, `format(1.0)`}, {`"{}"`, `format(1e16)`},
	{`"{:%}"`, `format(0.25)`}, {`"{:.1%}"`, `format(0.125)`}, {`"{:,.2f}"`, `format(1234567.891)`}, {`"{:G}"`, `format(1e-10)`}, {`"{:#g}"`, `format(1.0)`}, {`"{:z.1f}"`, `format(-0.01)`}, {`"{:=+8.2f}"`, `format(-3.5)`},
	{`"{!r} {!s} {!a}"`, `format("é", "x", "é")`}, {`"{0[1]} {1[k]}"`, `format([1, 2], {"k": "v"})`}, {`"{:{w}}|"`, `format("a", w=4)`}, {`"{} {}"`, `format(True, None)`}, {`"{:d}"`, `format(True)`},
	{`"{:c}"`, `format(233)`}, {`"{0.real}"`, `format(5)`}, {`"{}"`, `format([1, "a"])`},
	{`"{}"`, `format()`}, {`"{0}{}"`, `format(1, 2)`}, {`"{"`, `format(1)`}, {`"}"`, `format(1)`}, {`"{x}"`, `format(1)`}, {`"{:d}"`, `format("a")`}, {`"{:s}"`, `format(1)`}, {`"{:.2d}"`, `format(1)`}, {`"{0.nope}"`, `format(1)`},
	{`"{a}-{b}"`, `format_map({"a": 1, "b": "x"})`}, {`"{a}"`, `format_map({})`}, {`"{}"`, `format_map({})`},
	{`"abc"`, `format_map(5)`}, {`"{a}"`, `format_map(5)`}, {`"{a}"`, `format_map([1])`}, {`"{a}"`, `format_map("x")`}, {`"{0}"`, `format_map({})`},
	// translate
	{`"abc"`, `maketrans("ab", "xy")`}, {`"abc"`, `maketrans("ab", "xy", "c")`}, {`"abc"`, `maketrans({"a": "1", 98: None})`}, {`"abc"`, `maketrans("a", "xy")`},
	{`"abc"`, `translate({97: "x", 98: None, 99: 100})`}, {`"ñab"`, `translate({241: "n"})`}, {`"abc"`, `translate({97: 1114112})`},
	// int
	{`255`, `bit_length`}, {`-255`, `bit_length`}, {`0`, `bit_length`}, {`255`, `bit_count`}, {`-7`, `bit_count`},
	{`5`, `conjugate`}, {`-6`, `as_integer_ratio`}, {`5`, `is_integer`}, {`5`, `real`}, {`5`, `imag`}, {`5`, `numerator`}, {`5`, `denominator`}, {`5`, `imag(3)`}, {`5`, `bit_length(1)`},
	{`18446744073709551615`, `bit_length`}, {`18446744073709551615`, `real`},
	// bool, an int subclass
	{`True`, `bit_length`}, {`True`, `real`}, {`False`, `as_integer_ratio`}, {`True`, `conjugate`}, {`True`, `upper`},
	// float
	{`2.0`, `is_integer`}, {`2.5`, `is_integer`}, {`2.5`, `conjugate`}, {`0.75`, `as_integer_ratio`}, {`-0.1`, `as_integer_ratio`}, {`1e300`, `as_integer_ratio`},
	{`1.5`, `hex`}, {`-0.1`, `hex`}, {`0.0`, `hex`}, {`-0.0`, `hex`}, {`5e-324`, `hex`}, {`1e308`, `hex`}, {`2.5`, `real`}, {`2.5`, `imag`},
	// list
	{`["a", "b", "a"]`, `count("a")`}, {`[1, 1.0, True]`, `count(1)`}, {`["a", "b"]`, `index("b")`}, {`["a", "b"]`, `index("z")`}, {`["a", "b", "a"]`, `index("a", 1)`}, {`["a", "b"]`, `copy`},
	// dict
	{`{"a": 1, "b": 2}`, `get("a")`}, {`{"a": 1}`, `get("z")`}, {`{"a": 1}`, `get("z", "d")`}, {`{"a": 1}`, `get(1)`},
	{`{"a": 1, "b": 2}`, `keys`}, {`{"a": 1, "b": 2}`, `values`}, {`{"a": 1, "b": [2]}`, `items`}, {`{"a": 1}`, `copy`},
	// AttributeError: TTP keeps the value
	{`5`, `upper`}, {`"a"`, `bit_length`}, {`["a"]`, `upper`}, {`2.5`, `bit_length`}, {`{"a": 1}`, `index("a")`}, {`"a"`, `nosuch`},
	// wrong arguments
	{`"a"`, `upper(1)`}, {`"a"`, `upper(x=1)`}, {`"a"`, `zfill`}, {`"a"`, `center(1, 2, 3)`}, {`"a"`, `count`}, {`"a"`, `split(1)`}, {`"a"`, `splitlines(1, 2)`},
}

func goRun(t *testing.T, c diffCase) map[string]interface{} {
	self := goValue(t, c.Self)
	cl, err := call.Parse(c.Call)
	if err != nil {
		t.Fatalf("%s: %v", c.Call, err)
	}
	r, found, err := Call(self, cl.Name, cl.Args, cl.Kwargs)
	switch {
	case !found:
		return map[string]interface{}{"kind": "attr"}
	case err != nil:
		e, _ := err.(*Error)
		if e == nil {
			t.Fatalf("%s.%s: error %v is not an *Error", c.Self, c.Call, err)
		}
		return map[string]interface{}{"kind": "raise", "type": e.Type, "msg": e.Msg}
	}
	return map[string]interface{}{"kind": "value", "repr": pyRepr(r)}
}

// goValue reads a case's value. An integer too big for an int64 is a
// uint64, as to_int gives it.
func goValue(t *testing.T, src string) interface{} {
	t.Helper()
	if u, err := strconv.ParseUint(src, 10, 64); err == nil && u > math.MaxInt64 {
		return u
	}
	v, _, err := variable.PythonValueAt(src, 0, false)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return v
}

func runPython(t *testing.T, cases []diffCase) []map[string]interface{} {
	t.Helper()
	in, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(pythonBin(t), "-I", "testdata/pyeval.py")
	cmd.Stdin = bytes.NewReader(in)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pyeval.py: %v\n%s", err, stderr.String())
	}
	var res []map[string]interface{}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("pyeval.py output: %v\n%s", err, out)
	}
	if len(res) != len(cases) {
		t.Fatalf("pyeval.py gave %d results for %d cases", len(res), len(cases))
	}
	return res
}

// TestMethodsMatchPython runs every case through gottp and Python 3.12 and
// compares the result's repr, or that both raise the same exception class,
// or that both have no such attribute. Messages are not compared.
func TestMethodsMatchPython(t *testing.T) {
	py := runPython(t, diffCases)
	for i, c := range diffCases {
		want := py[i]
		got := goRun(t, c)
		if got["kind"] != want["kind"] || got["repr"] != want["repr"] || got["type"] != want["type"] {
			t.Errorf("%s.%s:\n  python: %v\n  gottp:  %v", c.Self, c.Call, want, got)
		}
	}
}

// TestDiffCasesCoverEveryMethod checks that the differential cases call
// every ported method of every type.
func TestDiffCasesCoverEveryMethod(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range diffCases {
		typ := typeOf(goValue(t, c.Self))
		if typ == nil {
			continue
		}
		if typ == boolType {
			typ = intType
		}
		seen[typ.name+"."+call.Name(c.Call)] = true
	}
	for _, typ := range []string{"str", "int", "float", "list", "dict"} {
		for _, n := range Names(typ) {
			if !seen[typ+"."+n] {
				t.Errorf("no differential case calls %s.%s", typ, n)
			}
		}
	}
}
