package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestMethodAttributeTable tests that method="table" returns list format
func TestMethodAttributeTable(t *testing.T) {
	template := `
<group name="interfaces" method="table">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
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

	// Should return list format (table method always returns list)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result with method=table, got %T", result)
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

	// With method=table, should always be a list
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
		t.Fatalf("Expected interfaces to be a list with method=table, got %T", interfaces)
	}

	// With method="table", each pattern match is saved separately
	// So we get 4 entries: interface Loopback0, ip Loopback0, interface Vlan100, ip Vlan100
	if len(interfacesList) != 4 {
		t.Errorf("Expected 4 entries (each pattern match is separate with method=table), got %d", len(interfacesList))
	}
}
