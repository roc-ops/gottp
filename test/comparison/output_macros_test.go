package comparison

import (
	"testing"
)

// Output macros (#142): <output macro="name"/> and
// <output functions="macro('name')"/> run the macro on the whole results
// structure, like Python TTP's output macro() (ttp/output/macro.py).

const outputMacroDefs = `<macro>
def add_flag(data):
    data.append({"flag": True})
    return data

def add_two(data):
    data.append({"two": 2})
    return data

def mutate_only(data):
    data.append("mutated")

def ret_false(data):
    return False

def ret_true(data):
    return True

def describe(data):
    return {"is_list": hasattr(data, "append"), "is_dict": hasattr(data, "keys"), "len": len(data)}

def collect_names(data):
    names = []
    stack = [data]
    for _ in range(1000):
        if not stack:
            break
        o = stack.pop(0)
        if hasattr(o, "keys"):
            for k in sorted(o.keys()):
                if k == "name":
                    names.append(o[k])
                else:
                    stack.append(o[k])
        elif hasattr(o, "append"):
            stack.extend(o)
    return {"names": names}
</macro>
`

const outputMacroGroup = `<group name="items">
item {{ name }}
</group>
`

const outputMacroData = "item a\nitem b\n"

func TestOutputMacro(t *testing.T) {
	requirePythonTTP(t)

	tests := []struct {
		name   string
		output string
	}{
		{"output_macro_attr", `<output macro="add_flag"/>`},
		{"output_macro_function", `<output functions="macro('add_flag')"/>`},
		{"output_macro_function_chain", `<output functions="macro('add_flag') | macro('add_two')"/>`},
		{"output_macro_attr_before_functions", `<output macro="add_flag" functions="macro('add_two')"/>`},
		{"output_macro_attr_after_functions", `<output functions="macro('add_two')" macro="add_flag"/>`},
		{"output_macro_two_outputs", "<output macro=\"add_flag\"/>\n<output macro=\"add_two\"/>"},
		{"output_macro_returns_none", `<output macro="mutate_only"/>`},
		{"output_macro_returns_false", `<output macro="ret_false"/>`},
		{"output_macro_returns_true", `<output macro="ret_true"/>`},
		{"output_macro_receives_list", `<output macro="describe"/>`},
		{"output_macro_undefined", `<output macro="no_such_macro"/>`},
		{"output_macro_walks_results", `<output macro="collect_names"/>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunComparison(t, tt.name, outputMacroDefs+outputMacroGroup+tt.output, outputMacroData, nil, nil)
		})
	}
}

// TestOutputMacroPerTemplate: with results="per_template" the macro gets
// the combined result (a dict), not a list.
func TestOutputMacroPerTemplate(t *testing.T) {
	requirePythonTTP(t)

	tests := []struct {
		name   string
		output string
	}{
		{"output_macro_per_template_receives_dict", `<output macro="describe"/>`},
		{"output_macro_per_template_walks_results", `<output macro="collect_names"/>`},
		{"output_macro_per_template_returns_false", `<output macro="ret_false"/>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := `<template results="per_template">` + outputMacroDefs + outputMacroGroup + tt.output + `</template>`
			RunComparison(t, tt.name, template, outputMacroData, nil, nil)
		})
	}
}

// TestOutputMacroNested runs a macro that restructures nested parent and
// child group results, the use the issue (#142) describes.
func TestOutputMacroNested(t *testing.T) {
	requirePythonTTP(t)

	template := `<macro>
def vlan_ports(data):
    out = []
    for res in data:
        vlans = res.get("vlans", [])
        if hasattr(vlans, "keys"):
            vlans = [vlans]
        for v in vlans:
            ports = v.get("ports", [])
            if hasattr(ports, "keys"):
                ports = [ports]
            out.append({"vlan": v["id"], "ports": [p["port"] for p in ports], "count": len(ports)})
    return out
</macro>
<group name="vlans">
vlan {{ id }} name {{ vname }}
 <group name="ports">
 untagged ethe {{ port }}
 </group>
</group>
<output macro="vlan_ports"/>`

	data := `vlan 10 name users
 untagged ethe 1/1/1
 untagged ethe 1/1/2
vlan 20 name voice
 untagged ethe 1/1/3
`
	RunComparison(t, "output_macro_nested", template, data, nil, nil)
}

// TestOutputMacroThenFormat: functions (including macros) run before the
// formatter.
func TestOutputMacroThenFormat(t *testing.T) {
	requirePythonTTP(t)

	RunComparison(t, "output_macro_then_json", outputMacroDefs+outputMacroGroup+`<output macro="add_flag" format="json"/>`, outputMacroData, nil, nil)
}
