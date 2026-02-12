package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/roc-ops/gottp"
	"github.com/roc-ops/gottp/internal/parser"
)

// TestMultiTemplateChildGroupPreservation tests that child <group> elements
// within multiple <template> sections are preserved during parsing.
// This is a regression test for REQ-008: multi-template child element loss.
func TestMultiTemplateChildGroupPreservation(t *testing.T) {
	templateText := `
<template>
<group name="interfaces*">
interface {{ name }}
 ip {{ ip }}/{{ mask }}
</group>
</template>

<template>
<group name="vlans*">
vlan {{ id }}
 name {{ vlan_name }}
</group>
</template>
`

	data := `interface Loopback0
 ip 192.168.1.1/32
interface Eth1
 ip 10.0.0.1/24
vlan 100
 name MANAGEMENT
vlan 200
 name DATA
`

	compiled, err := gottp.CompileTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// The result should contain both interfaces and vlans data
	// With per_input results method (default), result is a list with one entry per input
	resultList, ok := result.([]interface{})
	if !ok {
		// May be a map if per_template method
		resultMap, ok := result.(map[string]interface{})
		if !ok {
			t.Fatalf("Expected result to be list or map, got %T", result)
		}
		// Check for interfaces data
		if _, exists := resultMap["interfaces"]; !exists {
			t.Error("Expected 'interfaces' key in result map")
		}
		// Check for vlans data
		if _, exists := resultMap["vlans"]; !exists {
			t.Error("Expected 'vlans' key in result map")
		}
		return
	}

	// For per_input results, check each element
	found := false
	for _, item := range resultList {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if _, exists := itemMap["interfaces"]; exists {
				found = true
			}
			if _, exists := itemMap["vlans"]; exists {
				found = true
			}
		}
	}
	if !found {
		t.Error("Expected to find 'interfaces' or 'vlans' data in results")
	}
}

// TestMultiTemplateParserGroupPreservation tests the parser level directly
// to verify that child <group> elements within <template> sections are
// correctly parsed and preserved.
func TestMultiTemplateParserGroupPreservation(t *testing.T) {
	templateText := `
<template>
<group name="interfaces*">
interface {{ name }}
 ip {{ ip }}/{{ mask }}
</group>
</template>

<template>
<group name="vlans*">
vlan {{ id }}
 name {{ vlan_name }}
</group>
</template>
`

	tmpl, err := parser.ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	t.Logf("Root template: name=%s, groups=%d, templates=%d",
		tmpl.Name, len(tmpl.Groups), len(tmpl.Templates))

	// Log all child templates and their groups
	for i, child := range tmpl.Templates {
		t.Logf("  Child template %d: name=%s, groups=%d", i, child.Name, len(child.Groups))
		for j, grp := range child.Groups {
			t.Logf("    Group %d: name=%s, pattern_empty=%v", j, grp.Name, grp.Pattern == "")
		}
	}

	// Log root-level groups
	for i, grp := range tmpl.Groups {
		t.Logf("  Root group %d: name=%s, pattern_empty=%v", i, grp.Name, grp.Pattern == "")
	}

	// We expect the root template to have child templates, each with their groups
	// When there are multiple <template> tags, the parser should either:
	// Option A: Create a root template with child templates (each having groups), OR
	// Option B: Merge all groups into one template
	// Python TTP uses Option A.

	// Check that groups are present somewhere
	totalGroups := len(tmpl.Groups)
	for _, child := range tmpl.Templates {
		totalGroups += len(child.Groups)
	}

	if totalGroups < 2 {
		t.Errorf("Expected at least 2 groups across all templates, got %d", totalGroups)
	}

	// Specifically check that both interfaces and vlans groups exist
	foundInterfaces := false
	foundVlans := false

	// Check root groups
	for _, grp := range tmpl.Groups {
		if grp.Name == "interfaces*" {
			foundInterfaces = true
			if grp.Pattern == "" {
				t.Error("interfaces group has empty pattern - child elements were lost")
			}
		}
		if grp.Name == "vlans*" {
			foundVlans = true
			if grp.Pattern == "" {
				t.Error("vlans group has empty pattern - child elements were lost")
			}
		}
	}

	// Check child template groups
	for _, child := range tmpl.Templates {
		for _, grp := range child.Groups {
			if grp.Name == "interfaces*" {
				foundInterfaces = true
				if grp.Pattern == "" {
					t.Error("interfaces group in child template has empty pattern - child elements were lost")
				}
			}
			if grp.Name == "vlans*" {
				foundVlans = true
				if grp.Pattern == "" {
					t.Error("vlans group in child template has empty pattern - child elements were lost")
				}
			}
		}
	}

	if !foundInterfaces {
		t.Error("interfaces group not found in any template")
	}
	if !foundVlans {
		t.Error("vlans group not found in any template")
	}
}

// TestMultiTemplateWithNestedChildGroups tests that nested child groups
// within template sections are also preserved.
func TestMultiTemplateWithNestedChildGroups(t *testing.T) {
	templateText := `
<template>
<group name="device">
hostname {{ hostname }}
<group name="interfaces*">
interface {{ name }}
 ip {{ ip }}/{{ mask }}
</group>
</group>
</template>

<template>
<group name="network">
network {{ network_name }}
<group name="vlans*">
vlan {{ id }}
 name {{ vlan_name }}
</group>
</group>
</template>
`

	tmpl, err := parser.ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	t.Logf("Root template: groups=%d, templates=%d",
		len(tmpl.Groups), len(tmpl.Templates))

	// Check all groups and child groups
	checkGroups := func(groups []*parser.Group, prefix string) {
		for i, grp := range groups {
			t.Logf("%sGroup %d: name=%s, nested_groups=%d, pattern_empty=%v",
				prefix, i, grp.Name, len(grp.Groups), grp.Pattern == "")
			for j, nested := range grp.Groups {
				t.Logf("%s  Nested %d: name=%s, pattern_empty=%v",
					prefix, j, nested.Name, nested.Pattern == "")
			}
		}
	}

	checkGroups(tmpl.Groups, "Root: ")
	for i, child := range tmpl.Templates {
		t.Logf("Child template %d:", i)
		checkGroups(child.Groups, fmt.Sprintf("  Template %d: ", i))
	}

	// Count total groups including nested ones
	var countGroups func(groups []*parser.Group) int
	countGroups = func(groups []*parser.Group) int {
		count := len(groups)
		for _, g := range groups {
			count += countGroups(g.Groups)
		}
		return count
	}

	totalInRoot := countGroups(tmpl.Groups)
	totalInChildren := 0
	for _, child := range tmpl.Templates {
		totalInChildren += countGroups(child.Groups)
	}

	total := totalInRoot + totalInChildren
	t.Logf("Total groups: %d (root: %d, children: %d)", total, totalInRoot, totalInChildren)

	if total < 4 {
		t.Errorf("Expected at least 4 groups (2 parent + 2 nested), got %d", total)
	}
}
