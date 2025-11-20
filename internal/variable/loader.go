package variable

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	// For now, we'll use a simple approach
	// In production, we might want to use a Python parser or Starlark
	// For basic Python dict syntax, we can parse it manually
	result := make(map[string]interface{})
	
	// Simple key=value parsing for now
	// TODO: Implement proper Python dict parsing
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Try to parse key = value
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes if present
				value = strings.Trim(value, `"'`)
				result[key] = value
			}
		}
	}
	
	return result, nil
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

