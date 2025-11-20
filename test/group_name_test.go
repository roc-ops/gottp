package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestGroupNameAttribute tests group name attribute functionality
func TestGroupNameAttribute(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     string
	}{
		{
			name: "named group",
			template: `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`,
			data: `
interface Lo0
 ip address 1.1.1.1/32
!
`,
		},
		{
			name: "dynamic group name",
			template: `
<group name="vlans.{{ vlan }}">
vlan {{ vlan }}
 name {{ name }}
</group>
`,
			data: `
vlan 100
 name test_vlan
!
vlan 200
 name another_vlan
!
`,
		},
		{
			name: "nested named groups",
			template: `
<group name="device">
hostname {{ hostname }}
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>
</group>
`,
			data: `
hostname router1
interface Lo0
 ip address 1.1.1.1
!
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := gottp.CompileTemplate(tt.template)
			if err != nil {
				t.Fatalf("Failed to compile template: %v", err)
			}

			result, err := compiled.Parse(
				gottp.Inputs{"Default_Input": tt.data},
				nil,
				nil,
			)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			jsonData, _ := json.MarshalIndent(result, "", "  ")
			t.Logf("Result: %s", string(jsonData))

			if result == nil {
				t.Error("Result is nil")
			}
		})
	}
}

