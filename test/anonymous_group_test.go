package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestAnonymousGroup tests anonymous group functionality
func TestAnonymousGroup(t *testing.T) {
	// Anonymous group - no name attribute
	template := `
<group>
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	data := `
interface Lo0
 ip address 1.1.1.1/32
!
interface Lo1
 ip address 2.2.2.2/32
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

	// Anonymous groups should still produce results
	if result == nil {
		t.Error("Result is nil for anonymous group")
	}
}

