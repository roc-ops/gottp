package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestChainAttribute tests that chain attribute processes group functions
func TestChainAttribute(t *testing.T) {
	template := `
<group name="interfaces" chain="contains('ip')">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.1/24
 description Router-id-loopback
interface Vlan100
 description Management-VLAN
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

	// Should only have Loopback0 (Vlan100 should be filtered out by contains('ip'))
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

	// Handle both single map and list of maps
	var interfacesList []map[string]interface{}
	switch v := interfaces.(type) {
	case map[string]interface{}:
		// Single match - wrap in list
		interfacesList = []map[string]interface{}{v}
	case []interface{}:
		// List of interfaces
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				interfacesList = append(interfacesList, m)
			}
		}
	case []map[string]interface{}:
		interfacesList = v
	default:
		t.Fatalf("Expected interfaces to be a map or list, got %T", interfaces)
	}

	// Should only have one interface (Loopback0) since Vlan100 doesn't have 'ip'
	if len(interfacesList) != 1 {
		t.Errorf("Expected 1 interface (filtered by contains('ip')), got %d", len(interfacesList))
	}

	// Verify it's Loopback0
	if len(interfacesList) > 0 {
		interfaceMap := interfacesList[0]
		if interfaceName, ok := interfaceMap["interface"].(string); ok {
			if interfaceName != "Loopback0" {
				t.Errorf("Expected interface 'Loopback0', got '%s'", interfaceName)
			}
		}
	}
}

// TestChainAttributeWithSet tests chain with set function
func TestChainAttributeWithSet(t *testing.T) {
	template := `
<group name="interfaces" chain="set('ethernet', 'type')">
interface {{ interface }}
 description {{ description }}
</group>
`

	data := `
interface GigabitEthernet1
 description Uplink
interface GigabitEthernet2
 description Access
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

	// Should have both interfaces with 'type' set to 'ethernet'
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

	// Handle both single map and list of maps
	var interfacesList []map[string]interface{}
	switch v := interfaces.(type) {
	case map[string]interface{}:
		// Single match - wrap in list
		interfacesList = []map[string]interface{}{v}
	case []interface{}:
		// List of interfaces
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				interfacesList = append(interfacesList, m)
			}
		}
	case []map[string]interface{}:
		interfacesList = v
	default:
		t.Fatalf("Expected interfaces to be a map or list, got %T", interfaces)
	}

	if len(interfacesList) != 2 {
		t.Errorf("Expected 2 interfaces, got %d", len(interfacesList))
	}

	// Verify both have 'type' set
	for i, interfaceMap := range interfacesList {
		if typeVal, ok := interfaceMap["type"].(string); ok {
			if typeVal != "ethernet" {
				t.Errorf("Interface %d: Expected type 'ethernet', got '%s'", i, typeVal)
			}
		} else {
			t.Errorf("Interface %d: Expected 'type' field to be set", i)
		}
	}
}

