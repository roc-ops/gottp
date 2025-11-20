package comparison

import (
	"testing"
)

// TestInputAttributeLoadExample tests input load attribute from docs
func TestInputAttributeLoadExample(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<input name="test1" load="text" groups="interfaces.trunks">
interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166-173
!
interface GigabitEthernet3/4
 switchport trunk allowed vlan add 100-105
!
interface GigabitEthernet3/5
 switchport trunk allowed vlan add 459,531,704-707
</input>

<group name="interfaces.trunks">
interface {{ interface }}
 switchport trunk allowed vlan add {{ trunk_vlans }}
</group>`

	data := `interface GigabitEthernet3/3
 switchport trunk allowed vlan add 138,166-173
!
interface GigabitEthernet3/4
 switchport trunk allowed vlan add 100-105
!
interface GigabitEthernet3/5
 switchport trunk allowed vlan add 459,531,704-707
`

	RunComparison(t, "input_attribute_load_example", template, data, nil, nil)
}

