package variable

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loader loads variables from various formats
type Loader struct{}

// NewLoader creates a new variable loader
func NewLoader() *Loader {
	return &Loader{}
}

// Load loads variables based on format
func (l *Loader) Load(data string, format string) (map[string]interface{}, error) {
	switch format {
	case "python":
		return l.LoadPython(data)
	case "yaml":
		return l.LoadYAML(data)
	case "json":
		return l.LoadJSON(data)
	case "csv":
		return l.LoadCSV(data, "")
	case "ini":
		return l.LoadINI(data)
	default:
		return nil, fmt.Errorf("unsupported variable format: %s", format)
	}
}

// LoadPython loads Python-formatted variables
func (l *Loader) LoadPython(data string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	// Split by lines but handle multi-line dictionaries
	lines := strings.Split(data, "\n")
	
	var currentKey string
	var currentValue strings.Builder
	var inDict bool
	var braceDepth int
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Check if we're in a multi-line dictionary
		if inDict {
			currentValue.WriteString("\n")
			currentValue.WriteString(line)
			
			// Count braces to track dictionary depth
			for _, char := range line {
				if char == '{' {
					braceDepth++
				} else if char == '}' {
					braceDepth--
					if braceDepth == 0 {
						// End of dictionary
						inDict = false
						// Parse the dictionary value
						dictStr := currentValue.String()
						parsed, err := l.parsePythonValue(dictStr)
						if err == nil {
							result[currentKey] = parsed
						}
						currentKey = ""
						currentValue.Reset()
						continue
					}
				}
			}
			continue
		}
		
		// Try to parse key = value
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				// Check if value starts with { (dictionary)
				if strings.HasPrefix(value, "{") {
					// Multi-line dictionary
					currentKey = key
					currentValue.WriteString(value)
					inDict = true
					braceDepth = strings.Count(value, "{") - strings.Count(value, "}")
					
					// Check if dictionary closes on same line
					if braceDepth == 0 {
						// Single-line dictionary
						parsed, err := l.parsePythonValue(value)
						if err == nil {
							result[key] = parsed
						} else {
							// Fallback to string
							result[key] = value
						}
						inDict = false
						currentValue.Reset()
					}
				} else {
					// Simple value
					parsed, err := l.parsePythonValue(value)
					if err == nil {
						result[key] = parsed
					} else {
						// Remove quotes if present and use as string
						value = strings.Trim(value, `"'`)
						result[key] = value
					}
				}
			}
		}
	}
	
	return result, nil
}

// parsePythonValue parses a Python value (dict, list, bool, number, string)
func (l *Loader) parsePythonValue(value string) (interface{}, error) {
	value = strings.TrimSpace(value)
	
	// Handle Python booleans and None
	if value == "True" || value == "true" {
		return true, nil
	}
	if value == "False" || value == "false" {
		return false, nil
	}
	if value == "None" || value == "none" || value == "null" {
		return nil, nil
	}
	
	// Handle dictionaries (convert Python dict syntax to JSON-like)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		// Convert Python dict to JSON-like format
		jsonStr := l.pythonDictToJSON(value)
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			return result, nil
		}
		return nil, fmt.Errorf("failed to parse dictionary")
	}
	
	// Handle lists
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		// Convert Python list to JSON-like format
		jsonStr := l.pythonListToJSON(value)
		var result []interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			return result, nil
		}
		return nil, fmt.Errorf("failed to parse list")
	}
	
	// Try to parse as number
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal, nil
	}
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal, nil
	}
	
	// Remove quotes if present
	value = strings.Trim(value, `"'`)
	return value, nil
}

// pythonDictToJSON converts Python dict syntax to JSON
func (l *Loader) pythonDictToJSON(pythonDict string) string {
	// Replace Python booleans and None
	result := pythonDict
	result = strings.ReplaceAll(result, "True", "true")
	result = strings.ReplaceAll(result, "False", "false")
	result = strings.ReplaceAll(result, "None", "null")
	
	// Python allows single quotes for strings, JSON requires double quotes
	// But we need to be careful not to replace quotes inside strings
	// Simple approach: replace single quotes with double quotes (works for most cases)
	// This is a simplified approach - for production, use a proper parser
	result = strings.ReplaceAll(result, "'", `"`)
	
	return result
}

// pythonListToJSON converts Python list syntax to JSON
func (l *Loader) pythonListToJSON(pythonList string) string {
	// Replace Python booleans and None
	result := pythonList
	result = strings.ReplaceAll(result, "True", "true")
	result = strings.ReplaceAll(result, "False", "false")
	result = strings.ReplaceAll(result, "None", "null")
	
	// Replace single quotes with double quotes
	result = strings.ReplaceAll(result, "'", `"`)
	
	return result
}

// LoadYAML loads YAML-formatted variables
func (l *Loader) LoadYAML(data string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return result, nil
}

// LoadJSON loads JSON-formatted variables
func (l *Loader) LoadJSON(data string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return result, nil
}

// LoadCSV loads CSV-formatted variables
func (l *Loader) LoadCSV(data string, keyColumn string) (map[string]interface{}, error) {
	reader := csv.NewReader(strings.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return make(map[string]interface{}), nil
	}

	// First row is headers
	headers := records[0]
	result := make(map[string]interface{})

	// If key column specified, create map
	if keyColumn != "" {
		keyIndex := -1
		for i, h := range headers {
			if h == keyColumn {
				keyIndex = i
				break
			}
		}
		if keyIndex == -1 {
			return nil, fmt.Errorf("key column %s not found", keyColumn)
		}

		for i := 1; i < len(records); i++ {
			if keyIndex < len(records[i]) {
				key := records[i][keyIndex]
				item := make(map[string]interface{})
				for j, header := range headers {
					if j < len(records[i]) {
						item[header] = records[i][j]
					}
				}
				result[key] = item
			}
		}
	} else {
		// Convert to list of maps
		items := make([]map[string]interface{}, 0, len(records)-1)
		for i := 1; i < len(records); i++ {
			item := make(map[string]interface{})
			for j, header := range headers {
				if j < len(records[i]) {
					item[header] = records[i][j]
				}
			}
			items = append(items, item)
		}
		result["_items_"] = items
	}

	return result, nil
}

// LoadINI loads INI-formatted variables
func (l *Loader) LoadINI(data string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	currentSection := ""
	
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		
		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.Trim(line, "[]")
			result[currentSection] = make(map[string]interface{})
			continue
		}
		
		// Key=value or Key:value (INI format supports both)
		// Check for : first (more common in INI), then =
		var parts []string
		if strings.Contains(line, ":") {
			parts = strings.SplitN(line, ":", 2)
		} else if strings.Contains(line, "=") {
			parts = strings.SplitN(line, "=", 2)
		}
		
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			
			if currentSection != "" {
				if section, ok := result[currentSection].(map[string]interface{}); ok {
					section[key] = value
				}
			} else {
				result[key] = value
			}
		}
	}
	
	return result, nil
}

// LoadFromFile loads variables from a file
func (l *Loader) LoadFromFile(path string, format string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return l.Load(string(data), format)
}

// LoadFromReader loads variables from an io.Reader
func (l *Loader) LoadFromReader(reader io.Reader, format string) (map[string]interface{}, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read from reader: %w", err)
	}
	return l.Load(string(data), format)
}

