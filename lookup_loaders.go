package gottp

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// LoadLookupFromJSON parses JSON data into a named lookup table suitable for ParseOptions.Lookups.
// The JSON should be an object where keys map to objects (map of maps).
// Example JSON: {"65100": {"as_name": "Subs", "prefix_num": "734"}}
//
// Returns a single-entry map (name -> table) that can be directly assigned or merged
// into ParseOptions.Lookups.
func LoadLookupFromJSON(name string, data []byte) (map[string]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty JSON data")
	}

	// First unmarshal into a raw interface to validate structure
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate that the top level is an object (map)
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected JSON object (map of maps), got %T", raw)
	}

	// Validate and convert inner values to map[string]interface{}
	table := make(map[string]interface{}, len(rawMap))
	for key, value := range rawMap {
		innerMap, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object for key %q, got %T", key, value)
		}
		table[key] = innerMap
	}

	return map[string]map[string]interface{}{
		name: table,
	}, nil
}

// LoadLookupFromYAML parses YAML data into a named lookup table suitable for ParseOptions.Lookups.
// Example YAML:
//
//	"65100":
//	  as_name: Subs
//	  prefix_num: "734"
//
// Returns a single-entry map (name -> table) that can be directly assigned or merged
// into ParseOptions.Lookups.
func LoadLookupFromYAML(name string, data []byte) (map[string]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty YAML data")
	}

	// Unmarshal into a generic map
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if raw == nil {
		return nil, fmt.Errorf("YAML data is empty or null")
	}

	// Validate and convert inner values to map[string]interface{}
	table := make(map[string]interface{}, len(raw))
	for key, value := range raw {
		innerMap, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected mapping for key %q, got %T", key, value)
		}
		table[key] = innerMap
	}

	return map[string]map[string]interface{}{
		name: table,
	}, nil
}

// LoadLookupFromCSV parses CSV data into a named lookup table suitable for ParseOptions.Lookups.
// The first row is treated as headers. The keyColumn parameter specifies which column to use as the
// lookup key. If keyColumn is empty, the first column is used.
// Each row becomes an entry in the lookup table keyed by the keyColumn value, with all columns
// (including the key column) as fields.
//
// All values are strings since CSV does not carry type information.
//
// Returns a single-entry map (name -> table) that can be directly assigned or merged
// into ParseOptions.Lookups.
func LoadLookupFromCSV(name string, data []byte, keyColumn string) (map[string]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty CSV data")
	}

	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV data has no rows")
	}

	headers := records[0]
	if len(headers) == 0 {
		return nil, fmt.Errorf("CSV data has no columns")
	}

	// Determine the key column index
	keyIndex := 0
	if keyColumn != "" {
		keyIndex = -1
		for i, h := range headers {
			if h == keyColumn {
				keyIndex = i
				break
			}
		}
		if keyIndex == -1 {
			return nil, fmt.Errorf("key column %q not found in CSV headers %v", keyColumn, headers)
		}
	}

	// Build the lookup table
	table := make(map[string]interface{}, len(records)-1)
	for i := 1; i < len(records); i++ {
		record := records[i]
		if keyIndex >= len(record) {
			continue
		}

		key := record[keyIndex]
		row := make(map[string]interface{}, len(headers))
		for j, header := range headers {
			if j < len(record) {
				row[header] = record[j]
			}
		}
		table[key] = row
	}

	return map[string]map[string]interface{}{
		name: table,
	}, nil
}

// MergeLookups merges multiple lookup maps into a single map suitable for ParseOptions.Lookups.
// Later entries override earlier entries with the same name.
func MergeLookups(lookups ...map[string]map[string]interface{}) map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})
	for _, l := range lookups {
		for name, table := range l {
			result[name] = table
		}
	}
	return result
}

// LoadLookupsFromJSON parses JSON data containing multiple named lookup tables.
// The JSON should be a nested object: {"table_name": {"key": {"field": "value"}}}.
// This is useful when a single JSON file contains all lookup tables.
//
// Returns a map that can be directly assigned to ParseOptions.Lookups.
func LoadLookupsFromJSON(data []byte) (map[string]map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty JSON data")
	}

	// Use a decoder to preserve number types
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var raw interface{}
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Check for trailing content
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("expected JSON object (map of maps), got unexpected trailing content")
	}

	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected JSON object (map of maps of maps), got %T", raw)
	}

	result := make(map[string]map[string]interface{}, len(rawMap))
	for tableName, tableValue := range rawMap {
		tableMap, ok := tableValue.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected object for table %q, got %T", tableName, tableValue)
		}

		table := make(map[string]interface{}, len(tableMap))
		for key, value := range tableMap {
			innerMap, ok := value.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected object for key %q in table %q, got %T", key, tableName, value)
			}
			// Convert json.Number values to preserve types
			table[key] = convertJSONNumbers(innerMap)
		}
		result[tableName] = table
	}

	return result, nil
}

// convertJSONNumbers converts json.Number values in a map to their native Go types.
func convertJSONNumbers(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case json.Number:
			// Try int first, then float
			if i, err := val.Int64(); err == nil {
				result[k] = i
			} else if f, err := val.Float64(); err == nil {
				result[k] = f
			} else {
				result[k] = val.String()
			}
		case map[string]interface{}:
			result[k] = convertJSONNumbers(val)
		default:
			result[k] = v
		}
	}
	return result
}
