package yang

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

// Validator validates JSON data against YANG schemas
type Validator struct {
	moduleSet *ModuleSet
}

// NewValidator creates a new validator with a module set
func NewValidator(moduleSet *ModuleSet) *Validator {
	return &Validator{
		moduleSet: moduleSet,
	}
}

// Validate validates JSON data against a YANG path
// data can be a map[string]interface{} or []map[string]interface{}
// yangPath format: "module-name:path/to/node"
func (v *Validator) Validate(data interface{}, yangPath, groupName string) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []*ValidationError{},
		Warnings: []*ValidationError{},
	}

	// Find the schema entry for the path
	schemaEntry, err := v.moduleSet.FindSchemaByPath(yangPath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, &ValidationError{
			GroupName: groupName,
			Path:      yangPath,
			Message:   fmt.Sprintf("YANG path not found: %v", err),
		})
		return result
	}

	// Convert data to JSON and back to validate structure
	// Handle both single objects and arrays
	var dataToValidate interface{}
	switch d := data.(type) {
	case map[string]interface{}:
		dataToValidate = d
	case []map[string]interface{}:
		// For arrays, validate each item
		for i, item := range d {
			itemResult := v.validateSingleItem(item, schemaEntry, yangPath, groupName, i)
			if !itemResult.Valid {
				result.Valid = false
			}
			result.Errors = append(result.Errors, itemResult.Errors...)
			result.Warnings = append(result.Warnings, itemResult.Warnings...)
		}
		return result
	default:
		// Try to convert to map
		if jsonBytes, err := json.Marshal(data); err == nil {
			var jsonData map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &jsonData); err == nil {
				dataToValidate = jsonData
			} else {
				result.Valid = false
				result.Errors = append(result.Errors, &ValidationError{
					GroupName: groupName,
					Path:      yangPath,
					Message:   fmt.Sprintf("Invalid data type for validation: %T", data),
				})
				return result
			}
		} else {
			result.Valid = false
			result.Errors = append(result.Errors, &ValidationError{
				GroupName: groupName,
				Path:      yangPath,
				Message:   fmt.Sprintf("Invalid data type for validation: %T", data),
			})
			return result
		}
	}

	// Validate single item
	itemResult := v.validateSingleItem(dataToValidate.(map[string]interface{}), schemaEntry, yangPath, groupName, -1)
	if !itemResult.Valid {
		result.Valid = false
	}
	result.Errors = append(result.Errors, itemResult.Errors...)
	result.Warnings = append(result.Warnings, itemResult.Warnings...)

	return result
}

// validateSingleItem validates a single JSON object against a YANG schema entry
func (v *Validator) validateSingleItem(data map[string]interface{}, schemaEntry *yang.Entry, yangPath, groupName string, index int) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []*ValidationError{},
		Warnings: []*ValidationError{},
	}

	// Validate required fields
	if schemaEntry != nil {
		v.validateRequiredFields(data, schemaEntry, yangPath, groupName, index, result)
		v.validateFieldTypes(data, schemaEntry, yangPath, groupName, index, result)
		v.validateConstraints(data, schemaEntry, yangPath, groupName, index, result)
	}

	return result
}

// validateRequiredFields checks for required fields in the schema
func (v *Validator) validateRequiredFields(data map[string]interface{}, schemaEntry *yang.Entry, yangPath, groupName string, index int, result *ValidationResult) {
	if schemaEntry == nil || schemaEntry.Dir == nil {
		return
	}

	for name, childEntry := range schemaEntry.Dir {
		// Check if field is mandatory (not optional and not config false)
		if childEntry.Mandatory == yang.TSTrue {
			if _, exists := data[name]; !exists {
				result.Valid = false
				msg := fmt.Sprintf("Required field '%s' is missing", name)
				if index >= 0 {
					msg = fmt.Sprintf("Required field '%s' is missing in item %d", name, index)
				}
				result.Errors = append(result.Errors, &ValidationError{
					GroupName: groupName,
					Path:      yangPath,
					Field:     name,
					Message:   msg,
				})
			}
		}
	}
}

