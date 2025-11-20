package group

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Function represents a group function
type Function func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)

// Registry holds all registered group functions
type Registry struct {
	functions map[string]Function
}

// NewRegistry creates a new group function registry
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

// registerBuiltins registers all built-in group functions
func (r *Registry) registerBuiltins() {
	r.Register("contains", contains)
	r.Register("set", set)
	r.Register("record", record)
	r.Register("delete", deleteKey)
	r.Register("del", deleteKey) // Python TTP uses "del" as the function name
	r.Register("expand", expand)
	r.Register("itemize", itemize)
	r.Register("containsall", containsall)
	r.Register("exclude", exclude)
	r.Register("excludeall", excludeall)
	r.Register("equal", equal)
	r.Register("to_int", toInt)
	r.Register("contains_val", containsVal)
	r.Register("exclude_val", excludeVal)
	r.Register("sformat", sformat)
	r.Register("items2dict", items2dict)
	r.Register("to_ip", toIP)
	r.Register("cerberus", cerberus)
	r.Register("validate", validate)
}

// contains checks if data contains at least ONE of the specified keys
// Python TTP: returns True if at least one variable is found, False otherwise
func contains(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, false, fmt.Errorf("contains requires at least one argument")
	}

	// Check if at least one of the specified keys exists
	for _, key := range args {
		if _, exists := data[key]; exists {
			return data, true, nil
		}
	}

	// None of the keys found
	return data, false, nil
}

// set sets a variable in the data
// Python TTP signature: set(source, target="_use_source_", default="_no_default_value_")
// source - name of source variable to retrieve value from (first argument)
// target - name of field to save into (second argument, defaults to source name if "_use_source_")
// default - default value to use if source variable not found (third argument, format: default='value')
// If source is a variable name, use its value; otherwise use source as literal value
func set(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, true, fmt.Errorf("set requires at least one argument")
	}

	source := args[0] // First arg is source (variable to get value from)
	target := "_use_source_"
	defaultValue := "_no_default_value_"
	
	// Parse arguments - could be: "vrf", "vrf, default='global'", "vrf, target, default='value'"
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "default=") {
			// Extract default value: default='global' -> 'global'
			defaultValue = strings.TrimPrefix(arg, "default=")
			defaultValue = strings.Trim(defaultValue, `"'`)
		} else if target == "_use_source_" {
			// Second argument without default= is the target
			target = arg
		}
	}

	// Resolve source value
	// First check if it's a template variable in kwargs (vars or recorded vars)
	var sourceValue interface{}
	foundSource := false
	if kwargs != nil {
		// Check if it's a template variable or recorded var
		if varValue, ok := kwargs[source]; ok {
			sourceValue = varValue
			foundSource = true
		}
	}
	
	// If not found and default is provided, use default
	if !foundSource && defaultValue != "_no_default_value_" {
		sourceValue = defaultValue
		foundSource = true
	}
	
	// If still not found, check data
	if !foundSource {
		if varValue, ok := data[source]; ok {
			sourceValue = varValue
			foundSource = true
		}
	}
	
	// If still not found, use source as literal value
	if !foundSource {
		sourceValue = source
	}

	// Determine target name
	if target == "_use_source_" {
		target = source
	}

	// Set target field to source value
	data[target] = sourceValue

	return data, true, nil
}

// record records a value for path formation
// Python TTP signature: record(source, target="_use_source_")
// Records the value of source variable to template variables (for path formation)
// Does NOT modify the data itself - only records to template variables
// Also stores in global recorded vars (via _recorded_vars in kwargs) for cross-group access
func record(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, true, fmt.Errorf("record requires at least one argument")
	}

	source := args[0]
	target := source // Default target is source name

	if len(args) >= 2 {
		target = args[1]
	}

	// Record the value to template variables (kwargs) if source exists in data
	// This is used for path formation, not for modifying the data
	if sourceValue, exists := data[source]; exists {
		// Record to template variables (kwargs) so it can be used in path formation
		if kwargs != nil {
			kwargs[target] = sourceValue
			// Also store in global recorded vars (for cross-group access, e.g., sformat)
			if recordedVars, ok := kwargs["_recorded_vars"].(map[string]interface{}); ok {
				recordedVars[target] = sourceValue
			}
		}
	}

	// Don't modify data - just return it as-is
	return data, true, nil
}

