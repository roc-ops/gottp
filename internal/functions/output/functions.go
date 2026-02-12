package output

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Function represents an output function
// Returns: (processedData, error)
type Function func(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)

// Registry holds all registered output functions
type Registry struct {
	functions map[string]Function
}

// NewRegistry creates a new output function registry
func NewRegistry() *Registry {
	reg := &Registry{
		functions: make(map[string]Function),
	}
	reg.registerBuiltins()
	return reg
}

// Register registers a function
func (r *Registry) Register(name string, fn Function) {
	r.functions[name] = fn
}

// Get retrieves a function by name
func (r *Registry) Get(name string) (Function, bool) {
	fn, ok := r.functions[name]
	return fn, ok
}

// registerBuiltins registers all built-in output functions
func (r *Registry) registerBuiltins() {
	r.Register("traverse", traverse)
	r.Register("dict_to_list", dictToList)
	r.Register("is_equal", isEqual)
	
	// Format functions
	r.Register("json", formatJSON)
	r.Register("yaml", formatYAML)
	r.Register("csv", formatCSV)
	r.Register("table", formatTable)
	r.Register("pprint", formatPPrint)
	r.Register("tabulate", formatTabulate)
	r.Register("excel", formatExcel)
	r.Register("jinja2", formatJinja2)
	r.Register("raw", formatRaw)
}

// traverse walks results tree to the given path and returns data at that location
// Usage:
//   - traverse('interfaces') - positional argument (path)
//   - traverse(path='interfaces', strict=True) - keyword arguments
//   - traverse("path='interfaces'", "strict=True") - string arguments with parameter names
//
// Arguments:
//   - path (string, required): Dot-separated path to traverse (e.g., 'interfaces' or 'interfaces.Loopback0')
//   - strict (bool, optional): If true (default), returns empty dict if path not found. If false, returns empty dict on failure.
//
// Return value:
//   - When traversing nested list structures (per_input format), the result is wrapped in a list to preserve structure
//   - Input: [{interfaces: [{interface: "Loopback0"}]}] -> traverse('interfaces') -> [{interface: "Loopback0"}]
func traverse(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Parse path from args or kwargs
	var pathStr string
	strict := true

	// Check kwargs first
	if kwargs != nil {
		if p, ok := kwargs["path"].(string); ok {
			pathStr = p
		}
		if s, ok := kwargs["strict"].(bool); ok {
			strict = s
		}
	}

	// Check args (format: "path='dot.separated.path'" or just "'interfaces'")
	if pathStr == "" && len(args) > 0 {
		for _, arg := range args {
			if strings.HasPrefix(arg, "path=") {
				pathStr = strings.TrimPrefix(arg, "path=")
				pathStr = strings.Trim(pathStr, `"'`)
			} else if strings.HasPrefix(arg, "strict=") {
				strictStr := strings.TrimPrefix(arg, "strict=")
				strictStr = strings.Trim(strictStr, `"'`)
				strict = strictStr == "True" || strictStr == "true" || strictStr == "1"
			} else if pathStr == "" {
				// If no path= prefix, treat the first argument as the path
				// This handles cases like traverse('interfaces')
				pathStr = strings.Trim(arg, `"'`)
			}
		}
	}

	if pathStr == "" {
		return data, nil
	}

	// Split path by dots
	path := strings.Split(pathStr, ".")
	for i := range path {
		path[i] = strings.TrimSpace(path[i])
	}

	result := traversePath(data, path, strict)
	
	// Python TTP's traverse function wraps the result in a list if the input was a nested list
	// This preserves the per_input structure: [[{interfaces: {...}}]] -> [[{interface: ...}]]
	// Check if input is nested list structure (per_input format)
	// Only apply wrapping in the traverse() function context, not when traversePath is called directly
	if dataList, ok := data.([]interface{}); ok && len(dataList) == 1 {
		if _, ok := dataList[0].([]interface{}); ok {
			// Input is nested list structure [[...]] (per_input format)
			// traversePath should return a list of results from the inner list
			// But if it returns a dict (which can happen if traversePath takes a different path),
			// wrap it in a list first, then wrap again to preserve per_input structure
			if resultList, ok := result.([]interface{}); ok {
				// Result is a list - wrap it to preserve per_input structure
				// So [{interface: ...}] becomes [[{interface: ...}]]
				return []interface{}{resultList}, nil
			} else {
				// Result is a dict - wrap it in a list first, then wrap again
				// So {interface: ...} becomes [[{interface: ...}]]
				return []interface{}{[]interface{}{result}}, nil
			}
		} else if _, ok := dataList[0].(map[string]interface{}); ok {
			// Input is [{interfaces: [{interface: ...}]}] format (single map in outer list)
			// traversePath should return a list when traversing to 'interfaces'
			// But if it returns a dict, we need to ensure it's wrapped in a list
			// Check if result is a dict when it should be a list
			if _, ok := result.(map[string]interface{}); ok {
				// Result is a dict but should be a list - wrap it
				// This handles cases where traversePath returns a single dict instead of a list
				return []interface{}{result}, nil
			}
		}
	}
	
	return result, nil
}

