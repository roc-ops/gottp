package formatters

import (
	"encoding/json"
	"fmt"
)

// JSONFormatter formats results as JSON
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Format formats data as JSON
func (f *JSONFormatter) Format(data interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

// FormatString formats data as JSON string
func (f *JSONFormatter) FormatString(data interface{}) (string, error) {
	bytes, err := f.Format(data)
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}
	return string(bytes), nil
}

