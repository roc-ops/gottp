package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestLoadLookupFromJSON_Basic tests valid JSON parsing into a lookup table.
func TestLoadLookupFromJSON_Basic(t *testing.T) {
	jsonData := []byte(`{
		"65100": {"as_name": "Subs", "prefix_num": "734"},
		"65101": {"as_name": "Cust1", "prefix_num": "156"}
	}`)

	result, err := gottp.LoadLookupFromJSON("bgp_asn", jsonData)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify outer map has one entry with correct name
	if len(result) != 1 {
		t.Fatalf("Expected 1 entry in outer map, got %d", len(result))
	}

	table, ok := result["bgp_asn"]
	if !ok {
		t.Fatal("Expected 'bgp_asn' key in result")
	}

	// Verify two entries in the lookup table
	if len(table) != 2 {
		t.Fatalf("Expected 2 entries in table, got %d", len(table))
	}

	// Verify entry 65100
	entry, ok := table["65100"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{} for key '65100', got %T", table["65100"])
	}
	if entry["as_name"] != "Subs" {
		t.Errorf("Expected as_name='Subs', got %v", entry["as_name"])
	}
	if entry["prefix_num"] != "734" {
		t.Errorf("Expected prefix_num='734', got %v", entry["prefix_num"])
	}

	// Verify entry 65101
	entry2, ok := table["65101"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{} for key '65101', got %T", table["65101"])
	}
	if entry2["as_name"] != "Cust1" {
		t.Errorf("Expected as_name='Cust1', got %v", entry2["as_name"])
	}
}

// TestLoadLookupFromJSON_MalformedJSON tests that malformed JSON returns an error.
func TestLoadLookupFromJSON_MalformedJSON(t *testing.T) {
	jsonData := []byte(`{"65100": {"as_name": "Subs"`)

	_, err := gottp.LoadLookupFromJSON("bgp_asn", jsonData)
	if err == nil {
		t.Fatal("Expected error for malformed JSON, got nil")
	}
}

// TestLoadLookupFromJSON_EmptyData tests that empty bytes return an error.
func TestLoadLookupFromJSON_EmptyData(t *testing.T) {
	_, err := gottp.LoadLookupFromJSON("bgp_asn", []byte{})
	if err == nil {
		t.Fatal("Expected error for empty data, got nil")
	}

	_, err = gottp.LoadLookupFromJSON("bgp_asn", nil)
	if err == nil {
		t.Fatal("Expected error for nil data, got nil")
	}
}

// TestLoadLookupFromJSON_WrongStructure tests that a JSON array instead of object returns an error.
func TestLoadLookupFromJSON_WrongStructure(t *testing.T) {
	jsonData := []byte(`[{"as_name": "Subs"}]`)

	_, err := gottp.LoadLookupFromJSON("bgp_asn", jsonData)
	if err == nil {
		t.Fatal("Expected error for JSON array, got nil")
	}

	// Also test an object with non-object values
	jsonData2 := []byte(`{"65100": "not_a_map"}`)
	_, err = gottp.LoadLookupFromJSON("bgp_asn", jsonData2)
	if err == nil {
		t.Fatal("Expected error for non-object inner value, got nil")
	}
}

// TestLoadLookupFromYAML_Basic tests valid YAML parsing into a lookup table.
func TestLoadLookupFromYAML_Basic(t *testing.T) {
	yamlData := []byte(`"65100":
  as_name: Subs
  prefix_num: "734"
"65101":
  as_name: Cust1
  prefix_num: "156"
`)

	result, err := gottp.LoadLookupFromYAML("bgp_asn", yamlData)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify outer map has one entry
	if len(result) != 1 {
		t.Fatalf("Expected 1 entry in outer map, got %d", len(result))
	}

	table, ok := result["bgp_asn"]
	if !ok {
		t.Fatal("Expected 'bgp_asn' key in result")
	}

	if len(table) != 2 {
		t.Fatalf("Expected 2 entries in table, got %d", len(table))
	}

	entry, ok := table["65100"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{} for key '65100', got %T", table["65100"])
	}
	if entry["as_name"] != "Subs" {
		t.Errorf("Expected as_name='Subs', got %v", entry["as_name"])
	}
	if entry["prefix_num"] != "734" {
		t.Errorf("Expected prefix_num='734', got %v", entry["prefix_num"])
	}
}

// TestLoadLookupFromYAML_Malformed tests that invalid YAML returns an error.
func TestLoadLookupFromYAML_Malformed(t *testing.T) {
	yamlData := []byte(`
  - bad: yaml
  indent: wrong
    nested: invalid
  ]]]
`)

	_, err := gottp.LoadLookupFromYAML("bgp_asn", yamlData)
	if err == nil {
		t.Fatal("Expected error for malformed YAML, got nil")
	}
}

