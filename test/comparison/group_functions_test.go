package comparison

import (
	"testing"
)

// TestGroupFunctionBasic tests basic group functions
func TestGroupFunctionBasic(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	tests := []struct {
		name     string
		template string
		data     string
	}{
		{
			name: "contains",
			template: `<group name="interfaces" functions="contains('ip')">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
 description Router-id-loopback
interface Vlan100
 description Management-VLAN
`,
		},
		{
			name: "containsall",
			template: `<group name="interfaces" functions="containsall('ip', 'mask')">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
 description Router-id-loopback
interface Vlan100
 description Management-VLAN
`,
		},
		{
			name: "set",
			template: `<vars>
default_vrf = "default"
</vars>
<group name="interfaces" functions="set('vrf', 'default_vrf')">
interface {{ interface }}
 ip address {{ ip }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
`,
		},
		{
			name: "record",
			template: `<group name="interfaces" functions="record('last_interface')">
interface {{ interface }}
 ip address {{ ip }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`,
		},
		{
			name: "delete",
			template: `<group name="interfaces" functions="del('mask')">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
`,
		},
		{
			name: "exclude",
			template: `<group name="interfaces" functions="exclude('deleted')">
interface {{ interface }}
 ip address {{ ip }}
 deleted {{ deleted }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
 deleted true
`,
		},
		{
			name: "excludeall",
			template: `<group name="interfaces" functions="excludeall('deleted', 'disabled')">
interface {{ interface }}
 ip address {{ ip }}
 deleted {{ deleted }}
 disabled {{ disabled }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
 deleted true
 disabled true
`,
		},
		{
			name: "equal",
			template: `<group name="interfaces" functions="equal('status', 'active')">
interface {{ interface }}
 ip address {{ ip }}
 status {{ status }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
 status active
interface Vlan100
 ip address 10.0.0.1/24
 status inactive
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunComparison(t, tt.name, tt.template, tt.data, nil, nil)
		})
	}
}

// TestGroupFunctionTransformation tests transformation group functions
func TestGroupFunctionTransformation(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	tests := []struct {
		name     string
		template string
		data     string
	}{
		{
			name: "to_int",
			template: `<group name="interfaces" functions="to_int('port', 'vlan')">
interface {{ interface }}
 port {{ port }}
 vlan {{ vlan }}
</group>`,
			data: `interface Loopback0
 port 8080
 vlan 100
`,
		},
		{
			name: "to_ip",
			template: `<group name="interfaces" functions="to_ip('ip', 'gateway')">
interface {{ interface }}
 ip address {{ ip }}
 gateway {{ gateway }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1
 gateway 192.168.0.254
`,
		},
		{
			name: "expand",
			template: `<group name="interfaces" functions="expand">
interface {{ interface.name }}
 ip address {{ ip }}
</group>`,
			data: `interface Loopback0
 ip address 192.168.0.1/24
`,
		},
		{
			name: "itemize",
			template: `<group name="vlans" functions="itemize('vlan_list')">
vlan {{ vlan_list | split(",") }}
</group>`,
			data: `vlan 100,200,300
`,
		},
		{
			name: "sformat",
			template: `<group name="interfaces" functions="sformat('description', 'Interface {interface} on {device}')">
interface {{ interface }}
 device {{ device }}
</group>`,
			data: `interface Loopback0
 device router1
`,
		},
		{
			name: "items2dict",
			template: `<group name="config" functions="items2dict('key', 'value')">
key {{ key }}
value {{ value }}
</group>`,
			data: `key hostname
value router1
key domain
value example.com
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RunComparison(t, tt.name, tt.template, tt.data, nil, nil)
		})
	}
}

// TestGroupFunctionLookup tests lookup group function
func TestGroupFunctionLookup(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="ip_table">
'10.0.0.1': 'host1'
'10.0.0.2': 'host2'
</lookup>

<group name="interfaces" functions="lookup('ip', name='ip_table', add_field='hostname')">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 10.0.0.1/24
interface Vlan100
 ip address 10.0.0.2/24
`

	RunComparison(t, "lookup", template, data, nil, nil)
}

// TestGroupFunctionValidation tests validation group functions
func TestGroupFunctionValidation(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Skip if Cerberus is not installed in Python TTP
	// Python TTP requires Cerberus library to be installed for validation
	skipIfKnownDifference(t, "cerberus_validation")

	template := `<vars>
interface_schema = {
  "interface": {"type": "string", "required": true},
  "ip": {"type": "string", "regex": "^\\d+\\.\\d+\\.\\d+\\.\\d+$"},
  "mask": {"type": "integer", "min": 0, "max": 32}
}
</vars>

<group name="interfaces" functions="cerberus('interface_schema')">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "cerberus", template, data, nil, nil)
}

// TestGroupFunctionChain tests function chaining
func TestGroupFunctionChain(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces" functions="contains('ip') | record('last_interface') | del('mask')">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
 description Router-id-loopback
interface Vlan100
 description Management-VLAN
`

	RunComparison(t, "chain", template, data, nil, nil)
}

// TestGroupFunctionMacro tests macro group function
func TestGroupFunctionMacro(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<macro>
def process_interface(data):
    data["processed"] = True
    return data
</macro>

<group name="interfaces" macro="process_interface">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	RunComparison(t, "macro", template, data, nil, nil)
}

