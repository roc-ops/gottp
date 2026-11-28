package call

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// testdata/python.json is recorded from Python TTP 0.10.1 (get_attributes)
// and difflib.get_close_matches by testdata/gen_python.py:
//
//	/tmp/gottp-venv/bin/python -I testdata/gen_python.py testdata/python.json
type pythonFixture struct {
	Candidates []string              `json:"candidates"`
	Guess      map[string][]string   `json:"guess"`
	Calls      map[string]pythonCall `json:"calls"`
}

type pythonCall struct {
	Name   string                 `json:"name"`
	Args   []interface{}          `json:"args"`
	Kwargs map[string]interface{} `json:"kwargs"`
	Strs   []string               `json:"strs"`
}

func loadFixture(t *testing.T) pythonFixture {
	t.Helper()
	b, err := os.ReadFile("testdata/python.json")
	if err != nil {
		t.Fatal(err)
	}
	var f pythonFixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// jsonRound normalizes a value the way the fixture was decoded.
func jsonRound(t *testing.T, v interface{}) interface{} {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestGuessMatchesDifflib(t *testing.T) {
	f := loadFixture(t)
	for word, want := range f.Guess {
		got := Guess(word, f.Candidates)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Guess(%q) = %q, difflib gives %q", word, got, want)
		}
	}
}

func TestParseMatchesTTP(t *testing.T) {
	f := loadFixture(t)
	for text, want := range f.Calls {
		c, err := Parse(text)
		if err != nil {
			t.Errorf("Parse(%q): %v", text, err)
			continue
		}
		if c.Name != want.Name {
			t.Errorf("Parse(%q).Name = %q, TTP %q", text, c.Name, want.Name)
		}
		args := jsonRound(t, c.Args)
		if args == nil {
			args = []interface{}{}
		}
		if !reflect.DeepEqual(args, jsonRound(t, want.Args)) {
			t.Errorf("Parse(%q).Args = %v, TTP %v", text, args, want.Args)
		}
		kw := map[string]interface{}{}
		for _, k := range c.Kwargs {
			kw[k.Key] = k.Value
		}
		if !reflect.DeepEqual(jsonRound(t, kw), jsonRound(t, want.Kwargs)) {
			t.Errorf("Parse(%q).Kwargs = %v, TTP %v", text, kw, want.Kwargs)
		}
		strs := c.Strs
		if strs == nil {
			strs = []string{}
		}
		if !reflect.DeepEqual(strs, want.Strs) {
			t.Errorf("Parse(%q).Strs = %q, TTP str() gives %q", text, strs, want.Strs)
		}
	}
}

// TestParseNames: what TTP's call expression makes of text that isn't a
// well-formed call. A piece broken off by a "|" in quotes is a name.
func TestParseNames(t *testing.T) {
	for text, name := range map[string]string{
		`re("UP`:      `re("UP`,
		`DOWN")`:      `DOWN")`,
		`sformat("a`:  `sformat("a`,
		`{}")`:        `{}")`,
		`upper `:      `upper`,
		`x (1)`:       `x`,
		`_start_`:     `_start_`,
		`to_int()`:    `to_int`,
		`f(  )`:       `f`,
		"f(\"x\" )":   "f",
		`DOWN)")`:     `DOWN)")`,
		`re("(?:UP`:   `re("(?:UP`,
		`ORPHRASE`:    `ORPHRASE`,
		`foo(bar`:     `foo(bar`,
		`contains(a)`: `contains`,
	} {
		c, err := Parse(text)
		if err != nil {
			t.Errorf("Parse(%q): %v", text, err)
			continue
		}
		if c.Name != name {
			t.Errorf("Parse(%q).Name = %q, want %q", text, c.Name, name)
		}
	}
}

// TestParseErrors: calls Python TTP can't read (SystemExit).
func TestParseErrors(t *testing.T) {
	for _, text := range []string{
		`f("x)`,
		`f((x)`,
		`f(x))`,
		`a(b)(c)`,
		`f(Gig Eth)`,
		`f(-VM-)`,
		`f(1.1.1.1)`,
		`f(a=1, 2)`,
		`f(a=1, a=2)`,
		`f(,)`,
		`f(*x)`,
		`f(**x)`,
		`foo bar`,
		`f([1, 2)`,
		`f({"a" 1})`,
		"f(\"a\nb\")",
		`f(` + strings.Repeat("[", 500) + strings.Repeat("]", 500) + `)`,
	} {
		if c, err := Parse(text); err == nil {
			t.Errorf("Parse(%q) = %+v, want an error", text, c)
		}
	}
}

// TestParseFinishesQuickly guards the parser against unbounded work (#144):
// pathological input must fail or parse in well under a second.
func TestParseFinishesQuickly(t *testing.T) {
	inputs := []string{
		`macro(\"m\",\"n\")`,
		`f(` + strings.Repeat(`"\`, 5000) + `)`,
		`f(` + strings.Repeat(`(`, 100000) + `)`,
		strings.Repeat(`f(`, 50000) + strings.Repeat(`)`, 50000),
		`f(` + strings.Repeat(`a, `, 50000) + `)`,
		strings.Repeat("x", 100000) + "(",
	}
	for _, in := range inputs {
		done := make(chan struct{})
		go func() {
			_, _ = Parse(in)
			_, _ = ParseChain(in + "|" + in)
			_ = QuotedPipes(in + "|" + in)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("Parse(%.40q...) did not return within 2s", in)
		}
	}
}

func TestSplitIsQuoteBlind(t *testing.T) {
	got := Split(` re("UP|DOWN") | upper ||`)
	want := []string{`re("UP`, `DOWN")`, `upper`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %q, want %q", got, want)
	}
}

func TestQuotedPipes(t *testing.T) {
	got := QuotedPipes(`to_int | re("UP|DOWN") | sformat('a|b|{}') | upper`)
	want := []Broken{
		{Call: `re("UP|DOWN")`, Parts: []string{`re("UP`, `DOWN")`}},
		{Call: `sformat('a|b|{}')`, Parts: []string{`sformat('a`, `b`, `{}')`}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("QuotedPipes = %+v, want %+v", got, want)
	}
	if got := QuotedPipes(`re("a\"|b") | upper`); len(got) != 1 {
		t.Errorf("escaped quote: QuotedPipes = %+v, want one", got)
	}
	if got := QuotedPipes(`upper | lower`); got != nil {
		t.Errorf("QuotedPipes = %+v, want none", got)
	}
}

func TestResolvedStrs(t *testing.T) {
	c, err := Parse(`contains(v, "w", 3)`)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.ResolvedStrs(nil); &got[0] != &c.Strs[0] {
		t.Error("ResolvedStrs(nil) should return Strs itself")
	}
	got := c.ResolvedStrs(map[string]interface{}{"w": 7, "3": "no"})
	if want := []string{"v", "7", "3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ResolvedStrs = %q, want %q", got, want)
	}
	if c.Strs[1] != "w" {
		t.Error("ResolvedStrs changed Strs")
	}
}

func TestLookupOrder(t *testing.T) {
	builtin := func(name string) (string, bool) { return "builtin", name == "f" }
	if got, _ := Lookup("f", map[string]string{"f": "parse"}, map[string]string{"f": "compile"}, builtin); got != "parse" {
		t.Errorf("got %q, want the per-parse override", got)
	}
	if got, _ := Lookup("f", nil, map[string]string{"f": "compile"}, builtin); got != "compile" {
		t.Errorf("got %q, want the compile-time override", got)
	}
	if got, _ := Lookup("f", nil, nil, builtin); got != "builtin" {
		t.Errorf("got %q, want the built-in", got)
	}
	if _, ok := Lookup("g", nil, nil, builtin); ok {
		t.Error("unknown name found")
	}
}

func TestUnknownWarning(t *testing.T) {
	w := UnknownWarning(`match variable "v"`, "match variable", "to_itn", []string{"to_int", "upper"})
	if !strings.Contains(w, `"to_itn"`) || !strings.Contains(w, "to_int") || strings.Contains(w, "upper") {
		t.Errorf("warning %q", w)
	}
}