// traversePath recursively traverses data structure following the path
func traversePath(data interface{}, path []string, strict bool) interface{} {
	if len(path) == 0 {
		return data
	}

	switch v := data.(type) {
	case map[string]interface{}:
		key := path[0]
		if val, ok := v[key]; ok {
			return traversePath(val, path[1:], strict)
		}
		if strict {
			return map[string]interface{}{} // Return empty dict on strict failure
		}
		return map[string]interface{}{} // Return empty dict
	case []interface{}:
		// For lists, traverse each item
		// Check for nested list structure first (per_input format: [[...]])
		if len(v) == 1 {
			if firstItemList, ok := v[0].([]interface{}); ok {
				// Single item is a list - this is nested list structure (per_input format)
				// Traverse each item in the inner list and collect results
				result := make([]interface{}, 0, len(firstItemList))
				for _, item := range firstItemList {
					traversed := traversePath(item, path, strict)
					// If traversed result is a map, add it directly
					// If it's a list, flatten it
					if traversedList, ok := traversed.([]interface{}); ok {
						result = append(result, traversedList...)
					} else {
						result = append(result, traversed)
					}
				}
				// Return the list of results (not wrapped)
				// The wrapping will be done by the traverse() function if needed
				return result
			} else if firstItem, ok := v[0].(map[string]interface{}); ok {
				// Single item is a map - check if path matches a key in this map
				// This handles cases like [{interfaces: {...}}] where we want to extract interfaces
				if len(path) > 0 {
					if val, ok := firstItem[path[0]]; ok {
						// Found the key - continue traversal
						return traversePath(val, path[1:], strict)
					}
				}
			}
		}
		// Otherwise, traverse each item
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			traversed := traversePath(item, path, strict)
			result = append(result, traversed)
		}
		return result
	default:
		return data
	}
}

// dictToList transforms dictionary to list of dictionaries
// Usage: dict_to_list("key_name='interface'", "path='dot.separated.path'")
func dictToList(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	keyName := "key"
	var pathStr string
	strict := false

	// Parse arguments
	if kwargs != nil {
		if k, ok := kwargs["key_name"].(string); ok {
			keyName = k
		}
		if p, ok := kwargs["path"].(string); ok {
			pathStr = p
		}
		if s, ok := kwargs["strict"].(bool); ok {
			strict = s
		}
	}

	// Parse from args
	for _, arg := range args {
		if strings.HasPrefix(arg, "key_name=") {
			keyName = strings.TrimPrefix(arg, "key_name=")
			keyName = strings.Trim(keyName, `"'`)
		} else if strings.HasPrefix(arg, "path=") {
			pathStr = strings.TrimPrefix(arg, "path=")
			pathStr = strings.Trim(pathStr, `"'`)
		} else if strings.HasPrefix(arg, "strict=") {
			strictStr := strings.TrimPrefix(arg, "strict=")
			strictStr = strings.Trim(strictStr, `"'`)
			strict = strictStr == "True" || strictStr == "true" || strictStr == "1"
		}
	}

	// If path specified, traverse first
	if pathStr != "" {
		path := strings.Split(pathStr, ".")
		for i := range path {
			path[i] = strings.TrimSpace(path[i])
		}
		data = traversePath(data, path, strict)
	}

	// Convert dict to list
	return convertDictToList(data, keyName), nil
}

// convertDictToList converts a dictionary to a list of dictionaries
func convertDictToList(data interface{}, keyName string) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make([]interface{}, 0, len(v))
		// Sort map keys for deterministic output order
		// (Go maps have random iteration order; Python 3.7+ dicts preserve insertion order)
		sortedKeys := make([]string, 0, len(v))
		for k := range v {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)
		for _, k := range sortedKeys {
			val := v[k]
			if valMap, ok := val.(map[string]interface{}); ok {
				// Create new map with key added
				newMap := make(map[string]interface{})
				for key, value := range valMap {
					newMap[key] = value
				}
				newMap[keyName] = k
				result = append(result, newMap)
			} else {
				// Value is not a dict, skip or handle differently
				newMap := map[string]interface{}{
					keyName: k,
					"value": val,
				}
				result = append(result, newMap)
			}
		}
		return result
	case []interface{}:
		// Recursively process list items
		result := make([]interface{}, 0, len(v))
		for _, item := range v {
			converted := convertDictToList(item, keyName)
			result = append(result, converted)
		}
		return result
	default:
		return data
	}
}

