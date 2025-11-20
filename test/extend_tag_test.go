package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestExtendTagFromFile tests template extension from file
func TestExtendTagFromFile(t *testing.T) {
	// Get the test assets directory
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	assetPath := filepath.Join(testDir, "assets", "extend_vlan.txt")

	// Check if file exists
	if _, err := os.Stat(assetPath); os.IsNotExist(err) {
		t.Skipf("Test asset not found: %s", assetPath)
	}

	template := `
<extend template="` + assetPath + `"/>
`

	data := `
vlan 1234
 name some_vlan
!
vlan 910
 name one_more
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

	// Expected structure:
	// [{"vlans": {"1234": {"name": "some_vlan"}, "910": {"name": "one_more"}}}]
	if result == nil {
		t.Error("Result is nil")
	}
}

// TestExtendTagAnonymous tests extend with anonymous group
func TestExtendTagAnonymous(t *testing.T) {
	// Create anonymous group template
	anonTemplate := `
vlan {{ vlan }}
 name {{ name }}
`

	// Write to temp file
	testDir, _ := os.Getwd()
	assetPath := filepath.Join(testDir, "assets", "extend_vlan_anon.txt")
	os.WriteFile(assetPath, []byte(anonTemplate), 0644)
	defer os.Remove(assetPath)

	template := `
<extend template="` + assetPath + `"/>
`

	data := `
vlan 1234
 name some_vlan
!
vlan 910
 name one_more
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

	if result == nil {
		t.Error("Result is nil")
	}
}

