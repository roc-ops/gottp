package test

import (
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestGroupMacro_InvalidSyntaxWarnsAtCompileTime covers
// https://github.com/roc-ops/gottp/issues/26. `del data[key]` isn't valid
// syntax in this version of go.starlark.net at all, so the macro source
// fails to compile at registration time - which is intentionally
// non-fatal (matching Python TTP's "an unavailable macro is silently
// skipped" behavior). Previously that compile failure was swallowed with
// no visible error anywhere: CompileTemplate produced no warning, and the
// group's macro="relabel" attribute just silently did nothing at parse
// time. Now it should surface as a compile warning so a template author
// can actually discover why their macro never ran.
func TestGroupMacro_InvalidSyntaxWarnsAtCompileTime(t *testing.T) {
	template := `<template>
<macro>
def relabel(data):
    data["name"] = data["ifname"]
    del data["ifname"]
    return data
</macro>
<group name="interfaces*" macro="relabel">
interface {{ ifname }}
</group>
</template>`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	warnings := compiled.GetWarnings()
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "Starlark") || strings.Contains(w, "compile") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a compile warning about the invalid macro syntax, got: %v", warnings)
	}

	// The group's macro reference should still be silently ignored at
	// parse time (Python TTP behavior), not a hard parse error - the
	// warning above is how the author finds out, not a thrown error.
	result, err := compiled.Parse(gottp.Inputs{"Default_Input": "interface eth0\n"}, nil, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result == nil {
		t.Fatal("expected a result")
	}
}

// TestGroupMacro_UnknownMacroNameStillIgnored makes sure the fix for #26
// preserved Python-TTP-compatible behavior for a macro name that simply
// isn't registered - that should still be silently ignored, not an error.
func TestGroupMacro_UnknownMacroNameStillIgnored(t *testing.T) {
	template := `<template>
<group name="interfaces*" macro="does_not_exist">
interface {{ ifname }}
</group>
</template>`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := compiled.Parse(gottp.Inputs{"Default_Input": "interface eth0\n"}, nil, nil)
	if err != nil {
		t.Fatalf("expected unregistered macro name to be silently ignored, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a result")
	}
}

// TestGroupMacro_SuccessfulMacroStillWorks is a control to make sure the
// fix for #26 didn't break the working case.
func TestGroupMacro_SuccessfulMacroStillWorks(t *testing.T) {
	template := `<template>
<macro>
def relabel(data):
    data["name"] = data["ifname"]
    return data
</macro>
<group name="interfaces*" macro="relabel">
interface {{ ifname }}
</group>
</template>`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := compiled.Parse(gottp.Inputs{"Default_Input": "interface eth0\n"}, nil, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	list, ok := result.([]interface{})
	if !ok || len(list) == 0 {
		t.Fatalf("unexpected result shape: %#v", result)
	}
	top, ok := list[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected top-level entry: %#v", list[0])
	}
	ifaceList, ok := top["interfaces"].([]interface{})
	if !ok || len(ifaceList) == 0 {
		t.Fatalf("unexpected interfaces entry: %#v", top["interfaces"])
	}
	iface, ok := ifaceList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected interfaces[0] entry: %#v", ifaceList[0])
	}
	if iface["name"] != "eth0" {
		t.Errorf("expected macro-added \"name\" field, got: %#v", iface)
	}
}
