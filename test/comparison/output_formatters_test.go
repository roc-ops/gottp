package comparison

import (
	"testing"
)

// TestOutputFormatterJSON tests JSON output formatter
func TestOutputFormatterJSON(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="json">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	// For output formatters, we need to compare the formatted output
	// This is a simplified test - full implementation would capture output
	RunComparison(t, "json_formatter", template, data, nil, nil)
}

// TestOutputFormatterYAML tests YAML output formatter
func TestOutputFormatterYAML(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	skipIfKnownDifference(t, "yaml_formatter")

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="yaml">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	RunComparison(t, "yaml_formatter", template, data, nil, nil)
}

// TestOutputFormatterRaw tests raw output formatter
func TestOutputFormatterRaw(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="raw">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	RunComparison(t, "raw_formatter", template, data, nil, nil)
}

// TestOutputFormatterCSV tests CSV output formatter
func TestOutputFormatterCSV(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="csv">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "csv_formatter", template, data, nil, nil)
}

// TestOutputFormatterTable tests table output formatter
func TestOutputFormatterTable(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	skipIfKnownDifference(t, "table_formatter")

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="table">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "table_formatter", template, data, nil, nil)
}

// TestOutputFormatterPPrint tests pprint output formatter
func TestOutputFormatterPPrint(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="pprint">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	RunComparison(t, "pprint_formatter", template, data, nil, nil)
}

// TestOutputFormatterTabulate tests tabulate output formatter
func TestOutputFormatterTabulate(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	skipIfKnownDifference(t, "tabulate_formatter")

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="tabulate">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "tabulate_formatter", template, data, nil, nil)
}

// TestOutputFormatterExcel tests Excel output formatter
func TestOutputFormatterExcel(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	skipIfKnownDifference(t, "excel_formatter")

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="excel">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	// Excel output is binary, so comparison may need special handling
	RunComparison(t, "excel_formatter", template, data, nil, nil)
}

// TestOutputFormatterJinja2 tests Jinja2 output formatter
func TestOutputFormatterJinja2(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	skipIfKnownDifference(t, "jinja2_formatter")

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="jinja2">
Interfaces:
{% for item in _data_ %}
- {{ item.interface }}: {{ item.ip }}
{% endfor %}
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "jinja2_formatter", template, data, nil, nil)
}

// TestOutputFormatterN2G tests N2G output formatter
func TestOutputFormatterN2G(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	skipIfKnownDifference(t, "n2g_formatter")

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="n2g" module="yed">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	// N2G output is XML, may need special comparison
	RunComparison(t, "n2g_formatter", template, data, nil, nil)
}

// TestOutputFormatterWithPath tests output formatter with path
func TestOutputFormatterWithPath(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces.{{ interface }}">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="json" path="interfaces">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`

	RunComparison(t, "formatter_with_path", template, data, nil, nil)
}

// TestOutputFormatterWithFunctions tests output formatter with functions
func TestOutputFormatterWithFunctions(t *testing.T) {
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available")
	}

	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>

<output format="json" functions="traverse('interfaces')">
</output>`

	data := `interface Loopback0
 ip address 192.168.0.1/24
`

	RunComparison(t, "formatter_with_functions", template, data, nil, nil)
}

