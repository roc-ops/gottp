package formatters

import (
	"encoding/json"
	"testing"
)

func TestJSONFormatter_Format(t *testing.T) {
	formatter := &JSONFormatter{}

	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name:    "nil data",
			data:    nil,
			wantErr: false,
		},
		{
			name:    "string data",
			data:    "test string",
			wantErr: false,
		},
		{
			name:    "map data",
			data:    map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "list data",
			data:    []interface{}{"item1", "item2"},
			wantErr: false,
		},
		{
			name:    "nested data",
			data:    map[string]interface{}{"nested": map[string]interface{}{"key": "value"}},
			wantErr: false,
		},
		{
			name:    "empty map",
			data:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "empty list",
			data:    []interface{}{},
			wantErr: false,
		},
		{
			name:    "mixed types",
			data:    map[string]interface{}{"string": "test", "int": 42, "float": 3.14, "bool": true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONFormatter.Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Verify it's valid JSON
				var jsonData interface{}
				if err := json.Unmarshal(result, &jsonData); err != nil {
					t.Errorf("JSONFormatter.Format() returned invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestJSONFormatter_FormatString(t *testing.T) {
	formatter := &JSONFormatter{}

	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name:    "nil data",
			data:    nil,
			wantErr: false,
		},
		{
			name:    "string data",
			data:    "test string",
			wantErr: false,
		},
		{
			name:    "map data",
			data:    map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "list data",
			data:    []interface{}{"item1", "item2"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.FormatString(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONFormatter.FormatString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != "" {
				// Verify it's valid JSON
				var jsonData interface{}
				if err := json.Unmarshal([]byte(result), &jsonData); err != nil {
					t.Errorf("JSONFormatter.FormatString() returned invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestJSONFormatter_FormatWithOptions(t *testing.T) {
	formatter := &JSONFormatter{}

	tests := []struct {
		name    string
		data    interface{}
		options map[string]interface{}
		wantErr bool
	}{
		{
			name:    "with indent option",
			data:    map[string]interface{}{"key": "value"},
			options: map[string]interface{}{"indent": 2},
			wantErr: false,
		},
		{
			name:    "with sort_keys option",
			data:    map[string]interface{}{"z": 1, "a": 2},
			options: map[string]interface{}{"sort_keys": true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: JSONFormatter doesn't currently support options, but we test the basic functionality
			result, err := formatter.Format(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONFormatter.Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Verify it's valid JSON
				var jsonData interface{}
				if err := json.Unmarshal(result, &jsonData); err != nil {
					t.Errorf("JSONFormatter.Format() returned invalid JSON: %v", err)
				}
			}
		})
	}
}

func TestJSONFormatter_ErrorHandling(t *testing.T) {
	formatter := &JSONFormatter{}

	// Test with data that might cause issues
	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name:    "channel type (should fail)",
			data:    make(chan int),
			wantErr: true, // Channels can't be marshaled to JSON
		},
		{
			name:    "function type (should fail)",
			data:    func() {},
			wantErr: true, // Functions can't be marshaled to JSON
		},
		{
			name:    "complex number (should fail)",
			data:    complex(1, 2),
			wantErr: true, // Complex numbers can't be marshaled to JSON
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONFormatter.Format() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && result != nil {
				t.Logf("Unexpected success: %s", string(result))
			}
		})
	}
}

