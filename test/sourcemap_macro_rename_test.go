package test

import (
	"testing"

	"github.com/roc-ops/gottp"
)

// TestSourceMap_VariablesAliasedAfterMacroRename covers
// https://github.com/roc-ops/gottp/issues/25 - a group macro that renames
// a field (data["name"] = data["ifname"]) used to leave SourceMap.Variables
// only keyed by the pre-macro name ("ifname"), with nothing connecting the
// renamed field ("name") in the final Data back to the source span it
// actually came from.
func TestSourceMap_VariablesAliasedAfterMacroRename(t *testing.T) {
	template := `<template>
<macro>
def relabel(data):
    data["name"] = data["ifname"]
    data["computed"] = "no-source-for-this"
    return data
</macro>
<group name="interfaces*" macro="relabel">
interface {{ ifname }}
 description {{ description }}
</group>
</template>`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	data := "interface eth0\n description first\ninterface eth1\n description second\n"

	result, err := compiled.ParseWithValidation(
		gottp.Inputs{"Default_Input": data}, nil,
		&gottp.ParseOptions{EnableSourceMap: true},
	)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	input := result.SourceMap.Inputs["Default_Input"]
	if input == nil {
		t.Fatal("expected Default_Input in SourceMap")
	}

	// line 0: "interface eth0" - should now have both "ifname" (original)
	// and "name" (macro-renamed alias) pointing at the same span.
	line0 := input.Lines[0]
	if len(line0.Matches) != 1 {
		t.Fatalf("expected 1 match on line 0, got %d", len(line0.Matches))
	}
	m0 := line0.Matches[0]

	ifnameRange, ok := m0.Variables["ifname"]
	if !ok {
		t.Fatal("expected \"ifname\" to still have provenance")
	}
	nameRange, ok := m0.Variables["name"]
	if !ok {
		t.Fatal("expected \"name\" to be aliased to the same provenance as \"ifname\" after the macro rename")
	}
	if nameRange.StartCol != ifnameRange.StartCol || nameRange.EndCol != ifnameRange.EndCol {
		t.Errorf("expected \"name\" range to match \"ifname\" range, got name=%+v ifname=%+v", nameRange, ifnameRange)
	}

	// "computed" was fabricated by the macro out of thin air - it must NOT
	// get a (wrong) provenance entry.
	if _, ok := m0.Variables["computed"]; ok {
		t.Error("expected \"computed\" (macro-fabricated, no source) to have no provenance entry")
	}

	// line 2: "interface eth1" - same check for the second instance, to
	// make sure aliasing is applied per-instance, not just to the first.
	line2 := input.Lines[2]
	if len(line2.Matches) != 1 {
		t.Fatalf("expected 1 match on line 2, got %d", len(line2.Matches))
	}
	m2 := line2.Matches[0]
	if _, ok := m2.Variables["name"]; !ok {
		t.Error("expected \"name\" to be aliased on the second instance too")
	}
}
