package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestVoidAttribute tests that groups with void attribute skip saving results
func TestVoidAttribute(t *testing.T) {
	template := `
<group void="">
interface {{ interface }}
 description {{ description }}
</group>
`

	data := `
interface Loopback0
 description Router-id-loopback
interface Vlan100
 description Management-VLAN
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

	// Should be empty or minimal since void group results are skipped
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result entry (even if empty)")
	}

	resultMap, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	// Results should be empty since void group skips saving
	if len(resultMap) > 0 {
		t.Errorf("Expected empty results with void attribute, but got: %v", resultMap)
	}
}

// TestVoidAttributeWithNested tests void attribute with nested groups
func TestVoidAttributeWithNested(t *testing.T) {
	template := `
<group void="">
interface {{ interface }}
 <group name="ip_config">
 ip address {{ ip }}/{{ mask }}
 </group>
</group>
`

	data := `
interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
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

	// Python TTP 0.10.1: void drops only the parent's own records; the
	// child's records are saved from the results root. The anonymous void
	// parent's child path is "_anonymous_*.ip_config", so they form one
	// item of the input's result list (Parse returns one result per input).
	want := `[[{"ip_config":[{"ip":"192.168.0.1","mask":"24"},{"ip":"10.0.0.1","mask":"24"}]}]]`
	got, _ := json.Marshal(result)
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

