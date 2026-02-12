package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestUnnamedInnerGroupWithMethodTable tests that an unnamed inner <group>
// inside a named outer group with method="table" produces correct results,
// matching Python TTP behavior. Previously, the unnamed inner group caused
// the outer group to return empty results because parseGroup returned nil
// when the group had no direct patterns (all patterns were in the inner group).
func TestUnnamedInnerGroupWithMethodTable(t *testing.T) {
	// Template WITH unnamed inner group
	templateWithInner := `
<template>
<lookup name="ifTypes" load="json">
{
    "ifType_ethernet_csmacd": "ethernetCsmacd",
    "ifType_CMTSmac": "docsCableMaclayer",
    "ifType_CMTSDownStream": "docsCableDownstream",
    "ifType_CMTSUPStream_physical": "docsCableUpstream",
    "ifType_CMTSUPstream_logic": "docsCableUpstreamChannel",
    "ifType_l3ipvlan": "l3ipvlan",
    "ifType_softwareLoopback": "softwareLoopback",
    "IfType_CMTSVideoDownstream": "docsCableDownstream",
    "ifType_CMTSDOWNstream_RfPort": "cableDownstreamRfPort",
    "ifType_CMTSOfdmDownstream": "docsOfdmDownstream",
    "ifType_ipForward": "ipForward",
    "ifType_ofdma": "docsCableUpstreamChannel"
}
</lookup>
<group name="yang.if-mib:IF-MIB.ifTable.ifEntry*" method="table">
<group>
ifIndex: {{ ifIndex | DIGIT | to_int  | _start_}}
ifDescr: {{ ifDescr | ORPHRASE }}
ifType: {{ ifType | ORPHRASE | rlookup('ifTypes')}}
ifMtu: {{ ifMtu| DIGIT | to_int }}
ifSpeed: {{ ifSpeed | DIGIT | to_int }}
ifPhysAddress: {{ ifPhysAddress | ORPHRASE }}
ifAdminStatus: {{ ifAdminStatus | ORPHRASE | lower }}({{ignore}})
ifOperStatus: {{ ifOperStatus | ORPHRASE | lower }}({{ignore}})
ifLastChange: {{ ifLastChange | ORPHRASE }}
ifInOctets: {{ ifInOctets | DIGIT | to_int }}
ifHCInOctets: {{ ifHCInOctets | DIGIT | to_int }}
ifInUcastPkts: {{ ifInUcastPkts | DIGIT | to_int }}
ifHCInUcastPkts: {{ ifHCInUcastPkts | DIGIT | to_int }}
ifInDiscards: {{ ifInDiscards | DIGIT | to_int }}
ifInErrors: {{ ifInErrors | DIGIT | to_int }}
ifInUnknownProtos: {{ ifInUnknownProtos | DIGIT | to_int }}
ifOutOctets: {{ ifOutOctets | DIGIT | to_int }}
ifHCOutOctets: {{ ifHCOutOctets | DIGIT | to_int }}
ifOutUcastPkts: {{ ifOutUcastPkts | DIGIT | to_int }}
ifHCOutUcastPkts: {{ ifHCOutUcastPkts | DIGIT | to_int }}
ifOutDiscards: {{ ifOutDiscards | DIGIT | to_int }}
ifOutErrors: {{ ifOutErrors | DIGIT | to_int }}
ifName: {{ ifName | ORPHRASE }}
ifInMulticastPkts: {{ ifInMulticastPkts | DIGIT | to_int }}
ifHCInMulticastPkts: {{ ifHCInMulticastPkts | DIGIT | to_int }}
ifInBroadcastPkts: {{ ifInBroadcastPkts | DIGIT | to_int }}
ifHCInBroadcastPkts: {{ ifHCInBroadcastPkts | DIGIT | to_int }}
ifOutMulticastPkts: {{ ifOutMulticastPkts | DIGIT | to_int }}
ifHCOutMulticastPkts: {{ ifHCOutMulticastPkts | DIGIT | to_int }}
ifOutBroadcastPkts: {{ ifOutBroadcastPkts | DIGIT | to_int }}
ifHCOutBroadcastPkts: {{ ifHCOutBroadcastPkts | DIGIT | to_int }}
ifLinkUpDownTrapEnable: {{ ifLinkUpDownTrapEnable | ORPHRASE | lower }}
ifHighSpeed: {{ ifHighSpeed | DIGIT | to_int }}
ifPromiscuousMode: {{ ifPromiscuousMode | DIGIT | to_int }}
ifConnectorPresent: {{ ifConnectorPresent | DIGIT | to_int }}
ifAlias: {{ ifAlias | ORPHRASE }}
ifCounterDiscontinuityTime: {{ ifCounterDiscontinuityTime | ORPHRASE }}
</group>
</group>
</template>
`

	data := `r3r2-c100g>show iftable detail
total interface number is 894

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
ifInOctets: 228941791
ifHCInOctets: 228941791
ifInUcastPkts: 3454207
ifHCInUcastPkts: 3454207
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
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th
------------------------------------------------
ifIndex: 1000072
ifDescr: XGige 6/0
ifType: ifType_ethernet_csmacd
ifMtu: 1500
ifSpeed: 4294967295
ifPhysAddress: 00:17:10:2a:a1:81
ifAdminStatus: Up(1)
ifOperStatus: Up(1)
ifLastChange: 23 day 21h:15m:58s.62th
ifInOctets: 945984277
ifHCInOctets: 945984277
ifInUcastPkts: 1099491
ifHCInUcastPkts: 1099491
ifInDiscards: 0
ifInErrors: 0
ifInUnknownProtos: 296867
ifOutOctets: 137590080
ifHCOutOctets: 137590080
ifOutUcastPkts: 649688
ifHCOutUcastPkts: 649688
ifOutDiscards: 0
ifOutErrors: 0
ifName: XGige 6/0
ifInMulticastPkts: 1726049
ifHCInMulticastPkts: 1726049
ifInBroadcastPkts: 48288
ifHCInBroadcastPkts: 48288
ifOutMulticastPkts: 97545
ifHCOutMulticastPkts: 97545
ifOutBroadcastPkts: 65446
ifHCOutBroadcastPkts: 65446
ifLinkUpDownTrapEnable: Enable
ifHighSpeed: 10000
ifPromiscuousMode: 1
ifConnectorPresent: 1
ifAlias:
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th
------------------------------------------------
ifIndex: 1000073
ifDescr: XGige 6/1
ifType: ifType_ethernet_csmacd
ifMtu: 1500
ifSpeed: 4294967295
ifPhysAddress: 00:17:10:2a:a1:82
ifAdminStatus: Down(2)
ifOperStatus: Down(2)
ifLastChange: 0 day 00h:00m:00s.00th
ifInOctets: 0
ifHCInOctets: 0
ifInUcastPkts: 0
ifHCInUcastPkts: 0
ifInDiscards: 0
ifInErrors: 0
ifInUnknownProtos: 0
ifOutOctets: 0
ifHCOutOctets: 0
ifOutUcastPkts: 0
ifHCOutUcastPkts: 0
ifOutDiscards: 0
ifOutErrors: 0
ifName: XGige 6/1
ifInMulticastPkts: 0
ifHCInMulticastPkts: 0
ifInBroadcastPkts: 0
ifHCInBroadcastPkts: 0
ifOutMulticastPkts: 0
ifHCOutMulticastPkts: 0
ifOutBroadcastPkts: 0
ifHCOutBroadcastPkts: 0
ifLinkUpDownTrapEnable: Enable
ifHighSpeed: 10000
ifPromiscuousMode: 1
ifConnectorPresent: 1
ifAlias:
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th
------------------------------------------------
ifIndex: 1000074
ifDescr: XGige 6/2
ifType: ifType_ethernet_csmacd
ifMtu: 1500
ifSpeed: 4294967295
ifPhysAddress: 00:17:10:2a:a1:83
ifAdminStatus: Down(2)
ifOperStatus: Down(2)
ifLastChange: 0 day 00h:00m:00s.00th
ifInOctets: 0
ifHCInOctets: 0
ifInUcastPkts: 0
ifHCInUcastPkts: 0
ifInDiscards: 0
ifInErrors: 0
ifInUnknownProtos: 0
ifOutOctets: 0
ifHCOutOctets: 0
ifOutUcastPkts: 0
ifHCOutUcastPkts: 0
ifOutDiscards: 0
ifOutErrors: 0
ifName: XGige 6/2
ifInMulticastPkts: 0
ifHCInMulticastPkts: 0
ifInBroadcastPkts: 0
ifHCInBroadcastPkts: 0
ifOutMulticastPkts: 0
ifHCOutMulticastPkts: 0
ifOutBroadcastPkts: 0
ifHCOutBroadcastPkts: 0
ifLinkUpDownTrapEnable: Enable
ifHighSpeed: 10000
ifPromiscuousMode: 1
ifConnectorPresent: 1
ifAlias:
ifCounterDiscontinuityTime: 0 day 0h 0m:00s.00th
------------------------------------------------
`

	// Parse with the inner group template
	compiledInner, err := gottp.CompileTemplate(templateWithInner)
	if err != nil {
		t.Fatalf("Failed to compile template with inner group: %v", err)
	}

	resultInner, err := compiledInner.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse with inner group: %v", err)
	}

	jsonInner, _ := json.MarshalIndent(resultInner, "", "  ")
	t.Logf("Result WITH inner group:\n%s", string(jsonInner))

	// Validate the structure of the result
	resultList, ok := resultInner.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", resultInner)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}

	resultMap, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	// The result should not be empty -- before the fix, this was [{}]
	if len(resultMap) == 0 {
		t.Fatal("Result map should not be empty (before the fix, unnamed inner group produced empty results)")
	}

	// Navigate to the ifEntry list using the dotted path
	entries := navigatePath(t, resultMap, []string{"yang", "if-mib:IF-MIB", "ifTable", "ifEntry"})
	if entries == nil {
		t.Fatal("Could not navigate to yang.if-mib:IF-MIB.ifTable.ifEntry")
	}

	entryList, ok := entries.([]interface{})
	if !ok {
		t.Fatalf("Expected ifEntry to be a list, got %T", entries)
	}

	if len(entryList) != 4 {
		t.Fatalf("Expected 4 interfaces, got %d", len(entryList))
	}

	// Validate the first interface entry
	entry0, ok := entryList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected first entry to be a map, got %T", entryList[0])
	}

	// Check key fields that are always present
	assertIntField(t, entry0, "ifIndex", 1)
	assertIntField(t, entry0, "ifMtu", 1500)
	assertIntField(t, entry0, "ifSpeed", 100000000)
	assertIntField(t, entry0, "ifInOctets", 228941791)
	assertIntField(t, entry0, "ifHCInOctets", 228941791)
	assertIntField(t, entry0, "ifInUcastPkts", 3454207)
	assertIntField(t, entry0, "ifOutOctets", 1266)
	assertIntField(t, entry0, "ifOutUcastPkts", 11)
	assertIntField(t, entry0, "ifInDiscards", 0)
	assertIntField(t, entry0, "ifInErrors", 0)
	assertIntField(t, entry0, "ifOutDiscards", 0)
	assertIntField(t, entry0, "ifOutErrors", 0)

	// Check string fields
	assertStringField(t, entry0, "ifDescr", "eth 6/0")
	assertStringField(t, entry0, "ifPhysAddress", "00:17:10:2b:30:82")
	assertStringField(t, entry0, "ifAdminStatus", "up")
	assertStringField(t, entry0, "ifOperStatus", "up")
	assertStringField(t, entry0, "ifName", "eth 6/0")

	// Validate second entry (ifIndex 1000072)
	entry1, ok := entryList[1].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected second entry to be a map, got %T", entryList[1])
	}
	assertIntField(t, entry1, "ifIndex", 1000072)
	assertStringField(t, entry1, "ifDescr", "XGige 6/0")
	assertIntField(t, entry1, "ifInUnknownProtos", 296867)

	// Validate third entry (ifIndex 1000073 - Down interface)
	entry2, ok := entryList[2].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected third entry to be a map, got %T", entryList[2])
	}
	assertIntField(t, entry2, "ifIndex", 1000073)
	assertStringField(t, entry2, "ifDescr", "XGige 6/1")
	assertStringField(t, entry2, "ifAdminStatus", "down")
	assertStringField(t, entry2, "ifOperStatus", "down")

	// Validate fourth entry (ifIndex 1000074)
	entry3, ok := entryList[3].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected fourth entry to be a map, got %T", entryList[3])
	}
	assertIntField(t, entry3, "ifIndex", 1000074)
	assertStringField(t, entry3, "ifDescr", "XGige 6/2")
}

