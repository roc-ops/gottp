package formatters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// PPrintFormatter formats results in a pretty-printed, human-readable format
// Similar to Python's pprint module
type PPrintFormatter struct{}

// NewPPrintFormatter creates a new pprint formatter
func NewPPrintFormatter() *PPrintFormatter {
	return &PPrintFormatter{}
}

// Format formats data in a pretty-printed format
func (f *PPrintFormatter) Format(data interface{}) ([]byte, error) {
	result := f.formatValue(data, 0, "")
	return []byte(result), nil
}

// FormatString formats data as a pretty-printed string
func (f *PPrintFormatter) FormatString(data interface{}) (string, error) {
	return f.formatValue(data, 0, ""), nil
}

// formatValue recursively formats a value with indentation
func (f *PPrintFormatter) formatValue(value interface{}, indent int, prefix string) string {
	indentStr := strings.Repeat("  ", indent)
	
	switch v := value.(type) {
	case nil:
		return "None"
	case bool:
		if v {
			return "True"
		}
		return "False"
	case string:
		return fmt.Sprintf("'%s'", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", v)
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case []interface{}:
		if len(v) == 0 {
			return "[]"
		}
		var buf bytes.Buffer
		buf.WriteString("[\n")
		for i, item := range v {
			itemStr := f.formatValue(item, indent+1, "")
			buf.WriteString(indentStr + "  " + itemStr)
			if i < len(v)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		buf.WriteString(indentStr + "]")
		return buf.String()
	case map[string]interface{}:
		if len(v) == 0 {
			return "{}"
		}
		var buf bytes.Buffer
		buf.WriteString("{\n")
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		// Sort keys for consistent output
		for i := 0; i < len(keys)-1; i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		for i, k := range keys {
			keyStr := fmt.Sprintf("'%s'", k)
			valStr := f.formatValue(v[k], indent+1, "")
			buf.WriteString(indentStr + "  " + keyStr + ": " + valStr)
			if i < len(keys)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		buf.WriteString(indentStr + "}")
		return buf.String()
	default:
		// For unknown types, try JSON encoding as fallback
		jsonBytes, err := json.MarshalIndent(value, indentStr+"  ", "  ")
		if err == nil {
			return string(jsonBytes)
		}
		return fmt.Sprintf("%v", value)
	}
}

