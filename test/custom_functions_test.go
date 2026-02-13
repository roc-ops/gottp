package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestParseOptionsFunctions_MatchScope tests injecting a custom match function
// that transforms a matched value via the pipe syntax.
func TestParseOptionsFunctions_MatchScope(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | custom_transform }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Match: map[string]gottp.MatchFunc{
					"custom_transform": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
						return fmt.Sprintf("%v-custom", value), nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Verify result structure
	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 {
		t.Fatal("Expected non-empty result list")
	}
	resultMap, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}
	testGroup, ok := resultMap["test"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected test group in result")
	}
	if testGroup["hostname"] != "router1-custom" {
		t.Errorf("Expected hostname='router1-custom', got %v", testGroup["hostname"])
	}
}

// TestParseOptionsFunctions_MatchOverrideBuiltin tests that a custom match function
// can override the built-in "upper" function.
func TestParseOptionsFunctions_MatchOverrideBuiltin(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | upper }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	// First, parse without custom functions to verify built-in upper works
	result1, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse (no custom): %v", err)
	}

	resultList1, ok := result1.([]interface{})
	if !ok || len(resultList1) == 0 {
		t.Fatal("Expected non-empty result list (no custom)")
	}
	resultMap1, ok := resultList1[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected result map (no custom)")
	}
	testGroup1, ok := resultMap1["test"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected test group (no custom)")
	}
	if testGroup1["hostname"] != "ROUTER1" {
		t.Errorf("Expected built-in upper to produce 'ROUTER1', got %v", testGroup1["hostname"])
	}

	// Now, parse with a custom "upper" that reverses the string instead
	result2, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Match: map[string]gottp.MatchFunc{
					"upper": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
						s := fmt.Sprintf("%v", value)
						runes := []rune(s)
						for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
							runes[i], runes[j] = runes[j], runes[i]
						}
						return string(runes), nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse (with override): %v", err)
	}

	resultList2, ok := result2.([]interface{})
	if !ok || len(resultList2) == 0 {
		t.Fatal("Expected non-empty result list (with override)")
	}
	resultMap2, ok := resultList2[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected result map (with override)")
	}
	testGroup2, ok := resultMap2["test"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected test group (with override)")
	}
	if testGroup2["hostname"] != "1retuor" {
		t.Errorf("Expected custom upper to produce '1retuor', got %v", testGroup2["hostname"])
	}
}

