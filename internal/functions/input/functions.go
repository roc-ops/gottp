package input

import (
	"fmt"
	"regexp"
	"strings"
)

// Function represents an input function
// Returns: (processedData, shouldContinue, error)
// shouldContinue: false means stop processing further functions (condition failed)
type Function func(data string, args []string, kwargs map[string]interface{}) (string, bool, error)

// Registry holds all registered input functions
type Registry struct {
	functions map[string]Function
}

// NewRegistry creates a new input function registry
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

// registerBuiltins registers all built-in input functions
func (r *Registry) registerBuiltins() {
	r.Register("extract_commands", extractCommands)
	r.Register("test", test)
}

// extractCommands extracts command output from text data
// Usage: extract_commands("show interfaces", "show version")
func extractCommands(data string, args []string, kwargs map[string]interface{}) (string, bool, error) {
	if len(args) == 0 {
		return data, true, nil
	}

	// Get hostname from data (simplified - look for hostname pattern)
	// In Python TTP, this uses gethostname function
	hostname := getHostname(data)
	if hostname == "" {
		// No hostname found, return original data
		return data, true, nil
	}

	var result strings.Builder
	for _, command := range args {
		// Build regex pattern: hostname[#>] *command *\n([\S\s]+?)(?=hostname[#>]|$)
		pattern := fmt.Sprintf(`%s[#>] *%s *\n([\S\s]+?)(?=%s[#>]|$)`, 
			regexp.QuoteMeta(hostname), 
			regexp.QuoteMeta(command), 
			regexp.QuoteMeta(hostname))
		
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue // Skip invalid pattern
		}

		match := re.FindStringSubmatch(data)
		if len(match) > 1 {
			result.WriteString("\n")
			result.WriteString(match[1])
			result.WriteString("\n")
		}
	}

	if result.Len() > 0 {
		return result.String(), true, nil
	}

	return data, true, nil
}

// getHostname extracts hostname from data
// Looks for patterns like "hostname#", "hostname>", etc.
func getHostname(data string) string {
	// Try to find hostname pattern: word characters followed by # or >
	re := regexp.MustCompile(`(\w+)[#>]`)
	matches := re.FindStringSubmatch(data)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// test is a test function that prints information about the input data
func test(data string, args []string, kwargs map[string]interface{}) (string, bool, error) {
	fmt.Printf("Running input test function, data length %d symbols\n", len(data))
	return data, true, nil
}

