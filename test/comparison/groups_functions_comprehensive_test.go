package comparison

import (
	"testing"
)

// TestGroupFunctionContainsAllExample tests containsall function from docs
func TestGroupFunctionContainsAllExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces" containsall="ip, vrf">
interface {{ interface }}
  description {{ description }}
  ip address {{ ip }}/{{ mask }}
  vrf {{ vrf }}
</group>`

	data := `interface Port-Chanel11
  description Storage Management
!
interface Loopback0
  description RID
  ip address 10.0.0.3/24
!
interface Vlan777
  description Management
  ip address 192.168.0.1/24
  vrf MGMT
`

	RunComparison(t, "group_function_containsall_example", template, data, nil, nil)
}

// TestGroupFunctionContainsExample tests contains function from docs
func TestGroupFunctionContainsExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces" contains="ip, vrf">
interface {{ interface }}
  description {{ description }}
  ip address {{ ip }}/{{ mask }}
  vrf {{ vrf }}
</group>`

	data := `interface Port-Chanel11
  description Storage Management
!
interface Loopback0
  description RID
  ip address 10.0.0.3/24
!
interface Vlan777
  description Management
  ip address 192.168.0.1/24
  vrf MGMT
`

	RunComparison(t, "group_function_contains_example", template, data, nil, nil)
}

// TestGroupFunctionDelExample tests del function from docs
func TestGroupFunctionDelExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces-test1-31" del="description, ip">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description | ORPHRASE }}
 switchport port-security mac {{ sec_mac }}
</group>`

	data := `interface Vlan778
 description some description 1
 ip address 2002:fd37::91/124
!
interface Vlan779
 description some description 2
!
interface Vlan780
 switchport port-security mac 4
!
`

	RunComparison(t, "group_function_del_example", template, data, nil, nil)
}

// TestGroupFunctionSformatExample tests sformat function from docs
func TestGroupFunctionSformatExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
domain = "com"
</vars>

<group name="uptime">
{{ hostname | record("hostname")}} uptime is {{ uptime | PHRASE }}
</group>

<group name="fqdn_dets_1" sformat="string='{hostname}.{fqdn},{domain}', add_field='fqdn'">
Default domain is {{ fqdn }}
</group>`

	data := `switch-1 uptime is 27 weeks, 3 days, 10 hours, 46 minutes, 10 seconds
Default domain is lab.local
`

	RunComparison(t, "group_function_sformat_example", template, data, nil, nil)
}

// TestGroupFunctionItemizeExample tests itemize function from docs
func TestGroupFunctionItemizeExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces_list" itemize="interface">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Vlan778
 description some description 1
 ip address 2002:fd37::91/124
!
interface Vlan779
 description some description 2
!
interface Vlan780
 switchport port-security mac 4
 ip address 192.168.1.1/124
!
`

	RunComparison(t, "group_function_itemize_example", template, data, nil, nil)
}

// TestGroupFunctionContainsValExample1 tests contains_val function (Example-1 from docs)
func TestGroupFunctionContainsValExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces" contains_val="'ip', '2.2.2.2/24'">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Vlan779
 ip address 2.2.2.2/24
!
interface Vlan780
 ip address 2.2.2.3/24
!
`

	RunComparison(t, "group_function_contains_val_example1", template, data, nil, nil)
}

// TestGroupFunctionContainsValExample2 tests contains_val function with variable (Example-2 from docs)
func TestGroupFunctionContainsValExample2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
ip_in_question="1.1.1.1"
</vars>

<group contains_val="ip, ip_in_question">
interface {{ interface }}
ip address {{ ip }} {{ mask }}
</group>`

	data := `interface Lo0
ip address 124.171.238.50 32
!
interface Lo1
ip address 1.1.1.1 32
`

	vars := map[string]interface{}{
		"ip_in_question": "1.1.1.1",
	}

	RunComparison(t, "group_function_contains_val_example2", template, data, vars, nil)
}

// TestGroupFunctionExcludeValExample tests exclude_val function from docs
func TestGroupFunctionExcludeValExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
ip_in_question="1.1.1.1"
</vars>

<group exclude_val="ip, ip_in_question">
interface {{ interface }}
ip address {{ ip }} {{ mask }}
</group>`

	data := `interface Lo0
ip address 124.171.238.50 32
!
interface Lo1
ip address 1.1.1.1 32
`

	vars := map[string]interface{}{
		"ip_in_question": "1.1.1.1",
	}

	RunComparison(t, "group_function_exclude_val_example", template, data, vars, nil)
}

// TestGroupFunctionSetExample tests set function with default value from docs
func TestGroupFunctionSetExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="bgp_config">
router bgp {{ bgp_asn }}

<group name="VRFs" record="vrf">
 address-family {{ afi }} vrf {{ vrf }}
 address-family {{ afi | _start_ }}
  <group name="neighbors**.{{ neighbor }}**" method="table" set="vrf, default='global'">
  neighbor {{ neighbor | let("afi_activated", True) }} activate
  </group>
 exit-address-family {{ _end_ }}
</group>

</group>`

	data := `router bgp 65123
 !
 address-family ipv4
  neighbor 10.100.100.212 activate
  neighbor 10.227.147.122 activate
 exit-address-family
 !
 address-family ipv4 vrf VRF1
  neighbor 10.61.254.67 activate
  neighbor 10.61.254.68 activate
 exit-address-family
`

	RunComparison(t, "group_function_set_example", template, data, nil, nil)
}

// TestGroupFunctionExpandExample tests expand function from docs
func TestGroupFunctionExpandExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="cdp*" expand="">
Device ID: {{ target.id }}
  IP address: {{ target.top_label }}
Platform: {{ target.bottom_label | ORPHRASE }},  Capabilities: {{ ignore(ORPHRASE) }}
Interface: {{ src_label }},  Port ID (outgoing port): {{ trgt_label | ORPHRASE }}
</group>`

	data := `switch-1#show cdp neighbors detail
-------------------------
Device ID: switch-2
Entry address(es):
  IP address: 10.13.1.7
Platform: cisco WS-C6509,  Capabilities: Router Switch IGMP
Interface: GigabitEthernet4/6,  Port ID (outgoing port): GigabitEthernet1/5

-------------------------
Device ID: switch-3
Entry address(es):
  IP address: 10.17.14.1
Platform: cisco WS-C3560-48TS,  Capabilities: Switch IGMP
Interface: GigabitEthernet1/1,  Port ID (outgoing port): GigabitEthernet0/1
`

	RunComparison(t, "group_function_expand_example", template, data, nil, nil)
}

// TestGroupFunctionToIntIntlistExample tests to_int with intlist from docs
func TestGroupFunctionToIntIntlistExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group to_int="trunk_vlan, intlist=True">
interface {{ name }}
   switchport trunk allowed vlan {{ trunk_vlan | split(',') }}
</group>`

	data := `interface GigabitEthernet1/1
   switchport trunk allowed vlan 1,2,3,4
!
interface GigabitEthernet1/2
   switchport trunk allowed vlan 123
!
interface GigabitEthernet1/3
   switchport trunk allowed vlan foo,bar
!
interface GigabitEthernet1/4
!
`

	RunComparison(t, "group_function_to_int_intlist_example", template, data, nil, nil)
}