// deleteKey removes keys from data
// Python TTP: del="key1, key2" deletes all specified keys
func deleteKey(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, true, fmt.Errorf("delete requires at least one argument")
	}

	// Delete all specified keys
	for _, key := range args {
		delete(data, key)
	}

	return data, true, nil
}

// expand expands dot-separated match variable names to nested dictionary structure
// Example: {"target.x": "value1", "target.y": "value2"} -> {"target": {"x": "value1", "y": "value2"}}
func expand(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if data == nil {
		return data, true, nil
	}

	ret := make(map[string]interface{})

	// Expand match variable names to dictionary
	for key, value := range data {
		ref := ret
		keys := splitDotPath(key)

		// Navigate/create nested structure
		for i, k := range keys {
			if i == len(keys)-1 {
				// Last key - set the value
				ref[k] = value
				break
			}

			// Not the last key - create nested map if needed
			if nextRef, exists := ref[k]; exists {
				if nextMap, ok := nextRef.(map[string]interface{}); ok {
					ref = nextMap
				} else {
					// Key exists but is not a map - create new map
					newMap := make(map[string]interface{})
					ref[k] = newMap
					ref = newMap
				}
			} else {
				// Key doesn't exist - create new map
				newMap := make(map[string]interface{})
				ref[k] = newMap
				ref = newMap
			}
		}
	}

	return ret, true, nil
}

// splitDotPath splits a dot-separated path into individual keys
func splitDotPath(path string) []string {
	if path == "" {
		return []string{}
	}

	var keys []string
	var current strings.Builder

	for _, char := range path {
		if char == '.' {
			if current.Len() > 0 {
				keys = append(keys, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		keys = append(keys, current.String())
	}

	return keys
}

// itemize extracts a key from data and creates a list of values
// This is a special function that needs to work with the runtime to save items to a path
// For now, we'll mark the itemized value in the data and let the runtime handle saving
func itemize(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	// Get key from args or kwargs
	var key string
	if len(args) > 0 {
		key = args[0]
	} else if kwargs != nil {
		if k, ok := kwargs["key"].(string); ok {
			key = k
		}
	}

	if key == "" {
		return data, false, fmt.Errorf("itemize requires a key argument")
	}

	// Check if key exists in data
	value, exists := data[key]
	if !exists {
		// Key not found - invalidate group
		return data, false, nil
	}

	// Python TTP's itemize returns the value directly and modifies the path
	// For now, we'll return the value wrapped in a list to match Python TTP's behavior
	// The value should be stored directly in the group name as a list
	// We'll use a special key that the runtime can recognize and process

	// Remove the original key from data (as per Python implementation)
	delete(data, key)

	// Store itemize metadata
	data["_itemize_key"] = key
	data["_itemize_value"] = value

	return data, true, nil
}

// containsall checks if data contains ALL of the specified keys
func containsall(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, false, fmt.Errorf("containsall requires at least one argument")
	}

	// Check that ALL keys exist
	for _, key := range args {
		if _, exists := data[key]; !exists {
			return data, false, nil
		}
	}

	return data, true, nil
}

// exclude invalidates group results if ANY of the given keys are present
func exclude(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, true, nil // No keys to exclude, so valid
	}

	// If ANY key exists, invalidate
	for _, key := range args {
		if _, exists := data[key]; exists {
			return data, false, nil
		}
	}

	return data, true, nil
}

// excludeall invalidates group results if ALL of the given keys are present
func excludeall(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, true, nil // No keys to exclude, so valid
	}

	// Check that ALL keys exist
	allExist := true
	for _, key := range args {
		if _, exists := data[key]; !exists {
			allExist = false
			break
		}
	}

	// If all keys exist, invalidate
	if allExist {
		return data, false, nil
	}

	return data, true, nil
}

// equal verifies that a key's value equals the provided value
func equal(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 2 {
		return data, false, fmt.Errorf("equal requires key and value arguments")
	}

	key := args[0]
	expectedValue := args[1]

	value, exists := data[key]
	if !exists {
		return data, false, nil
	}

	// Compare values
	if fmt.Sprintf("%v", value) != expectedValue {
		return data, false, nil
	}

	return data, true, nil
}

