package pymethod

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/roc-ops/gottp/internal/call"
)

// methodCase is a call and what Python 3.12 gives for it (checked against
// Python by TestMethodsMatchPython where the same call is a diffCase).
type methodCase struct {
	self  interface{}
	call  string
	want  interface{} // the result; ignored when raise or missing is set
	raise string      // the exception class Python raises
	miss  bool        // AttributeError: TTP keeps the value
}

func runMethodCases(t *testing.T, cases []methodCase) {
	t.Helper()
	for _, c := range cases {
		cl, err := call.Parse(c.call)
		if err != nil {
			t.Fatalf("%s: %v", c.call, err)
		}
		got, found, err := Call(c.self, cl)
		switch {
		case c.miss:
			if found {
				t.Errorf("%#v.%s: found, want AttributeError", c.self, c.call)
			}
		case c.raise != "":
			e, ok := err.(*Error)
			if !found || !ok || e.Type != c.raise {
				t.Errorf("%#v.%s: (%#v, %v, %v), want %s", c.self, c.call, got, found, err, c.raise)
			}
		case !found || err != nil:
			t.Errorf("%#v.%s: found %v, err %v", c.self, c.call, found, err)
		case !reflect.DeepEqual(got, c.want):
			t.Errorf("%#v.%s = %#v, want %#v", c.self, c.call, got, c.want)
		}
	}
}

func list(v ...interface{}) []interface{} { return v }

func TestStrMethods(t *testing.T) {
	runMethodCases(t, []methodCase{
		{self: "hello wORLD", call: "capitalize", want: "Hello world"},
		{self: "ßtraße", call: "capitalize", want: "Sstraße"},
		{self: "Straße", call: "casefold", want: "strasse"},
		{self: "ΣΑΣ", call: "lower", want: "σας"},
		{self: "straße", call: "upper", want: "STRASSE"},
		{self: "Hello ßΣ", call: "swapcase", want: "hELLO SSς"},
		{self: "hello-world they're", call: "title", want: "Hello-World They'Re"},
		{self: "abç", call: "isalpha", want: true},
		{self: "ab1", call: "isalpha", want: false},
		{self: "", call: "isalpha", want: false},
		{self: "½", call: "isalnum", want: true},
		{self: "١٢٣", call: "isdecimal", want: true},
		{self: "²", call: "isdecimal", want: false},
		{self: "²", call: "isdigit", want: true},
		{self: "½", call: "isnumeric", want: true},
		{self: " \t\x1c ", call: "isspace", want: true},
		{self: "é", call: "isascii", want: false},
		{self: "", call: "isascii", want: true},
		{self: "a\tb", call: "isprintable", want: false},
		{self: "_foo1", call: "isidentifier", want: true},
		{self: "1foo", call: "isidentifier", want: false},
		{self: "abc1", call: "islower", want: true},
		{self: "123", call: "islower", want: false},
		{self: "ABC1", call: "isupper", want: true},
		{self: "Hello World", call: "istitle", want: true},
		{self: "Hello world", call: "istitle", want: false},
		{self: "abc", call: `center(12, "*")`, want: "****abc*****"},
		{self: "ab", call: `center(5, "é")`, want: "ééabé"},
		{self: "abc", call: `center("a")`, raise: "TypeError"},
		{self: "abc", call: `center(9, "**")`, raise: "TypeError"},
		{self: "ab", call: `ljust(5, "-")`, want: "ab---"},
		{self: "ab", call: `rjust(5, "0")`, want: "000ab"},
		{self: "-42", call: "zfill(6)", want: "-00042"},
		{self: "banana", call: `count("a")`, want: 3},
		{self: "banana", call: `count("")`, want: 7},
		{self: "ñaña", call: `find("a", 2)`, want: 3},
		{self: "abc", call: `find("z")`, want: -1},
		{self: "abcabc", call: `rfind("b")`, want: 4},
		{self: "abc", call: `index("z")`, raise: "ValueError"},
		{self: "abc", call: `rindex("z")`, raise: "ValueError"},
		{self: "hello", call: `startswith(("x", "hel"))`, want: true},
		{self: "hello", call: `startswith(["x", "hel"])`, raise: "TypeError"},
		{self: "hello", call: `endswith("l", 0, 4)`, want: true},
		{self: "hello", call: "endswith", raise: "TypeError"},
		{self: "　hi ", call: "strip", want: "hi"},
		{self: "abcba", call: `strip("ab")`, want: "c"},
		{self: "xxhi", call: `lstrip("x")`, want: "hi"},
		{self: "1,2,3,", call: `rstrip(",")`, want: "1,2,3"},
		{self: "pre-x", call: `removeprefix("pre-")`, want: "x"},
		{self: "1,2,3,", call: `removesuffix(",")`, want: "1,2,3"},
		{self: "aaaa", call: `replace("a", "b", 2)`, want: "bbaa"},
		{self: "abc", call: `replace("", "-")`, want: "-a-b-c-"},
		{self: "a, b", call: `split(",")`, want: list("a", " b")},
		{self: "  a  b c  ", call: "split", want: list("a", "b", "c")},
		{self: "a b c", call: "split(None, 1)", want: list("a", "b c")},
		{self: "abc", call: `split("")`, raise: "ValueError"},
		{self: "  a  b c  ", call: "rsplit(None, 1)", want: list("  a  b", "c")},
		{self: "a,b,c", call: `rsplit(",", 1)`, want: list("a,b", "c")},
		{self: "a\nb\r\nc", call: "splitlines", want: list("a", "b", "c")},
		{self: "a\nb\r\n", call: "splitlines(True)", want: list("a\n", "b\r\n")},
		{self: "a,b,c", call: `partition(",")`, want: list("a", ",", "b,c")},
		{self: "abc", call: `rpartition(",")`, want: list("", "", "abc")},
		{self: "ab\tc", call: "expandtabs(4)", want: "ab  c"},
		{self: "-", call: `join(["a", "b"])`, want: "a-b"},
		{self: "-", call: "join([1, 2])", raise: "TypeError"},
		{self: "{}-{x}", call: `format(1, x="y")`, want: "1-y"},
		{self: "{:+,.2f}", call: "format(1234.5)", want: "+1,234.50"},
		{self: "{}", call: "format()", raise: "IndexError"},
		{self: "{a}", call: `format_map({"a": 1})`, want: "1"},
		{self: "abc", call: `maketrans("ab", "xy")`, want: map[string]interface{}{"97": 120, "98": 121}},
		{self: "abc", call: `translate({97: "x", 98: None})`, want: "xc"},
		{self: "abc", call: "encode", miss: true}, // not ported
	})
}

