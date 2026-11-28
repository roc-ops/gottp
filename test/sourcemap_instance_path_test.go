package test

import (
	"testing"

	"github.com/roc-ops/gottp"
)

// TestSourceMap_ResultPathResolvesPerInstance covers
// https://github.com/roc-ops/gottp/issues/24 - when a group repeats
// (name="interfaces*"), every match's ResultPath used to come back as the
// literal, unresolved group-name pattern ("interfaces*") instead of the
// per-instance path ("interfaces[0]", "interfaces[1]", ...) documented on
// MatchMapping.ResultPath.
func TestSourceMap_ResultPathResolvesPerInstance(t *testing.T) {
	template := `<template>
<group name="interfaces*">
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

	paths := map[int]string{}
	for _, line := range input.Lines {
		for _, m := range line.Matches {
			paths[line.LineNumber] = m.ResultPath
		}
	}

	if got := paths[0]; got != "interfaces[0]" {
		t.Errorf("line 0: expected ResultPath %q, got %q", "interfaces[0]", got)
	}
	if got := paths[1]; got != "interfaces[0]" {
		t.Errorf("line 1: expected ResultPath %q, got %q", "interfaces[0]", got)
	}
	if got := paths[2]; got != "interfaces[1]" {
		t.Errorf("line 2: expected ResultPath %q, got %q", "interfaces[1]", got)
	}
	if got := paths[3]; got != "interfaces[1]" {
		t.Errorf("line 3: expected ResultPath %q, got %q", "interfaces[1]", got)
	}
}

// TestSourceMap_ResultPathSingleInstanceUnaffected covers a repeated group
// (name="interfaces*") with a single match. Its record is stored as a
// one-element list ({"interfaces": [{...}]}), so its ResultPath is
// "interfaces[0]". Before refactor A2 (#60) it was the unresolved
// "interfaces*".
func TestSourceMap_ResultPathSingleInstanceUnaffected(t *testing.T) {
	template := `<template>
<group name="interfaces*">
interface {{ ifname }}
 description {{ description }}
</group>
</template>`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	data := "interface eth0\n description first\n"

	result, err := compiled.ParseWithValidation(
		gottp.Inputs{"Default_Input": data}, nil,
		&gottp.ParseOptions{EnableSourceMap: true},
	)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	input := result.SourceMap.Inputs["Default_Input"]
	for _, line := range input.Lines {
		for _, m := range line.Matches {
			if m.ResultPath != "interfaces[0]" {
				t.Errorf("line %d: expected ResultPath %q, got %q", line.LineNumber, "interfaces[0]", m.ResultPath)
			}
		}
	}
}
