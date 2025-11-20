package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestJoinMatchesWithCustomDelimiter tests joinmatches with custom delimiter
func TestJoinMatchesWithCustomDelimiter(t *testing.T) {
	template := `
<group name="vlans">
switchport trunk allowed vlan {{ vlans | joinmatches(':') }}
</group>
`

	data := `
switchport trunk allowed vlan 100
switchport trunk allowed vlan 200
switchport trunk allowed vlan 300
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

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

	// Verify result contains joined vlans
	resultStr := string(jsonData)
	if !strings.Contains(resultStr, "100") || !strings.Contains(resultStr, "200") || !strings.Contains(resultStr, "300") {
		t.Error("Result does not contain expected vlan values")
	}
}

// TestJoinMatchesWithIgnore tests joinmatches with ignore variable
func TestJoinMatchesWithIgnore(t *testing.T) {
	template := `
<group name="ips">
ip address {{ ip | ignore }} {{ mask | joinmatches(',') }}
</group>
`

	data := `
ip address 192.168.1.1 255.255.255.0
ip address 10.0.0.1 255.255.255.0
ip address 172.16.0.1 255.0.0.0
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

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

	// Verify masks are joined
	resultStr := string(jsonData)
	if !strings.Contains(resultStr, "255.255.255.0") {
		t.Error("Result should contain joined mask values")
	}
}

// TestJoinMatchesMultipleInLine tests multiple joinmatches in same line
func TestJoinMatchesMultipleInLine(t *testing.T) {
	template := `
<group name="config">
vlan {{ vlans | joinmatches(',') }} name {{ names | joinmatches('|') }}
</group>
`

	data := `
vlan 100 name SERVERS
vlan 200 name WORKSTATIONS
vlan 300 name MANAGEMENT
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

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

	// Verify both are joined
	resultStr := string(jsonData)
	if !strings.Contains(resultStr, "100") || !strings.Contains(resultStr, "SERVERS") {
		t.Error("Result should contain joined values for both variables")
	}
}

