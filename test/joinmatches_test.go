package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestJoinMatches tests the joinmatches function
func TestJoinMatches(t *testing.T) {
	template := `
<input load="text">
interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166,173 
 switchport trunk allowed vlan add 400,401,410
</input>
 
<group>
interface {{ interface }}
 switchport trunk allowed vlan add {{ trunk_vlans | to_list | joinmatches }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166,173 
 switchport trunk allowed vlan add 400,401,410
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

	// Verify the result structure
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

	// Check for interface
	iface, ok := resultMap["interface"]
	if !ok {
		t.Error("Expected 'interface' key in result")
	} else if iface != "GigabitEthernet3/3" {
		t.Errorf("Expected interface 'GigabitEthernet3/3', got '%v'", iface)
	}

	// Check for trunk_vlans - should be a list with two items
	trunkVlans, ok := resultMap["trunk_vlans"]
	if !ok {
		t.Error("Expected 'trunk_vlans' key in result")
	} else {
		trunkVlansList, ok := trunkVlans.([]interface{})
		if !ok {
			t.Errorf("Expected trunk_vlans to be a list, got %T", trunkVlans)
		} else if len(trunkVlansList) != 2 {
			t.Errorf("Expected trunk_vlans to have 2 items, got %d", len(trunkVlansList))
		} else {
			// Check values
			if trunkVlansList[0] != "138,166,173" {
				t.Errorf("Expected first trunk_vlans value '138,166,173', got '%v'", trunkVlansList[0])
			}
			if trunkVlansList[1] != "400,401,410" {
				t.Errorf("Expected second trunk_vlans value '400,401,410', got '%v'", trunkVlansList[1])
			}
		}
	}
}

