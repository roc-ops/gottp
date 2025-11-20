package formatters

import (
	"fmt"
)

// RawFormatter formats results as raw text
type RawFormatter struct{}

// NewRawFormatter creates a new raw formatter
func NewRawFormatter() *RawFormatter {
	return &RawFormatter{}
}

// Format formats data as raw string
func (f *RawFormatter) Format(data interface{}) ([]byte, error) {
	return []byte(fmt.Sprintf("%v", data)), nil
}

// FormatString formats data as raw string
func (f *RawFormatter) FormatString(data interface{}) (string, error) {
	return fmt.Sprintf("%v", data), nil
}

