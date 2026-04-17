package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestNestedGroups tests nested group functionality
func TestNestedGroups(t *testing.T) {
	template := `
<group name="device">
hostname {{ hostname }}
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>
</group>
`

	data := `
hostname router1
interface Lo0
 ip address 1.1.1.1
!
interface Lo1
 ip address 2.2.2.2
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
}

// TestNestedGroupsWithMultipleInstances tests nested groups with multiple parent instances
// Note: This test uses advanced features (_start_, to_list, joinmatches) that may not be fully implemented
func TestNestedGroupsWithMultipleInstances(t *testing.T) {
	t.Skip("Advanced features (_start_, to_list, joinmatches) not yet fully implemented")
	
	template := `
<group name="vrfs">
vrf {{ name }}
  <group name="import">
  import route-target {{ _start_ }}
   {{ import | to_list | joinmatches }}
  </group>
  !
  <group name="export">
  export route-target {{ _start_ }}
   {{ export | to_list | joinmatches }}
  </group>
</group>
`

	data := `
vrf xyz
 address-family ipv4 unicast
  import route-target
   65000:3507
   65000:3511
   65000:5453
   65000:5535
  !
  export route-target
   65000:5453
   65000:5535
  !
 !
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
}

// TestNestedGroupKeyFields verifies that GetKeyFields() includes keys from
// nested groups, not just top-level groups.  DH's converter calls
// GetKeyFields() to resolve list keys for gNMI path construction.  Before the
// collectGroupKeys fix, nested group keys were silently omitted.
func TestNestedGroupKeyFields(t *testing.T) {
	template := `
<group name="qam-entry*" keys="port-name">
interface qam {{ port-name }}
 <group name="channel-entry*" keys="channel-id">
 channel {{ channel-id | to_int }} frequency {{ frequency | to_int }}
 </group>
</group>
`
	data := `
interface qam 0/0
 channel 1 frequency 111000000
 channel 2 frequency 117000000
`

	ct, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	result, err := ct.ParseWithValidation(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if result == nil || result.Data == nil {
		t.Fatal("Result should not be nil")
	}

	keyFields := result.KeyFields

	// Top-level group key must be present
	if keys, ok := keyFields["qam-entry"]; !ok {
		t.Error("KeyFields missing top-level group 'qam-entry'")
	} else if len(keys) != 1 || keys[0] != "port-name" {
		t.Errorf("qam-entry keys = %v, want [port-name]", keys)
	}

	// Nested group key must also be present
	if keys, ok := keyFields["channel-entry"]; !ok {
		t.Error("KeyFields missing nested group 'channel-entry' — DH will report no_key_metadata")
	} else if len(keys) != 1 || keys[0] != "channel-id" {
		t.Errorf("channel-entry keys = %v, want [channel-id]", keys)
	}
}

// TestAnonymousGroupWithNested tests anonymous group with nested groups
// Note: This test uses advanced features (void, absolute path "/") that may not be fully implemented
func TestAnonymousGroupWithNested(t *testing.T) {
	// void attribute is now implemented, but absolute path "/" is not yet supported
	t.Skip("Absolute path '/' not yet fully implemented")
	
	template := `
<group void="">
interface {{ interface }}
 description {{ description | ORPHRASE }}
 <group name="/">
 ip address {{ ip }} {{ mask }}
 </group>
</group>
`

	data := `
interface GigabitEthernet1
 description some info
 vrf forwarding MGMT
 ip address 10.123.89.56 255.255.255.0
interface GigabitEthernet2
 ip address 10.123.89.55 255.255.255.0
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

