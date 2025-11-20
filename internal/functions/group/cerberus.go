package group

import (
	"fmt"
	"regexp"
	"strings"
)

// cerberus filters group results using schema-based validation
// Similar to Python Cerberus, but uses a Go-compatible validation approach
// Schema can be provided as:
// 1. A variable name containing the schema (map[string]interface{})
// 2. A schema definition string (YAML/JSON-like, simplified)
//
// Example: functions="cerberus('schema_var')" or functions="cerberus('schema', schema='{type: string, required: true}')"
func cerberus(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		// No schema provided, skip validation
		return data, true, nil
	}

	schemaName := args[0]
	schemaName = strings.Trim(schemaName, `"'`)

	// Get schema from kwargs (passed from runtime/vars)
	var schema map[string]interface{}
	if kwargs != nil {
		if s, ok := kwargs["schema"].(map[string]interface{}); ok {
			schema = s
		} else if s, ok := kwargs[schemaName].(map[string]interface{}); ok {
			schema = s
		}
	}

	if schema == nil {
		// Schema not found, skip validation (don't fail)
		return data, true, nil
	}

	// Validate data against schema
	valid, err := validateAgainstSchema(data, schema)
	if err != nil {
		// Validation error - invalid schema or data
		// Return false to filter out this result
		return data, false, nil
	}

	if !valid {
		// Data doesn't match schema - filter out
		return data, false, nil
	}

	// Data is valid - keep it
	return data, true, nil
}

// validateAgainstSchema validates data against a Cerberus-like schema
// Supports basic Cerberus schema rules:
// - type: string, integer, float, boolean, dict, list
// - required: true/false
// - min, max: for numbers and strings
// - regex: for strings
// - allowed: list of allowed values
// - forbidden: list of forbidden values
func validateAgainstSchema(data map[string]interface{}, schema map[string]interface{}) (bool, error) {
	// Iterate through schema rules
	for field, rule := range schema {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			// Simple rule - just check if field exists
			if rule == "required" || rule == true {
				if _, exists := data[field]; !exists {
					return false, nil
				}
			}
			continue
		}

		// Get field value
		value, exists := data[field]

		// Check required
		if required, ok := ruleMap["required"].(bool); ok && required {
			if !exists {
				return false, nil
			}
		}

		// If field doesn't exist and not required, skip validation
		if !exists {
			continue
		}

		// Validate type
		if typeStr, ok := ruleMap["type"].(string); ok {
			if !validateType(value, typeStr) {
				return false, nil
			}
		}

		// Validate min/max for numbers
		if min, ok := ruleMap["min"]; ok {
			if !validateMin(value, min) {
				return false, nil
			}
		}
		if max, ok := ruleMap["max"]; ok {
			if !validateMax(value, max) {
				return false, nil
			}
		}

		// Validate regex for strings
		if regex, ok := ruleMap["regex"].(string); ok {
			if !validateRegex(value, regex) {
				return false, nil
			}
		}

		// Validate allowed values
		if allowed, ok := ruleMap["allowed"]; ok {
			if !validateAllowed(value, allowed) {
				return false, nil
			}
		}

		// Validate forbidden values
		if forbidden, ok := ruleMap["forbidden"]; ok {
			if !validateForbidden(value, forbidden) {
				return false, nil
			}
		}

		// Validate nested schemas (for dict types)
		if nestedSchema, ok := ruleMap["schema"].(map[string]interface{}); ok {
			if valueMap, ok := value.(map[string]interface{}); ok {
				valid, err := validateAgainstSchema(valueMap, nestedSchema)
				if err != nil || !valid {
					return false, err
				}
			}
		}
	}

	return true, nil
}

// validateType checks if value matches the expected type
func validateType(value interface{}, typeStr string) bool {
	switch strings.ToLower(typeStr) {
	case "string":
		_, ok := value.(string)
		return ok
	case "integer", "int":
		switch value.(type) {
		case int, int8, int16, int32, int64:
			return true
		}
		return false
	case "float", "number":
		switch value.(type) {
		case float32, float64, int, int8, int16, int32, int64:
			return true
		}
		return false
	case "boolean", "bool":
		_, ok := value.(bool)
		return ok
	case "dict", "map":
		_, ok := value.(map[string]interface{})
		return ok
	case "list", "array":
		_, ok := value.([]interface{})
		return ok
	default:
		return true // Unknown type, allow it
	}
}

// validateMin checks if value is >= min
func validateMin(value interface{}, min interface{}) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		val := int64(0)
		switch vv := v.(type) {
		case int:
			val = int64(vv)
		case int8:
			val = int64(vv)
		case int16:
			val = int64(vv)
		case int32:
			val = int64(vv)
		case int64:
			val = vv
		}
		minVal := int64(0)
		switch m := min.(type) {
		case int:
			minVal = int64(m)
		case int64:
			minVal = m
		case float64:
			minVal = int64(m)
		}
		return val >= minVal
	case float32, float64:
		val := 0.0
		switch vv := v.(type) {
		case float32:
			val = float64(vv)
		case float64:
			val = vv
		}
		minVal := 0.0
		switch m := min.(type) {
		case int:
			minVal = float64(m)
		case float64:
			minVal = m
		}
		return val >= minVal
	case string:
		minLen := 0
		switch m := min.(type) {
		case int:
			minLen = m
		case int64:
			minLen = int(m)
		case float64:
			minLen = int(m)
		}
		return len(v) >= minLen
	}
	return true
}

// validateMax checks if value is <= max
func validateMax(value interface{}, max interface{}) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		val := int64(0)
		switch vv := v.(type) {
		case int:
			val = int64(vv)
		case int8:
			val = int64(vv)
		case int16:
			val = int64(vv)
		case int32:
			val = int64(vv)
		case int64:
			val = vv
		}
		maxVal := int64(0)
		switch m := max.(type) {
		case int:
			maxVal = int64(m)
		case int64:
			maxVal = m
		case float64:
			maxVal = int64(m)
		}
		return val <= maxVal
	case float32, float64:
		val := 0.0
		switch vv := v.(type) {
		case float32:
			val = float64(vv)
		case float64:
			val = vv
		}
		maxVal := 0.0
		switch m := max.(type) {
		case int:
			maxVal = float64(m)
		case float64:
			maxVal = m
		}
		return val <= maxVal
	case string:
		maxLen := 0
		switch m := max.(type) {
		case int:
			maxLen = m
		case int64:
			maxLen = int(m)
		case float64:
			maxLen = int(m)
		}
		return len(v) <= maxLen
	}
	return true
}

// validateRegex checks if string value matches regex pattern
func validateRegex(value interface{}, pattern string) bool {
	str, ok := value.(string)
	if !ok {
		return false
	}
	// Use regexp package
	matched, err := regexp.MatchString(pattern, str)
	return err == nil && matched
}

// validateAllowed checks if value is in allowed list
func validateAllowed(value interface{}, allowed interface{}) bool {
	allowedList, ok := allowed.([]interface{})
	if !ok {
		return true // Invalid allowed format, allow value
	}

	for _, allowedVal := range allowedList {
		if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", allowedVal) {
			return true
		}
	}
	return false
}

// validateForbidden checks if value is NOT in forbidden list
func validateForbidden(value interface{}, forbidden interface{}) bool {
	forbiddenList, ok := forbidden.([]interface{})
	if !ok {
		return true // Invalid forbidden format, allow value
	}

	for _, forbiddenVal := range forbiddenList {
		if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", forbiddenVal) {
			return false // Value is forbidden
		}
	}
	return true // Value is not forbidden
}

