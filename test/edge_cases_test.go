package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestMatchVarWithHyphen tests match variables with hyphens
func TestMatchVarWithHyphen(t *testing.T) {
	template := `
<group name="interfaces">
interface {{ interface-name }}
 ip address {{ ip-address }}/{{ mask }}
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.1/24
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

	// Verify parsing succeeded
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestMatchVarWithDot tests match variables with dots (should be expanded)
func TestMatchVarWithDot(t *testing.T) {
	template := `
<group name="config" functions="expand">
{{ target.x }}
{{ target.y }}
</group>
`

	data := `
value1
value2
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

	// Verify expand worked
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestNewlineWithCarriageReturn tests handling of \r\n line endings
func TestNewlineWithCarriageReturn(t *testing.T) {
	template := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	// Data with \r\n line endings
	data := "interface Loopback0\r\n ip address 192.168.0.1/24\r\n"

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

	// Verify parsing succeeded
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestGroupWithDefaultsOnly tests group with only default values
func TestGroupWithDefaultsOnly(t *testing.T) {
	template := `
<group name="interfaces" default="unknown">
interface {{ interface | default('unknown') }}
</group>
`

	data := `
interface Loopback0
interface Vlan100
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

	// Verify parsing succeeded
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestMixedTabsAndSpaces tests handling of mixed tabs and spaces
func TestMixedTabsAndSpaces(t *testing.T) {
	template := `
<group name="interfaces">
interface {{ interface }}
	 ip address {{ ip }}/{{ mask }}
</group>
`

	data := `
interface Loopback0
	 ip address 192.168.0.1/24
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

	// Verify parsing succeeded
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestWideKeyValueRecordMerge tests that patterns spread across 30+ lines
// of key-value CLI output merge into a single group entry. SNMP ifEntry
// records have 37+ fields — patterns matching fields at the top and bottom
// must merge into one entry.
func TestWideKeyValueRecordMerge(t *testing.T) {
	template := `<group name="ifEntry">
ifIndex: {{ ifIndex | to_int }}
ifDescr: {{ ifDescr | ORPHRASE }}
ifHCOutBroadcastPkts: {{ ifHCOutBroadcastPkts | to_int }}
ifName: {{ ifName | ORPHRASE }}
</group>`

	data := `total interface number is 894

------------------------------------------------
ifIndex: 1
ifDescr: eth 6/0
ifType: ifType_ethernet_csmacd
ifMtu: 1500
ifSpeed: 100000000
ifPhysAddress: 00:17:10:2b:30:82
ifAdminStatus: Up(1)
ifOperStatus: Up(1)
ifLastChange: 0 day 00h:01m:30s.60th
ifInOctets: 378399415
ifHCInOctets: 378399415
ifInUcastPkts: 5698591
ifHCInUcastPkts: 5698591
ifInDiscards: 0
ifInErrors: 0
ifInUnknownProtos: 0
ifOutOctets: 1266
ifHCOutOctets: 1266
ifOutUcastPkts: 11
ifHCOutUcastPkts: 11
ifOutDiscards: 0
ifOutErrors: 0
ifName: eth 6/0
ifInMulticastPkts: 0
ifHCInMulticastPkts: 0
ifInBroadcastPkts: 0
ifHCInBroadcastPkts: 0
ifOutMulticastPkts: 0
ifHCOutMulticastPkts: 0
ifOutBroadcastPkts: 0
ifHCOutBroadcastPkts: 0
ifLinkUpDownTrapEnable: Enable
ifHighSpeed: 100
ifPromiscuousMode: 1
ifConnectorPresent: 1
ifAlias:
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	result, err := compiled.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 {
		t.Fatalf("Expected list result with at least one entry, got %T", result)
	}

	entry, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map entry, got %T", resultList[0])
	}

	ifEntry, ok := entry["ifEntry"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected ifEntry map, got %T", entry["ifEntry"])
	}

	// All 4 fields must be present in a single merged entry
	for _, field := range []string{"ifIndex", "ifDescr", "ifHCOutBroadcastPkts", "ifName"} {
		if _, exists := ifEntry[field]; !exists {
			t.Errorf("Field %q missing from ifEntry (maxGapLines too small for 37-field records?)", field)
		}
	}

	if v, ok := ifEntry["ifHCOutBroadcastPkts"]; ok {
		// Should be 0 (int after to_int)
		jsonVal, _ := json.Marshal(v)
		if string(jsonVal) != "0" {
			t.Errorf("ifHCOutBroadcastPkts: got %s, want 0", jsonVal)
		}
	}
}

