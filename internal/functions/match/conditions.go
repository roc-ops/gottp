package match

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ConditionResult represents the result of a condition function
// If Condition is false, the match should be rejected
type ConditionResult struct {
	Value     interface{}
	Condition bool
}

// IsConditionFunction checks if a function name is a condition function
func IsConditionFunction(name string) bool {
	conditionFunctions := map[string]bool{
		"contains":         true,
		"contains_re":     true,
		"startswith":      true,
		"startswith_re":   true,
		"endswith":        true,
		"endswith_re":     true,
		"notstartswith":   true,
		"notstartswith_re": true,
		"notendswith":     true,
		"notendswith_re":   true,
		"exclude":         true,
		"exclude_re":      true,
		"equal":           true,
		"notequal":        true,
		"isdigit":         true,
		"notdigit":        true,
		"greaterthan":     true,
		"lessthan":        true,
		"is_ip":           true,
		"cidr_match":      true,
	}
	return conditionFunctions[name]
}

// RegisterConditionFunctions registers all condition functions
func (r *Registry) RegisterConditionFunctions() {
	r.Register("contains", contains)
	r.Register("contains_re", containsRe)
	r.Register("startswith", startswith)
	r.Register("startswith_re", startswithRe)
	r.Register("endswith", endswith)
	r.Register("endswith_re", endswithRe)
	r.Register("notstartswith", notstartswith)
	r.Register("notstartswith_re", notstartswithRe)
	r.Register("notendswith", notendswith)
	r.Register("notendswith_re", notendswithRe)
	r.Register("exclude", exclude)
	r.Register("exclude_re", excludeRe)
	r.Register("equal", equal)
	r.Register("notequal", notequal)
	r.Register("isdigit", isdigit)
	r.Register("notdigit", notdigit)
	r.Register("greaterthan", greaterthan)
	r.Register("lessthan", lessthan)
	r.Register("is_ip", isIP)
	r.Register("cidr_match", cidrMatch)
}

