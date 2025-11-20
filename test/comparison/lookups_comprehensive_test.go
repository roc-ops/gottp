package comparison

import (
	"testing"
)

// TestLookupCSVExample tests CSV lookup from docs
func TestLookupCSVExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="aux_csv" load="csv">
ASN,as_name,as_description,prefix_num
65100,Subs,Private ASN,734
65200,Privs,Undef ASN,121
</lookup>

<group name="bgp_config">
router bgp {{ bgp_as | lookup("aux_csv", add_field="as_details") }}
</group>`

	data := `router bgp 65100
`

	RunComparison(t, "lookup_csv_example", template, data, nil, nil)
}

// TestLookupINIExample tests INI lookup from docs
func TestLookupINIExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="locations" load="ini">
[cities]
-mel- : 7 Name St, Suburb A, Melbourne, Postal Code
-bri- : 8 Name St, Suburb B, Brisbane, Postal Code
</lookup>

<group name="bgp_config">
router bgp {{ bgp_as }}
 <group name="peers">
  neighbor {{ peer }}
    description {{ description | rlookup("locations.cities", add_field="location") }}
 </group>
</group>`

	data := `router bgp 65100
  neighbor 10.145.1.9
    description vic-mel-core1
  !
  neighbor 192.168.101.1
    description qld-bri-core1
`

	RunComparison(t, "lookup_ini_example", template, data, nil, nil)
}

// TestLookupYAMLExample tests YAML lookup from docs
func TestLookupYAMLExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="yaml_look" load="yaml">
'65100':
  as_description: Private ASN
  as_name: Subs
  prefix_num: '734'
'65101':
  as_description: Cust-1 ASN
  as_name: Cust1
  prefix_num: '156'
</lookup>

<group name="bgp_config">
router bgp {{ bgp_as | lookup("yaml_look", add_field="as_details") }}
</group>`

	data := `router bgp 65100
`

	RunComparison(t, "lookup_yaml_example", template, data, nil, nil)
}

