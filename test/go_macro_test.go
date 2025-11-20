package test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestGoMacroBasic tests basic native Go macro execution
func TestGoMacroBasic(t *testing.T) {
	template := `<group name="test" macro="add_processed">
value {{ value }}
</group>`

	data := `value 5
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Create runtime and register Go macro
	runtime := compiled.NewRuntime()

	// Register native Go macro
	runtime.GetMacroRegistry().RegisterGoMacro("add_processed", func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		data["processed"] = true
		// Value might be string or number
		if val, ok := data["value"].(float64); ok {
			data["value"] = val * 2
		} else if valStr, ok := data["value"].(string); ok {
			// Try to convert and double
			if val, err := strconv.ParseFloat(valStr, 64); err == nil {
				data["value"] = val * 2
			}
		}
		return data, true, nil
	})

	result, err := runtime.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Verify result
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}

	// Result structure is [{"test": {...}}]
	firstItem, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	testGroup, ok := firstItem["test"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected test group, got %T", firstItem["test"])
	}

	// Verify macro was applied
	if processed, ok := testGroup["processed"].(bool); !ok || !processed {
		t.Error("Expected macro to set processed=true")
	}

	// Value might be string or number depending on parsing
	val := testGroup["value"]
	if valStr, ok := val.(string); ok {
		if valStr != "10" {
			t.Errorf("Expected macro to double value to 10, got %v", valStr)
		}
	} else if valNum, ok := val.(float64); ok {
		if valNum != 10 {
			t.Errorf("Expected macro to double value to 10, got %v", valNum)
		}
	} else {
		t.Errorf("Expected value to be string or number, got %T: %v", val, val)
	}
}

// TestGoMacroPriority tests that Go macros take priority over Starlark macros
func TestGoMacroPriority(t *testing.T) {
	template := `<macro>
def add_processed(data):
    data["processed"] = "starlark"
    return data
</macro>

<group name="test" macro="add_processed">
value {{ value }}
</group>`

	data := `value 5
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Create runtime and register Go macro with same name
	runtime := compiled.NewRuntime()

	// Register native Go macro (should take priority)
	runtime.GetMacroRegistry().RegisterGoMacro("add_processed", func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		data["processed"] = "go"
		return data, true, nil
	})

	result, err := runtime.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Verify Go macro was used (not Starlark)
	resultList, ok := result.([]interface{})
	if !ok || len(resultList) == 0 {
		t.Fatal("Expected non-empty list result")
	}

	firstItem, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	testGroup, ok := firstItem["test"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected test group, got %T", firstItem["test"])
	}

	if processed, ok := testGroup["processed"].(string); !ok || processed != "go" {
		t.Errorf("Expected Go macro to be used (processed='go'), got %v", processed)
	}
}