func TestIntMethods(t *testing.T) {
	big64, _ := new(big.Int).SetString("18446744073709551616", 10)
	runMethodCases(t, []methodCase{
		{self: int64(255), call: "bit_length", want: 8},
		{self: int64(-255), call: "bit_length", want: 8},
		{self: int64(255), call: "bit_count", want: 8},
		{self: int64(-5), call: "conjugate", want: -5},
		{self: int64(-6), call: "as_integer_ratio", want: list(-6, 1)},
		{self: int64(5), call: "is_integer", want: true},
		{self: int64(5), call: "real", want: 5},
		{self: int64(5), call: "imag", want: 0},
		{self: int64(5), call: "imag(3)", want: 0}, // an attribute ignores arguments
		{self: int64(5), call: "numerator", want: 5},
		{self: int64(5), call: "denominator", want: 1},
		{self: uint64(18446744073709551615), call: "bit_length", want: 64},
		{self: uint64(18446744073709551615), call: "real", want: uint64(18446744073709551615)},
		{self: big64, call: "bit_length", miss: true}, // not a value gottp makes
		{self: 5, call: "bit_length(1)", raise: "TypeError"},
		{self: true, call: "bit_length", want: 1},
		{self: true, call: "real", want: 1},
		{self: false, call: "as_integer_ratio", want: list(0, 1)},
		{self: int64(5), call: "upper", miss: true},
	})
}

func TestFloatMethods(t *testing.T) {
	runMethodCases(t, []methodCase{
		{self: 2.0, call: "is_integer", want: true},
		{self: 2.5, call: "is_integer", want: false},
		{self: 2.5, call: "conjugate", want: 2.5},
		{self: 0.75, call: "as_integer_ratio", want: list(3, 4)},
		{self: -0.1, call: "as_integer_ratio", want: list(-3602879701896397, 36028797018963968)},
		{self: 1.5, call: "hex", want: "0x1.8000000000000p+0"},
		{self: 5e-324, call: "hex", want: "0x0.0000000000001p-1022"},
		{self: 2.5, call: "real", want: 2.5},
		{self: 2.5, call: "imag", want: 0.0},
		{self: 2.5, call: "bit_length", miss: true},
	})
}

func TestListMethods(t *testing.T) {
	runMethodCases(t, []methodCase{
		{self: list("a", "b", "a"), call: `count("a")`, want: 2},
		{self: list(1, 1.0, true), call: "count(1)", want: 3},
		{self: list("a", "b"), call: `index("b")`, want: 1},
		{self: list("a", "b", "a"), call: `index("a", 1)`, want: 2},
		{self: list("a", "b"), call: `index("z")`, raise: "ValueError"},
		{self: list("a", "b"), call: "copy", want: list("a", "b")},
		{self: list("b", "a"), call: "sort", miss: true}, // not ported
		{self: list("a"), call: "upper", miss: true},
	})
}

func TestDictMethods(t *testing.T) {
	d := map[string]interface{}{"b": 2, "a": list(1)}
	runMethodCases(t, []methodCase{
		{self: d, call: `get("b")`, want: 2},
		{self: d, call: `get("z")`, want: nil},
		{self: d, call: `get("z", "d")`, want: "d"},
		{self: d, call: "get(1)", want: nil},
		{self: d, call: "keys", want: list("a", "b")},
		{self: d, call: "values", want: list(list(1), 2)},
		{self: d, call: "items", want: list(list("a", list(1)), list("b", 2))},
		{self: d, call: "copy", want: map[string]interface{}{"b": 2, "a": list(1)}},
		{self: d, call: "update", miss: true}, // not ported
		{self: d, call: `index("a")`, miss: true},
	})
}

func TestNames(t *testing.T) {
	for _, n := range []string{"title", "zfill", "bit_length", "real", "hex", "index", "keys"} {
		if !IsName(n) {
			t.Errorf("IsName(%q) = false", n)
		}
	}
	for _, n := range []string{"nosuchfn", "sort", "encode", "append", "update"} {
		if IsName(n) {
			t.Errorf("IsName(%q) = true", n)
		}
	}
	if got := Types("index"); got != "str, list" {
		t.Errorf(`Types("index") = %q`, got)
	}
	for _, n := range []string{"sort", "reverse", "pop", "update", "setdefault", "encode", "to_bytes"} {
		if _, ok := NotPorted("scope", n); !ok {
			t.Errorf("NotPorted(%q) = false", n)
		}
	}
	if _, ok := NotPorted("scope", "upper"); ok {
		t.Error(`NotPorted("upper") = true`)
	}
}
