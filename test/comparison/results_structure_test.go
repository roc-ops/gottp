package comparison

import (
	"testing"
)

// TestResultsStructureGroupName tests group name attribute with nested paths
func TestResultsStructureGroupName(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces.vlan.L3.vrf-enabled">
interface {{ interface }}
  description {{ description }}
  ip address {{ ip }}/{{ mask }}
  vrf {{ vrf }}
</group>`

	data := `interface Vlan777
  description Management
  ip address 192.168.0.1/24
  vrf MGMT
`

	RunComparison(t, "results_structure_group_name", template, data, nil, nil)
}

// TestResultsStructureDynamicPathExample1 tests dynamic path (Example-1 from docs)
func TestResultsStructureDynamicPathExample1(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces.{{ interface }}">
interface {{ interface }}
  description {{ description }}
  ip address {{ ip }}/{{ mask }}
  vrf {{ vrf }}
</group>`

	data := `interface Port-Chanel11
  description Storage
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

	RunComparison(t, "results_structure_dynamic_path_example1", template, data, nil, nil)
}

// TestResultsStructureDynamicPathExample2 tests dynamic path with partial substitution
func TestResultsStructureDynamicPathExample2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces.cool_{{ interface }}_interface">
interface {{ interface }}
  description {{ description }}
  ip address {{ ip }}/{{ mask }}
  vrf {{ vrf }}
</group>`

	data := `interface Port-Chanel11
  description Storage
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

	RunComparison(t, "results_structure_dynamic_path_example2", template, data, nil, nil)
}

// TestResultsStructureDynamicPathExample3 tests nested dynamic paths
func TestResultsStructureDynamicPathExample3(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
hostname = "gethostname"
</vars>

<group name="{{ hostname }}.router.bgp.BGP_AS_{{ asn }}">
router bgp {{ asn }}
 <group name="vrf.{{ vrf_name }}">
  vrf {{ vrf_name }}
   <group name="neighbor.{{ neighbor_ip }}">
    neighbor {{ neighbor_ip }}
      remote-as {{ remote_as }}
      description {{ description }}
     <group name="address-family.{{ af }}">
      address-family {{ af }} unicast
        route-map {{ route_map_in }} in
        route-map {{ route_map_out }} out
     </group>
   </group>
 </group>
</group>`

	data := `router bgp 65100
  vrf CUST-1
    neighbor 59.100.71.193
      remote-as 65101
      description peer-1
      address-family ipv4 unicast
        route-map RPL-1-IMPORT-v4 in
        route-map RPL-1-EXPORT-V4 out
      address-family ipv6 unicast
        route-map RPL-1-IMPORT-V6 in
        route-map RPL-1-EXPORT-V6 out
    neighbor 59.100.71.209
      remote-as 65102
      description peer-2
      address-family ipv4 unicast
        route-map AAPTVRF-LB-BGP-IMPORT-V4 in
        route-map AAPTVRF-LB-BGP-EXPORT-V4 out
`

	vars := map[string]interface{}{
		"hostname": "gethostname",
	}

	RunComparison(t, "results_structure_dynamic_path_example3", template, data, vars, nil)
}

// TestResultsStructureAnonymousGroup tests anonymous group (group without name attribute)
func TestResultsStructureAnonymousGroup(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group>
interface {{ interface }}
  description {{ description }}
<group name = "ips">
  ip address {{ ip }}/{{ mask }}
</group>
  vrf {{ vrf }}
!{{_end_}}
</group>`

	data := `interface Port-Chanel11
  description Storage
!
interface Loopback0
  description RID
  ip address 10.0.0.3/24
!
interface Vlan777
  description Management
  ip address 192.168.0.1/24
  vrf MGMT
!
`

	RunComparison(t, "results_structure_anonymous_group", template, data, nil, nil)
}

// TestResultsStructurePathFormatters tests path formatters (* and **)
func TestResultsStructurePathFormatters(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces*.vlan.L3*.vrf-enabled">
interface {{ interface }}
  description {{ description }}
  ip address {{ ip }}/{{ mask }}
  vrf {{ vrf }}
</group>`

	data := `interface Vlan777
  description Management
  ip address 192.168.0.1/24
  vrf MGMT
`

	RunComparison(t, "results_structure_path_formatters", template, data, nil, nil)
}
