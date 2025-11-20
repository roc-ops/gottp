package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
	"github.com/roc-ops/gottp/internal/functions/group"
)

// TestExpandFunction tests the expand group function directly
func TestExpandFunction(t *testing.T) {
	registry := group.NewRegistry()
	
	fn, ok := registry.Get("expand")
	if !ok {
		t.Fatal("expand function not found")
	}
	
	// Test data with dot-separated keys
	data := map[string]interface{}{
		"target.x": "value1",
		"target.y": "value2",
		"other":    "value3",
	}
	
	result, keep, err := fn(data, []string{}, nil)
	if err != nil {
		t.Fatalf("expand failed: %v", err)
	}
	
	if !keep {
		t.Error("expand should keep the result")
	}
	
	// Verify nested structure
	target, ok := result["target"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected 'target' to be a map, got %T", result["target"])
	}
	
	if target["x"] != "value1" {
		t.Errorf("Expected target.x to be 'value1', got %v", target["x"])
	}
	
	if target["y"] != "value2" {
		t.Errorf("Expected target.y to be 'value2', got %v", target["y"])
	}
	
	if result["other"] != "value3" {
		t.Errorf("Expected 'other' to be 'value3', got %v", result["other"])
	}
}

// TestExpandInTemplate tests expand function in a template
func TestExpandInTemplate(t *testing.T) {
	template := `
<group name="test" functions="expand">
x={{ target.x }} y={{ target.y }}
</group>
`

	data := `
x=value1 y=value2
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

	test, ok := resultMap["test"]
	if !ok {
		t.Fatal("Expected 'test' key in result")
	}

	var testList []map[string]interface{}
	switch v := test.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				testList = append(testList, m)
			}
		}
	case []map[string]interface{}:
		testList = v
	case map[string]interface{}:
		testList = []map[string]interface{}{v}
	}

	if len(testList) == 0 {
		t.Fatal("Expected at least one test result")
	}

	// Check if expand worked - should have nested target structure
	firstTest := testList[0]
	target, ok := firstTest["target"].(map[string]interface{})
	if !ok {
		t.Logf("Result structure: %+v", firstTest)
		t.Error("Expected 'target' to be a nested map after expand")
	} else {
		if target["x"] != "value1" {
			t.Errorf("Expected target.x to be 'value1', got %v", target["x"])
		}
		if target["y"] != "value2" {
			t.Errorf("Expected target.y to be 'value2', got %v", target["y"])
		}
	}
}

// TestItemizeFunction tests the itemize group function directly
func TestItemizeFunction(t *testing.T) {
	registry := group.NewRegistry()
	
	fn, ok := registry.Get("itemize")
	if !ok {
		t.Fatal("itemize function not found")
	}
	
	// Test data
	data := map[string]interface{}{
		"interface": "Vlan778",
		"ip":        "192.168.1.1",
	}
	
	kwargs := map[string]interface{}{
		"key": "interface",
	}
	
	result, keep, err := fn(data, []string{}, kwargs)
	if err != nil {
		t.Fatalf("itemize failed: %v", err)
	}
	
	if !keep {
		t.Error("itemize should keep the result when key exists")
	}
	
	// Verify interface key was removed
	if _, exists := result["interface"]; exists {
		t.Error("Expected 'interface' key to be removed")
	}
	
	// Verify itemize metadata was added
	if result["_itemize_key"] != "interface" {
		t.Error("Expected _itemize_key to be set")
	}
	
	if result["_itemize_value"] != "Vlan778" {
		t.Error("Expected _itemize_value to be 'Vlan778'")
	}
}

// TestItemizeMissingKey tests itemize when key doesn't exist
func TestItemizeMissingKey(t *testing.T) {
	registry := group.NewRegistry()
	
	fn, ok := registry.Get("itemize")
	if !ok {
		t.Fatal("itemize function not found")
	}
	
	// Test data without the key
	data := map[string]interface{}{
		"ip": "192.168.1.1",
	}
	
	kwargs := map[string]interface{}{
		"key": "interface",
	}
	
	_, keep, err := fn(data, []string{}, kwargs)
	if err != nil {
		t.Fatalf("itemize failed: %v", err)
	}
	
	// Should invalidate group when key doesn't exist
	if keep {
		t.Error("Expected itemize to invalidate group when key doesn't exist")
	}
}

