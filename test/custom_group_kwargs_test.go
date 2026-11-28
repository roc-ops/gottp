package test

import (
	"testing"

	"github.com/roc-ops/gottp"
)

// Custom attribute group functions must not receive gottp's live
// recorded-vars map (#141 review): user code could change internal state.
func TestCustomAttrGroupFunctionNoRecordedVarsKwarg(t *testing.T) {
	var sawKey bool
	opts := &gottp.CompileOptions{Functions: &gottp.Functions{Group: map[string]gottp.GroupFunc{
		"tagx": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
			if _, ok := kwargs["_recorded_vars"]; ok {
				sawKey = true
			}
			return data, true, nil
		},
	}}}
	tmpl := "<group name=\"g\" record=\"name, n\" tagx=\"\">\niface {{ name }}\n</group>"
	compiled, err := gottp.CompileTemplateWithOptions(tmpl, opts)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := compiled.Parse(gottp.Inputs{"Default_Input": "iface A\niface B\n"}, nil, nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sawKey {
		t.Error("custom attribute group function received _recorded_vars")
	}
}

// A custom group function called from functions= gets the same kwargs as one
// given as an attribute (refactor E1, #149): template vars, parse vars and
// recorded vars, with the record() value of the record before it, but not
// gottp's live _recorded_vars map or the record's _defaults.
func TestCustomChainGroupFunctionKwargs(t *testing.T) {
	var internal bool
	var seen []interface{}
	opts := &gottp.CompileOptions{Functions: &gottp.Functions{Group: map[string]gottp.GroupFunc{
		"tagx": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
			if _, ok := kwargs["_recorded_vars"]; ok {
				internal = true
			}
			if _, ok := kwargs["_defaults"]; ok {
				internal = true
			}
			seen = append(seen, kwargs["n"])
			return data, true, nil
		},
	}}}
	tmpl := "<group name=\"g\" functions=\"tagx\" record=\"name, n\">\niface {{ name }}\n</group>"
	compiled, err := gottp.CompileTemplateWithOptions(tmpl, opts)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := compiled.Parse(gottp.Inputs{"Default_Input": "iface A\niface B\n"}, nil, nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if internal {
		t.Error("custom chain group function received _recorded_vars or _defaults")
	}
	// tagx runs before record= on each record, so it sees the value the
	// previous record recorded.
	if len(seen) != 2 || seen[0] != nil || seen[1] != "A" {
		t.Errorf("kwargs[n] per record = %v, want [<nil> A]", seen)
	}
}
