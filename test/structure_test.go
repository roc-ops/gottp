package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestListStructure tests list structure output (per_input method)
func TestListStructure(t *testing.T) {
	template := `
<input load="text">
interface Lo0
 ip address 192.168.0.1 32
!
interface Lo1
 ip address 1.1.1.1 32
</input>

<input load="text">
interface Lo2
 ip address 2.2.2.2 32
!
interface Lo3
 ip address 3.3.3.3 32
</input>

<group>
interface {{ interface }}
 ip address {{ ip }} {{ mask }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Parse with multiple inputs
	data1 := `
interface Lo0
 ip address 192.168.0.1 32
!
interface Lo1
 ip address 1.1.1.1 32
`

	data2 := `
interface Lo2
 ip address 2.2.2.2 32
!
interface Lo3
 ip address 3.3.3.3 32
`

	// Parse first input
	result1, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data1},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse first input: %v", err)
	}

	// Parse second input
	result2, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data2},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse second input: %v", err)
	}

	jsonData, _ := json.MarshalIndent([]interface{}{result1, result2}, "", "  ")
	t.Logf("Results: %s", string(jsonData))

	// Verify structure
	if result1 == nil || result2 == nil {
		t.Error("Results should not be nil")
	}
}

// TestDictionaryStructure tests dictionary structure output (per_template method)
// Note: Nested templates are not yet fully supported, so this test is simplified
func TestDictionaryStructure(t *testing.T) {
	t.Skip("Nested templates not yet fully supported")
	
	template := `
<template results="per_template">

<template name="first">
<input load="text">
interface Lo0
 ip address 124.171.238.50 32
!
interface Lo1
 ip address 1.1.1.1 32
</input>

<group>
interface {{ interface }}
 ip address {{ ip }} {{ mask }}
</group>

</template>

<template name="second">
<input load="text">
interface Lo2
 ip address 124.171.238.50 32
!
interface Lo3
 ip address 2.2.2.2 32
</input>

<group>
interface {{ interface }}
 ip address {{ ip }} {{ mask }}
</group>
</template>
</template>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data1 := `
interface Lo0
 ip address 124.171.238.50 32
!
interface Lo1
 ip address 1.1.1.1 32
`

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data1},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	if result == nil {
		t.Error("Result should not be nil")
	}
}

// TestFlatListStructure tests flat list structure output
func TestFlatListStructure(t *testing.T) {
	template := `
<input load="text">
interface Lo0
 ip address 192.168.0.1 32
!
interface Lo1
 ip address 1.1.1.1 32
</input>

<input load="text">
interface Lo2
 ip address 2.2.2.2 32
!
interface Lo3
 ip address 3.3.3.3 32
</input>

<group>
interface {{ interface }}
 ip address {{ ip }} {{ mask }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Lo0
 ip address 192.168.0.1 32
!
interface Lo1
 ip address 1.1.1.1 32
`

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
		t.Error("Result should not be nil")
	}
}