// TestUnnamedInnerGroupSimple tests a simpler case of unnamed inner group
// merging into parent group, verifying the fix works for basic scenarios.
func TestUnnamedInnerGroupSimple(t *testing.T) {
	templateWithInner := `
<group name="interfaces*">
<group>
interface {{ name }}
 ip address {{ ip }}/{{ mask }}
</group>
</group>
`
	templateWithoutInner := `
<group name="interfaces*">
interface {{ name }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	data := `
interface Lo0
 ip address 1.1.1.1/32
interface Lo1
 ip address 2.2.2.2/32
`

	// Parse with inner group
	compiledInner, err := gottp.CompileTemplate(templateWithInner)
	if err != nil {
		t.Fatalf("Failed to compile inner template: %v", err)
	}
	resultInner, err := compiledInner.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse with inner group: %v", err)
	}

	// Parse without inner group
	compiledFlat, err := gottp.CompileTemplate(templateWithoutInner)
	if err != nil {
		t.Fatalf("Failed to compile flat template: %v", err)
	}
	resultFlat, err := compiledFlat.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse without inner group: %v", err)
	}

	jsonInner, _ := json.MarshalIndent(resultInner, "", "  ")
	jsonFlat, _ := json.MarshalIndent(resultFlat, "", "  ")

	t.Logf("Result WITH inner group:\n%s", string(jsonInner))
	t.Logf("Result WITHOUT inner group:\n%s", string(jsonFlat))

	// Both should produce equivalent output
	if string(jsonInner) != string(jsonFlat) {
		t.Errorf("Output mismatch: unnamed inner group should produce identical results to flat template.\nWith inner group:\n%s\nWithout inner group:\n%s", string(jsonInner), string(jsonFlat))
	}

	// Result should not be nil or empty
	if resultInner == nil {
		t.Fatal("Inner group result should not be nil")
	}

	resultList, ok := resultInner.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", resultInner)
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

	interfacesList, ok := interfaces.([]interface{})
	if !ok {
		t.Fatalf("Expected interfaces to be a list, got %T", interfaces)
	}

	if len(interfacesList) < 2 {
		t.Fatalf("Expected at least 2 interfaces, got %d", len(interfacesList))
	}
}

// navigatePath traverses a nested map using a slice of keys.
func navigatePath(t *testing.T, m map[string]interface{}, keys []string) interface{} {
	t.Helper()
	var current interface{} = m
	for _, key := range keys {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			t.Errorf("Expected map at key %q, got %T", key, current)
			return nil
		}
		current, ok = currentMap[key]
		if !ok {
			t.Errorf("Key %q not found in map (available keys: %v)", key, mapKeys(currentMap))
			return nil
		}
	}
	return current
}

// mapKeys returns the keys of a map for debugging.
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// assertIntField checks that a field in a map is a numeric type with the expected value.
func assertIntField(t *testing.T, m map[string]interface{}, key string, expected int) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("Expected field %q not found", key)
		return
	}
	switch v := val.(type) {
	case float64:
		if int(v) != expected {
			t.Errorf("Field %q: expected %d, got %v", key, expected, v)
		}
	case int:
		if v != expected {
			t.Errorf("Field %q: expected %d, got %d", key, expected, v)
		}
	case int64:
		if int(v) != expected {
			t.Errorf("Field %q: expected %d, got %d", key, expected, v)
		}
	default:
		t.Errorf("Field %q: expected int-like type, got %T (%v)", key, val, val)
	}
}

// assertStringField checks that a field in a map is a string with the expected value.
func assertStringField(t *testing.T, m map[string]interface{}, key string, expected string) {
	t.Helper()
	val, ok := m[key]
	if !ok {
		t.Errorf("Expected field %q not found", key)
		return
	}
	str, ok := val.(string)
	if !ok {
		t.Errorf("Field %q: expected string, got %T (%v)", key, val, val)
		return
	}
	if str != expected {
		t.Errorf("Field %q: expected %q, got %q", key, expected, str)
	}
}
