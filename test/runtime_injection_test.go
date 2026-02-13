package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestParseOptionsLookups_Basic tests injecting lookup tables via ParseOptions
// when the template has no <lookup> tag at all.
func TestParseOptionsLookups_Basic(t *testing.T) {
	template := `
<group name="bgp_config">
router bgp {{ bgp_as | lookup("bgp_asn", add_field="as_details") }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "router bgp 65100"

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Lookups: map[string]map[string]interface{}{
				"bgp_asn": {
					"65100": map[string]interface{}{
						"as_description": "Runtime ASN",
						"as_name":        "RuntimeSubs",
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
	bgpConfig, ok := resultMap["bgp_config"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected bgp_config in result")
	}
	if bgpConfig["bgp_as"] != "65100" {
		t.Errorf("Expected bgp_as=65100, got %v", bgpConfig["bgp_as"])
	}
	asDetails, ok := bgpConfig["as_details"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected as_details from lookup")
	}
	if asDetails["as_description"] != "Runtime ASN" {
		t.Errorf("Expected as_description='Runtime ASN', got %v", asDetails["as_description"])
	}
	if asDetails["as_name"] != "RuntimeSubs" {
		t.Errorf("Expected as_name='RuntimeSubs', got %v", asDetails["as_name"])
	}
}

// TestParseOptionsLookups_OverrideCompiled tests that runtime lookups override
// compiled (template-embedded) lookups with the same name.
func TestParseOptionsLookups_OverrideCompiled(t *testing.T) {
	template := `
<lookup name="bgp_asn" load="yaml">
'65100':
  as_description: Compiled ASN
  as_name: CompiledSubs
  prefix_num: '734'
</lookup>

<group name="bgp_config">
router bgp {{ bgp_as | lookup("bgp_asn", add_field="as_details") }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "router bgp 65100"

	// Parse with runtime lookups that override the compiled one
	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Lookups: map[string]map[string]interface{}{
				"bgp_asn": {
					"65100": map[string]interface{}{
						"as_description": "Runtime Override",
						"as_name":        "RuntimeOverride",
						"prefix_num":     "999",
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

	resultList := result.([]interface{})
	resultMap := resultList[0].(map[string]interface{})
	bgpConfig := resultMap["bgp_config"].(map[string]interface{})
	asDetails := bgpConfig["as_details"].(map[string]interface{})

	if asDetails["as_description"] != "Runtime Override" {
		t.Errorf("Expected runtime override for as_description, got %v", asDetails["as_description"])
	}
	if asDetails["as_name"] != "RuntimeOverride" {
		t.Errorf("Expected runtime override for as_name, got %v", asDetails["as_name"])
	}
	if asDetails["prefix_num"] != "999" {
		t.Errorf("Expected runtime override for prefix_num, got %v", asDetails["prefix_num"])
	}
}

// TestParseOptionsVars_Basic tests injecting variables via ParseOptions.Vars
// that override template <vars>.
func TestParseOptionsVars_Basic(t *testing.T) {
	template := `
<vars>
var_1 = "compiled_value"
</vars>

<group name="bgp_config">
router bgp {{ bgp_as }}
{{ var_1_value | set(var_1) }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "router bgp 65100"

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Vars: map[string]interface{}{
				"var_1": "runtime_value",
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	resultList := result.([]interface{})
	resultMap := resultList[0].(map[string]interface{})
	bgpConfig := resultMap["bgp_config"].(map[string]interface{})

	if bgpConfig["var_1_value"] != "runtime_value" {
		t.Errorf("Expected var_1_value='runtime_value', got %v", bgpConfig["var_1_value"])
	}
}

// TestParseOptionsVars_PrecedenceOverParseVars tests that ParseOptions.Vars
// has higher precedence than Parse() vars parameter.
func TestParseOptionsVars_PrecedenceOverParseVars(t *testing.T) {
	template := `
<group name="bgp_config">
router bgp {{ bgp_as }}
{{ key_value | set(key) }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "router bgp 65100"

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		gottp.Vars{"key": "from_parse"},
		&gottp.ParseOptions{
			Vars: map[string]interface{}{
				"key": "from_options",
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	resultList := result.([]interface{})
	resultMap := resultList[0].(map[string]interface{})
	bgpConfig := resultMap["bgp_config"].(map[string]interface{})

	if bgpConfig["key_value"] != "from_options" {
		t.Errorf("Expected key_value='from_options' (ParseOptions.Vars wins), got %v", bgpConfig["key_value"])
	}
}

// TestCompileOnceParseMany_Lookups tests that a single compiled template
// can be parsed multiple times with different runtime lookups, producing
// independent results each time.
func TestCompileOnceParseMany_Lookups(t *testing.T) {
	template := `
<group name="bgp_config">
router bgp {{ bgp_as | lookup("bgp_asn", add_field="as_details") }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "router bgp 65100"

	descriptions := []string{"ASN-Alpha", "ASN-Beta", "ASN-Gamma"}

	for _, desc := range descriptions {
		result, err := compiled.Parse(
			gottp.Inputs{"Default_Input": data},
			nil,
			&gottp.ParseOptions{
				Lookups: map[string]map[string]interface{}{
					"bgp_asn": {
						"65100": map[string]interface{}{
							"as_description": desc,
						},
					},
				},
			},
		)
		if err != nil {
			t.Fatalf("Failed to parse with desc=%s: %v", desc, err)
		}

		resultList := result.([]interface{})
		resultMap := resultList[0].(map[string]interface{})
		bgpConfig := resultMap["bgp_config"].(map[string]interface{})
		asDetails := bgpConfig["as_details"].(map[string]interface{})

		if asDetails["as_description"] != desc {
			t.Errorf("Parse with desc=%s: expected as_description=%s, got %v", desc, desc, asDetails["as_description"])
		}
	}
}

// TestCompileOnceParseMany_Vars tests that a single compiled template
// can be parsed multiple times with different runtime vars, producing
// independent results each time.
func TestCompileOnceParseMany_Vars(t *testing.T) {
	template := `
<group name="bgp_config">
router bgp {{ bgp_as }}
{{ site_value | set(site) }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "router bgp 65100"

	sites := []string{"dc1", "dc2", "dc3"}

	for _, site := range sites {
		result, err := compiled.Parse(
			gottp.Inputs{"Default_Input": data},
			nil,
			&gottp.ParseOptions{
				Vars: map[string]interface{}{
					"site": site,
				},
			},
		)
		if err != nil {
			t.Fatalf("Failed to parse with site=%s: %v", site, err)
		}

		resultList := result.([]interface{})
		resultMap := resultList[0].(map[string]interface{})
		bgpConfig := resultMap["bgp_config"].(map[string]interface{})

		if bgpConfig["site_value"] != site {
			t.Errorf("Parse with site=%s: expected site_value=%s, got %v", site, site, bgpConfig["site_value"])
		}
	}
}

// TestParseOptions_NilFields_Regression tests that ParseOptions with nil
// Lookups and Vars fields behaves identically to nil options.
func TestParseOptions_NilFields_Regression(t *testing.T) {
	template := `
<group name="bgp_config">
router bgp {{ bgp_as }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "router bgp 65100"

	// Parse with nil options
	resultNil, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse with nil options: %v", err)
	}

	// Parse with empty ParseOptions (nil Lookups and Vars)
	resultEmpty, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{},
	)
	if err != nil {
		t.Fatalf("Failed to parse with empty options: %v", err)
	}

	jsonNil, _ := json.Marshal(resultNil)
	jsonEmpty, _ := json.Marshal(resultEmpty)

	if string(jsonNil) != string(jsonEmpty) {
		t.Errorf("Results differ between nil and empty options:\n  nil:   %s\n  empty: %s", string(jsonNil), string(jsonEmpty))
	}
}

// TestParseOptions_LookupsAndVarsCombined tests using both Lookups and Vars
// in the same ParseOptions.
func TestParseOptions_LookupsAndVarsCombined(t *testing.T) {
	template := `
<group name="bgp_config">
router bgp {{ bgp_as | lookup("bgp_asn", add_field="as_details") }}
{{ site_value | set(site) }}
</group>
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	data := "router bgp 65100"

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Lookups: map[string]map[string]interface{}{
				"bgp_asn": {
					"65100": map[string]interface{}{
						"as_description": "Combined Test ASN",
						"as_name":        "CombinedSubs",
					},
				},
			},
			Vars: map[string]interface{}{
				"site": "combined-dc",
			},
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	resultList := result.([]interface{})
	resultMap := resultList[0].(map[string]interface{})
	bgpConfig := resultMap["bgp_config"].(map[string]interface{})

	// Verify lookup worked
	asDetails, ok := bgpConfig["as_details"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected as_details from lookup")
	}
	if asDetails["as_description"] != "Combined Test ASN" {
		t.Errorf("Expected as_description='Combined Test ASN', got %v", asDetails["as_description"])
	}

	// Verify vars worked
	if bgpConfig["site_value"] != "combined-dc" {
		t.Errorf("Expected site_value='combined-dc', got %v", bgpConfig["site_value"])
	}
}