// TestGoMacroFilter tests that Go macros can filter matches using keep flag
func TestGoMacroFilter(t *testing.T) {
	template := `<group name="interfaces" macro="filter_active">
interface {{ interface }}
 status {{ status }}
</group>`

	data := `interface Loopback0
 status active
interface Vlan100
 status inactive
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Create runtime and register Go macro
	runtime := compiled.NewRuntime()

	// Register native Go macro that filters
	runtime.GetMacroRegistry().RegisterGoMacro("filter_active", func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		if status, ok := data["status"].(string); ok && status == "active" {
			return data, true, nil // Keep this match
		}
		return data, false, nil // Filter out this match
	})

	result, err := runtime.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Verify only active interfaces are kept
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) != 1 {
		t.Errorf("Expected 1 result (only active), got %d", len(resultList))
	}

	firstItem, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	// interfacesGroup might be a single map (when filtered) or a list
	var firstInterface map[string]interface{}
	switch v := firstItem["interfaces"].(type) {
	case map[string]interface{}:
		// Single interface (filtered result)
		firstInterface = v
	case []interface{}:
		// List of interfaces
		if len(v) != 1 {
			t.Errorf("Expected 1 interface (only active), got %d", len(v))
			return
		}
		var ok bool
		firstInterface, ok = v[0].(map[string]interface{})
		if !ok {
			t.Fatalf("Expected interface to be a map, got %T", v[0])
			return
		}
	case []map[string]interface{}:
		// List of interface maps
		if len(v) != 1 {
			t.Errorf("Expected 1 interface (only active), got %d", len(v))
			return
		}
		firstInterface = v[0]
	default:
		t.Fatalf("Expected interfaces group to be a map or list, got %T", firstItem["interfaces"])
		return
	}

	if status, ok := firstInterface["status"].(string); !ok || status != "active" {
		t.Errorf("Expected only active interfaces, got status=%v", status)
	}
}

// TestGoMacroCableModem tests the cable modem bonded macro example
func TestGoMacroCableModem(t *testing.T) {
	template := `<group name="show_cable_modem*" macro="ds_bonded, us_bonded">
{{mac-address | MAC | mac_eui}} {{ip-address | IP }}         {{us-intf}}      {{ds-intf }}     {{status}}     {{prim-sid}}    {{rx-power}}    {{timing-offset}}      {{num-cpes}}    {{bpi-enabled}}  {{rphy-node}}    {{mac-domain}} 
</group>`

	data := `001d.d370.4ff2 10.4.13.20      31:0/0.0/0*    31:0/0/24*   online      6277 20.0   1656   1    no 31   31 
001d.d6ca.9b92 10.4.21.232     60:0/1.3/0     60:0/0/21*   online      5224 20.0   1678   1    no 60   60 
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Create runtime and register Go macros
	runtime := compiled.NewRuntime()

	// Register ds_bonded macro
	runtime.GetMacroRegistry().RegisterGoMacro("ds_bonded", func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		dsIntf, ok := data["ds-intf"].(string)
		if !ok {
			return data, true, nil
		}

		if len(dsIntf) > 0 {
			lastChar := dsIntf[len(dsIntf)-1]
			if lastChar == '*' {
				data["ds-intf"] = dsIntf[:len(dsIntf)-1]
				data["ds-bonded"] = true
				data["ds-impaired"] = false
			} else if lastChar == '#' {
				data["ds-bonded"] = true
				data["ds-impaired"] = true
			} else {
				data["ds-bonded"] = false
				data["ds-impaired"] = false
			}
		}
		return data, true, nil
	})

	// Register us_bonded macro
	runtime.GetMacroRegistry().RegisterGoMacro("us_bonded", func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
		usIntf, ok := data["us-intf"].(string)
		if !ok {
			return data, true, nil
		}

		if len(usIntf) > 0 {
			lastChar := usIntf[len(usIntf)-1]
			if lastChar == '*' {
				data["us-intf"] = usIntf[:len(usIntf)-1]
				data["us-bonded"] = true
				data["us-impaired"] = false
			} else if lastChar == '#' {
				data["us-bonded"] = true
				data["us-impaired"] = true
			} else {
				data["us-bonded"] = false
				data["us-impaired"] = false
			}
		}
		return data, true, nil
	})

	result, err := runtime.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Verify macros were applied
	resultList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected list result, got %T", result)
	}

	if len(resultList) == 0 {
		t.Fatal("Expected at least one result")
	}

	// Result structure is [{"show_cable_modem": [[{...}, {...}]]}]
	firstItem, ok := resultList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map result, got %T", resultList[0])
	}

	cableModemGroup, ok := firstItem["show_cable_modem"].([]interface{})
	if !ok {
		t.Fatalf("Expected show_cable_modem group, got %T", firstItem["show_cable_modem"])
	}

	if len(cableModemGroup) == 0 {
		t.Fatal("Expected at least one cable modem result")
	}

	// Unwrap nested lists - structure is [[{...}, {...}]]
	var modemList []interface{}
	if innerList, ok := cableModemGroup[0].([]interface{}); ok {
		// Check if it's a list of maps or another nested list
		if len(innerList) > 0 {
			if innerInnerList, ok := innerList[0].([]interface{}); ok {
				// Double nested: [[[{...}]]]
				modemList = innerInnerList
			} else {
				// Single nested: [[{...}]]
				modemList = innerList
			}
		} else {
			modemList = innerList
		}
	} else {
		modemList = cableModemGroup
	}

	if len(modemList) == 0 {
		t.Fatal("Expected at least one modem in list")
	}

	// Check first modem - might be a map or a list of maps
	var firstModem map[string]interface{}
	if modemMap, ok := modemList[0].(map[string]interface{}); ok {
		firstModem = modemMap
	} else if modemListOfMaps, ok := modemList[0].([]map[string]interface{}); ok && len(modemListOfMaps) > 0 {
		firstModem = modemListOfMaps[0]
	} else {
		t.Fatalf("Expected modem to be a map, got %T", modemList[0])
	}

	// Verify ds_bonded was applied
	if dsBonded, ok := firstModem["ds-bonded"].(bool); !ok || !dsBonded {
		t.Error("Expected ds-bonded=true for interface ending with *")
	}

	if dsIntf, ok := firstModem["ds-intf"].(string); !ok || dsIntf != "31:0/0/24" {
		t.Errorf("Expected ds-intf to have * removed, got %v", dsIntf)
	}
}

