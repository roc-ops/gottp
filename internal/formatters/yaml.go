package formatters

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// YAMLFormatter formats results as YAML
type YAMLFormatter struct{}

// NewYAMLFormatter creates a new YAML formatter
func NewYAMLFormatter() *YAMLFormatter {
	return &YAMLFormatter{}
}

// Format formats data as YAML
func (f *YAMLFormatter) Format(data interface{}) ([]byte, error) {
	return yaml.Marshal(data)
}

// FormatString formats data as YAML string
func (f *YAMLFormatter) FormatString(data interface{}) (string, error) {
	bytes, err := f.Format(data)
	if err != nil {
		return "", fmt.Errorf("failed to format YAML: %w", err)
	}
	return string(bytes), nil
}

