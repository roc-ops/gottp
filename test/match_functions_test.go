package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestMatchFunctions tests various match functions
func TestMatchFunctions(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     string
		validate func(t *testing.T, result interface{})
	}{
		{
			name: "upper function",
			template: `
<group>
{{ value | upper }}
</group>
`,
			data: "test_value",
			validate: func(t *testing.T, result interface{}) {
				jsonData, _ := json.Marshal(result)
				t.Logf("Result: %s", string(jsonData))
			},
		},
		{
			name: "lower function",
			template: `
<group>
{{ value | lower }}
</group>
`,
			data: "TEST_VALUE",
			validate: func(t *testing.T, result interface{}) {
				jsonData, _ := json.Marshal(result)
				t.Logf("Result: %s", string(jsonData))
			},
		},
		{
			name: "IP function",
			template: `
<group>
ip address {{ ip | IP }}
</group>
`,
			data: "ip address 192.168.0.1",
			validate: func(t *testing.T, result interface{}) {
				jsonData, _ := json.Marshal(result)
				t.Logf("Result: %s", string(jsonData))
			},
		},
		{
			name: "split function",
			template: `
<group>
{{ values | split(',') }}
</group>
`,
			data: "a,b,c",
			validate: func(t *testing.T, result interface{}) {
				jsonData, _ := json.Marshal(result)
				t.Logf("Result: %s", string(jsonData))
			},
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

			tt.validate(t, result)
		})
	}
}

