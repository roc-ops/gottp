package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestCompileOptions_MatchScope tests registering a custom match function via CompileOptions
// that transforms a matched value via the pipe syntax.
func TestCompileOptions_MatchScope(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | custom_transform }}
</group>
`

	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
		Functions: &gottp.Functions{
			Match: map[string]gottp.MatchFunc{
				"custom_transform": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
					return fmt.Sprintf("%v-compiled", value), nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil, // no ParseOptions needed — function baked in at compile time
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	hostname := extractHostnameFromResult(t, result)
	if hostname != "router1-compiled" {
		t.Errorf("Expected hostname='router1-compiled', got %v", hostname)
	}
}

// TestCompileOptions_GroupScope tests registering a custom group function via CompileOptions.
func TestCompileOptions_GroupScope(t *testing.T) {
	template := `
<group name="interfaces" custom_filter="Loopback">
interface {{ name }}
 ip address {{ ip }}
</group>
`

	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
		Functions: &gottp.Functions{
			Group: map[string]gottp.GroupFunc{
				"custom_filter": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
					if len(args) > 0 {
						name, _ := data["name"].(string)
						if !strings.Contains(name, args[0]) {
							return data, false, nil // exclude
						}
					}
					return data, true, nil
				},
			},
		},
	})
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
		nil,
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

// TestCompileOptions_MacroScope tests registering a custom macro function via CompileOptions,
// replacing the RegisterGoMacro pattern.
func TestCompileOptions_MacroScope(t *testing.T) {
	template := `
<group name="test" macro="add_suffix">
hostname {{ name }}
</group>
`

	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
		Functions: &gottp.Functions{
			Macro: map[string]gottp.MacroFunc{
				"add_suffix": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
					if name, ok := data["name"].(string); ok {
						data["name"] = name + "-compiled"
					}
					return data, true, nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

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
	if testGroup["name"] != "router1-compiled" {
		t.Errorf("Expected name='router1-compiled', got %v", testGroup["name"])
	}
}

// TestCompileOptions_OverrideBuiltin tests that CompileOptions functions override built-in
// functions with the same name.
func TestCompileOptions_OverrideBuiltin(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | upper }}
</group>
`

	// Compile with a custom "upper" that reverses instead of uppercasing
	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
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
	})
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

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

	hostname := extractHostnameFromResult(t, result)
	// Our custom "upper" reverses the string
	if hostname != "1retuor" {
		t.Errorf("Expected hostname='1retuor' (reversed by compile-time upper), got %v", hostname)
	}
}

// TestCompileOptions_ParseOptionsOverrides tests that ParseOptions functions override
// CompileOptions functions with the same name.
func TestCompileOptions_ParseOptionsOverrides(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | transform }}
</group>
`

	// Compile with a "transform" that appends "-compiled"
	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
		Functions: &gottp.Functions{
			Match: map[string]gottp.MatchFunc{
				"transform": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
					return fmt.Sprintf("%v-compiled", value), nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	// Parse with a "transform" that appends "-runtime" (should override compile-time)
	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Match: map[string]gottp.MatchFunc{
					"transform": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
						return fmt.Sprintf("%v-runtime", value), nil
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

	hostname := extractHostnameFromResult(t, result)
	if hostname != "router1-runtime" {
		t.Errorf("Expected hostname='router1-runtime' (ParseOptions overrides CompileOptions), got %v", hostname)
	}
}

// TestCompileOptions_PersistAcrossParses tests that compile-time functions persist
// across multiple Parse() calls.
func TestCompileOptions_PersistAcrossParses(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | custom_transform }}
</group>
`

	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
		Functions: &gottp.Functions{
			Match: map[string]gottp.MatchFunc{
				"custom_transform": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
					return fmt.Sprintf("%v-compiled", value), nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Parse 3 times with different input data
	inputs := []string{"hostname router1", "hostname switch2", "hostname firewall3"}
	expected := []string{"router1-compiled", "switch2-compiled", "firewall3-compiled"}

	for i, input := range inputs {
		result, err := compiled.Parse(
			gottp.Inputs{"Default_Input": input},
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("Parse %d failed: %v", i+1, err)
		}

		hostname := extractHostnameFromResult(t, result)
		if hostname != expected[i] {
			t.Errorf("Parse %d: expected '%s', got '%s'", i+1, expected[i], hostname)
		}
	}
}

// TestCompileOptions_NilOptions tests that CompileTemplateWithOptions with nil options
// behaves identically to CompileTemplate.
func TestCompileOptions_NilOptions(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | upper }}
</group>
`

	// Compile with nil options
	compiled1, err := gottp.CompileTemplateWithOptions(template, nil)
	if err != nil {
		t.Fatalf("Failed to compile with nil options: %v", err)
	}

	// Compile with standard CompileTemplate
	compiled2, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile with CompileTemplate: %v", err)
	}

	data := "hostname router1"

	result1, err := compiled1.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Parse with nil options compiled failed: %v", err)
	}

	result2, err := compiled2.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Parse with CompileTemplate compiled failed: %v", err)
	}

	h1 := extractHostnameFromResult(t, result1)
	h2 := extractHostnameFromResult(t, result2)

	if h1 != "ROUTER1" {
		t.Errorf("Nil options: expected 'ROUTER1', got '%s'", h1)
	}
	if h2 != "ROUTER1" {
		t.Errorf("CompileTemplate: expected 'ROUTER1', got '%s'", h2)
	}
	if h1 != h2 {
		t.Errorf("Results differ: nil options='%s', CompileTemplate='%s'", h1, h2)
	}
}

// TestCompileOptions_MacroOverridePrecedence tests that a parse-time macro overrides
// a compile-time macro with the same name.
func TestCompileOptions_MacroOverridePrecedence(t *testing.T) {
	template := `
