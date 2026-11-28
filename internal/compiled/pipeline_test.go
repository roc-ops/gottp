package compiled

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/call"
	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/functions/group"
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

func TestDefaultOf(t *testing.T) {
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
		`none`:               nil,
		`true`:               true,
		`FALSE`:              false,
	}
	for in, want := range cases {
		c, err := call.Parse("default(" + in + ")")
		if err != nil {
			t.Fatalf("default(%s): %v", in, err)
		}
		got := defaultOf(c)
		if n, ok := got.(int); ok {
			got = int64(n)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("default(%s) = %#v, want %#v", in, got, want)
		}
	}
}

// A parse uses the template's shared pipeline unless its vars name a
// variable the pipeline depends on, or ParseOptions.Functions overrides a
// group function; those parses build their own and leave the shared one
// alone.
func TestSharedPipelineOverrides(t *testing.T) {
	compiled := compileForTest(t, `<template>
<vars>
hostname="R1"
ch="set('x', 'tag')"
</vars>
<group name="intf" chain="ch">
interface {{ name }}
 hn:{{ hn | default("hostname") }}
</group>
</template>`)
	r := NewRuntime(compiled)
	inputs := map[string]string{"Default_Input": "interface Gi1\n"}
	parse := func(vars map[string]interface{}, opts *ParseOptions) string {
		t.Helper()
		res, err := r.Parse(inputs, vars, opts)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		return fmt.Sprint(res)
	}
	plain := parse(nil, nil)
	if !strings.Contains(plain, "hn:R1") || !strings.Contains(plain, "tag:x") {
		t.Fatalf("plain parse = %s", plain)
	}
	if got := parse(map[string]interface{}{"hostname": "R2"}, nil); !strings.Contains(got, "hn:R2") {
		t.Errorf("vars hostname=R2: %s", got)
	}
	if got := parse(nil, &ParseOptions{Vars: map[string]interface{}{"ch": "set('y', 'tag')"}}); !strings.Contains(got, "tag:y") {
		t.Errorf("ParseOptions.Vars ch: %s", got)
	}
	setY := func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		data["tag"] = "custom"
		return data, true, nil
	}
	if got := parse(nil, &ParseOptions{Functions: &RuntimeFunctions{Group: map[string]group.Function{"set": setY}}}); !strings.Contains(got, "tag:custom") {
		t.Errorf("ParseOptions.Functions set: %s", got)
	}
	// An unrelated var keeps the shared pipeline, and the overrides above
	// didn't change it.
	if got := parse(map[string]interface{}{"other": "z"}, nil); got != plain {
		t.Errorf("after overrides = %s, want %s", got, plain)
	}
}
