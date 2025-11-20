package test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/formatters"
)

// TestJSONFormatterBasic tests basic JSON formatting
func TestJSONFormatterBasic(t *testing.T) {
	formatter := formatters.NewJSONFormatter()
	
	data := map[string]interface{}{
		"name": "test",
		"value": 123,
		"active": true,
	}
	
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("Failed to format JSON: %v", err)
	}
	
	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Formatted output is not valid JSON: %v", err)
	}
	
	// Verify values
	if parsed["name"] != "test" {
		t.Errorf("Expected name='test', got %v", parsed["name"])
	}
	if parsed["value"] != float64(123) { // JSON numbers are float64
		t.Errorf("Expected value=123, got %v", parsed["value"])
	}
	if parsed["active"] != true {
		t.Errorf("Expected active=true, got %v", parsed["active"])
	}
}

// TestJSONFormatterString tests FormatString method
func TestJSONFormatterString(t *testing.T) {
	formatter := formatters.NewJSONFormatter()
	
	data := map[string]interface{}{
		"test": "data",
	}
	
	result, err := formatter.FormatString(data)
	if err != nil {
		t.Fatalf("Failed to format JSON string: %v", err)
	}
	
	if !strings.Contains(result, "test") {
		t.Error("Formatted string does not contain expected data")
	}
	
	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("Formatted string is not valid JSON: %v", err)
	}
}

// TestJSONFormatterList tests JSON formatting with list data
func TestJSONFormatterList(t *testing.T) {
	formatter := formatters.NewJSONFormatter()
	
	data := []interface{}{
		map[string]interface{}{"id": 1, "name": "first"},
		map[string]interface{}{"id": 2, "name": "second"},
	}
	
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("Failed to format JSON list: %v", err)
	}
	
	// Verify it's valid JSON
	var parsed []interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Formatted output is not valid JSON: %v", err)
	}
	
	if len(parsed) != 2 {
		t.Errorf("Expected 2 items, got %d", len(parsed))
	}
}

// TestJSONFormatterNested tests JSON formatting with nested structures
func TestJSONFormatterNested(t *testing.T) {
	formatter := formatters.NewJSONFormatter()
	
	data := map[string]interface{}{
		"parent": map[string]interface{}{
			"child": "value",
			"number": 42,
		},
		"list": []interface{}{1, 2, 3},
	}
	
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("Failed to format nested JSON: %v", err)
	}
	
	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("Formatted output is not valid JSON: %v", err)
	}
	
	parent, ok := parsed["parent"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected parent to be a map")
	}
	
	if parent["child"] != "value" {
		t.Errorf("Expected child='value', got %v", parent["child"])
	}
}

// TestYAMLFormatterBasic tests basic YAML formatting
func TestYAMLFormatterBasic(t *testing.T) {
	formatter := formatters.NewYAMLFormatter()
	
	data := map[string]interface{}{
		"name": "test",
		"value": 123,
		"active": true,
	}
	
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("Failed to format YAML: %v", err)
	}
	
	resultStr := string(result)
	
	// Verify YAML structure
	if !strings.Contains(resultStr, "name:") {
		t.Error("YAML output does not contain 'name:'")
	}
	if !strings.Contains(resultStr, "test") {
		t.Error("YAML output does not contain expected value")
	}
}

// TestYAMLFormatterString tests FormatString method
func TestYAMLFormatterString(t *testing.T) {
	formatter := formatters.NewYAMLFormatter()
	
	data := map[string]interface{}{
		"test": "data",
		"number": 42,
	}
	
	result, err := formatter.FormatString(data)
	if err != nil {
		t.Fatalf("Failed to format YAML string: %v", err)
	}
	
	if !strings.Contains(result, "test:") {
		t.Error("Formatted string does not contain expected YAML structure")
	}
}

// TestYAMLFormatterList tests YAML formatting with list data
func TestYAMLFormatterList(t *testing.T) {
	formatter := formatters.NewYAMLFormatter()
	
	data := []interface{}{
		map[string]interface{}{"id": 1, "name": "first"},
		map[string]interface{}{"id": 2, "name": "second"},
	}
	
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("Failed to format YAML list: %v", err)
	}
	
	resultStr := string(result)
	
	// Verify list structure
	if !strings.Contains(resultStr, "-") {
		t.Error("YAML list output should contain '-' indicators")
	}
}

// TestYAMLFormatterNested tests YAML formatting with nested structures
func TestYAMLFormatterNested(t *testing.T) {
	formatter := formatters.NewYAMLFormatter()
	
	data := map[string]interface{}{
		"parent": map[string]interface{}{
			"child": "value",
		},
	}
	
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("Failed to format nested YAML: %v", err)
	}
	
	resultStr := string(result)
	
	// Verify nested structure
	if !strings.Contains(resultStr, "parent:") {
		t.Error("YAML output should contain 'parent:'")
	}
	if !strings.Contains(resultStr, "child:") {
		t.Error("YAML output should contain 'child:'")
	}
}

// TestRawFormatterBasic tests basic raw formatter
func TestRawFormatterBasic(t *testing.T) {
	formatter := formatters.NewRawFormatter()
	
	data := "test string"
	
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("Failed to format raw string: %v", err)
	}
	
	if string(result) != data {
		t.Errorf("Expected '%s', got '%s'", data, string(result))
	}
}

