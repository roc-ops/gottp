package comparison

import (
	"testing"
)

// TestLookupBasic tests basic lookup table functionality
func TestLookupBasic(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="ip_table">
'10.0.0.1': 'host1'
'10.0.0.2': 'host2'
'10.0.0.3': 'host3'
</lookup>

<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 hostname {{ ip | lookup("ip_table") }}
</group>`

	data := `interface Loopback0
 ip address 10.0.0.1/24
interface Vlan100
 ip address 10.0.0.2/24
interface Vlan200
 ip address 10.0.0.4/24
`

	RunComparison(t, "lookup_basic", template, data, nil, nil)
}

// TestLookupYAML tests YAML lookup table
func TestLookupYAML(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="ip_table" load="yaml">
'10.0.0.1': 'host1'
'10.0.0.2': 'host2'
</lookup>

<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 hostname {{ ip | lookup("ip_table") }}
</group>`

	data := `interface Loopback0
 ip address 10.0.0.1/24
`

	RunComparison(t, "lookup_yaml", template, data, nil, nil)
}

// TestLookupJSON tests JSON lookup table
func TestLookupJSON(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="ip_table" load="json">
{"10.0.0.1": "host1", "10.0.0.2": "host2"}
</lookup>

<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 hostname {{ ip | lookup("ip_table") }}
</group>`

	data := `interface Loopback0
 ip address 10.0.0.1/24
`

	RunComparison(t, "lookup_json", template, data, nil, nil)
}

// TestLookupReverse tests reverse lookup
func TestLookupReverse(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="ip_table">
'10.0.0.1': 'host1'
'10.0.0.2': 'host2'
</lookup>

<group name="interfaces">
interface {{ interface }}
 hostname {{ hostname }}
 ip {{ hostname | rlookup("ip_table") }}
</group>`

	data := `interface Loopback0
 hostname host1
interface Vlan100
 hostname host2
`

	RunComparison(t, "lookup_reverse", template, data, nil, nil)
}

// TestLookupGlobPattern tests glob pattern lookup
func TestLookupGlobPattern(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="domain_table">
'*.example.com': 'internal'
'*.external.com': 'external'
'host.example.com': 'special'
</lookup>

<group name="hosts">
hostname {{ hostname }}
domain_type {{ hostname | gpvlookup("domain_table") }}
</group>`

	data := `hostname host.example.com
hostname server.external.com
hostname test.example.com
`

	RunComparison(t, "lookup_glob_pattern", template, data, nil, nil)
}

// TestLookupGroupFunction tests lookup in group function
func TestLookupGroupFunction(t *testing.T) {
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

	RunComparison(t, "lookup_group_function", template, data, nil, nil)
}

// TestLookupWithUpdate tests lookup with update option
func TestLookupWithUpdate(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="ip_table">
'10.0.0.1': {'hostname': 'host1', 'site': 'dc1'}
'10.0.0.2': {'hostname': 'host2', 'site': 'dc2'}
</lookup>

<group name="interfaces" functions="lookup('ip', name='ip_table', update=True)">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 10.0.0.1/24
`

	RunComparison(t, "lookup_with_update", template, data, nil, nil)
}

// TestLookupMultipleValues tests lookup with multiple values
func TestLookupMultipleValues(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<lookup name="vlan_table">
'100': ['port1', 'port2']
'200': ['port3', 'port4']
</lookup>

<group name="vlans">
vlan {{ vlan }}
ports {{ vlan | lookup("vlan_table") }}
</group>`

	data := `vlan 100
vlan 200
`

	RunComparison(t, "lookup_multiple_values", template, data, nil, nil)
}

