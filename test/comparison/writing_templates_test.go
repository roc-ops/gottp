package comparison

import (
	"testing"
)

// TestWritingTemplatesHierarchical tests hierarchical configuration parsing
func TestWritingTemplatesHierarchical(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="bgp_cfg">
router bgp {{ ASN }}
 <group name="ipv4_afi">
 address-family ipv4 unicast {{ _start_ }}
  router-id {{ bgp_rid }}
 </group>

 <group name="vrfs">
 vrf {{ vrf }}
  rd {{ rd }}

  <group name="neighbors">
  neighbor {{ neighbor }}
   remote-as {{ neighbor_asn }}
   <group name="ipv4_afi">
   address-family ipv4 unicast {{ _start_ }}
    send-community-ebgp {{ send_community_ebgp | set("Enabled") }}
    route-policy {{ RPL_IN }} in
    route-policy {{ RPL_OUT }} out
   </group>
  </group>
 </group>
</group>`

	data := `router bgp 12.34
 address-family ipv4 unicast
  router-id 1.1.1.1
 !
 vrf CT2S2
  rd 102:103
  !
  neighbor 10.1.102.102
   remote-as 102.103
   address-family ipv4 unicast
    send-community-ebgp
    route-policy vCE102-link1.102 in
    route-policy vCE102-link1.102 out
   !
  !
  neighbor 10.2.102.102
   remote-as 102.103
   address-family ipv4 unicast
    route-policy vCE102-link2.102 in
    route-policy vCE102-link2.102 out
   !
 !
 vrf AS65000
  rd 102:104
  !
  neighbor 10.1.37.7
   remote-as 65000
   address-family ipv4 labeled-unicast
    route-policy PASS-ALL in
    route-policy PASS-ALL out
`

	RunComparison(t, "writing_templates_hierarchical", template, data, nil, nil)
}

// TestWritingTemplatesTextTable tests text table parsing
func TestWritingTemplatesTextTable(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="table_data" method="table">
Internet  {{ ip | IP }}  {{ age }}   {{ mac }}  ARPA   {{ interface }}
Internet  {{ ip | IP }}  {{ age }}   {{ mac }}  ARPA   {{ interface }}  *
</group>`

	data := `Protocol  Address     Age (min)  Hardware Addr   Type   Interface
Internet  10.12.13.1        98   0950.5785.5cd1  ARPA   FastEthernet2.13
Internet  10.12.13.3       131   0150.7685.14d5  ARPA   GigabitEthernet2.13
Internet  10.12.13.4       198   0950.5C8A.5c41  ARPA   GigabitEthernet2.17
Internet  10.12.14.5       -     0950.5C8A.5d42  ARPA   GigabitEthernet3
Internet  10.12.15.6       164   0950.5C8A.5e43  ARPA   GigabitEthernet4.21  *
`

	RunComparison(t, "writing_templates_text_table", template, data, nil, nil)
}

