package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestGroupSetFunction tests group set function
func TestGroupSetFunction(t *testing.T) {
	template := `
<input load="text">
hostname r2
!
interface GigabitEthernet1
 vrf forwarding MGMT
 ip address 10.123.89.55 255.255.255.0
</input>

<group void="">
hostname {{ hostname | record(hostname_abc) }}
</group>

<group>
interface {{ interface }}
 description {{ description | ORPHRASE }}
 ip address {{ ip }} {{ mask }}
 {{ hostname | set(hostname_abc) }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
hostname r2
!
interface GigabitEthernet1
 vrf forwarding MGMT
 ip address 10.123.89.55 255.255.255.0
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

// TestGroupRecordFunction tests group record function
func TestGroupRecordFunction(t *testing.T) {
	template := `
<input load="text">
hostname r2
!
interface GigabitEthernet1
 ip address 10.123.89.55 255.255.255.0
</input>

<group void="">
hostname {{ hostname | record(hostname_abc) }}
</group>

<group>
interface {{ interface }}
 ip address {{ ip }} {{ mask }}
 {{ hostname | set(hostname_abc) }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
hostname r2
!
interface GigabitEthernet1
 ip address 10.123.89.55 255.255.255.0
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

// TestGroupContainsFunction tests group contains function
func TestGroupContainsFunction(t *testing.T) {
	template := `
<input load="text">
interface Lo0
 ip address 1.1.1.1 32
!
interface Lo1
 description test
 ip address 2.2.2.2 32
</input>

<group contains="description">
interface {{ interface }}
 description {{ description }}
 ip address {{ ip }} {{ mask }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `
interface Lo0
 ip address 1.1.1.1 32
!
interface Lo1
 description test
 ip address 2.2.2.2 32
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

