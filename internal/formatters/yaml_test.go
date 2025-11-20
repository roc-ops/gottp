package formatters

import (
	"testing"
)

func TestYAMLFormatter_Format(t *testing.T) {
	formatter := &YAMLFormatter{}

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
				t.Errorf("YAMLFormatter.Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && len(result) == 0 && tt.data != nil {
				t.Errorf("YAMLFormatter.Format() returned empty result for non-nil data")
			}
		})
	}
}

func TestYAMLFormatter_FormatString(t *testing.T) {
	formatter := &YAMLFormatter{}

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
				t.Errorf("YAMLFormatter.FormatString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result == "" && tt.data != nil {
				t.Errorf("YAMLFormatter.FormatString() returned empty string for non-nil data")
			}
		})
	}
}

func TestYAMLFormatter_ErrorHandling(t *testing.T) {
	formatter := &YAMLFormatter{}

	// Test with data that might cause issues
	tests := []struct {
		name        string
		data        interface{}
		shouldPanic bool
	}{
		{
			name:        "channel type (should panic)",
			data:        make(chan int),
			shouldPanic: true, // Channels can't be marshaled to YAML and will panic
		},
		{
			name:        "function type (should panic)",
			data:        func() {},
			shouldPanic: true, // Functions can't be marshaled to YAML and will panic
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var panicked bool
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
					}
				}()
				_, _ = formatter.Format(tt.data)
			}()
			if panicked != tt.shouldPanic {
				t.Errorf("YAMLFormatter.Format() panic = %v, shouldPanic %v", panicked, tt.shouldPanic)
			}
		})
	}
}

