package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestStarlarkMacroBasic tests basic Starlark macro execution
func TestStarlarkMacroBasic(t *testing.T) {
	template := `
<macro>
def uppercase(data):
    return data.upper()
</macro>

<group name="interfaces">
interface {{ interface | macro('uppercase') }}
</group>
`

	data := `
interface loopback0
interface vlan100
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

	// Verify result
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestStarlarkMacroWithData tests Starlark macro that processes data
func TestStarlarkMacroWithData(t *testing.T) {
	template := `
<macro>
def add_prefix(data):
    return "PREFIX_" + str(data)
</macro>

<group name="interfaces">
interface {{ interface | macro('add_prefix') }}
</group>
`

	data := `
interface Loopback0
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

	// Verify macro was applied
	resultStr := string(jsonData)
	if !strings.Contains(resultStr, "PREFIX_") {
		t.Error("Expected macro to add prefix to interface name")
	}
}

