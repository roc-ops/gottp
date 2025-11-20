package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
	"github.com/roc-ops/gottp/internal/functions/group"
)

// TestDeleteGroupFunctionDirect tests delete group function directly
func TestDeleteGroupFunctionDirect(t *testing.T) {
	registry := group.NewRegistry()
	
	fn, ok := registry.Get("delete")
	if !ok {
		t.Fatal("delete function not found")
	}
	
	data := map[string]interface{}{
		"keep":   "value1",
		"remove": "value2",
		"other":  "value3",
	}
	
	result, keep, err := fn(data, []string{"remove"}, nil)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	
	if !keep {
		t.Error("delete should keep the result")
	}
	
	// Verify key was removed
	if _, exists := result["remove"]; exists {
		t.Error("Expected 'remove' key to be deleted")
	}
	
	// Verify other keys still exist
	if result["keep"] != "value1" {
		t.Error("Expected 'keep' key to still exist")
	}
	
	if result["other"] != "value3" {
		t.Error("Expected 'other' key to still exist")
	}
}

// TestDeleteGroupFunctionInTemplate tests delete function in a template
func TestDeleteGroupFunctionInTemplate(t *testing.T) {
	template := `
<group name="interfaces" functions="delete('temp')">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 temp={{ temp }}
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.1/24
 temp=test
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

	// Verify result structure
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}

	resultMap, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	interfaces, ok := resultMap["interfaces"]
	if !ok {
		t.Fatal("Expected 'interfaces' key in result")
	}

	var interfacesList []map[string]interface{}
	switch v := interfaces.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				interfacesList = append(interfacesList, m)
			}
		}
	case []map[string]interface{}:
		interfacesList = v
	case map[string]interface{}:
		interfacesList = []map[string]interface{}{v}
	}

	if len(interfacesList) == 0 {
		t.Fatal("Expected at least one interface")
	}

	// Verify temp key was deleted
	firstInterface := interfacesList[0]
	if _, exists := firstInterface["temp"]; exists {
		t.Error("Expected 'temp' key to be deleted")
	}
	
	// Verify other keys still exist
	if firstInterface["interface"] != "Loopback0" {
		t.Error("Expected 'interface' key to still exist")
	}
}

// TestDeleteMultipleKeys tests deleting multiple keys
func TestDeleteMultipleKeys(t *testing.T) {
	template := `
<group name="interfaces" functions="delete('temp1') | delete('temp2')">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 temp1={{ temp1 }}
 temp2={{ temp2 }}
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.1/24
 temp1=test1
 temp2=test2
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

	// Verify both temp keys were deleted
	resultStr := string(jsonData)
	if strings.Contains(resultStr, "temp1") || strings.Contains(resultStr, "temp2") {
		t.Error("Expected both temp1 and temp2 keys to be deleted")
	}
}

