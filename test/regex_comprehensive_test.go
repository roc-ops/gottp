package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestPipeSeparatedRegexes tests pipe-separated regex patterns
func TestPipeSeparatedRegexes(t *testing.T) {
	template := `
<input load="text">
Protocol  Address     Age (min)  Hardware Addr   Type   Interface
Internet  10.12.13.1        98   0950.5785.5cd1  ARPA   FastEthernet2.13
Internet  10.12.13.2        98   0950.5785.5cd2  ARPA   Loopback0
Internet  10.12.13.3       131   0150.7685.14d5  ARPA   GigabitEthernet2.13
Internet  10.12.13.4       198   0950.5C8A.5c41  ARPA   GigabitEthernet2.17
</input>

<vars>
INTF_RE = r"GigabitEthernet\\S+|Fast\\S+"
</vars>

<group name="arp_test">
Internet  {{ ip | IP }}  {{ age | re(r"\\d+") }}   {{ mac }}  ARPA   {{ interface | re("INTF_RE") }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
Protocol  Address     Age (min)  Hardware Addr   Type   Interface
Internet  10.12.13.1        98   0950.5785.5cd1  ARPA   FastEthernet2.13
Internet  10.12.13.2        98   0950.5785.5cd2  ARPA   Loopback0
Internet  10.12.13.3       131   0150.7685.14d5  ARPA   GigabitEthernet2.13
Internet  10.12.13.4       198   0950.5C8A.5c41  ARPA   GigabitEthernet2.17
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

// TestMACRegexFormatter tests MAC address regex formatting
func TestMACRegexFormatter(t *testing.T) {
	template := `
<input load="text">
Protocol  Address     Age (min)  Hardware Addr      Type   Interface
Internet  10.12.13.2        98   0950:5785:5cd2     ARPA   Loopback0
Internet  10.12.13.3       131   0150.7685.14d5     ARPA   GigabitEthernet2.13
Internet  10.12.13.1        98   0950-5785-5cd1     ARPA   FastEthernet2.13
Internet  10.12.13.4       198   09:50:5C:8A:5c:41  ARPA   GigabitEthernet2.17
</input>

<group name="arp_test">
Internet  {{ ip }}  {{ age }}   {{ mac | MAC }}  ARPA   {{ interface }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
Protocol  Address     Age (min)  Hardware Addr      Type   Interface
Internet  10.12.13.2        98   0950:5785:5cd2     ARPA   Loopback0
Internet  10.12.13.3       131   0150.7685.14d5     ARPA   GigabitEthernet2.13
Internet  10.12.13.1        98   0950-5785-5cd1     ARPA   FastEthernet2.13
Internet  10.12.13.4       198   09:50:5C:8A:5c:41  ARPA   GigabitEthernet2.17
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

// TestORPHRASEPattern tests ORPHRASE pattern matching
func TestORPHRASEPattern(t *testing.T) {
	template := `
<group>
interface {{ interface }}
 description {{ description | ORPHRASE }}
</group>
`

	data := `
interface Port-Channel11
  description Storage Management
interface Loopback0
  description RID
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

	if result == nil {
		t.Error("Result should not be nil")
	}
}

// TestDynamicGroupNames tests dynamic group name formation
func TestDynamicGroupNames(t *testing.T) {
	template := `
<group name="vlans.{{ vlan }}">
vlan {{ vlan }}
 name {{ name }}
</group>
`

	data := `
vlan 100
 name test_vlan
!
vlan 200
 name another_vlan
!
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

	if result == nil {
		t.Error("Result should not be nil")
	}

	// Verify dynamic group names are created
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

	// Check for dynamic group names - Python TTP creates nested structures from dotted paths
	// So "vlans.{{ vlan }}" with vlan=100 creates: vlans -> 100 -> match
	// We should check for the nested structure, not flat keys
	vlans, ok := resultMap["vlans"]
	if !ok {
		t.Logf("Available keys: %v", getKeys(resultMap))
		t.Error("Expected 'vlans' key in result")
		return
	}

	vlansMap, ok := vlans.(map[string]interface{})
	if !ok {
		t.Errorf("Expected 'vlans' to be a map, got %T", vlans)
		return
	}

	// Check for dynamic keys like "100" and "200" within vlans
	hasVlan100 := false
	hasVlan200 := false
	for key := range vlansMap {
		if key == "100" {
			hasVlan100 = true
		}
		if key == "200" {
			hasVlan200 = true
		}
	}

	if !hasVlan100 || !hasVlan200 {
		t.Logf("Available vlan keys: %v", getKeys(vlansMap))
		t.Errorf("Expected dynamic group names '100' and '200' within 'vlans', got hasVlan100=%v, hasVlan200=%v", hasVlan100, hasVlan200)
	}
}
