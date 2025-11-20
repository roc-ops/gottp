package comparison

import (
	"testing"
)

// TestTemplateVariablesGethostname tests gethostname getter
func TestTemplateVariablesGethostname(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars name="vars">
hostname_var = "gethostname"
</vars>

<group name="interfaces">
interface {{ interface }}
 description {{ description }}
</group>`

	data := `switch1#show run int
interface GigabitEthernet3/11
 description input_1_data
`

	RunComparison(t, "template_variables_gethostname", template, data, nil, nil)
}

// TestTemplateVariablesNameAttribute tests vars name attribute with dynamic path
func TestTemplateVariablesNameAttribute(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars name="vars.info**.{{ hostname }}">
hostname='switch-1'
serial='AS4FCVG456'
model='WS-3560-PS'
</vars>

<vars name="vars.ip*">
IP="Undefined"
MASK="255.255.255.255"
</vars>

<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }} {{ mask }}
</group>`

	data := `interface Vlan777
 ip address 192.168.0.1/24
`

	RunComparison(t, "template_variables_name_attribute", template, data, nil, nil)
}

