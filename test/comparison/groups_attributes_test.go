package comparison

import (
	"testing"
)

// TestGroupAttributeName tests the name attribute
func TestGroupAttributeName(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "group_attribute_name", template, data, nil, nil)
}

// TestGroupAttributeInput tests the input attribute (Example-1 from docs)
func TestGroupAttributeInputExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<input name="test1" load="text">
interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166-173
</input>

<group name="interfaces" input="test1">
interface {{ interface }}
 switchport trunk allowed vlan add {{ trunk_vlans }}
</group>`

	data := `interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166-173
`

	RunComparison(t, "group_attribute_input_example1", template, data, nil, nil)
}

// TestGroupAttributeInputExample2 tests the input attribute with multiple inputs (Example-2 from docs)
func TestGroupAttributeInputExample2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<input name="input_1" load="text">
interface GigabitEthernet3/11
 description input_1_data
 switchport trunk allowed vlan add 111,222
!
</input>

<input name="input_2" load="text">
interface GigabitEthernet3/22
 description input_2_data
 switchport trunk allowed vlan add 222,888
!
</input>

<group name="interfaces.trunks" input="input_1">
interface {{ interface }}
 switchport trunk allowed vlan add {{ trunk_vlans }}
 description {{ description | ORPHRASE }}
 {{ group_id | set("group_1") }}
!{{ _end_ }}
</group>

<group name="interfaces.trunks" input="input_2">
interface {{ interface }}
 switchport trunk allowed vlan add {{ trunk_vlans }}
 description {{ description | ORPHRASE }}
 {{ group_id | set("group_2") }}
!{{ _end_ }}
</group>`

	// For multiple inputs, we need to pass them separately
	// This test will need special handling since RunComparison expects single data string
	// We'll test with combined data for now
	data := `interface GigabitEthernet3/11
 description input_1_data
 switchport trunk allowed vlan add 111,222
!
interface GigabitEthernet3/22
 description input_2_data
 switchport trunk allowed vlan add 222,888
!
`

	RunComparison(t, "group_attribute_input_example2", template, data, nil, nil)
}

// TestGroupAttributeDefaultExample1 tests default attribute with string value (Example-1 from docs)
func TestGroupAttributeDefaultExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Note: Removed <input> tag since data is passed separately via RunComparison
	template := `<group name="interfaces" default="some_default_value">
interface {{ interface }}
 description {{ description }}
 switchport trunk allowed vlan add {{ trunk_vlans }}
 ip address {{ ip }}
</group>`

	data := `interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166-173
`

	RunComparison(t, "group_attribute_default_example1", template, data, nil, nil)
}

// TestGroupAttributeDefaultExample2 tests default attribute with no matches (Example-2 from docs)
func TestGroupAttributeDefaultExample2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Note: Removed <input> tag since data is passed separately via RunComparison
	template := `<group name="uptime**">
device-hostame uptime is {{ uptime | PHRASE }}
<group name="software">
 software version {{ version | default("uncknown") }}
</group>
</group>

<group name="domain" default="Uncknown">
Default domain is {{ fqdn }}
</group>`

	data := `device-hostame uptime is 27 weeks, 3 days, 10 hours, 46 minutes, 10 seconds
`

	RunComparison(t, "group_attribute_default_example2", template, data, nil, nil)
}

// TestGroupAttributeDefaultExample3 tests default attribute with dictionary variable (Example-3 from docs) - CRITICAL
// Note: This example uses two separate <input> tags, which Python TTP processes separately
// For now, we test with a single input to verify the default attribute works
func TestGroupAttributeDefaultExample3(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Simplified version with single input to test default attribute
	// Note: Removed <input> tag since data is passed separately via RunComparison
	template := `<vars>
var_name = {
    "L3": True,
    "has_ip": True
}
</vars>

<group name="interfaces">
interface {{ interface }}
 description {{ description | ORPHRASE }}
 <group name="IPv4_addresses" default="var_name">
 ip address {{ IP }} {{ MASK }}
 </group>
</group>`

	data := `interface Lo0
 ip address 1.1.1.1 255.255.255.255
!
interface Lo1
 description this interface has description
`

	vars := map[string]interface{}{
		"var_name": map[string]interface{}{
			"L3":     true,
			"has_ip": true,
		},
	}

	RunComparison(t, "group_attribute_default_example3", template, data, vars, nil)
}

// TestGroupAttributeMethod tests the method attribute (table vs group)
func TestGroupAttributeMethod(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="arp" method="table">
Internet  {{ ip }}  {{ age | DIGIT }}   {{ mac }}  ARPA   {{ interface }}
Internet  {{ ip }}  -                   {{ mac }}  ARPA   {{ interface }}
</group>`

	data := `CSR1Kv-3-lab#show ip arp
Protocol  Address          Age (min)  Hardware Addr   Type   Interface
Internet  10.1.13.1              98   0050.5685.5cd1  ARPA   GigabitEthernet2.13
Internet  10.1.13.3               -   0050.5685.14d5  ARPA   GigabitEthernet2.13
`

	RunComparison(t, "group_attribute_method_table", template, data, nil, nil)
}

// TestGroupAttributeVoid tests the void attribute
func TestGroupAttributeVoid(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<group name="ignored" void="True">
! {{ comment }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
! This is a comment
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "group_attribute_void", template, data, nil, nil)
}

// TestGroupAttributeFunctions tests the functions attribute
func TestGroupAttributeFunctions(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Skip this test - Python TTP has issues parsing functions attribute with nested quotes
	// The template uses functions="contains('ip'), record('last_interface')" which causes
	// Python TTP's parser to fail with: TypeError: 'str' object is not callable
	// This is a Python TTP limitation, not a GoTTP issue
	t.Skip("Python TTP cannot parse functions attribute with nested quotes - known limitation")

	template := `<group name="interfaces" functions="contains('ip'), record('last_interface')">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 description Management-VLAN
`

	RunComparison(t, "group_attribute_functions", template, data, nil, nil)
}

