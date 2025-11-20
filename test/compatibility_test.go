package test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestBasicParsing tests basic template parsing functionality
func TestBasicParsing(t *testing.T) {
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

	// Convert to JSON for comparison
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal result: %v", err)
	}

	// Basic check - result should not be empty
	if len(string(jsonData)) < 10 {
		t.Errorf("Result is too short: %s", string(jsonData))
	}

	t.Logf("Result: %s", string(jsonData))
}


// TestAnswer1 tests a Stack Overflow answer example
func TestAnswer1(t *testing.T) {
	data := `
#*Approximate Distance Oracles with Improved Query Time.
#@Christian Wulff-Nilsen
#t2015
#cEncyclopedia of Algorithms
#index555036b37cea80f954149ffc

#*Subset Sum Algorithm for Bin Packing.
#@Julián Mestre
#t2015
#cEncyclopedia of Algorithms
#index555036b37cea80f954149ffd
`

	template := `
#*{{ info | ORPHRASE }}
#@{{ author | ORPHRASE }}
#t{{ year }}
#c{{ title | ORPHRASE }}
#index{{ index }}
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

	// Verify we got results
	if result == nil {
		t.Error("Result is nil")
	}
}

// TestBasicExtendTag tests basic template extension (inline for now)
func TestBasicExtendTag(t *testing.T) {
	// For now, test without file extension since we need to set up paths properly
	mainTemplate := `
<group name="vlans.{{ vlan }}">
vlan {{ vlan }}
 name {{ name }}
</group>
`

	data := `
vlan 1234
 name some_vlan
!
vlan 910
 name one_more
!
`

	compiled, err := gottp.CompileTemplate(mainTemplate)
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

	// Verify structure
	if result == nil {
		t.Error("Result is nil")
	}
}

// compareResults compares two results for equality
func compareResults(got, want interface{}) bool {
	// Convert both to JSON and compare
	gotJSON, err1 := json.Marshal(got)
	wantJSON, err2 := json.Marshal(want)

	if err1 != nil || err2 != nil {
		return false
	}

	// Unmarshal both to compare structure
	var gotVal, wantVal interface{}
	json.Unmarshal(gotJSON, &gotVal)
	json.Unmarshal(wantJSON, &wantVal)

	return reflect.DeepEqual(gotVal, wantVal)
}