// toInt converts specified keys to integers
// Python TTP: to_int="key1, key2" or to_int="key, intlist=True"
// If intlist=True, convert each item in the list to integer
func toInt(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, true, nil // No keys specified, return as-is
	}

	// Check for intlist parameter
	intlist := false
	var keys []string
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if strings.HasPrefix(arg, "intlist=") {
			// Parse intlist=True or intlist=False
			value := strings.TrimPrefix(arg, "intlist=")
			value = strings.TrimSpace(value)
			intlist = (value == "True" || value == "true")
		} else if arg != "" {
			keys = append(keys, arg)
		}
	}

	// If no keys specified (only intlist parameter), convert all values
	if len(keys) == 0 {
		// Convert all values in data
		for key, value := range data {
			if intlist {
				// Convert list items to integers
				if listValue, ok := value.([]interface{}); ok {
					convertedList := make([]interface{}, len(listValue))
					for idx, item := range listValue {
						str := strings.TrimSpace(fmt.Sprintf("%v", item))
						if intVal, err := strconv.Atoi(str); err == nil {
							convertedList[idx] = intVal
						} else {
							// If conversion fails, keep original value
							convertedList[idx] = item
						}
					}
					data[key] = convertedList
				} else {
					// Not a list - convert directly
					str := strings.TrimSpace(fmt.Sprintf("%v", value))
					if intVal, err := strconv.Atoi(str); err == nil {
						data[key] = intVal
					}
				}
			} else {
				// Not intlist - convert directly
				str := strings.TrimSpace(fmt.Sprintf("%v", value))
				if intVal, err := strconv.Atoi(str); err == nil {
					data[key] = intVal
				}
			}
		}
	} else {
		// Convert specified keys
		for _, key := range keys {
			if value, exists := data[key]; exists {
				if intlist {
					// Convert list items to integers
					if listValue, ok := value.([]interface{}); ok {
						convertedList := make([]interface{}, len(listValue))
						for idx, item := range listValue {
							str := strings.TrimSpace(fmt.Sprintf("%v", item))
							if intVal, err := strconv.Atoi(str); err == nil {
								convertedList[idx] = intVal
							} else {
								// If conversion fails, keep original value
								convertedList[idx] = item
							}
						}
						data[key] = convertedList
					} else {
						// Not a list - convert directly
						str := strings.TrimSpace(fmt.Sprintf("%v", value))
						if intVal, err := strconv.Atoi(str); err == nil {
							data[key] = intVal
						}
					}
				} else {
					// Not intlist - convert directly
					str := strings.TrimSpace(fmt.Sprintf("%v", value))
					if intVal, err := strconv.Atoi(str); err == nil {
						data[key] = intVal
					}
					// If conversion fails, leave value as-is (like Python TTP)
				}
			}
		}
	}

	return data, true, nil
}

// containsVal checks if a certain key contains a certain value
func containsVal(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 2 {
		return data, false, fmt.Errorf("contains_val requires key and value arguments")
	}

	key := args[0]
	expectedValue := args[1]

	// Resolve expectedValue from template variables if it's a variable name
	// First check if it's a quoted string (remove quotes)
	if (strings.HasPrefix(expectedValue, `"`) && strings.HasSuffix(expectedValue, `"`)) ||
		(strings.HasPrefix(expectedValue, `'`) && strings.HasSuffix(expectedValue, `'`)) {
		expectedValue = strings.Trim(expectedValue, `"'`)
	} else if kwargs != nil {
		// Check if it's a template variable
		if varValue, ok := kwargs[expectedValue]; ok {
			expectedValue = fmt.Sprintf("%v", varValue)
		}
	}

	value, exists := data[key]
	if !exists {
		return data, false, nil
	}

	// Check if value contains the expected value (as string)
	valueStr := fmt.Sprintf("%v", value)
	if strings.Contains(valueStr, expectedValue) {
		return data, true, nil
	}

	return data, false, nil
}

