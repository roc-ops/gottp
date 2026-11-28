package test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// Output macros (#142). test/comparison covers Starlark macros against
// Python TTP; these cover what the comparison suite can't: native Go
// macros, and errors where Python TTP raises.

const outputMacroGroup = `<group name="items">
item {{ name }}
</group>
`

func parseOutputMacro(t *testing.T, template string, opts *gottp.CompileOptions) (interface{}, error) {
	t.Helper()
	compiled, err := gottp.CompileTemplateWithOptions(template, opts)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return compiled.Parse(gottp.Inputs{"Default_Input": "item a\nitem b\n"}, nil, nil)
}

func TestOutputMacro_StarlarkPerInput(t *testing.T) {
	template := `<template results="per_input">
<macro>
def add_flag(data):
    data.append({"flag": True})
    return data
</macro>
` + outputMacroGroup + `<output macro="add_flag"/>
</template>`
	got, err := parseOutputMacro(t, template, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{
		map[string]interface{}{"items": []interface{}{
			map[string]interface{}{"name": "a"},
			map[string]interface{}{"name": "b"},
		}},
		map[string]interface{}{"flag": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func goMacroOpts() *gottp.CompileOptions {
	return &gottp.CompileOptions{Functions: &gottp.Functions{
		Macro: map[string]gottp.MacroFunc{
			"count_items": func(data map[string]interface{}, _ []string, _ map[string]interface{}) (map[string]interface{}, bool, error) {
				items, _ := data["items"].([]interface{})
				data["count"] = len(items)
				return data, true, nil
			},
			"drop_all": func(data map[string]interface{}, _ []string, _ map[string]interface{}) (map[string]interface{}, bool, error) {
				return nil, false, nil
			},
		},
	}}
}

func TestOutputMacro_GoMacroPerTemplate(t *testing.T) {
	for _, output := range []string{`<output macro="count_items"/>`, `<output functions="macro('count_items')"/>`} {
		template := `<template results="per_template">` + outputMacroGroup + output + `</template>`
		got, err := parseOutputMacro(t, template, goMacroOpts())
		if err != nil {
			t.Fatalf("%s: %v", output, err)
		}
		m, ok := got.(map[string]interface{})
		if !ok || m["count"] != 2 {
			t.Errorf("%s: got %#v, want items with count 2", output, got)
		}
	}
}

// A Go macro returning keep=false acts as a Python macro returning False:
// False replaces the results.
func TestOutputMacro_GoMacroFalse(t *testing.T) {
	template := `<template results="per_template">` + outputMacroGroup + `<output macro="drop_all"/></template>`
	got, err := parseOutputMacro(t, template, goMacroOpts())
	if err != nil {
		t.Fatal(err)
	}
	if got != false {
		t.Errorf("got %#v, want false", got)
	}
}

// Go macros take a map; per_input results are a list, so the macro can't
// run and the parse fails instead of silently skipping it.
func TestOutputMacro_GoMacroPerInputErrors(t *testing.T) {
	template := `<template results="per_input">` + outputMacroGroup + `<output macro="count_items"/></template>`
	_, err := parseOutputMacro(t, template, goMacroOpts())
	if err == nil || !strings.Contains(err.Error(), "count_items") {
		t.Errorf("got err %v, want an error naming count_items", err)
	}
}

// gottp extension: a comma-separated list runs each macro in order.
// Python TTP 0.10.1 turns macro="a, b" into macro(data, "a", "b"), which
// raises TypeError.
func TestOutputMacro_SeveralNamesRunInOrder(t *testing.T) {
	for _, output := range []string{`<output macro="a, b"/>`, `<output functions="macro('a', 'b')"/>`} {
		template := `<template results="per_template">
<macro>
def a(data):
    data["order"] = ["a"]
    return data
def b(data):
    data["order"] = data["order"] + ["b"]
    return data
</macro>
` + outputMacroGroup + output + `</template>`
		res, err := parseOutputMacro(t, template, nil)
		if err != nil {
			t.Fatalf("%s: %v", output, err)
		}
		if got := fmt.Sprint(res); !strings.Contains(got, "order:[a b]") {
			t.Errorf("%s: got %s, want order [a b]", output, got)
		}
	}
}

// A macro that fails at run time is an error, not a silent skip (#26).
func TestOutputMacro_RuntimeErrorPropagates(t *testing.T) {
	template := `<template>
<macro>
def boom(data):
    return data["missing"]
</macro>
` + outputMacroGroup + `<output macro="boom"/></template>`
	_, err := parseOutputMacro(t, template, nil)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("got err %v, want an error naming boom", err)
	}
}
