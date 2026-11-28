package gottp

import (
	"encoding/json"
	"testing"
)

// TestRecordGroupOutputOrderDeterministic is a regression test for #78:
// sibling matches of a group using record= must come out in input order
// (Python TTP 0.10.1 order) on every run, not in Go map order.
func TestRecordGroupOutputOrderDeterministic(t *testing.T) {
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

	// Output of Python TTP 0.10.1 parser.result() for the same inputs.
	want := `[[{"bgp_config":{"VRFs":[` +
		`{"afi":"ipv4","neighbors":{"10.100.100.212":{"afi_activated":true,"vrf":"global"},"10.227.147.122":{"afi_activated":true,"vrf":"global"}}},` +
		`{"afi":"ipv4","neighbors":{"10.61.254.67":{"afi_activated":true,"vrf":"VRF1"},"10.61.254.68":{"afi_activated":true,"vrf":"VRF1"}},"vrf":"VRF1"}` +
		`],"bgp_asn":"65123"}}]]`

	for i := 0; i < 50; i++ {
		p := NewParser()
		if err := p.AddTemplate(template); err != nil {
			t.Fatalf("AddTemplate: %v", err)
		}
		p.AddInput(data, "")
		if err := p.Parse(); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		got, err := json.Marshal(p.Result())
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(got) != want {
			t.Fatalf("run %d: output differs from Python TTP 0.10.1\n got: %s\nwant: %s", i, got, want)
		}
	}
}
