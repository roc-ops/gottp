package match

import (
	"fmt"
	"regexp"
	"strings"
)

// Cached compiled regexes for performance
var (
	ipRegex = regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}(?:\*)?$`)
)

// Function represents a match function
type Function func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)

// Registry holds all registered match functions
type Registry struct {
	functions map[string]Function
}

// NewRegistry creates a new function registry
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

// registerBuiltins registers all built-in match functions
func (r *Registry) registerBuiltins() {
	r.Register("upper", upper)
	r.Register("lower", lower)
	r.Register("strip", strip)
	r.Register("split", split)
	r.Register("join", join)
	r.Register("replace", replace)
	r.Register("IP", ip)
	r.Register("mac_eui", macEUI)

	// Register additional functions
	r.registerMoreFunctions()

	// Register condition functions
	r.RegisterConditionFunctions()
}

// upper converts string to uppercase
func upper(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	return strings.ToUpper(str), nil
}

// lower converts string to lowercase
func lower(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	return strings.ToLower(str), nil
}

// strip removes leading and trailing whitespace
func strip(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	return strings.TrimSpace(str), nil
}

// split splits a string by delimiter
func split(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	delim := ","
	if len(args) > 0 {
		delim = args[0]
	}
	parts := strings.Split(str, delim)
	// Return as slice
	result := make([]interface{}, len(parts))
	for i, part := range parts {
		result[i] = strings.TrimSpace(part)
	}
	return result, nil
}

// join joins a slice of strings
func join(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	delim := ","
	if len(args) > 0 {
		delim = args[0]
	}

	// Convert value to slice
	var parts []string
	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
	case []string:
		parts = v
	default:
		return fmt.Sprintf("%v", value), nil
	}

	return strings.Join(parts, delim), nil
}

// replace replaces occurrences of old with new
func replace(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	if len(args) < 2 {
		return str, nil
	}
	old := args[0]
	new := args[1]
	return strings.ReplaceAll(str, old, new), nil
}

// ip validates and normalizes IP address
func ip(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))
	// Use cached compiled regex for better performance
	if ipRegex.MatchString(str) {
		return str, nil
	}
	return str, nil // Return as-is if not valid IP
}

// macEUI converts MAC address to EUI format (matching Python TTP behavior)
// Python TTP: removes delimiters, converts to lowercase, formats with colons
// Example: "0007.1122.7a73" -> "00:07:11:22:7a:73"
func macEUI(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))

	// Optimize separator removal: use a single pass with strings.Map
	// This is more efficient than multiple ReplaceAll calls
	var buf strings.Builder
	buf.Grow(len(str)) // Pre-allocate capacity

	for _, r := range str {
		if r != ':' && r != '-' && r != '.' {
			buf.WriteRune(r)
		}
	}
	str = buf.String()

	// Convert to lowercase (matching Python TTP behavior)
	str = strings.ToLower(str)

	// Validate: should only contain letters and numbers
	if !isAlphanumeric(str) {
		return value, nil // Return original if not alphanumeric
	}

	// Pad with zeros if length is less than 12 (handles trailing zeros stripped by devices)
	if len(str) < 12 {
		str = str + strings.Repeat("0", 12-len(str))
	}

	// Convert to EUI format with colons (xx:xx:xx:xx:xx:xx) - lowercase
	if len(str) == 12 {
		// Use string concatenation instead of fmt.Sprintf for better performance
		result := str[0:2] + ":" + str[2:4] + ":" + str[4:6] + ":" +
			str[6:8] + ":" + str[8:10] + ":" + str[10:12]
		return result, nil
	}
	return str, nil
}

// isAlphanumeric checks if a string contains only alphanumeric characters
func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}
