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

// TestMethodTableWithStartIndicator tests that method="table" + _start_ on one
// pattern (without an inner group) still produces separate records for each
// pattern match, matching Python TTP behavior. In Python TTP, method="table"
// unconditionally makes all patterns into start patterns; _start_ does NOT
// override this behavior. Each pattern match is a separate record.
//
// This is a regression test for the table + _start_ conflict (REQ-009).
func TestMethodTableWithStartIndicator(t *testing.T) {
	template := `
<group name="entries*" method="table">
ID: {{ id | _start_ }}
Name: {{ name }}
Value: {{ value }}
</group>
`

	data := `
ID: 1
Name: Alice
Value: 100
ID: 2
Name: Bob
Value: 200
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

	// Navigate to entries
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

	entries, ok := resultMap["entries"]
	if !ok {
		t.Fatal("Expected 'entries' key in result")
	}

	var entriesList []map[string]interface{}
	switch v := entries.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				entriesList = append(entriesList, m)
			}
		}
	case []map[string]interface{}:
		entriesList = v
	default:
		t.Fatalf("Expected entries to be a list, got %T", entries)
	}

	// With method="table", _start_ does NOT override the table behavior.
	// In Python TTP, method="table" makes ALL patterns start patterns, so each
	// pattern match creates a separate record. We expect 6 separate records:
	// {id:1}, {name:Alice}, {value:100}, {id:2}, {name:Bob}, {value:200}
	if len(entriesList) != 6 {
		t.Errorf("Expected 6 entries (method=table makes each pattern match separate, _start_ does not override), got %d", len(entriesList))
		for i, e := range entriesList {
			t.Logf("  Entry %d: %v", i, e)
		}
	}

	// Verify some specific entries
	if len(entriesList) >= 6 {
		// First entry should have id
		if id, ok := entriesList[0]["id"]; !ok || id != "1" {
			t.Errorf("Expected first entry to have id=1, got %v", entriesList[0])
		}
		// Second entry should have name
		if name, ok := entriesList[1]["name"]; !ok || name != "Alice" {
			t.Errorf("Expected second entry to have name=Alice, got %v", entriesList[1])
		}
		// Third entry should have value
		if value, ok := entriesList[2]["value"]; !ok || value != "100" {
			t.Errorf("Expected third entry to have value=100, got %v", entriesList[2])
		}
	}
}

// TestMethodTableWithStartInnerGroup tests that method="table" on the outer
// group with _start_ in an unnamed inner group produces merged records. The
// inner group inherits method="group" (default), so _start_ works normally:
// only the _start_ pattern starts new records and other patterns merge into it.
//
// This matches Python TTP behavior and is the pattern used in the UR-002 template.
func TestMethodTableWithStartInnerGroup(t *testing.T) {
	template := `
<group name="entries*" method="table">
<group>
ifIndex: {{ ifIndex | _start_ }}
ifDescr: {{ ifDescr | ORPHRASE }}
ifMtu: {{ ifMtu }}
</group>
</group>
`

	data := `
ifIndex: 1
ifDescr: eth 6/0
ifMtu: 1500
ifIndex: 2
ifDescr: XGige 6/0
ifMtu: 9000
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

	// Navigate to entries
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

	entries, ok := resultMap["entries"]
	if !ok {
		t.Fatal("Expected 'entries' key in result")
	}

	var entriesList []map[string]interface{}
	switch v := entries.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				entriesList = append(entriesList, m)
			}
		}
	case []map[string]interface{}:
		entriesList = v
	default:
		t.Fatalf("Expected entries to be a list, got %T", entries)
	}

	// With inner group, method="table" applies to the outer group, but the inner
	// group uses default method="group", so _start_ works normally. We expect 2
	// merged records, each with ifIndex, ifDescr, and ifMtu.
	if len(entriesList) != 2 {
		t.Errorf("Expected 2 merged entries (inner group uses method=group with _start_), got %d", len(entriesList))
		for i, e := range entriesList {
			t.Logf("  Entry %d: %v", i, e)
		}
	}

	// Verify the entries are properly merged
	if len(entriesList) >= 2 {
		// First entry
		if idx, ok := entriesList[0]["ifIndex"]; !ok || idx != "1" {
			t.Errorf("Expected first entry ifIndex=1, got %v", entriesList[0]["ifIndex"])
		}
		if descr, ok := entriesList[0]["ifDescr"]; !ok || descr != "eth 6/0" {
			t.Errorf("Expected first entry ifDescr='eth 6/0', got %v", entriesList[0]["ifDescr"])
		}
		if mtu, ok := entriesList[0]["ifMtu"]; !ok || mtu != "1500" {
			t.Errorf("Expected first entry ifMtu=1500, got %v", entriesList[0]["ifMtu"])
		}
		// Second entry
		if idx, ok := entriesList[1]["ifIndex"]; !ok || idx != "2" {
			t.Errorf("Expected second entry ifIndex=2, got %v", entriesList[1]["ifIndex"])
		}
		if descr, ok := entriesList[1]["ifDescr"]; !ok || descr != "XGige 6/0" {
			t.Errorf("Expected second entry ifDescr='XGige 6/0', got %v", entriesList[1]["ifDescr"])
		}
		if mtu, ok := entriesList[1]["ifMtu"]; !ok || mtu != "9000" {
			t.Errorf("Expected second entry ifMtu=9000, got %v", entriesList[1]["ifMtu"])
		}
	}
}