// TestLoadLookupFromCSV_Basic tests valid CSV parsing with an explicit key column.
func TestLoadLookupFromCSV_Basic(t *testing.T) {
	csvData := []byte(`asn,as_name,prefix_num
65100,Subs,734
65101,Cust1,156
`)

	result, err := gottp.LoadLookupFromCSV("bgp_asn", csvData, "asn")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	table, ok := result["bgp_asn"]
	if !ok {
		t.Fatal("Expected 'bgp_asn' key in result")
	}

	if len(table) != 2 {
		t.Fatalf("Expected 2 entries in table, got %d", len(table))
	}

	entry, ok := table["65100"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{} for key '65100', got %T", table["65100"])
	}
	if entry["as_name"] != "Subs" {
		t.Errorf("Expected as_name='Subs', got %v", entry["as_name"])
	}
	if entry["prefix_num"] != "734" {
		t.Errorf("Expected prefix_num='734', got %v", entry["prefix_num"])
	}
	// Key column should also be present in the row data
	if entry["asn"] != "65100" {
		t.Errorf("Expected asn='65100', got %v", entry["asn"])
	}
}

// TestLoadLookupFromCSV_DefaultKeyColumn tests that empty keyColumn uses the first column.
func TestLoadLookupFromCSV_DefaultKeyColumn(t *testing.T) {
	csvData := []byte(`asn,as_name,prefix_num
65100,Subs,734
65101,Cust1,156
`)

	result, err := gottp.LoadLookupFromCSV("bgp_asn", csvData, "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	table := result["bgp_asn"]

	// First column (asn) should be the key
	entry, ok := table["65100"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected entry for key '65100', got %T", table["65100"])
	}
	if entry["as_name"] != "Subs" {
		t.Errorf("Expected as_name='Subs', got %v", entry["as_name"])
	}
}

// TestLoadLookupFromCSV_MissingKeyColumn tests that a missing key column returns an error.
func TestLoadLookupFromCSV_MissingKeyColumn(t *testing.T) {
	csvData := []byte(`asn,as_name,prefix_num
65100,Subs,734
`)

	_, err := gottp.LoadLookupFromCSV("bgp_asn", csvData, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for missing key column, got nil")
	}
}

// TestLoadLookupFromCSV_EmptyData tests that empty bytes return an error.
func TestLoadLookupFromCSV_EmptyData(t *testing.T) {
	_, err := gottp.LoadLookupFromCSV("bgp_asn", []byte{}, "asn")
	if err == nil {
		t.Fatal("Expected error for empty CSV data, got nil")
	}

	_, err = gottp.LoadLookupFromCSV("bgp_asn", nil, "asn")
	if err == nil {
		t.Fatal("Expected error for nil CSV data, got nil")
	}
}

// TestLoadLookup_IntegrationWithParseOptions loads lookup data from JSON, passes it
// to ParseOptions.Lookups, and verifies the lookup works end-to-end during parsing.
func TestLoadLookup_IntegrationWithParseOptions(t *testing.T) {
	// Compile a template that uses a lookup
	template := `
<group name="bgp_config">
router bgp {{ bgp_as | lookup("bgp_asn", add_field="as_details") }}
</group>
`
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Load lookup from JSON
	lookupJSON := []byte(`{
		"65100": {
			"as_description": "Private ASN",
			"as_name": "Subs",
			"prefix_num": "734"
		},
		"65101": {
			"as_description": "Customer ASN",
			"as_name": "Cust1",
			"prefix_num": "156"
		}
	}`)

	lookups, err := gottp.LoadLookupFromJSON("bgp_asn", lookupJSON)
	if err != nil {
		t.Fatalf("Failed to load lookup from JSON: %v", err)
	}

	// Parse with the loaded lookups
	data := "router bgp 65100"
	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		&gottp.ParseOptions{
			Lookups: lookups,
		},
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Verify the lookup resolved correctly
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
		t.Errorf("Expected bgp_as='65100', got %v", bgpConfig["bgp_as"])
	}
	asDetails, ok := bgpConfig["as_details"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected as_details from lookup")
	}
	if asDetails["as_description"] != "Private ASN" {
		t.Errorf("Expected as_description='Private ASN', got %v", asDetails["as_description"])
	}
	if asDetails["as_name"] != "Subs" {
		t.Errorf("Expected as_name='Subs', got %v", asDetails["as_name"])
	}
}
