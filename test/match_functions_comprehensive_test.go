package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestMatchFunctionsComprehensive tests various match functions from Python TTP
func TestMatchFunctionsComprehensive(t *testing.T) {
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
		{
			name: "join function",
			template: `
<group>
{{ values | join(',') }}
</group>
`,
			data: "a,b,c",
			validate: func(t *testing.T, result interface{}) {
				jsonData, _ := json.Marshal(result)
				t.Logf("Result: %s", string(jsonData))
			},
		},
		{
			name: "replace function",
			template: `
<group>
{{ value | replace('old', 'new') }}
</group>
`,
			data: "old_value",
			validate: func(t *testing.T, result interface{}) {
				jsonData, _ := json.Marshal(result)
				t.Logf("Result: %s", string(jsonData))
			},
		},
		{
			name: "strip function",
			template: `
<group>
{{ value | strip }}
</group>
`,
			data: "  test  ",
			validate: func(t *testing.T, result interface{}) {
				jsonData, _ := json.Marshal(result)
				t.Logf("Result: %s", string(jsonData))
			},
		},
		{
			name: "MAC function",
			template: `
<group>
{{ mac | MAC }}
</group>
`,
			data: "00:11:22:33:44:55",
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

// TestMultiLinePatternMerging tests that multi-line patterns are properly merged
func TestMultiLinePatternMerging(t *testing.T) {
	template := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.1/24
 description Router-id-loopback
!
interface Vlan100
 ip address 10.0.0.1/24
 description Management-VLAN
!
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

	// Result is wrapped in a list (per_input method)
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}

	// Get first result
	resultMap, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	interfaces, ok := resultMap["interfaces"]
	if !ok {
		t.Fatal("Expected 'interfaces' key in result")
	}

	// Handle both []interface{} and []map[string]interface{}
	var interfacesList []map[string]interface{}
	switch v := interfaces.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				interfacesList = append(interfacesList, m)
			}
		}
	case []map[string]interface{}:
		interfacesList = v
	default:
		t.Fatalf("Expected interfaces to be a list, got %T", interfaces)
	}

	if len(interfacesList) == 0 {
		t.Fatal("Expected at least one interface result")
	}

	// Check first interface has all fields merged
	firstInterface := interfacesList[0]

	// Verify all expected fields are present
	expectedFields := []string{"interface", "ip", "mask", "description"}
	for _, field := range expectedFields {
		if _, ok := firstInterface[field]; !ok {
			t.Errorf("Expected field '%s' in merged result", field)
		}
	}

	// Verify we have 2 interfaces
	if len(interfacesList) != 2 {
		t.Errorf("Expected 2 interfaces, got %d", len(interfacesList))
	}
}