// contains checks if value contains any of the given patterns
func contains(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	// Check if any pattern is contained in the string
	for _, pattern := range args {
		if strings.Contains(str, pattern) {
			return &ConditionResult{Value: value, Condition: true}, nil
		}
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// containsRe checks if value contains a regex pattern
func containsRe(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	pattern := args[0]
	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	if re.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// startswith checks if value starts with the given prefix (non-regex)
func startswith(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	prefix := args[0]
	if strings.HasPrefix(str, prefix) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// startswithRe checks if value starts with a regex pattern
func startswithRe(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	pattern := args[0]
	re, err := regexp.Compile("^" + pattern)
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	if re.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// endswith checks if value ends with the given suffix (non-regex)
func endswith(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	suffix := args[0]
	if strings.HasSuffix(str, suffix) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// endswithRe checks if value ends with a regex pattern
func endswithRe(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	pattern := args[0]
	re, err := regexp.Compile(pattern + "$")
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	if re.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// notstartswith checks if value does NOT start with the given prefix (non-regex)
func notstartswith(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	prefix := args[0]
	if !strings.HasPrefix(str, prefix) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// notstartswithRe checks if value does NOT start with a regex pattern
func notstartswithRe(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	pattern := args[0]
	re, err := regexp.Compile("^" + pattern)
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	if !re.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// notendswith checks if value does NOT end with the given suffix (non-regex)
func notendswith(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	suffix := args[0]
	if !strings.HasSuffix(str, suffix) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// notendswithRe checks if value does NOT end with a regex pattern
func notendswithRe(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	pattern := args[0]
	re, err := regexp.Compile(pattern + "$")
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	if !re.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// exclude checks if value does NOT contain the pattern
func exclude(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	pattern := args[0]
	if !strings.Contains(str, pattern) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// excludeRe checks if value does NOT match a regex pattern
func excludeRe(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	pattern := args[0]
	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	if !re.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// equal checks if value equals the given value
func equal(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	expected := args[0]
	actual := fmt.Sprintf("%v", value)
	
	if actual == expected {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// notequal checks if value does NOT equal the given value
func notequal(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	expected := args[0]
	actual := fmt.Sprintf("%v", value)
	
	if actual != expected {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// isdigit checks if value is a digit string (all characters are digits)
func isdigit(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))
	
	if str == "" {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	// Check if all characters are digits
	digitRegex := regexp.MustCompile(`^\d+$`)
	if digitRegex.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// notdigit checks if value is NOT a digit string
func notdigit(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))
	
	if str == "" {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	// Check if all characters are digits
	digitRegex := regexp.MustCompile(`^\d+$`)
	if !digitRegex.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// greaterthan checks if value is greater than the given value (numeric comparison)
func greaterthan(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	// Convert value to float
	valStr := strings.TrimSpace(fmt.Sprintf("%v", value))
	valFloat, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	// Convert threshold to float
	thresholdStr := strings.TrimSpace(args[0])
	thresholdFloat, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	if valFloat > thresholdFloat {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// lessthan checks if value is less than the given value (numeric comparison)
func lessthan(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	// Convert value to float
	valStr := strings.TrimSpace(fmt.Sprintf("%v", value))
	valFloat, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	// Convert threshold to float
	thresholdStr := strings.TrimSpace(args[0])
	thresholdFloat, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	if valFloat < thresholdFloat {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// isIP checks if value is a valid IP address
func isIP(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))
	
	// Basic IPv4 validation
	ipv4Regex := regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`)
	if ipv4Regex.MatchString(str) {
		// Validate each octet is 0-255
		parts := strings.Split(str, ".")
		valid := true
		for _, part := range parts {
			octet, err := strconv.Atoi(part)
			if err != nil || octet < 0 || octet > 255 {
				valid = false
				break
			}
		}
		if valid {
			return &ConditionResult{Value: value, Condition: true}, nil
		}
	}
	
	// Basic IPv6 validation (simplified)
	ipv6Regex := regexp.MustCompile(`^(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$|^::1$|^::$`)
	if ipv6Regex.MatchString(str) {
		return &ConditionResult{Value: value, Condition: true}, nil
	}
	
	return &ConditionResult{Value: value, Condition: false}, nil
}

// cidrMatch checks if IP address overlaps with given CIDR prefix
func cidrMatch(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) == 0 {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	ipStr := strings.TrimSpace(fmt.Sprintf("%v", value))
	cidrStr := strings.TrimSpace(args[0])
	
	// Parse CIDR (e.g., "192.168.0.0/24")
	if !strings.Contains(cidrStr, "/") {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	parts := strings.Split(cidrStr, "/")
	if len(parts) != 2 {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	networkStr := parts[0]
	prefixLen, err := strconv.Atoi(parts[1])
	if err != nil || prefixLen < 0 || prefixLen > 32 {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	// Simple check: if IP starts with the same network prefix
	// This is a simplified implementation - for full CIDR matching,
	// you'd need to convert IPs to integers and check bitwise
	
	// For now, do a simple prefix match on the IP string
	// This works for simple cases but not perfect CIDR matching
	networkParts := strings.Split(networkStr, ".")
	ipParts := strings.Split(ipStr, ".")
	
	if len(networkParts) != 4 || len(ipParts) != 4 {
		return &ConditionResult{Value: value, Condition: false}, nil
	}
	
	// Check if IP is in the same network (simplified)
	// Calculate how many octets to check based on prefix length
	octetsToCheck := prefixLen / 8
	bitsInLastOctet := prefixLen % 8
	
	// Check full octets
	for i := 0; i < octetsToCheck && i < 4; i++ {
		if networkParts[i] != ipParts[i] {
			return &ConditionResult{Value: value, Condition: false}, nil
		}
	}
	
	// Check partial octet if needed
	if bitsInLastOctet > 0 && octetsToCheck < 4 {
		networkOctet, err1 := strconv.Atoi(networkParts[octetsToCheck])
		ipOctet, err2 := strconv.Atoi(ipParts[octetsToCheck])
		if err1 != nil || err2 != nil {
			return &ConditionResult{Value: value, Condition: false}, nil
		}
		
		// Create mask for the last octet
		mask := (0xFF << (8 - bitsInLastOctet)) & 0xFF
		if (networkOctet & mask) != (ipOctet & mask) {
			return &ConditionResult{Value: value, Condition: false}, nil
		}
	}
	
	return &ConditionResult{Value: value, Condition: true}, nil
}