// excludeVal checks if a certain key contains a certain value (inverse of containsVal)
func excludeVal(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 2 {
		return data, true, nil // No key/value to exclude, so valid
	}

	key := args[0]
	excludedValue := args[1]

	// Resolve excludedValue from template variables if it's a variable name
	// First check if it's a quoted string (remove quotes)
	if (strings.HasPrefix(excludedValue, `"`) && strings.HasSuffix(excludedValue, `"`)) ||
		(strings.HasPrefix(excludedValue, `'`) && strings.HasSuffix(excludedValue, `'`)) {
		excludedValue = strings.Trim(excludedValue, `"'`)
	} else if kwargs != nil {
		// Check if it's a template variable
		if varValue, ok := kwargs[excludedValue]; ok {
			excludedValue = fmt.Sprintf("%v", varValue)
		}
	}

	value, exists := data[key]
	if !exists {
		return data, true, nil // Key doesn't exist, so valid
	}

	// Check if value contains the excluded value (as string)
	valueStr := fmt.Sprintf("%v", value)
	if strings.Contains(valueStr, excludedValue) {
		return data, false, nil // Contains excluded value, invalidate
	}

	return data, true, nil
}

// sformat formats a string using data values and template variables
// Example: sformat('description', 'Interface {interface} on {device}')
// The format string uses {key} placeholders that are replaced with values from data
func sformat(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 2 {
		return data, true, fmt.Errorf("sformat requires format string and field name arguments")
	}

	// Python TTP signature: sformat(data, string, add_field)
	// string - format string with {key} placeholders
	// add_field - name of field to store formatted result
	// Format order: 1) global vars (record), 2) template vars, 3) match results (later override earlier)

	stringArg := args[0] // The format string
	addField := args[1]  // The field name to store result in

	// Build a map with all available variables for formatting
	// Order: 1) kwargs (template vars and recorded vars), 2) data (match results override)
	formatData := make(map[string]interface{})
	// First add kwargs (template vars and recorded vars)
	if kwargs != nil {
		for k, v := range kwargs {
			formatData[k] = v
		}
	}
	// Then add data (match results override template vars)
	for k, v := range data {
		formatData[k] = v
	}

	// Format the string using Python-style .format() behavior
	// Replace {key} with values from formatData
	result := stringArg
	for k, v := range formatData {
		placeholder := "{" + k + "}"
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}

	// Set the formatted result in data using addField as the key
	// This matches Python TTP: data[add_field] = string.format(**data)
	data[addField] = result

	return data, true, nil
}

// items2dict combines values of key_name and value_name keys into a dictionary
// Example: items2dict('key', 'value') where data has key=['a','b'] and value=[1,2]
// Results in: {'a': 1, 'b': 2}
func items2dict(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 2 {
		return data, true, fmt.Errorf("items2dict requires key_name and value_name arguments")
	}

	keyName := args[0]
	valueName := args[1]

	// Get key and value from data
	key, keyExists := data[keyName]
	value, valueExists := data[valueName]

	if !keyExists || !valueExists {
		return data, true, nil // Missing keys, return as-is
	}

	// Python TTP's items2dict does: data[data.pop(key_name)] = data.pop(value_name)
	// This means it:
	// 1. Gets the value of key_name (e.g., 'hostname')
	// 2. Gets the value of value_name (e.g., 'router1')
	// 3. Removes both keys from data
	// 4. Creates a new key using the key_name's value: data['hostname'] = 'router1'

	keyStr := fmt.Sprintf("%v", key)

	// Remove original keys (Python TTP does data.pop())
	delete(data, keyName)
	delete(data, valueName)

	// Create new key using the key_name's value
	data[keyStr] = value

	return data, true, nil
}

// toIP converts specified keys to IP address objects (strings for now)
func toIP(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		return data, true, nil // No keys specified, return as-is
	}

	// Convert each specified key to IP address format
	for _, key := range args {
		if value, exists := data[key]; exists {
			str := strings.TrimSpace(fmt.Sprintf("%v", value))

			// Basic IP validation and normalization
			// For now, just validate and return as string
			// In the future, could return IP address objects
			ipv4Regex := regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}(?:/[0-9]{1,2})?$`)
			if ipv4Regex.MatchString(str) {
				// Validate each octet
				parts := strings.Split(strings.Split(str, "/")[0], ".")
				valid := true
				for _, part := range parts {
					octet, err := strconv.Atoi(part)
					if err != nil || octet < 0 || octet > 255 {
						valid = false
						break
					}
				}
				if valid {
					data[key] = str // Store validated IP
				}
			}
			// If not valid IP, leave as-is (like Python TTP)
		}
	}

	return data, true, nil
}