// validateFieldTypes validates field types against the schema
func (v *Validator) validateFieldTypes(data map[string]interface{}, schemaEntry *yang.Entry, yangPath, groupName string, index int, result *ValidationResult) {
	if schemaEntry == nil || schemaEntry.Dir == nil {
		return
	}

	for name, value := range data {
		childEntry, exists := schemaEntry.Dir[name]
		if !exists {
			// Field not in schema - this might be okay if schema allows additional fields
			// For now, we'll just warn
			result.Warnings = append(result.Warnings, &ValidationError{
				GroupName: groupName,
				Path:      yangPath,
				Field:     name,
				Message:   fmt.Sprintf("Field '%s' not defined in YANG schema", name),
			})
			continue
		}

		// Validate type
		if childEntry.Type != nil {
			// Handle leaf-list: validate each element individually
			if slice, ok := value.([]interface{}); ok {
				for i, elem := range slice {
					if !v.validateValueType(elem, childEntry.Type, childEntry) {
						result.Valid = false
						actualType := fmt.Sprintf("%T", elem)
						msg := fmt.Sprintf("Field '%s' leaf-list element [%d] has invalid type %s, expected %s", name, i, actualType, childEntry.Type.Name)
						if index >= 0 {
							msg = fmt.Sprintf("Field '%s' leaf-list element [%d] has invalid type %s in item %d, expected %s", name, i, actualType, index, childEntry.Type.Name)
						}
						result.Errors = append(result.Errors, &ValidationError{
							GroupName: groupName,
							Path:      yangPath,
							Field:     name,
							Message:   msg,
						})
					}
				}
			} else if !v.validateValueType(value, childEntry.Type, childEntry) {
				result.Valid = false
				// Provide more detailed error message with actual type
				actualType := fmt.Sprintf("%T", value)
				msg := fmt.Sprintf("Field '%s' has invalid type %s, expected %s", name, actualType, childEntry.Type.Name)
				if index >= 0 {
					msg = fmt.Sprintf("Field '%s' has invalid type %s in item %d, expected %s", name, actualType, index, childEntry.Type.Name)
				}
				result.Errors = append(result.Errors, &ValidationError{
					GroupName: groupName,
					Path:      yangPath,
					Field:     name,
					Message:   msg,
				})
			}
		}

		// Recursively validate nested objects
		if childEntry.Dir != nil {
			if nestedMap, ok := value.(map[string]interface{}); ok {
				nestedResult := v.validateSingleItem(nestedMap, childEntry, yangPath+"/"+name, groupName, index)
				if !nestedResult.Valid {
					result.Valid = false
				}
				result.Errors = append(result.Errors, nestedResult.Errors...)
				result.Warnings = append(result.Warnings, nestedResult.Warnings...)
			} else if nestedArray, ok := value.([]interface{}); ok && childEntry.ListAttr != nil {
				// Validate list items
				for i, item := range nestedArray {
					if itemMap, ok := item.(map[string]interface{}); ok {
						itemResult := v.validateSingleItem(itemMap, childEntry, yangPath+"/"+name, groupName, i)
						if !itemResult.Valid {
							result.Valid = false
						}
						result.Errors = append(result.Errors, itemResult.Errors...)
						result.Warnings = append(result.Warnings, itemResult.Warnings...)
					}
				}
			}
		}
	}
}

// validateValueType checks if a value matches the expected YANG type
func (v *Validator) validateValueType(value interface{}, yangType *yang.YangType, entry *yang.Entry) bool {
	if yangType == nil {
		return true
	}

	switch yangType.Kind {
	case yang.Ystring:
		_, ok := value.(string)
		return ok
	case yang.Yint8, yang.Yint16, yang.Yint32, yang.Yint64:
		switch value.(type) {
		case int, int8, int16, int32, int64, float64:
			return true
		}
		return false
	case yang.Yuint8, yang.Yuint16, yang.Yuint32, yang.Yuint64:
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float64:
			// Check if it's non-negative
			if f, ok := value.(float64); ok {
				return f >= 0
			}
			return true
		}
		return false
	case yang.Ybool:
		_, ok := value.(bool)
		return ok
	case yang.Ydecimal64:
		// decimal64 can represent both integers and floating point numbers
		// Accept both int and float64 types, and try to convert strings
		switch v := value.(type) {
		case float64, float32:
			return true
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		case string:
			// Try to parse string as float64
			_, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			return err == nil
		}
		return false
	case yang.Yenum, yang.Yidentityref:
		// Enum values should be strings
		_, ok := value.(string)
		return ok
	case yang.Yleafref:
		// Leafref can be various types depending on target
		return true
	case yang.Yempty:
		// Empty type - value should be null or empty
		return value == nil
	case yang.Yunion:
		// Union type - check if value matches any of the union types
		if yangType.Type != nil {
			for _, unionType := range yangType.Type {
				if v.validateValueType(value, unionType, entry) {
					return true
				}
			}
		}
		return false
	default:
		// For unknown types, be permissive
		return true
	}
}

