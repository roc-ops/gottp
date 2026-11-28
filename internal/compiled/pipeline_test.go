package compiled

import (
	"reflect"
	"testing"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/parser"
)

func compileForTest(t *testing.T, text string) *compiler.CompiledTemplate {
	t.Helper()
	tmpl, err := parser.ParseTemplate(text)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	compiled, err := compiler.NewCompiler().CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile template: %v", err)
	}
	return compiled
}

func stepNames(pl *groupPipeline) []string {
	var names []string
	for _, s := range pl.steps {
		names = append(names, s.name)
	}
	return names
}

// The function list follows template attribute order, with functions=
// expanded in place, as TTP's _group_class builds it.
func TestPipelineStepsFollowAttributeOrder(t *testing.T) {
	compiled := compileForTest(t, `<template>
<macro>
def m(data):
    return data
</macro>
<group name="g" record="x" functions="contains('x') | macro('m') | set('a', 'b')" void="" del="x" macro="m">
a {{ x }}
</group>
</template>`)
	g := compiled.Groups[0]
	r := NewRuntime(compiled)
	got := stepNames(r.buildPipeline(g, nil))
	want := []string{"record", "contains", "macro", "set", "void", "del", "macro"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("steps = %v, want %v", got, want)
	}

	// Without attribute order (a hand-built group, or an old serialized
	// template) the order is the fixed one gottp used before E1.
	g.AttrOrder = nil
	got = stepNames(r.buildPipeline(g, nil))
	want = []string{"record", "del", "contains", "macro", "set", "macro", "void"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fallback steps = %v, want %v", got, want)
	}
}

// Defaults come from default= (string), default() and unconditional set();
// default() and set() override the default= string, as in TTP's
// group.defaults.
func TestPipelineDefaults(t *testing.T) {
	compiled := compileForTest(t, `<group name="g" default="dflt">
interface {{ name }}
 description {{ desc | default("d") }}
 mtu {{ mtu }}
 {{ kind | set("phy") }}
</group>`)
	r := NewRuntime(compiled)
	pl := r.buildPipeline(compiled.Groups[0], nil)
	want := map[string]interface{}{"name": "dflt", "desc": "d", "mtu": "dflt", "kind": "phy"}
	if !reflect.DeepEqual(pl.defaults, want) {
		t.Errorf("defaults = %v, want %v", pl.defaults, want)
	}

	rec := map[string]interface{}{"name": "A"}
	value, keep, itemized, err := r.runPipeline(pl, rec, nil, false, nil)
	if err != nil || !keep || itemized {
		t.Fatalf("runPipeline: keep=%v itemized=%v err=%v", keep, itemized, err)
	}
	if got := value.(map[string]interface{}); !reflect.DeepEqual(got, map[string]interface{}{"name": "A", "desc": "d", "mtu": "dflt", "kind": "phy"}) {
		t.Errorf("record = %v", got)
	}

	// A merge_with_last record gets no defaults.
	value, _, _, _ = r.runPipeline(pl, map[string]interface{}{"mtu": "9"}, nil, true, nil)
	if got := value.(map[string]interface{}); !reflect.DeepEqual(got, map[string]interface{}{"mtu": "9"}) {
		t.Errorf("merge_with_last record = %v", got)
	}
}

func TestDefaultArg(t *testing.T) {
	cases := map[string]interface{}{
		`"x"`:                "x",
		`'x'`:                "x",
		``:                   "None",
		`False`:              false,
		`None`:               nil,
		`5`:                  int64(5),
		`[]`:                 []interface{}{},
		`{"defined": False}`: map[string]interface{}{"defined": false},
		`word`:               "word",
	}
	for in, want := range cases {
		got := defaultArg(in)
		if n, ok := got.(int); ok {
			got = int64(n)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("defaultArg(%q) = %#v, want %#v", in, got, want)
		}
	}
}
