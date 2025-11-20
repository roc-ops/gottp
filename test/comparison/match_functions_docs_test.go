package comparison

import (
	"testing"
)

// TestMatchFunctionChainExample1 tests chain function (Example-1 from docs)
func TestMatchFunctionChainExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
vlans = "unrange(rangechar='-', joinchar=',') | split(',') | join(':') | joinmatches(':')"
</vars>

<group name="interfaces">
interface {{ interface }}
 switchport trunk allowed vlan add {{ trunk_vlans | chain('vlans') }}
</group>`

	data := `interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166-173
 switchport trunk allowed vlan add 400,401,410
`

	vars := map[string]interface{}{
		"vlans": "unrange(rangechar='-', joinchar=',') | split(',') | join(':') | joinmatches(':')",
	}

	RunComparison(t, "match_function_chain_example1", template, data, vars, nil)
}

// TestMatchFunctionRecordExample tests record function from docs
func TestMatchFunctionRecordExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces" input="in1">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 ip vrf forwarding {{ vrf | record("VRF") }}
 switchport port-security mac {{ sec_mac }}
</group>

<group name="interfaces" input="in2">
interface {{ interface }}
 description {{ description | ORPHRASE | record("my_description") }}
 switchport port-security mac {{ sec_mac }}
 {{ my_vrf | set("VRF") }}
 {{ my_descript | set("my_description") }}
</group>`

	// Combine both inputs
	data := `interface Vlan778
 ip address 2002:fd37::91/124
 ip vrf forwarding VRF_NAME_1
 switchport port-security mac 4
!
interface Vlan779
 description some description input2
!
interface Vlan780
 switchport port-security mac 4
!
`

	RunComparison(t, "match_function_record_example", template, data, nil, nil)
}

// TestMatchFunctionLetExample tests let function from docs
func TestMatchFunctionLetExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 description {{ description | let("description_undefined") }}
 ip address {{ ip | contains("24") | let("netmask", "255.255.255.0") }}
</group>`

	data := `interface Loopback0
 description Management
 ip address 192.168.0.113/24
!
`

	RunComparison(t, "match_function_let_example", template, data, nil, nil)
}

// TestMatchFunctionJoinExample1 tests join function (Example-1 from docs)
func TestMatchFunctionJoinExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `description {{ description | join('.') }}`

	data := `description someimportantdescription
`

	RunComparison(t, "match_function_join_example1", template, data, nil, nil)
}

// TestMatchFunctionJoinExample2 tests join function with split (Example-2 from docs)
func TestMatchFunctionJoinExample2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `interface {{ interface }}
 switchport trunk allowed vlan add {{ trunk_vlans | split(',') | join(':') }}
`

	data := `interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166,173,400,401,410
`

	RunComparison(t, "match_function_join_example2", template, data, nil, nil)
}

// TestMatchFunctionAppendExample tests append function from docs
func TestMatchFunctionAppendExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `interface {{ interface | append(' - non production') }}
`

	data := `interface Ge3/3
`

	RunComparison(t, "match_function_append_example", template, data, nil, nil)
}

// TestMatchFunctionUnrangeExample tests unrange function from docs
func TestMatchFunctionUnrangeExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `interface {{ interface }}
 switchport trunk allowed vlan add {{ trunk_vlans | unrange(rangechar='-', joinchar=',') }}
`

	data := `interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166,170-173
`

	RunComparison(t, "match_function_unrange_example", template, data, nil, nil)
}

// TestMatchFunctionSetExample1 tests set function (Example-1 from docs)
func TestMatchFunctionSetExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
mys_set_var = "my_set_value"
</vars>

<group name="interfacesset">
interface {{ interface }}
 switchport mode access {{ mode_access | set("True") }}
 switchport trunk encapsulation dot1q {{ encap | set("dot1q") }}
 switchport mode trunk {{ mode | set("Trunk") }} {{ vlans | set("all_vlans") }}
 shutdown {{ disabled | set("True") }} {{ test_var | set("mys_set_var") }}
!{{ _end_ }}
</group>`

	data := `interface GigabitEthernet3/4
 switchport mode access
 switchport trunk encapsulation dot1q
 switchport mode trunk
 switchport nonegotiate
 shutdown
!
interface GigabitEthernet3/7
 switchport mode access
 switchport mode trunk
 switchport nonegotiate
!
`

	vars := map[string]interface{}{
		"mys_set_var": "my_set_value",
	}

	RunComparison(t, "match_function_set_example1", template, data, vars, nil)
}

// TestMatchFunctionReplaceallExample1 tests replaceall (Example-1 from docs)
func TestMatchFunctionReplaceallExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="ifs">
interface {{ interface | replaceall('GE', 'GigabitEthernet', 'GigEthernet', 'GeEthernet') }}
</group>`

	data := `interface GigabitEthernet3/3
interface GigEthernet5/7
interface GeEthernet1/5
`

	RunComparison(t, "match_function_replaceall_example1", template, data, nil, nil)
}

// TestMatchFunctionReplaceallExample2_1 tests replaceall with list variable (Example-2.1 from docs)
func TestMatchFunctionReplaceallExample2_1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
intf_replace = ['GigabitEthernet', 'GigEthernet', 'GeEthernet']
</vars>

<group name="ifs">
interface {{ interface | replaceall('GE', 'intf_replace') }}
</group>`

	data := `interface GigabitEthernet3/3
interface GigEthernet5/7
interface GeEthernet1/5
`

	vars := map[string]interface{}{
		"intf_replace": []interface{}{"GigabitEthernet", "GigEthernet", "GeEthernet"},
	}

	RunComparison(t, "match_function_replaceall_example2_1", template, data, vars, nil)
}

// TestMatchFunctionReplaceallExample2_2 tests replaceall with list variable and empty replacement (Example-2.2 from docs)
func TestMatchFunctionReplaceallExample2_2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
intf_replace = ['GigabitEthernet', 'GigEthernet', 'GeEthernet']
</vars>

<group name="ifs">
interface {{ interface | replaceall('intf_replace') }}
</group>`

	data := `interface GigabitEthernet3/3
interface GigEthernet5/7
interface GeEthernet1/5
`

	vars := map[string]interface{}{
		"intf_replace": []interface{}{"GigabitEthernet", "GigEthernet", "GeEthernet"},
	}

	RunComparison(t, "match_function_replaceall_example2_2", template, data, vars, nil)
}

// TestMatchFunctionResuballExample tests resuball function from docs
func TestMatchFunctionResuballExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
intf_replace = {
                'Ge': ['^GigabitEthernet'],
                'Te': ['^TenGigabitEthernet']
                }
</vars>

<group name="ifs">
interface {{ interface | resuball('intf_replace') }}
</group>`

	data := `interface GigabitEthernet3/3
interface TenGigabitEthernet3/3
`

	vars := map[string]interface{}{
		"intf_replace": map[string]interface{}{
			"Ge": []interface{}{"^GigabitEthernet"},
			"Te": []interface{}{"^TenGigabitEthernet"},
		},
	}

	RunComparison(t, "match_function_resuball_example", template, data, vars, nil)
}

