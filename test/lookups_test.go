package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestLookupTable tests lookup table functionality
func TestLookupTable(t *testing.T) {
	template := `
<lookup name="bgp_asn" load="yaml">
'65100':
  as_description: Private ASN
  as_name: Subs
  prefix_num: '734'
'65101':
  as_description: Cust-1 ASN
  as_name: Cust1
  prefix_num: '156'
</lookup>

<input load="text">
router bgp 65100
</input>

<group name="bgp_config">
router bgp {{ bgp_as | lookup("bgp_asn", add_field="as_details") }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
router bgp 65100
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

	if result == nil {
		t.Error("Result should not be nil")
	}
}

// TestLookupFromVars tests lookup using variables
func TestLookupFromVars(t *testing.T) {
	template := `
<vars>
lookup_data = {
    '65100': {
        'as_description': 'Private ASN',
        'as_name': 'Subs',
    }
}
</vars>

<input load="text">
router bgp 65100
</input>

<group name="bgp_config">
router bgp {{ bgp_as }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
router bgp 65100
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

	if result == nil {
		t.Error("Result should not be nil")
	}
}

