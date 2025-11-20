package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestContainsReInline tests contains_re condition function with inline pattern
func TestContainsReInline(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage Management
interface Loopback0
  description RID
interface Vlan777
  description Management
</input>

<group>
interface {{ interface | contains_re("Port-Channel") }}
  description {{ description }}
  {{ is_lag | set(True) }}
  {{ is_loopback| set(False) }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage Management
interface Loopback0
  description RID
interface Vlan777
  description Management
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

	// Verify result structure
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}

	groupList, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	// Should only have Port-Channel11 (Loopback0 and Vlan777 should be filtered out)
	interfaces, ok := groupList["interface"]
	if !ok {
		t.Fatal("Expected 'interface' key in result")
	}

	// Result should be a list with one item
	interfaceList, ok := interfaces.([]interface{})
	if !ok {
		// Might be a single item, not a list
		if interfaceStr, ok := interfaces.(string); ok {
			if interfaceStr != "Port-Channel11" {
				t.Errorf("Expected interface 'Port-Channel11', got '%s'", interfaceStr)
			}
			return
		}
		t.Fatalf("Expected list of interfaces, got %T", interfaces)
	}

	if len(interfaceList) != 1 {
		t.Errorf("Expected 1 interface, got %d", len(interfaceList))
	}

	// Check first interface
	if interfaceMap, ok := interfaceList[0].(map[string]interface{}); ok {
		if iface, ok := interfaceMap["interface"].(string); ok {
			if iface != "Port-Channel11" {
				t.Errorf("Expected interface 'Port-Channel11', got '%s'", iface)
			}
		}
	}
}

// TestContainsReFromVars tests contains_re condition function with variable pattern
func TestContainsReFromVars(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
</input>

<vars>
pattern = "Port-Channel"
</vars>

<group>
interface {{ interface | contains_re(pattern) }}
  description {{ description }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
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

	// Should have Port-Channel11 and Port-Channel12 (Loopback0 and Vlan777 should be filtered out)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestContainsInline tests contains condition function with inline pattern
func TestContainsInline(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
</input>

<group>
interface {{ interface | contains("Port") }}
  description {{ description }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
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

	// Should have Port-Channel11 and Port-Channel12 (Loopback0 and Vlan777 should be filtered out)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestEqualInline tests equal condition function
func TestEqualInline(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
</input>

<group>
interface {{ interface | equal("Port-Channel12") }}
  description {{ description }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
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

	// Should only have Port-Channel12
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestNotEqualInline tests notequal condition function
func TestNotEqualInline(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
</input>

<group>
interface {{ interface | notequal("Port-Channel12") }}
  description {{ description }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
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

	// Should have Port-Channel11, Loopback0, and Vlan777 (Port-Channel12 should be filtered out)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestStartswithInline tests startswith condition function with inline pattern
func TestStartswithInline(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
</input>

<group>
interface {{ interface | startswith("Port") }}
  description {{ description }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
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

	// Should only have Port-Channel11 and Port-Channel12 (Loopback0 and Vlan777 should be filtered out)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestEndswithInline tests endswith condition function with inline pattern
func TestEndswithInline(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
</input>

<group>
interface {{ interface | endswith("0") }}
  description {{ description }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
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

	// Should only have Loopback0 (ends with "0")
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestNotStartswithInline tests notstartswith condition function with inline pattern
func TestNotStartswithInline(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
</input>

<group>
interface {{ interface | notstartswith("Port") }}
  description {{ description }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
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

	// Should only have Loopback0 and Vlan777 (Port-Channel11 and Port-Channel12 should be filtered out)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestNotEndswithInline tests notendswith condition function with inline pattern
func TestNotEndswithInline(t *testing.T) {
	template := `
<input load="text">
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
</input>

<group>
interface {{ interface | notendswith("0") }}
  description {{ description }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Port-Channel11
  description Storage
interface Loopback0
  description RID
interface Port-Channel12
  description Management
interface Vlan777
  description Management
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

	// Should have Port-Channel11, Port-Channel12, and Vlan777 (Loopback0 should be filtered out)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestIgnoreIndicator tests ignore indicator functionality
func TestIgnoreIndicator(t *testing.T) {
	template := `
<group name="interfaces">
{{ interface }} is up, line protocol is up
  Hardware is Gt96k FE, address is {{ ignore }} (bia {{MAC}})
  MTU {{ mtu }} bytes, BW 100000 Kbit/sec, DLY 1000 usec,
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
FastEthernet0/0 is up, line protocol is up
  Hardware is Gt96k FE, address is c201.1d00.0000 (bia c201.1d00.1234)
  MTU 1500 bytes, BW 100000 Kbit/sec, DLY 1000 usec,
FastEthernet0/1 is up, line protocol is up
  Hardware is Gt96k FE, address is b20a.1e00.8777 (bia c201.1d00.1111)
  MTU 1500 bytes, BW 100000 Kbit/sec, DLY 1000 usec,
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

	// Result structure: [{"interfaces": [{"interface": "...", "MAC": "...", "mtu": "..."}]}]
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

	// Handle both []interface{} and []map[string]interface{}
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
	default:
		t.Fatalf("Expected interfaces to be a list, got %T", interfaces)
	}

	if len(interfacesList) == 0 {
		t.Fatal("Expected at least one interface result")
	}

	// Check first interface
	firstInterface := interfacesList[0]
	
	// Check that ignore is not in the results
	if _, hasIgnore := firstInterface["ignore"]; hasIgnore {
		t.Error("Expected 'ignore' to not appear in results")
	}
	// Should have MAC, interface, and mtu
	if _, hasMAC := firstInterface["MAC"]; !hasMAC {
		t.Error("Expected 'MAC' in results")
	}
	if _, hasInterface := firstInterface["interface"]; !hasInterface {
		t.Error("Expected 'interface' in results")
	}
	if _, hasMTU := firstInterface["mtu"]; !hasMTU {
		t.Error("Expected 'mtu' in results")
	}
}
