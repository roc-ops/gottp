package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestMatchVarWithHyphen tests match variables with hyphens
func TestMatchVarWithHyphen(t *testing.T) {
	template := `
<group name="interfaces">
interface {{ interface-name }}
 ip address {{ ip-address }}/{{ mask }}
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.1/24
`

	compiled, err := gottp.CompileTemplate(template)
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

	// Verify parsing succeeded
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestMatchVarWithDot tests match variables with dots (should be expanded)
func TestMatchVarWithDot(t *testing.T) {
	template := `
<group name="config" functions="expand">
{{ target.x }}
{{ target.y }}
</group>
`

	data := `
value1
value2
`

	compiled, err := gottp.CompileTemplate(template)
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

	// Verify expand worked
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestNewlineWithCarriageReturn tests handling of \r\n line endings
func TestNewlineWithCarriageReturn(t *testing.T) {
	template := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	// Data with \r\n line endings
	data := "interface Loopback0\r\n ip address 192.168.0.1/24\r\n"

	compiled, err := gottp.CompileTemplate(template)
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

	// Verify parsing succeeded
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestGroupWithDefaultsOnly tests group with only default values
func TestGroupWithDefaultsOnly(t *testing.T) {
	template := `
<group name="interfaces" default="unknown">
interface {{ interface | default('unknown') }}
</group>
`

	data := `
interface Loopback0
interface Vlan100
`

	compiled, err := gottp.CompileTemplate(template)
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

	// Verify parsing succeeded
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestMixedTabsAndSpaces tests handling of mixed tabs and spaces
func TestMixedTabsAndSpaces(t *testing.T) {
	template := `
<group name="interfaces">
interface {{ interface }}
	 ip address {{ ip }}/{{ mask }}
</group>
`

	data := `
interface Loopback0
	 ip address 192.168.0.1/24
`

	compiled, err := gottp.CompileTemplate(template)
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

	// Verify parsing succeeded
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