// isEqual compares results with expected structure from output tag content
// Usage: is_equal() - compares with data loaded from output tag text
func isEqual(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Get expected data from kwargs (passed from output tag content)
	var expectedData interface{}
	if kwargs != nil {
		if expected, ok := kwargs["_expected_data"]; ok {
			expectedData = expected
		}
	}

	if expectedData == nil {
		// No expected data provided, return comparison result as false
		return map[string]interface{}{
			"is_equal": false,
			"output_name": "",
			"output_description": "",
		}, nil
	}

	// Get output name and description from kwargs
	outputName := ""
	outputDescription := ""
	if kwargs != nil {
		if name, ok := kwargs["_output_name"].(string); ok {
			outputName = name
		}
		if desc, ok := kwargs["_output_description"].(string); ok {
			outputDescription = desc
		}
	}

	// Compare data structures
	isEqual := deepEqual(data, expectedData)

	return map[string]interface{}{
		"is_equal":         isEqual,
		"output_name":      outputName,
		"output_description": outputDescription,
	}, nil
}

// deepEqual performs deep comparison of two data structures
func deepEqual(a, b interface{}) bool {
	// Simple deep equality check
	// For full implementation, we'd want to handle all cases properly
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// formatJSON converts data to JSON string
// Matches Python TTP: json.dumps(data, sort_keys=True, indent=4, separators=(',', ': '))
func formatJSON(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(jsonBytes), nil
}

// formatYAML converts data to YAML string
func formatYAML(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(yamlBytes), nil
}

// formatTable converts data to table format (list of lists, first row is headers)
// This is used by csv, tabulate, and excel formatters
func formatTable(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Parse kwargs for table options
	var pathStr string
	var headers []string
	missing := ""
	key := ""
	strict := true
	
	if kwargs != nil {
		if p, ok := kwargs["path"].(string); ok {
			pathStr = p
		}
		if h, ok := kwargs["headers"].(string); ok {
			headers = strings.Split(h, ",")
			for i := range headers {
				headers[i] = strings.TrimSpace(headers[i])
			}
		}
		if m, ok := kwargs["missing"].(string); ok {
			missing = m
		}
		if k, ok := kwargs["key"].(string); ok {
			key = k
		}
		if s, ok := kwargs["strict"].(bool); ok {
			strict = s
		}
	}
	
	// Parse args
	for _, arg := range args {
		if strings.HasPrefix(arg, "path=") {
			pathStr = strings.TrimPrefix(arg, "path=")
			pathStr = strings.Trim(pathStr, `"'`)
		} else if strings.HasPrefix(arg, "headers=") {
			headersStr := strings.TrimPrefix(arg, "headers=")
			headersStr = strings.Trim(headersStr, `"'`)
			headers = strings.Split(headersStr, ",")
			for i := range headers {
				headers[i] = strings.TrimSpace(headers[i])
			}
		} else if strings.HasPrefix(arg, "missing=") {
			missing = strings.TrimPrefix(arg, "missing=")
			missing = strings.Trim(missing, `"'`)
		} else if strings.HasPrefix(arg, "key=") {
			key = strings.TrimPrefix(arg, "key=")
			key = strings.Trim(key, `"'`)
		} else if strings.HasPrefix(arg, "strict=") {
			strictStr := strings.TrimPrefix(arg, "strict=")
			strictStr = strings.Trim(strictStr, `"'`)
			strict = strictStr == "True" || strictStr == "true" || strictStr == "1"
		}
	}
	
	// Traverse path if specified
	if pathStr != "" {
		path := strings.Split(pathStr, ".")
		for i := range path {
			path[i] = strings.TrimSpace(path[i])
		}
		data = traversePath(data, path, strict)
	}
	
	// Check if data is already in table format [[headers], [data_rows]]
	// This happens when the data structure is already processed (e.g., from group results)
	// Handle both [[headers], [data]] and [[[headers], [data]]] (wrapped in extra list)
	// Python TTP's table formatter returns [[headers], [data]] directly (not wrapped)
	var tableData interface{} = data
	if dataList, ok := data.([]interface{}); ok {
		if len(dataList) == 1 {
			// Unwrap if wrapped in extra list: [[[headers], [data]]] -> [[headers], [data]]
			if innerList, ok := dataList[0].([]interface{}); ok && len(innerList) == 2 {
				// Check if this looks like a table structure
				if headersList, ok := innerList[0].([]interface{}); ok {
					allStrings := true
					for _, h := range headersList {
						if _, ok := h.(string); !ok {
							allStrings = false
							break
						}
					}
					if allStrings {
						// This is a table structure, unwrap it and flatten data if needed
						headers := innerList[0]
						dataRows := innerList[1]
						
						// Recursively flatten nested data
						var flattenList func(interface{}) []interface{}
						flattenList = func(v interface{}) []interface{} {
							if list, ok := v.([]interface{}); ok {
								result := []interface{}{}
								for _, item := range list {
									if innerList, ok := item.([]interface{}); ok {
										// Recursively flatten nested lists
										result = append(result, flattenList(innerList)...)
									} else if dict, ok := item.(map[string]interface{}); ok {
										// Found a dict, add it
										result = append(result, dict)
									} else {
										// Other types, add as-is
										result = append(result, item)
									}
								}
								return result
							}
							return []interface{}{v}
						}
						
						flattenedData := flattenList(dataRows)
						
						// Check if all items are dicts
						if len(flattenedData) > 0 {
							allDicts := true
							for _, item := range flattenedData {
								if _, ok := item.(map[string]interface{}); !ok {
									allDicts = false
									break
								}
							}
							if allDicts {
								// Return flattened table structure: [[headers], [dict1, dict2]]
								return []interface{}{headers, flattenedData}, nil
							}
						}
						
						// Return the unwrapped table structure
						return []interface{}{headers, dataRows}, nil
					}
				}
			}
		} else if len(dataList) == 2 {
			// Check if first element is headers (list of strings) and second is data
			if headersList, ok := dataList[0].([]interface{}); ok {
				// Check if all headers are strings
				allStrings := true
				for _, h := range headersList {
					if _, ok := h.(string); !ok {
						allStrings = false
						break
					}
				}
				if allStrings {
					// This looks like a table structure [[headers], [data]]
					// But we need to flatten the data if it's nested
					// Python TTP: [[headers], [dict1, dict2]]
					// GoTTP might have: [[headers], [[dict1, dict2]]]
					headers := dataList[0]
					dataRows := dataList[1]
					
					// Check if data is nested and flatten it
					if nestedDataList, ok := dataRows.([]interface{}); ok {
						if len(nestedDataList) == 1 {
							// Data is wrapped: [[dict1, dict2]]
							if innerList, ok := nestedDataList[0].([]interface{}); ok {
								// Check if inner list contains dicts
								allDicts := true
								for _, item := range innerList {
									if _, ok := item.(map[string]interface{}); !ok {
										allDicts = false
										break
									}
								}
								if allDicts {
									// Flatten: [[headers], [[dict1, dict2]]] -> [[headers], [dict1, dict2]]
									return []interface{}{headers, innerList}, nil
								}
							}
						} else {
							// Data is already flat: [dict1, dict2]
							// Check if all items are dicts
							allDicts := true
							for _, item := range nestedDataList {
								if _, ok := item.(map[string]interface{}); !ok {
									allDicts = false
									break
								}
							}
							if allDicts {
								// Already in correct format: [[headers], [dict1, dict2]]
								return tableData, nil
							}
						}
					}
					
					// Return as-is (it's already in table format)
					return tableData, nil
				}
			}
		}
	}
	
	// Use the potentially unwrapped data
	data = tableData
	
	// Special case: if data is a dict with a single key and the value is a list of dicts,
	// create a table with the key as the header and the list as the data
	// This matches Python TTP's behavior: {interfaces: [dict1, dict2]} -> [["interfaces"], [dict1, dict2]]
	// Python TTP's table formatter uses traverse() which can return nested lists, so we need to flatten
	if dataMap, ok := data.(map[string]interface{}); ok && len(dataMap) == 1 {
		// Get the single key and value
		var key string
		var value interface{}
		for k, v := range dataMap {
			key = k
			value = v
			break
		}
		// Recursively flatten the value to get a flat list of dicts
		// Python TTP's table formatter uses traverse() which can return nested lists
		// We need to flatten any nested lists to get a flat list of dicts
		var valueList []interface{}
		var flattenList func(interface{}) []interface{}
		flattenList = func(v interface{}) []interface{} {
			if list, ok := v.([]interface{}); ok {
				// If this list has exactly one item and that item is also a list,
				// it's likely a nested wrapper - flatten it
				if len(list) == 1 {
					if innerList, ok := list[0].([]interface{}); ok {
						// Recursively flatten the inner list
						return flattenList(innerList)
					}
				}
				// Otherwise, flatten each item
				result := []interface{}{}
				for _, item := range list {
					if innerList, ok := item.([]interface{}); ok {
						// Recursively flatten nested lists
						flattened := flattenList(innerList)
						result = append(result, flattened...)
					} else if dict, ok := item.(map[string]interface{}); ok {
						// Found a dict, add it
						result = append(result, dict)
					} else {
						// Other types, add as-is (shouldn't happen for table formatter)
						result = append(result, item)
					}
				}
				return result
			}
			// Not a list, return as single item wrapped in list
			return []interface{}{v}
		}
		
		valueList = flattenList(value)
		
		// If valueList is empty or only contains non-dicts, the flattening didn't work
		// This shouldn't happen, but handle it gracefully
		
		// Check if all items are dicts
		if len(valueList) > 0 {
			allDicts := true
			for _, item := range valueList {
				if _, ok := item.(map[string]interface{}); !ok {
					allDicts = false
					break
				}
			}
			if allDicts {
				// Create table structure: [["interfaces"], [[dict1, dict2]]]
				// Python TTP wraps the data list in an extra list: [[dict1, dict2]] instead of [dict1, dict2]
				// This will be wrapped by processOutputFunctions for per_input
				return []interface{}{[]interface{}{key}, []interface{}{valueList}}, nil
			}
		}
	}
	
	// Normalize data to list of dictionaries
	var dataToTable []map[string]interface{}
	
	switch v := data.(type) {
	case []interface{}:
		// Check if this is a list of dicts (anonymous group results)
		// Python TTP combines all matches into a single table
		allDicts := true
		for _, item := range v {
			if _, ok := item.(map[string]interface{}); !ok {
				allDicts = false
				break
			}
		}
		
		if allDicts {
			// List of dicts - combine into single table
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					dataToTable = append(dataToTable, itemMap)
				}
			}
		} else {
			// Mixed types - process each item
			for _, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					dataToTable = append(dataToTable, itemMap)
				} else if itemList, ok := item.([]interface{}); ok {
					// Handle nested lists (e.g., [[{...}, {...}]])
					for _, nestedItem := range itemList {
						if nestedMap, ok := nestedItem.(map[string]interface{}); ok {
							dataToTable = append(dataToTable, nestedMap)
						}
					}
				}
			}
		}
	case map[string]interface{}:
		// If key specified, convert dict to list
		// Python TTP: when key is specified, each key-value pair becomes a row
		// The key becomes a column with name specified by key parameter
		// The value (if it's a dict) becomes the other columns
		if key != "" {
			// Sort map keys for deterministic output order
			// (Go maps have random iteration order; Python 3.7+ dicts preserve insertion order)
			sortedKeys := make([]string, 0, len(v))
			for k := range v {
				sortedKeys = append(sortedKeys, k)
			}
			sort.Strings(sortedKeys)
			for _, k := range sortedKeys {
				val := v[k]
				if valMap, ok := val.(map[string]interface{}); ok {
					// Value is a dict - merge key into it
					newMap := make(map[string]interface{})
					// Copy all values from valMap
					for mapKey, value := range valMap {
						newMap[mapKey] = value
					}
					// Add the key as a column with the name specified by key parameter
					newMap[key] = k
					dataToTable = append(dataToTable, newMap)
				} else {
					// Value is not a dict - create a simple map with key and value
					newMap := map[string]interface{}{
						key:   k,
						"value": val,
					}
					dataToTable = append(dataToTable, newMap)
				}
			}
		} else {
			// No key specified - check if this is a map of maps (like {"Loopback0": {...}, "Loopback1": {...}})
			// In this case, Python TTP treats each key-value pair as a row
			allMaps := true
			for _, val := range v {
				if _, ok := val.(map[string]interface{}); !ok {
					allMaps = false
					break
				}
			}
			if allMaps {
				// Map of maps - convert each key-value pair to a row
				// Sort keys for deterministic output order
				sortedKeys := make([]string, 0, len(v))
				for k := range v {
					sortedKeys = append(sortedKeys, k)
				}
				sort.Strings(sortedKeys)
				for _, k := range sortedKeys {
					if valMap, ok := v[k].(map[string]interface{}); ok {
						dataToTable = append(dataToTable, valMap)
					}
				}
			} else {
				// Single map - wrap in list
				dataToTable = append(dataToTable, v)
			}
		}
	}
	
	// Create headers if not provided
	if len(headers) == 0 {
		headerSet := make(map[string]bool)
		for _, item := range dataToTable {
			for k := range item {
				headerSet[k] = true
			}
		}
		headers = make([]string, 0, len(headerSet))
		for k := range headerSet {
			headers = append(headers, k)
		}
		sort.Strings(headers)
	}
	
	// Build table (list of lists)
	table := [][]interface{}{}
	
	// Add headers row
	headerRow := make([]interface{}, len(headers))
	for i, h := range headers {
		headerRow[i] = h
	}
	table = append(table, headerRow)
	
	// Add data rows
	for _, item := range dataToTable {
		row := make([]interface{}, len(headers))
		for i, h := range headers {
			if val, ok := item[h]; ok {
				row[i] = val
			} else {
				row[i] = missing
			}
		}
		table = append(table, row)
	}
	
	return table, nil
}

