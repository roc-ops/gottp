package yang

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

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
	result := v.validate(data, yangPath, groupName)
	result.Warnings = dedupeOnce(result.Warnings)
	return result
}

// dedupeOnce drops repeats of warnings marked once (same field and message).
func dedupeOnce(warnings []*ValidationError) []*ValidationError {
	seen := map[[2]string]bool{}
	out := warnings[:0]
	for _, w := range warnings {
		if w.once {
			key := [2]string{w.Field, w.Message}
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, w)
	}
	return out
}

func (v *Validator) validate(data interface{}, yangPath, groupName string) *ValidationResult {
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
		if e := levelError(d, schemaEntry, yangPath, groupName); e != nil {
			result.Valid = false
			result.Errors = append(result.Errors, e)
		}
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

	if e := levelError([]map[string]interface{}{dataToValidate.(map[string]interface{})}, schemaEntry, yangPath, groupName); e != nil {
		result.Valid = false
		result.Errors = append(result.Errors, e)
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

// validateRequiredFields checks the mandatory nodes of schemaEntry that the
// data tree must hold (RFC 7950 7.6.5, 7.9.4). A choice is not a data node:
// it is satisfied by a node from any one of its cases, nodes from two cases
// are an error, and the mandatory nodes of a case apply only when that case
// is present (yangson behaves the same).
func (v *Validator) validateRequiredFields(data map[string]interface{}, schemaEntry *yang.Entry, yangPath, groupName string, index int, result *ValidationResult) {
	if schemaEntry == nil || schemaEntry.Dir == nil {
		return
	}
	inItem := ""
	if index >= 0 {
		inItem = fmt.Sprintf(" in item %d", index)
	}
	fail := func(field, msg string) {
		result.Valid = false
		result.Errors = append(result.Errors, &ValidationError{
			GroupName: groupName,
			Path:      yangPath,
			Field:     field,
			Message:   msg + inItem,
		})
	}

	var check func(entry *yang.Entry)
	check = func(entry *yang.Entry) {
		for _, name := range sortedChildNames(entry) {
			child := entry.Dir[name]
			if !child.IsChoice() {
				if child.Mandatory == yang.TSTrue {
					if _, exists := data[name]; !exists {
						fail(name, fmt.Sprintf("Required field '%s' is missing", name))
					}
				}
				continue
			}
			var active []*yang.Entry
			var present [][]string
			for _, caseName := range sortedChildNames(child) {
				c := child.Dir[caseName]
				if nodes := presentDataNodes(c, data); len(nodes) > 0 {
					active = append(active, c)
					present = append(present, nodes)
				}
			}
			switch len(active) {
			case 0:
				if child.Mandatory == yang.TSTrue {
					fail(name, fmt.Sprintf("Required choice '%s' is missing: no node from any of its cases (%s) is present",
						name, strings.Join(sortedChildNames(child), ", ")))
				}
			case 1:
				if active[0].IsCase() {
					check(active[0])
				}
			default:
				var cases []string
				for i, c := range active {
					cases = append(cases, fmt.Sprintf("%s (%s)", c.Name, strings.Join(present[i], ", ")))
				}
				fail(name, fmt.Sprintf("Fields of choice '%s' come from more than one case: %s", name, strings.Join(cases, ", ")))
			}
		}
	}
	check(schemaEntry)
}

// presentDataNodes returns the names of the data nodes of entry that data
// holds: entry itself when it is a data node, else the data nodes under it
// (entry being a case, or a node of a case).
func presentDataNodes(entry *yang.Entry, data map[string]interface{}) []string {
	if !entry.IsChoice() && !entry.IsCase() {
		if _, ok := data[entry.Name]; ok {
			return []string{entry.Name}
		}
		return nil
	}
	var names []string
	for _, name := range sortedChildNames(entry) {
		names = append(names, presentDataNodes(entry.Dir[name], data)...)
	}
	return names
}

// sortedChildNames returns the names in entry.Dir, sorted, so errors come
// in a stable order.
func sortedChildNames(entry *yang.Entry) []string {
	names := make([]string, 0, len(entry.Dir))
	for name := range entry.Dir {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// validateFieldTypes validates field types against the schema
func (v *Validator) validateFieldTypes(data map[string]interface{}, schemaEntry *yang.Entry, yangPath, groupName string, index int, result *ValidationResult) {
	if schemaEntry == nil || schemaEntry.Dir == nil {
		return
	}

	for name, value := range data {
		childEntry, exists := lookupChild(schemaEntry, name)
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
						got, expected := describeMismatch(elem, childEntry.Type)
						msg := fmt.Sprintf("Field '%s' leaf-list element [%d] has %s, expected %s", name, i, got, expected)
						if index >= 0 {
							msg = fmt.Sprintf("Field '%s' leaf-list element [%d] has %s in item %d, expected %s", name, i, got, index, expected)
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
				got, expected := describeMismatch(value, childEntry.Type)
				msg := fmt.Sprintf("Field '%s' has %s, expected %s", name, got, expected)
				if index >= 0 {
					msg = fmt.Sprintf("Field '%s' has %s in item %d, expected %s", name, got, index, expected)
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
		s, ok := value.(string)
		if ok && entry != nil && entry.Type != yangType {
			// A union member: the value is of this member type only if it
			// also meets the member's patterns. (A leaf's own patterns are
			// checked, with a specific message, by validateConstraints.)
			_, ok = v.checkPatterns(s, yangType)
		}
		return ok
	case yang.Yint8, yang.Yint16, yang.Yint32, yang.Yint64:
		switch val := value.(type) {
		case int, int8, int16, int32, int64:
			return true
		case float64:
			// RFC 7950 9.2: integer types admit whole numbers only. A
			// whole float64 is what a JSON round trip makes of an int.
			return isWhole(val)
		}
		return false
	case yang.Yuint8, yang.Yuint16, yang.Yuint32, yang.Yuint64:
		switch val := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		case float64:
			return val >= 0 && isWhole(val)
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
	case yang.Yenum:
		// RFC 7950 9.6: the value must be one of the assigned names.
		s, ok := value.(string)
		return ok && yangType.Enum != nil && yangType.Enum.IsDefined(s)
	case yang.Yidentityref:
		s, ok := value.(string)
		return ok && identityrefAccepts(s, yangType.IdentityBase, entry)
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
		childEntry, exists := lookupChild(schemaEntry, name)
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

		// Validate pattern constraints for string types. Union members'
		// patterns are checked by validateValueType.
		if len(childEntry.Type.Pattern) > 0 && childEntry.Type.Kind == yang.Ystring {
			check := func(s string, element int) {
				failed, ok := v.checkPatterns(s, childEntry.Type)
				if ok {
					return
				}
				result.Valid = false
				what := fmt.Sprintf("Field '%s'", name)
				if element >= 0 {
					what = fmt.Sprintf("Field '%s' leaf-list element [%d]", name, element)
				}
				verb := "does not match"
				if failed.invert {
					verb = "matches invert-match"
				}
				msg := fmt.Sprintf("%s value %q %s pattern '%s'", what, s, verb, failed.pattern)
				if index >= 0 {
					msg += fmt.Sprintf(" in item %d", index)
				}
				result.Errors = append(result.Errors, &ValidationError{
					GroupName: groupName,
					Path:      yangPath,
					Field:     name,
					Message:   msg,
				})
			}
			if slice, ok := value.([]interface{}); ok {
				for i, elem := range slice {
					if s, ok := elem.(string); ok {
						check(s, i)
					}
				}
			} else if s, ok := value.(string); ok {
				check(s, -1)
			}
		}

		// A pattern RE2 can't express leaves the value unchecked: say so
		// rather than pass silently. Reported once per Validate call.
		for _, msg := range v.uncheckablePatterns(childEntry.Type) {
			result.Warnings = append(result.Warnings, &ValidationError{
				GroupName: groupName,
				Path:      yangPath,
				Field:     name,
				Message:   fmt.Sprintf("Field '%s' %s; values are not checked against it", name, msg),
				once:      true,
			})
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
	// RFC 7950 9.4.4: the length of a string is its number of characters.
	length := float64(utf8.RuneCountInString(value))
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

// isWhole reports whether f is a finite number with no fractional part.
func isWhole(f float64) bool {
	return !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f)
}

// lookupChild finds the data node called name under entry. choice and case
// nodes are schema-only (RFC 7950 7.9): their children appear in the data
// tree as children of the choice's parent, so the lookup descends through
// them, including nested choices.
func lookupChild(entry *yang.Entry, name string) (*yang.Entry, bool) {
	if entry == nil {
		return nil, false
	}
	if child, ok := entry.Dir[name]; ok && !child.IsChoice() && !child.IsCase() {
		return child, true
	}
	for _, child := range entry.Dir {
		if child.IsChoice() || child.IsCase() {
			if found, ok := lookupChild(child, name); ok {
				return found, true
			}
		}
	}
	return nil, false
}

// describeMismatch describes, for an error message, why value failed
// yangType: what was found and what was expected.
func describeMismatch(value interface{}, yangType *yang.YangType) (got, expected string) {
	got, expected = fmt.Sprintf("invalid type %T", value), yangType.Name
	s, isString := value.(string)
	if !isString {
		return got, expected
	}
	switch yangType.Kind {
	case yang.Yenum:
		if yangType.Enum != nil {
			// List the names in value (declaration) order.
			var names []string
			for _, val := range yangType.Enum.Values() {
				names = append(names, yangType.Enum.Name(val))
			}
			return fmt.Sprintf("invalid value %q", s), "one of " + strings.Join(names, ", ")
		}
	case yang.Yidentityref:
		if base := yangType.IdentityBase; base != nil {
			return fmt.Sprintf("invalid value %q", s), fmt.Sprintf("an identity derived from %s:%s", identityModule(base), base.Name)
		}
	case yang.Yunion:
		return fmt.Sprintf("invalid value %q", s), "a value of one of the " + yangType.Name + " member types"
	}
	return got, expected
}

// identityrefAccepts reports whether value names an identity derived from
// base (RFC 7950 9.10.2; the base itself is not a valid value). The JSON
// encoding is RFC 7951 6.8: "module:identity", where the module prefix may
// be left out only when the identity is defined in the leaf's own module.
func identityrefAccepts(value string, base *yang.Identity, entry *yang.Entry) bool {
	if base == nil {
		return false
	}
	modName, name, qualified := strings.Cut(value, ":")
	if !qualified {
		name, modName = value, ""
		if entry != nil {
			if m, err := entry.InstantiatingModule(); err == nil {
				modName = m
			}
		}
	}
	for _, id := range base.Values {
		if id.Name != name {
			continue
		}
		// An unknown leaf module (modName == "") accepts the bare name.
		if modName == "" || identityModule(id) == modName {
			return true
		}
	}
	return false
}

// identityModule returns the name of the module that defines id; for an
// identity in a submodule, that is the module the submodule belongs to.
func identityModule(id *yang.Identity) string {
	root := yang.RootNode(id)
	if root == nil {
		return ""
	}
	if root.Kind() == "submodule" && root.BelongsTo != nil {
		return root.BelongsTo.Name
	}
	return root.Name
}

// levelError returns an error when items have fields but none of them is
// defined under entry. The path is then at the wrong level (usually the
// container rather than the list each match maps to, or a leaf), and
// nothing would be checked. The error suggests the children of entry that
// do define some of the fields.
func levelError(items []map[string]interface{}, entry *yang.Entry, yangPath, groupName string) *ValidationError {
	fieldSet := map[string]bool{}
	for _, item := range items {
		for name := range item {
			if _, ok := lookupChild(entry, name); ok {
				return nil
			}
			fieldSet[name] = true
		}
	}
	if len(fieldSet) == 0 {
		return nil
	}
	fields := make([]string, 0, len(fieldSet))
	for name := range fieldSet {
		fields = append(fields, name)
	}
	sort.Strings(fields)

	var suggestions []string
	var collect func(e *yang.Entry)
	collect = func(e *yang.Entry) {
		for _, child := range e.Dir {
			if child.IsChoice() || child.IsCase() {
				collect(child)
				continue
			}
			for _, name := range fields {
				if _, ok := lookupChild(child, name); ok {
					suggestions = append(suggestions, strings.TrimRight(yangPath, "/")+"/"+child.Name)
					break
				}
			}
		}
	}
	collect(entry)
	sort.Strings(suggestions)

	msg := fmt.Sprintf("YANG path '%s' defines none of the fields %s; the path must name the node each match maps to (a list, not its container)",
		yangPath, strings.Join(fields, ", "))
	if len(suggestions) > 0 {
		msg += fmt.Sprintf(": did you mean %s?", strings.Join(suggestions, " or "))
	}
	return &ValidationError{GroupName: groupName, Path: yangPath, Message: msg}
}
