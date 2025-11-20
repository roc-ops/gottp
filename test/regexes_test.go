package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestRegexes tests various regex patterns from Python TTP tests
func TestRegexes(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     string
		expected map[string]interface{}
	}{
		{
			name: "basic variable extraction",
			template: `
<group>
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`,
			data: `
interface Loopback0
 ip address 192.168.0.1/24
!
`,
			expected: map[string]interface{}{
				"interface": "Loopback0",
				"ip":        "192.168.0.1",
				"mask":      "24",
			},
		},
		{
			name: "multiple matches",
			template: `
<group>
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`,
			data: `
interface Lo0
 ip address 1.1.1.1/32
!
interface Lo1
 ip address 2.2.2.2/32
!
`,
			expected: nil, // Will be a list
		},
		{
			name: "nested groups",
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
			expected: nil, // Will be nested structure
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

			// Basic validation
			if result == nil {
				t.Error("Result is nil")
			}
		})
	}
}