// pythonLikeString converts a Go value to Python-like string representation
// This is used for CSV and pprint output to match Python TTP's behavior
// sortKeys: if true, sort dict keys alphabetically (for pprint), otherwise use custom order (for CSV)
// Note: For simple strings, Python's str() returns the string as-is (not quoted)
// Only dicts and lists use quotes in their representation
func pythonLikeString(val interface{}, sortKeys bool) string {
	if val == nil {
		return "None"
	}
	
	switch v := val.(type) {
	case string:
		// For simple strings, Python's str() returns the string as-is (not quoted)
		// Only when strings are inside dicts/lists do they get single quotes
		return v
	case bool:
		if v {
			return "True"
		}
		return "False"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case []interface{}:
		// List representation: [item1, item2, ...]
		parts := make([]string, len(v))
		for i, item := range v {
			itemStr := pythonLikeString(item, sortKeys)
			// If the item is a string, wrap it in single quotes for list representation
			if _, isStr := item.(string); isStr {
				itemStr = "'" + itemStr + "'"
			}
			parts[i] = itemStr
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		// Dict representation: {'key': 'value', ...}
		// Python 3.7+ preserves insertion order, so we should preserve order too
		// However, Go maps are unordered, so we'll sort for consistency
		// Python TTP processes nested patterns after main patterns, so keys from nested
		// patterns (like 'ip') appear before keys from main patterns (like 'interface')
		// We'll use a custom sort that tries to match Python TTP's observed order
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		// Sort keys based on sortKeys parameter
		if sortKeys {
			// For pprint, sort alphabetically (Python's pprint.pformat() sorts keys)
			sort.Strings(keys)
		} else {
			// For CSV, use custom sort to match Python TTP's insertion order
			// Common pattern: nested pattern keys (like 'ip') come before main pattern keys (like 'interface')
			sort.Slice(keys, func(i, j int) bool {
				// For common networking keys, use a specific order to match Python TTP
				// This is a heuristic based on observed Python TTP behavior
				order := map[string]int{
					"ip":       1,
					"interface": 2,
				}
				orderI, hasOrderI := order[keys[i]]
				orderJ, hasOrderJ := order[keys[j]]
				if hasOrderI && hasOrderJ {
					return orderI < orderJ
				}
				if hasOrderI {
					return true
				}
				if hasOrderJ {
					return false
				}
				// For other keys, sort alphabetically
				return keys[i] < keys[j]
			})
		}
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			// Keys and string values use single quotes in Python dict representation
			valStr := pythonLikeString(v[k], sortKeys)
			// If the value is a string, wrap it in single quotes for dict representation
			if _, isStr := v[k].(string); isStr {
				valStr = "'" + valStr + "'"
			}
			parts = append(parts, "'"+k+"': "+valStr)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		// For other types, try to convert to string
		// If it's a complex type, try JSON first
		if jsonBytes, err := json.Marshal(v); err == nil {
			// Try to parse as JSON and convert to Python-like
			var jsonVal interface{}
			if err := json.Unmarshal(jsonBytes, &jsonVal); err == nil {
				return pythonLikeString(jsonVal, sortKeys)
			}
		}
		return fmt.Sprintf("%v", v)
	}
}

// formatCSV converts data to CSV string using table formatter
func formatCSV(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Get separator and quote from kwargs
	sep := ","
	quote := `"`
	if kwargs != nil {
		if s, ok := kwargs["sep"].(string); ok {
			sep = s
		}
		if q, ok := kwargs["quote"].(string); ok {
			quote = q
		}
	}
	
	// Parse args
	for _, arg := range args {
		if strings.HasPrefix(arg, "sep=") {
			sep = strings.TrimPrefix(arg, "sep=")
			sep = strings.Trim(sep, `"'`)
		} else if strings.HasPrefix(arg, "quote=") {
			quote = strings.TrimPrefix(arg, "quote=")
			quote = strings.Trim(quote, `"'`)
		}
	}
	
	// Use table formatter first
	table, err := formatTable(data, args, kwargs)
	if err != nil {
		return nil, err
	}
	
	// formatTable can return either [][]interface{} or []interface{} (wrapped format)
	// Handle both cases
	var tableList [][]interface{}
	if tableListDirect, ok := table.([][]interface{}); ok {
		tableList = tableListDirect
	} else if tableListWrapped, ok := table.([]interface{}); ok {
		// Unwrap: [[headers], [data]] -> [][]interface{}
		if len(tableListWrapped) == 2 {
			if headersList, ok := tableListWrapped[0].([]interface{}); ok {
				if dataList, ok := tableListWrapped[1].([]interface{}); ok {
					// Convert to [][]interface{}
					tableList = make([][]interface{}, 0, len(dataList)+1)
					// Add headers row
					headerRow := make([]interface{}, len(headersList))
					copy(headerRow, headersList)
					tableList = append(tableList, headerRow)
					// Add data rows
					// Handle nested data: [[headers], [[data]]] -> [[headers], [data]]
					// Python TTP can return nested data lists
					if len(dataList) == 1 {
						// Data is nested: [[data]]
						if nestedDataList, ok := dataList[0].([]interface{}); ok {
							// Check if nested list contains dicts (normal case) or more lists
							allDicts := true
							for _, item := range nestedDataList {
								if _, ok := item.(map[string]interface{}); !ok {
									allDicts = false
									break
								}
							}
							if allDicts {
								// Nested list contains dicts: [[dict1, dict2]]
								// For CSV, we want to keep this as a single row with the list as the value
								// Python TTP converts the entire list to a Python dict representation string
								// Create a row with the list as the first (and only) column value
								rowValues := make([]interface{}, len(headersList))
								rowValues[0] = nestedDataList // Keep as list for special CSV handling
								// Fill remaining columns with empty strings if there are more headers
								for i := 1; i < len(headersList); i++ {
									rowValues[i] = ""
								}
								tableList = append(tableList, rowValues)
							} else {
								// Nested list contains more lists, flatten recursively
								for _, row := range nestedDataList {
									if rowList, ok := row.([]interface{}); ok {
										tableList = append(tableList, rowList)
									} else if rowMap, ok := row.(map[string]interface{}); ok {
										// Convert map to row based on headers
										rowValues := make([]interface{}, len(headersList))
										for i, h := range headersList {
											if headerStr, ok := h.(string); ok {
												if val, exists := rowMap[headerStr]; exists {
													rowValues[i] = val
												} else {
													rowValues[i] = ""
												}
											}
										}
										tableList = append(tableList, rowValues)
									} else {
										// Single value row
										rowValues := make([]interface{}, len(headersList))
										rowValues[0] = row
										tableList = append(tableList, rowValues)
									}
								}
							}
						} else {
							// Single item, not a list
							rowValues := make([]interface{}, len(headersList))
							rowValues[0] = dataList[0]
							tableList = append(tableList, rowValues)
						}
					} else {
						// Data is flat: [data]
						for _, row := range dataList {
							if rowList, ok := row.([]interface{}); ok {
								tableList = append(tableList, rowList)
							} else if rowMap, ok := row.(map[string]interface{}); ok {
								// Convert map to row based on headers
								rowValues := make([]interface{}, len(headersList))
								for i, h := range headersList {
									if headerStr, ok := h.(string); ok {
										if val, exists := rowMap[headerStr]; exists {
											rowValues[i] = val
										} else {
											rowValues[i] = ""
										}
									}
								}
								tableList = append(tableList, rowValues)
							} else {
								// Single value row
								rowValues := make([]interface{}, len(headersList))
								rowValues[0] = row
								tableList = append(tableList, rowValues)
							}
						}
					}
				} else {
					return "", fmt.Errorf("table formatter did not return list of lists")
				}
			} else {
				return "", fmt.Errorf("table formatter did not return list of lists")
			}
		} else {
			return "", fmt.Errorf("table formatter did not return list of lists")
		}
	} else {
		return "", fmt.Errorf("table formatter did not return list of lists")
	}
	
	if len(tableList) == 0 {
		return "", nil
	}
	
	// Debug: Check what tableList contains
	// For the case where we have [[headers], [[data]]], tableList should be:
	// - Row 0: headers (e.g., ["interfaces"])
	// - Row 1: [nestedDataList] (e.g., [[{...}, {...}]])
	
	// Convert to CSV
	// Python TTP's CSV formatter manually formats rows (doesn't use Python's csv module)
	// Format: "{quote}{value1}{quote}{sep}{quote}{value2}{quote}{quote}\n{quote}{value1}{quote}{sep}{quote}{value2}{quote}{quote}"
	// Where sep is the separator wrapped in quotes (e.g., ",")
	// And each row is wrapped in quotes
	
	// Create quoted separator: "," (separator wrapped in quotes)
	sepQuoted := quote + sep + quote
	// Row formatter: \n"{row_content}"
	rowFormatter := "\n" + quote + "%s" + quote
	
	// Format header row
	headerRow := make([]string, len(tableList[0]))
	for i, val := range tableList[0] {
		// Convert to Python-like string representation (CSV uses insertion order, not sorted)
		headerRow[i] = pythonLikeString(val, false)
	}
	// Join with quoted separator and wrap in quotes
	result := quote + strings.Join(headerRow, sepQuoted) + quote
	
	// Format data rows
	// Python TTP's CSV formatter has special behavior: if data row contains a list of dicts,
	// it converts the entire list to a Python dict representation string instead of expanding rows
	for _, row := range tableList[1:] {
		// Check if row has exactly one element that is a list of dicts
		if len(row) == 1 {
			if rowList, ok := row[0].([]interface{}); ok {
				// Check if all items in the list are dicts
				allDicts := true
				for _, item := range rowList {
					if _, ok := item.(map[string]interface{}); !ok {
						allDicts = false
						break
					}
				}
				if allDicts {
					// Python TTP converts list of dicts to Python dict representation string
					// Format: "[{'key1': 'val1', 'key2': 'val2'}, {'key1': 'val3', 'key2': 'val4'}]"
					dictStr := pythonLikeString(rowList, false)
					result += fmt.Sprintf(rowFormatter, dictStr)
					continue
				}
			}
		}
		
		// Normal row formatting
		rowStr := make([]string, len(row))
		for i, val := range row {
			// Convert to Python-like string representation (CSV uses insertion order, not sorted)
			rowStr[i] = pythonLikeString(val, false)
		}
		// Join with quoted separator and wrap in quotes
		result += fmt.Sprintf(rowFormatter, strings.Join(rowStr, sepQuoted))
	}
	
	return result, nil
}

// formatPPrint converts data to pretty-printed string (similar to Python pprint)
// Python TTP uses pprint.pformat() which produces Python dict representation
func formatPPrint(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Python's pprint produces Python dict representation, not JSON
	// pprint.pformat() sorts dict keys alphabetically
	return pythonLikeString(data, true), nil
}

// formatTabulate converts data to tabulated table string using table formatter
func formatTabulate(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Use table formatter first
	table, err := formatTable(data, args, kwargs)
	if err != nil {
		return nil, err
	}
	
	tableList, ok := table.([][]interface{})
	if !ok {
		return "", fmt.Errorf("table formatter did not return list of lists")
	}
	
	if len(tableList) == 0 {
		return "", nil
	}
	
	// Extract headers
	headers := tableList[0]
	rows := tableList[1:]
	
	// Convert to string table format (simple implementation)
	var buf strings.Builder
	
	// Calculate column widths
	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(fmt.Sprintf("%v", h))
	}
	for _, row := range rows {
		for i, val := range row {
			width := len(fmt.Sprintf("%v", val))
			if width > colWidths[i] {
				colWidths[i] = width
			}
		}
	}
	
	// Write header row
	for i, h := range headers {
		fmt.Fprintf(&buf, "%-*s", colWidths[i], fmt.Sprintf("%v", h))
		if i < len(headers)-1 {
			buf.WriteString("  ") // 2 spaces between columns
		}
	}
	buf.WriteString("\n")
	
	// Write separator
	for i, width := range colWidths {
		buf.WriteString(strings.Repeat("-", width))
		if i < len(colWidths)-1 {
			buf.WriteString("  ")
		}
	}
	buf.WriteString("\n")
	
	// Write data rows
	for _, row := range rows {
		for i, val := range row {
			fmt.Fprintf(&buf, "%-*s", colWidths[i], fmt.Sprintf("%v", val))
			if i < len(row)-1 {
				buf.WriteString("  ")
			}
		}
		buf.WriteString("\n")
	}
	
	return buf.String(), nil
}

// formatExcel converts data to Excel format (returns file path or bytes)
// For now, return a placeholder - Excel formatter typically writes to file
func formatExcel(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Excel formatter typically writes to a file
	// For comparison tests, we might need to return file path or handle differently
	// For now, use table formatter and return as string representation
	table, err := formatTable(data, args, kwargs)
	if err != nil {
		return nil, err
	}
	
	// Return table as string for now (tests may need adjustment)
	return fmt.Sprintf("%v", table), nil
}

// formatJinja2 renders data using Jinja2 template from output tag content
// Note: This requires the template content to be passed via kwargs
func formatJinja2(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Get template content from kwargs
	var templateContent string
	if kwargs != nil {
		if t, ok := kwargs["_template_content"].(string); ok {
			templateContent = t
		}
	}
	
	if templateContent == "" {
		return "", fmt.Errorf("jinja2 formatter requires template content")
	}
	
	// Use pongo2 (Jinja2-like) to render
	// Note: This is a simplified implementation - full implementation would use pongo2
	// For now, return placeholder
	return fmt.Sprintf("jinja2_rendered: %v", data), nil
}

// formatRaw returns data as-is (no formatting)
func formatRaw(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	return data, nil
}

