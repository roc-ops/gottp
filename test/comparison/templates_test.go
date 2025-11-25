package comparison

import (
	"os"
	"testing"
)

// TestTemplateStructure tests basic template structure
func TestTemplateStructure(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "template_structure", template, data, nil, nil)
}

// TestTemplateNestedGroups tests nested groups
func TestTemplateNestedGroups(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="device">
hostname {{ hostname }}

<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>
</group>`

	data := `hostname router1
interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
hostname router2
interface Loopback0
 ip address 192.168.1.1/24
`

	RunComparison(t, "template_nested_groups", template, data, nil, nil)
}

// TestTemplateDynamicPath tests dynamic path formation
func TestTemplateDynamicPath(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces.{{ interface }}">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "template_dynamic_path", template, data, nil, nil)
}

// TestTemplateVariables tests template variables
func TestTemplateVariables(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<vars>
site = "datacenter1"
default_vrf = "default"
</vars>

<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 site {{ site }}
 vrf {{ default_vrf }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	vars := map[string]interface{}{
		"site":        "datacenter1",
		"default_vrf": "default",
	}

	RunComparison(t, "template_variables", template, data, vars, nil)
}

// TestTemplateExtend tests template extension
func TestTemplateExtend(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Note: Extend tag may need special handling
	// For now, test with inline template combining both groups
	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>
<group name="vlans">
vlan {{ vlan }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
vlan 100
`

	RunComparison(t, "template_extend", template, data, nil, nil)
}

// TestTemplateAnonymousGroup tests anonymous groups
func TestTemplateAnonymousGroup(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group>
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	RunComparison(t, "template_anonymous_group", template, data, nil, nil)
}

// TestTemplateMethodAttribute tests method attribute
func TestTemplateMethodAttribute(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces" method="table">
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "template_method_attribute", template, data, nil, nil)
}

// TestTemplateInputOutput tests input and output attributes
func TestTemplateInputOutput(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<input name="config1">
interface {{ interface }}
 ip address {{ ip }}
</input>

<group name="interfaces" input="config1">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output name="output1" format="json">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	RunComparison(t, "template_input_output", template, data, nil, nil)
}

// TestTemplateStartEndIndicator tests start and end indicators
func TestTemplateStartEndIndicator(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
_start_
interface {{ interface }}
 ip address {{ ip }}
_end_
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "template_start_end_indicator", template, data, nil, nil)
}

// TestTemplateLineIndicator tests line indicator
func TestTemplateLineIndicator(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
_line_
interface {{ interface }}
 ip address {{ ip }}
</group>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "template_line_indicator", template, data, nil, nil)
}

// TestIssue13 tests issue #13: Inconsistent output compared to Python TTP
// This test loads a gottp-config.json file exported from the editor
// To add a test case, place the gottp-config.json file in test/comparison/fixtures/issue_13/
func TestIssue13(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	// Try to find config file in fixtures
	configPath, err := fixturePath("issue_13", "gottp-config.json")
	if err != nil {
		t.Fatalf("Failed to get fixture path: %v", err)
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("Config file not found at %s - skipping test. To add a test case, export your editor configuration and place it at this path.", configPath)
	}

	// Run comparison with config file
	RunComparisonWithConfig(t, "issue_13", configPath)
}