// TestParseOptionsFunctions_GroupScope tests injecting a custom group function
// that filters results based on a condition.
func TestParseOptionsFunctions_GroupScope(t *testing.T) {
	template := `
<group name="interfaces" custom_filter="Loopback">
interface {{ name }}
 ip address {{ ip }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := `interface Loopback0
 ip address 1.1.1.1
interface GigabitEthernet0/0
 ip address 10.0.0.1`

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Group: map[string]gottp.GroupFunc{
					"custom_filter": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
						// Filter: only keep if name contains the first arg
						if len(args) > 0 {
							name, _ := data["name"].(string)
							if !strings.Contains(name, args[0]) {
								return data, false, nil // false = exclude
							}
						}
						return data, true, nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Verify only Loopback interface is in results
	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 {
		t.Fatal("Expected non-empty result list")
	}
	resultMap, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected result map")
	}
	interfaces, ok := resultMap["interfaces"]
	if !ok {
		t.Fatal("Expected interfaces in result")
	}

	// It could be a list or a single map
	switch v := interfaces.(type) {
	case []interface{}:
		if len(v) != 1 {
			t.Errorf("Expected 1 interface after filter, got %d", len(v))
		}
		ifMap, ok := v[0].(map[string]interface{})
		if !ok {
			t.Fatal("Expected interface to be a map")
		}
		if ifMap["name"] != "Loopback0" {
			t.Errorf("Expected name='Loopback0', got %v", ifMap["name"])
		}
	case map[string]interface{}:
		if v["name"] != "Loopback0" {
			t.Errorf("Expected name='Loopback0', got %v", v["name"])
		}
	default:
		t.Fatalf("Unexpected type for interfaces: %T", interfaces)
	}
}

// TestParseOptionsFunctions_MacroScope tests injecting a custom macro function
// via Functions.Macro.
func TestParseOptionsFunctions_MacroScope(t *testing.T) {
	template := `
<group name="test" macro="add_suffix">
hostname {{ name }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Macro: map[string]gottp.MacroFunc{
					"add_suffix": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
						if name, ok := data["name"].(string); ok {
							data["name"] = name + "-modified"
						}
						return data, true, nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Verify the macro was applied
	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 {
		t.Fatal("Expected non-empty result list")
	}
	resultMap, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected result map")
	}
	testGroup, ok := resultMap["test"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected test group")
	}
	if testGroup["name"] != "router1-modified" {
		t.Errorf("Expected name='router1-modified', got %v", testGroup["name"])
	}
}

// TestCompileOnceParseMany_DifferentFunctions tests compiling a template once
// and parsing it multiple times with different custom functions.
func TestCompileOnceParseMany_DifferentFunctions(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | transform }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	// Parse 1: transform appends "-alpha"
	result1, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Match: map[string]gottp.MatchFunc{
					"transform": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
						return fmt.Sprintf("%v-alpha", value), nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Parse 1 failed: %v", err)
	}

	// Parse 2: transform appends "-beta"
	result2, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Match: map[string]gottp.MatchFunc{
					"transform": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
						return fmt.Sprintf("%v-beta", value), nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Parse 2 failed: %v", err)
	}

	// Parse 3: no custom functions (transform should be unknown, value unchanged)
	result3, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Parse 3 failed: %v", err)
	}

	// Verify result 1
	hostname1 := extractHostname(t, result1)
	if hostname1 != "router1-alpha" {
		t.Errorf("Parse 1: expected 'router1-alpha', got %v", hostname1)
	}

	// Verify result 2
	hostname2 := extractHostname(t, result2)
	if hostname2 != "router1-beta" {
		t.Errorf("Parse 2: expected 'router1-beta', got %v", hostname2)
	}

	// Verify result 3 (no transform applied, value should be raw)
	hostname3 := extractHostname(t, result3)
	if hostname3 != "router1" {
		t.Errorf("Parse 3: expected 'router1' (no transform), got %v", hostname3)
	}
}

// TestParseOptionsFunctions_NilFields tests that nil Functions behaves the same as no options.
func TestParseOptionsFunctions_NilFields(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | upper }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	// Parse with nil Functions
	result1, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: nil,
		},
	)
	if err != nil {
		t.Fatalf("Parse with nil Functions failed: %v", err)
	}

	// Parse with empty Functions struct
	result2, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{},
		},
	)
	if err != nil {
		t.Fatalf("Parse with empty Functions failed: %v", err)
	}

	// Parse with no options at all
	result3, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Parse with nil options failed: %v", err)
	}

	// All three should produce the same result (built-in upper)
	h1 := extractHostname(t, result1)
	h2 := extractHostname(t, result2)
	h3 := extractHostname(t, result3)

	if h1 != "ROUTER1" {
		t.Errorf("Nil Functions: expected 'ROUTER1', got %v", h1)
	}
	if h2 != "ROUTER1" {
		t.Errorf("Empty Functions: expected 'ROUTER1', got %v", h2)
	}
	if h3 != "ROUTER1" {
		t.Errorf("Nil options: expected 'ROUTER1', got %v", h3)
	}
}

// TestParseOptionsFunctions_MatchInChain tests that a custom match function
// can be found inside chain().
func TestParseOptionsFunctions_MatchInChain(t *testing.T) {
	template := `
<vars>
my_chain = "upper | custom_exclaim"
</vars>

<group name="test">
hostname {{ hostname | chain("my_chain") }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Match: map[string]gottp.MatchFunc{
					"custom_exclaim": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
						return fmt.Sprintf("%v!!!", value), nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	hostname := extractHostname(t, result)
	// upper should produce "ROUTER1", then custom_exclaim should add "!!!"
	if hostname != "ROUTER1!!!" {
		t.Errorf("Expected 'ROUTER1!!!', got %v", hostname)
	}
}

// extractHostname is a test helper that extracts the hostname from a standard result structure.
func extractHostname(t *testing.T, result interface{}) string {
	t.Helper()
	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 {
		t.Fatal("Expected non-empty result list")
		return ""
	}
	resultMap, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected result map")
		return ""
	}
	testGroup, ok := resultMap["test"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected test group")
		return ""
	}
	hostname, _ := testGroup["hostname"].(string)
	return hostname
}
