package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestLineIndicatorBasic tests basic _line_ indicator usage
func TestLineIndicatorBasic(t *testing.T) {
	template := `
<group name="vrrp">
{{ interface }} - Group {{ VRRP_Group }}
<group name="_">
{{ VRRP_Description | _line_ }}
State is {{ ignore }} {{ _end_ }}
</group>
State is {{ VRRP_State }}
Virtual IP address is {{ VRRP_Virtual_IP }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
GigabitEthernet1 - Group 100
DC-LAN Subnet
State is Master
Virtual IP address is 192.168.10.1
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

	// Should capture VRRP_Description from the line "DC-LAN Subnet"
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

// TestLineIndicatorWithContains tests _line_ with contains condition
func TestLineIndicatorWithContains(t *testing.T) {
	template := `
<group>
interface {{ interface }}
 description {{ description }}
 {{ port_security_cfg | _line_ | contains("port-security") | joinmatches }}
! {{ _end_ }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Loopback0
 description Router-id-loopback
 ip address 192.168.0.113/24
!
interface Gi0/37
 description CPE_Acces
 switchport port-security
 switchport port-security maximum 5
 switchport port-security mac-address sticky
!
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

	// Should capture port-security lines for Gi0/37 interface
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}
}