// TestRawFormatterString tests FormatString method
func TestRawFormatterString(t *testing.T) {
	formatter := formatters.NewRawFormatter()
	
	data := "test output"
	
	result, err := formatter.FormatString(data)
	if err != nil {
		t.Fatalf("Failed to format raw string: %v", err)
	}
	
	if result != data {
		t.Errorf("Expected '%s', got '%s'", data, result)
	}
}

// TestRawFormatterWithMap tests raw formatter with map (should convert to string)
func TestRawFormatterWithMap(t *testing.T) {
	formatter := formatters.NewRawFormatter()
	
	data := map[string]interface{}{
		"key": "value",
	}
	
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("Failed to format map as raw: %v", err)
	}
	
	resultStr := string(result)
	
	// Should contain the map representation
	if !strings.Contains(resultStr, "key") || !strings.Contains(resultStr, "value") {
		t.Error("Raw formatter should convert map to string representation")
	}
}

// TestTableFormatterBasic tests basic table formatting
func TestTableFormatterBasic(t *testing.T) {
	formatter := formatters.NewTableFormatter()
	
	data := []map[string]interface{}{
		{"name": "Alice", "age": 30},
		{"name": "Bob", "age": 25},
		{"name": "Charlie", "age": 35},
	}
	
	table, err := formatter.Format(data, nil)
	if err != nil {
		t.Fatalf("Failed to format table: %v", err)
	}
	
	if len(table) == 0 {
		t.Fatal("Table should have at least a header row")
	}
	
	// First row should be headers
	headers := table[0]
	if len(headers) < 2 {
		t.Errorf("Expected at least 2 headers, got %d", len(headers))
	}
	
	// Should have data rows
	if len(table) < 2 {
		t.Errorf("Expected at least 2 rows (header + data), got %d", len(table))
	}
	
	// Verify headers contain expected keys
	hasName := false
	hasAge := false
	for _, header := range headers {
		if header == "name" {
			hasName = true
		}
		if header == "age" {
			hasAge = true
		}
	}
	
	if !hasName || !hasAge {
		t.Error("Table headers should contain 'name' and 'age'")
	}
}

// TestTableFormatterWithCustomHeaders tests table with custom headers
func TestTableFormatterWithCustomHeaders(t *testing.T) {
	formatter := formatters.NewTableFormatter()
	
	data := []map[string]interface{}{
		{"name": "Alice", "age": 30},
		{"name": "Bob", "age": 25},
	}
	
	options := &formatters.TableOptions{
		Headers: []string{"Name", "Age"},
	}
	
	table, err := formatter.Format(data, options)
	if err != nil {
		t.Fatalf("Failed to format table with custom headers: %v", err)
	}
	
	if len(table) == 0 {
		t.Fatal("Table should have headers")
	}
	
	headers := table[0]
	if headers[0] != "Name" || headers[1] != "Age" {
		t.Errorf("Expected custom headers ['Name', 'Age'], got %v", headers)
	}
}

// TestTableFormatterWithMissingValues tests table with missing values
func TestTableFormatterWithMissingValues(t *testing.T) {
	formatter := formatters.NewTableFormatter()
	
	data := []map[string]interface{}{
		{"name": "Alice", "age": 30},
		{"name": "Bob"}, // missing age
	}
	
	options := &formatters.TableOptions{
		Missing: "N/A",
	}
	
	table, err := formatter.Format(data, options)
	if err != nil {
		t.Fatalf("Failed to format table with missing values: %v", err)
	}
	
	if len(table) < 3 {
		t.Fatalf("Expected at least 3 rows, got %d", len(table))
	}
	
	// Check second data row (index 2) for missing value
	row := table[2]
	ageIndex := -1
	for i, header := range table[0] {
		if header == "age" {
			ageIndex = i
			break
		}
	}
	
	if ageIndex >= 0 && ageIndex < len(row) {
		if row[ageIndex] != "N/A" {
			t.Errorf("Expected missing value 'N/A', got '%s'", row[ageIndex])
		}
	}
}

// TestTableFormatterWithKey tests table with key option
func TestTableFormatterWithKey(t *testing.T) {
	formatter := formatters.NewTableFormatter()
	
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{"name": "Alice", "age": 30},
			map[string]interface{}{"name": "Bob", "age": 25},
		},
	}
	
	options := &formatters.TableOptions{
		Key: "users",
	}
	
	table, err := formatter.Format(data, options)
	if err != nil {
		t.Fatalf("Failed to format table with key: %v", err)
	}
	
	if len(table) < 2 {
		t.Errorf("Expected at least 2 rows, got %d", len(table))
	}
}

// TestTableFormatterWithListInterface tests table with []interface{}
func TestTableFormatterWithListInterface(t *testing.T) {
	formatter := formatters.NewTableFormatter()
	
	data := []interface{}{
		map[string]interface{}{"id": 1, "value": "first"},
		map[string]interface{}{"id": 2, "value": "second"},
	}
	
	table, err := formatter.Format(data, nil)
	if err != nil {
		t.Fatalf("Failed to format table from []interface{}: %v", err)
	}
	
	if len(table) < 2 {
		t.Errorf("Expected at least 2 rows, got %d", len(table))
	}
}

// TestTableFormatterEmptyData tests table with empty data
func TestTableFormatterEmptyData(t *testing.T) {
	formatter := formatters.NewTableFormatter()
	
	data := []map[string]interface{}{}
	
	table, err := formatter.Format(data, nil)
	if err != nil {
		t.Fatalf("Failed to format empty table: %v", err)
	}
	
	if len(table) != 0 {
		t.Errorf("Expected empty table, got %d rows", len(table))
	}
}

