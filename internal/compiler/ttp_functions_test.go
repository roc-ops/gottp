package compiler

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// TestTTPFunctionNames checks ttpFunctionNames against Python TTP 0.10.1's
// function tables (testdata/ttp_functions.json, recorded by
// testdata/gen_ttp_functions.py), and pins the TTP functions gottp doesn't
// implement (#157 review, N3).
func TestTTPFunctionNames(t *testing.T) {
	data, err := os.ReadFile("testdata/ttp_functions.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[Scope][]string
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ttpFunctionNames, want) {
		t.Errorf("ttpFunctionNames = %v\nwant (testdata/ttp_functions.json) %v", ttpFunctionNames, want)
	}
	got := map[Scope][]string{}
	for s, k := range scopeNames() {
		for n := range k.unsupported {
			got[s] = append(got[s], n)
		}
		slices.Sort(got[s])
	}
	wantUnsupported := map[Scope][]string{
		ScopeGroup:  {"lookup", "str_to_unicode"},
		ScopeOutput: {"deepdiff", "validate", "validate_yangson"},
	}
	if !reflect.DeepEqual(got, wantUnsupported) {
		t.Errorf("unsupported TTP functions = %v, want %v", got, wantUnsupported)
	}
}

// TestUnsupportedTTPFunctionWarning: a Python TTP function gottp doesn't
// implement is "not supported by gottp", with no suggestions, not unknown.
func TestUnsupportedTTPFunctionWarning(t *testing.T) {
	ct := compileText(t, `<group name="g" functions="lookup('x') | str_to_unicode">
v {{ x }}
</group>
<output functions="deepdiff | validate | validate_yangson"/>`)
	var n int
	for _, w := range ct.Warnings {
		if strings.Contains(w, "not supported by gottp") {
			n++
			if strings.Contains(w, "unknown") || strings.Contains(w, "most similar") {
				t.Errorf("warning %q: want no 'unknown' and no suggestions", w)
			}
		}
	}
	if n != 5 {
		t.Errorf("got %d 'not supported' warnings, want 5: %q", n, ct.Warnings)
	}
}
