package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestTemplateVariables tests template variable functionality
func TestTemplateVariables(t *testing.T) {
	template := `
<vars>
var_1 = "value_1"
var_2 = "value_2"
var_3 = [1, 2, 3, 4, "a"]
</vars>

<input load="text">
router bgp 65100
</input>

<group name="bgp_config">
router bgp {{ bgp_as }}
{{ var_1_value | set(var_1) }}
{{ var_2_value | set(var_2) }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
router bgp 65100
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

// TestTemplateVariablesYAML tests YAML variable loading
func TestTemplateVariablesYAML(t *testing.T) {
	template := `
<vars load="yaml">
var_1: value_1
var_2: value_2
var_3: [1, 2, 3, 4, "a"]
</vars>

<input load="text">
router bgp 65100
</input>

<group name="bgp_config">
router bgp {{ bgp_as }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
router bgp 65100
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

// TestTemplateVariablesNested tests nested variable paths
func TestTemplateVariablesNested(t *testing.T) {
	template := `
<vars name="my.var.s">
a = 1
b = 2
</vars>

<input load="text">
interface Port-Chanel11
  description Storage Management
interface Loopback0
  description RID
</input>

<group>
interface {{ interface }}
  description {{ description | ORPHRASE }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Chanel11
  description Storage Management
interface Loopback0
  description RID
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

