package group

import (
	"fmt"
	"strings"
)

// validate adds Cerberus validation information to results without filtering
// Unlike cerberus(), this function doesn't filter - it adds validation metadata
// Example: functions="validate('schema_var')" adds _validation_errors_ field
func validate(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error) {
	if len(args) < 1 {
		// No schema provided, skip validation
		return data, true, nil
	}

	schemaName := args[0]
	schemaName = strings.Trim(schemaName, `"'`)

	// Get schema from kwargs (passed from runtime/vars)
	// Schema can be:
	// 1. Directly in kwargs["schema"]
	// 2. In kwargs[schemaName] (variable name)
	// 3. In kwargs as a variable reference
	var schema map[string]interface{}
	if kwargs != nil {
		// Try direct schema key
		if s, ok := kwargs["schema"].(map[string]interface{}); ok {
			schema = s
		} else if s, ok := kwargs[schemaName].(map[string]interface{}); ok {
			// Try variable name
			schema = s
		} else {
			// Try to find schema in all kwargs (template variables)
			for k, v := range kwargs {
				if k == schemaName {
					if s, ok := v.(map[string]interface{}); ok {
						schema = s
						break
					}
				}
			}
		}
	}

	if schema == nil {
		// Schema not found, skip validation (don't fail)
		return data, true, nil
	}

	// Validate data against schema and collect errors
	errors := validateAndCollectErrors(data, schema)

	// Add validation information to data
	if len(errors) > 0 {
		data["_validation_errors_"] = errors
		data["_validation_valid_"] = false
	} else {
		data["_validation_valid_"] = true
	}

	// Always return true (don't filter) - validation info is added
	return data, true, nil
}

// validateAndCollectErrors validates data against schema and returns list of errors
func validateAndCollectErrors(data map[string]interface{}, schema map[string]interface{}) []map[string]interface{} {
	var errors []map[string]interface{}

	// Iterate through schema rules
	for field, rule := range schema {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			// Simple rule - just check if field exists
			if rule == "required" || rule == true {
				if _, exists := data[field]; !exists {
					errors = append(errors, map[string]interface{}{
						"field":   field,
						"error":   "required field missing",
						"message": fmt.Sprintf("Field '%s' is required", field),
					})
				}
			}
			continue
		}

		// Get field value
		value, exists := data[field]

		// Check required
		if required, ok := ruleMap["required"].(bool); ok && required {
			if !exists {
				errors = append(errors, map[string]interface{}{
					"field":   field,
					"error":   "required field missing",
					"message": fmt.Sprintf("Field '%s' is required", field),
				})
				continue
			}
		}

		// If field doesn't exist and not required, skip validation
		if !exists {
			continue
		}

		// Validate type
		if typeStr, ok := ruleMap["type"].(string); ok {
			if !validateType(value, typeStr) {
				errors = append(errors, map[string]interface{}{
					"field":   field,
					"error":   "type mismatch",
					"message": fmt.Sprintf("Field '%s' must be of type '%s', got %T", field, typeStr, value),
				})
			}
		}

		// Validate min/max for numbers
		if min, ok := ruleMap["min"]; ok {
			if !validateMin(value, min) {
				errors = append(errors, map[string]interface{}{
					"field":   field,
					"error":   "value too small",
					"message": fmt.Sprintf("Field '%s' must be >= %v", field, min),
				})
			}
		}
		if max, ok := ruleMap["max"]; ok {
			if !validateMax(value, max) {
				errors = append(errors, map[string]interface{}{
					"field":   field,
					"error":   "value too large",
					"message": fmt.Sprintf("Field '%s' must be <= %v", field, max),
				})
			}
		}

		// Validate regex for strings
		if regex, ok := ruleMap["regex"].(string); ok {
			if !validateRegex(value, regex) {
				errors = append(errors, map[string]interface{}{
					"field":   field,
					"error":   "regex mismatch",
					"message": fmt.Sprintf("Field '%s' does not match pattern '%s'", field, regex),
				})
			}
		}

		// Validate allowed values
		if allowed, ok := ruleMap["allowed"]; ok {
			if !validateAllowed(value, allowed) {
				errors = append(errors, map[string]interface{}{
					"field":   field,
					"error":   "value not allowed",
					"message": fmt.Sprintf("Field '%s' must be one of %v", field, allowed),
				})
			}
		}

		// Validate forbidden values
		if forbidden, ok := ruleMap["forbidden"]; ok {
			if !validateForbidden(value, forbidden) {
				errors = append(errors, map[string]interface{}{
					"field":   field,
					"error":   "value forbidden",
					"message": fmt.Sprintf("Field '%s' must not be one of %v", field, forbidden),
				})
			}
		}

		// Validate nested schemas (for dict types)
		if nestedSchema, ok := ruleMap["schema"].(map[string]interface{}); ok {
			if valueMap, ok := value.(map[string]interface{}); ok {
				nestedErrors := validateAndCollectErrors(valueMap, nestedSchema)
				// Prefix nested errors with field name
				for _, err := range nestedErrors {
					err["field"] = fmt.Sprintf("%s.%v", field, err["field"])
					errors = append(errors, err)
				}
			}
		}
	}

	return errors
}