<group name="test" macro="transform_macro">
hostname {{ name }}
</group>
`

	// Compile with a macro that appends "-compiled"
	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
		Functions: &gottp.Functions{
			Macro: map[string]gottp.MacroFunc{
				"transform_macro": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
					if name, ok := data["name"].(string); ok {
						data["name"] = name + "-compiled"
					}
					return data, true, nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

	// Parse 1: no ParseOptions — compile-time macro should apply
	result1, err := compiled.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Parse 1 failed: %v", err)
	}

	name1 := extractNameFromResult(t, result1)
	if name1 != "router1-compiled" {
		t.Errorf("Parse 1: expected 'router1-compiled', got '%s'", name1)
	}

	// Parse 2: ParseOptions macro overrides compile-time macro
	result2, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Macro: map[string]gottp.MacroFunc{
					"transform_macro": func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
						if name, ok := data["name"].(string); ok {
							data["name"] = name + "-runtime"
						}
						return data, true, nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Parse 2 failed: %v", err)
	}

	name2 := extractNameFromResult(t, result2)
	if name2 != "router1-runtime" {
		t.Errorf("Parse 2: expected 'router1-runtime', got '%s'", name2)
	}

	// Parse 3: no ParseOptions again — compile-time macro should still work
	result3, err := compiled.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Parse 3 failed: %v", err)
	}

	name3 := extractNameFromResult(t, result3)
	if name3 != "router1-compiled" {
		t.Errorf("Parse 3: expected 'router1-compiled' (compile-time macro restored), got '%s'", name3)
	}
}

// TestCompileOptions_ThreeLayerPrecedence tests the full 3-layer precedence chain:
// built-in "upper" < CompileOptions "upper" < ParseOptions "upper"
func TestCompileOptions_ThreeLayerPrecedence(t *testing.T) {
	template := `
<group name="test">
hostname {{ hostname | upper }}
</group>
`

	data := "hostname router1"

	// Layer 1: built-in upper
	compiled1, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template (no options): %v", err)
	}

	result1, err := compiled1.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Parse (built-in) failed: %v", err)
	}
	h1 := extractHostnameFromResult(t, result1)
	if h1 != "ROUTER1" {
		t.Errorf("Built-in upper: expected 'ROUTER1', got '%s'", h1)
	}

	// Layer 2: CompileOptions overrides built-in
	compiled2, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
		Functions: &gottp.Functions{
			Match: map[string]gottp.MatchFunc{
				"upper": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
					return fmt.Sprintf("%v-COMPILE", value), nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to compile template (with options): %v", err)
	}

	result2, err := compiled2.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Parse (compile-time) failed: %v", err)
	}
	h2 := extractHostnameFromResult(t, result2)
	if h2 != "router1-COMPILE" {
		t.Errorf("CompileOptions upper: expected 'router1-COMPILE', got '%s'", h2)
	}

	// Layer 3: ParseOptions overrides CompileOptions
	result3, err := compiled2.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Functions: &gottp.Functions{
				Match: map[string]gottp.MatchFunc{
					"upper": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
						return fmt.Sprintf("%v-PARSE", value), nil
					},
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("Parse (parse-time) failed: %v", err)
	}
	h3 := extractHostnameFromResult(t, result3)
	if h3 != "router1-PARSE" {
		t.Errorf("ParseOptions upper: expected 'router1-PARSE', got '%s'", h3)
	}

	// Verify compile-time function still works after parse-time override
	result4, err := compiled2.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
	if err != nil {
		t.Fatalf("Parse (after override) failed: %v", err)
	}
	h4 := extractHostnameFromResult(t, result4)
	if h4 != "router1-COMPILE" {
		t.Errorf("After override: expected 'router1-COMPILE' (restored), got '%s'", h4)
	}
}

// TestCompileOptions_MatchInChain tests that a compile-time match function works
// inside chain().
func TestCompileOptions_MatchInChain(t *testing.T) {
	template := `
<vars>
my_chain = "upper | custom_exclaim"
</vars>

<group name="test">
hostname {{ hostname | chain("my_chain") }}
</group>
`

	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
		Functions: &gottp.Functions{
			Match: map[string]gottp.MatchFunc{
				"custom_exclaim": func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
					return fmt.Sprintf("%v!!!", value), nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "hostname router1"

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

	hostname := extractHostnameFromResult(t, result)
	// upper should produce "ROUTER1", then custom_exclaim should add "!!!"
	if hostname != "ROUTER1!!!" {
		t.Errorf("Expected 'ROUTER1!!!', got %v", hostname)
	}
}

// extractHostnameFromResult is a test helper that extracts the hostname from a standard result structure.
func extractHostnameFromResult(t *testing.T, result interface{}) string {
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

// extractNameFromResult is a test helper that extracts the name from a standard result structure.
func extractNameFromResult(t *testing.T, result interface{}) string {
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
	name, _ := testGroup["name"].(string)
	return name
}
