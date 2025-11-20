package compiled

import (
	"fmt"
	"regexp"
	"strings"
)

// ResultManager manages result formation and path handling
type ResultManager struct {
	results map[string]interface{}
	pathChar string
}

// NewResultManager creates a new result manager
func NewResultManager(pathChar string) *ResultManager {
	if pathChar == "" {
		pathChar = "."
	}
	return &ResultManager{
		results:  make(map[string]interface{}),
		pathChar: pathChar,
	}
}

// AddResult adds a result at the specified path
func (rm *ResultManager) AddResult(path string, value interface{}) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Split path by pathChar
	parts := strings.Split(path, rm.pathChar)
	
	// Navigate/create the nested structure
	current := rm.results
	for i, part := range parts[:len(parts)-1] {
		// Handle dynamic paths with asterisks
		part = strings.TrimSuffix(part, "*")
		
		if _, exists := current[part]; !exists {
			current[part] = make(map[string]interface{})
		}
		
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return fmt.Errorf("path conflict at %s: expected map, got %T", strings.Join(parts[:i+1], rm.pathChar), current[part])
		}
		current = next
	}
	
	// Set the final value
	finalKey := parts[len(parts)-1]
	finalKey = strings.TrimSuffix(finalKey, "*")
	
	// Handle list paths (ending with *)
	if strings.HasSuffix(parts[len(parts)-1], "*") {
		// Append to list
		if list, ok := current[finalKey].([]interface{}); ok {
			current[finalKey] = append(list, value)
		} else {
			current[finalKey] = []interface{}{value}
		}
	} else {
		current[finalKey] = value
	}
	
	return nil
}

// GetResults returns all results
func (rm *ResultManager) GetResults() map[string]interface{} {
	return rm.results
}

// FormPath forms a dynamic path by replacing variables
func (rm *ResultManager) FormPath(pathTemplate string, vars map[string]interface{}) (string, error) {
	result := pathTemplate
	
	// Find all {{ variable }} patterns
	re := regexp.MustCompile(`\{\{\s*(\S+)\s*\}\}`)
	matches := re.FindAllStringSubmatch(pathTemplate, -1)
	
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		varName := match[1]
		
		// Get variable value
		value, ok := vars[varName]
		if !ok {
			return "", fmt.Errorf("variable %s not found for path", varName)
		}
		
		// Replace in template
		result = strings.ReplaceAll(result, match[0], fmt.Sprintf("%v", value))
	}
	
	return result, nil
}

