package streaming_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// gateCases are templates using a group feature that parseGroupStream does
// not implement. Each streamed without error before #130 and gave records
// different from Parse; now ParseStream refuses them up front and
// WhyNotStreamable names the feature.
var gateCases = []struct {
	name     string
	template string
	input    string
	reason   string
}{
	{"chain itemize", `<group name="e*" chain="contains('ip') | itemize(key='name')">
interface {{ name }}
 ip {{ ip }}
</group>`, "interface a\n ip 1\n", `group function "itemize"`},
	{"functions pipe-separated", `<group name="e*" functions="contains('ip') | itemize(key='name')">
interface {{ name }}
 ip {{ ip }}
</group>`, "interface a\n ip 1\n", `group function "itemize"`},
	{"contains attribute", `<group name="e*" contains="ip">
interface {{ name }}
 ip {{ ip }}
</group>`, "interface a\ninterface b\n ip 1\n", "contains= attribute"},
	{"record attribute", `<group name="e*" record="vrf">
vrf {{ vrf }} rt {{ rt }}
</group>`, "vrf a rt 1\nvrf a rt 2\n", "record= attribute"},
	{"default attribute", `<group name="e*" default="none">
interface {{ name }}
 ip {{ ip }}
</group>`, "interface a\n", "default= attribute"},
	{"void", `<group name="e*" void="">
interface {{ name }}
</group>`, "interface a\n", "is void"},
	{"set() defaults", `<group name="e*">
interface {{ name }}
{{ enabled | set("yes") }}
</group>`, "interface a\n", "set() defaults"},
	{"method table", `<group name="e*" method="table">
a {{ x }}
b {{ y }}
</group>`, "a 1\nb 2\n", `method="table"`},
	{"dynamic path", `<group name="devices.{{ host }}">
{{ host }} {{ intf }}
</group>`, "r1 gi0\n", "dynamic path"},
	{"ignore with template var", `<vars>
st = "UP|DOWN"
</vars>
<group name="e*">
{{ n }} {{ ignore("st") }} {{ w }}
</group>`, "a UP 1\n", "ignore() with a template variable"},
}

func TestParseStreamGate_RejectsUnimplementedFeatures(t *testing.T) {
	for _, tc := range gateCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := gottp.CompileTemplate(tc.template)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			streamable, reasons := gottp.WhyNotStreamable(c)
			if streamable || !anyContains(reasons, tc.reason) {
				t.Errorf("WhyNotStreamable = %v %q, want a reason containing %q", streamable, reasons, tc.reason)
			}
			called := false
			err = c.ParseStream(gottp.Inputs{"Default_Input": tc.input}, nil, nil,
				func(map[string]interface{}, [2]int, string) error { called = true; return nil })
			var gateErr *gottp.TemplateNotStreamableError
			if !errors.As(err, &gateErr) {
				t.Fatalf("ParseStream error = %v, want *TemplateNotStreamableError", err)
			}
			if called {
				t.Error("fn was called although the template is not streamable")
			}
		})
	}
}

// A group attribute naming a custom group function (compile-time or
// per-parse) is applied by Parse, so ParseStream refuses it. The compiler
// can't know these functions, so the check runs when ParseStream starts.
func TestParseStreamGate_CustomGroupFunctionAttribute(t *testing.T) {
	mark := func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		data["marked"] = true
		return data, true, nil
	}
	tmpl := `<group name="e*" mark="">
interface {{ name }}
</group>`
	noop := func(map[string]interface{}, [2]int, string) error { return nil }

	c, err := gottp.CompileTemplateWithOptions(tmpl, &gottp.CompileOptions{
		Functions: &gottp.Functions{Group: map[string]gottp.GroupFunc{"mark": mark}},
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	err = c.ParseStream(gottp.Inputs{"Default_Input": "interface a\n"}, nil, nil, noop)
	var gateErr *gottp.TemplateNotStreamableError
	if !errors.As(err, &gateErr) || !anyContains(gateErr.Reasons, "mark=") {
		t.Fatalf("compile-time function: ParseStream error = %v, want a not-streamable reason naming mark=", err)
	}

	c, err = gottp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if err := c.ParseStream(gottp.Inputs{"Default_Input": "interface a\n"}, nil, nil, noop); err != nil {
		t.Fatalf("without the custom function the template streams: %v", err)
	}
	err = c.ParseStream(gottp.Inputs{"Default_Input": "interface a\n"}, nil,
		&gottp.ParseOptions{Functions: &gottp.Functions{Group: map[string]gottp.GroupFunc{"mark": mark}}}, noop)
	if !errors.As(err, &gateErr) || !anyContains(gateErr.Reasons, "mark=") {
		t.Fatalf("per-parse function: ParseStream error = %v, want a not-streamable reason naming mark=", err)
	}
}

// Templates the gate still accepts stream the records Parse returns.
func TestParseStreamGate_AcceptedTemplatesMatchParse(t *testing.T) {
	cases := []struct {
		name, template, input string
	}{
		{"chain filter", `<group name="e*" chain="contains('ip')">
interface {{ name | _start_ }}
 ip {{ ip }}
</group>`, "interface a\n ip 1\ninterface b\ninterface c\n ip 3\n"},
		{"pipe-separated allowlisted functions", `<group name="e*" functions="contains('ip') | del('ip')">
interface {{ name | _start_ }}
 ip {{ ip }}
</group>`, "interface a\n ip 1\ninterface b\n"},
		{"record emptied by del is dropped", `<group name="e*" chain="del('name')">
interface {{ name }}
</group>`, "interface a\ninterface b\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := gottp.CompileTemplate(tc.template)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			inputs := gottp.Inputs{"Default_Input": tc.input}
			parsed, err := c.Parse(inputs, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			want := flattenRecords(parsed)
			var got []map[string]interface{}
			if err := c.ParseStream(inputs, nil, nil, func(m map[string]interface{}, _ [2]int, _ string) error {
				got = append(got, m)
				return nil
			}); err != nil {
				t.Fatalf("ParseStream: %v", err)
			}
			if len(want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("records differ\nParse:       %v\nParseStream: %v", want, got)
			}
		})
	}
}

func anyContains(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