// validateConstraints validates YANG constraints (ranges, patterns, etc.)
func (v *Validator) validateConstraints(data map[string]interface{}, schemaEntry *yang.Entry, yangPath, groupName string, index int, result *ValidationResult) {
	if schemaEntry == nil || schemaEntry.Dir == nil {
		return
	}

	for name, value := range data {
		childEntry, exists := schemaEntry.Dir[name]
		if !exists || childEntry.Type == nil {
			continue
		}

		// Validate range constraints for numeric types
		if childEntry.Type.Range != nil {
			if slice, ok := value.([]interface{}); ok {
				// Validate range for each leaf-list element
				for i, elem := range slice {
					if !v.validateRange(elem, childEntry.Type.Range) {
						result.Valid = false
						msg := fmt.Sprintf("Field '%s' leaf-list element [%d] value is out of range", name, i)
						if index >= 0 {
							msg = fmt.Sprintf("Field '%s' leaf-list element [%d] value is out of range in item %d", name, i, index)
						}
						result.Errors = append(result.Errors, &ValidationError{
							GroupName: groupName,
							Path:      yangPath,
							Field:     name,
							Message:   msg,
						})
					}
				}
			} else if !v.validateRange(value, childEntry.Type.Range) {
				result.Valid = false
				msg := fmt.Sprintf("Field '%s' value is out of range", name)
				if index >= 0 {
					msg = fmt.Sprintf("Field '%s' value is out of range in item %d", name, index)
				}
				result.Errors = append(result.Errors, &ValidationError{
					GroupName: groupName,
					Path:      yangPath,
					Field:     name,
					Message:   msg,
				})
			}
		}

		// Validate length constraints for string types
		if childEntry.Type.Length != nil {
			if slice, ok := value.([]interface{}); ok {
				for i, elem := range slice {
					if strElem, ok := elem.(string); ok {
						if !v.validateLength(strElem, childEntry.Type.Length) {
							result.Valid = false
							msg := fmt.Sprintf("Field '%s' leaf-list element [%d] length is out of range", name, i)
							if index >= 0 {
								msg = fmt.Sprintf("Field '%s' leaf-list element [%d] length is out of range in item %d", name, i, index)
							}
							result.Errors = append(result.Errors, &ValidationError{
								GroupName: groupName,
								Path:      yangPath,
								Field:     name,
								Message:   msg,
							})
						}
					}
				}
			} else if strValue, ok := value.(string); ok {
				if !v.validateLength(strValue, childEntry.Type.Length) {
					result.Valid = false
					msg := fmt.Sprintf("Field '%s' length is out of range", name)
					if index >= 0 {
						msg = fmt.Sprintf("Field '%s' length is out of range in item %d", name, index)
					}
					result.Errors = append(result.Errors, &ValidationError{
						GroupName: groupName,
						Path:      yangPath,
						Field:     name,
						Message:   msg,
					})
				}
			}
		}

		// Validate pattern constraints for string types
		if childEntry.Type.Pattern != nil && len(childEntry.Type.Pattern) > 0 {
			// Pattern validation would require regex matching
			// For now, we'll skip this as it requires more complex implementation
			// TODO: Implement pattern validation
		}
	}
}

// validateRange validates a numeric value against a YANG range constraint
func (v *Validator) validateRange(value interface{}, ranges yang.YangRange) bool {
	// Convert value to float64 for comparison
	var numValue float64
	switch val := value.(type) {
	case int:
		numValue = float64(val)
	case int8:
		numValue = float64(val)
	case int16:
		numValue = float64(val)
	case int32:
		numValue = float64(val)
	case int64:
		numValue = float64(val)
	case uint:
		numValue = float64(val)
	case uint8:
		numValue = float64(val)
	case uint16:
		numValue = float64(val)
	case uint32:
		numValue = float64(val)
	case uint64:
		numValue = float64(val)
	case float32:
		numValue = float64(val)
	case float64:
		numValue = val
	case string:
		// Try to parse string as float64
		parsed, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return false
		}
		numValue = parsed
	default:
		return false
	}

	// Check if value is within any of the ranges
	for _, r := range ranges {
		// Convert Number to float64 via string parsing
		minStr := r.Min.String()
		maxStr := r.Max.String()
		var min, max float64
		if _, err := fmt.Sscanf(minStr, "%f", &min); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(maxStr, "%f", &max); err != nil {
			continue
		}
		if numValue >= min && numValue <= max {
			return true
		}
	}
	return false
}

// validateLength validates a string length against a YANG length constraint
func (v *Validator) validateLength(value string, lengths yang.YangRange) bool {
	length := float64(len(value))
	for _, l := range lengths {
		// Convert Number to float64 via string parsing
		minStr := l.Min.String()
		maxStr := l.Max.String()
		var min, max float64
		if _, err := fmt.Sscanf(minStr, "%f", &min); err != nil {
			continue
		}
		if _, err := fmt.Sscanf(maxStr, "%f", &max); err != nil {
			continue
		}
		if length >= min && length <= max {
			return true
		}
	}
	return false
}

