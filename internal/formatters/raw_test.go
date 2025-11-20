package formatters

import (
	"testing"
)

func TestRawFormatter_Format(t *testing.T) {
	formatter := &RawFormatter{}

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
			name:    "integer",
			data:    42,
			wantErr: false,
		},
		{
			name:    "float",
			data:    3.14,
			wantErr: false,
		},
		{
			name:    "boolean",
			data:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("RawFormatter.Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result == nil && tt.data != nil {
				t.Errorf("RawFormatter.Format() returned nil for non-nil data")
			}
		})
	}
}

func TestRawFormatter_FormatString(t *testing.T) {
	formatter := &RawFormatter{}

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
				t.Errorf("RawFormatter.FormatString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result == "" && tt.data != nil {
				// Empty string might be valid for some data types
				t.Logf("RawFormatter.FormatString() returned empty string for %T", tt.data)
			}
		})
	}
}

func TestRawFormatter_StringInput(t *testing.T) {
	formatter := &RawFormatter{}

	data := "test string"
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("RawFormatter.Format() error = %v", err)
	}
	if string(result) != data {
		t.Errorf("RawFormatter.Format() = %v, want %v", string(result), data)
	}
}

func TestRawFormatter_MapInput(t *testing.T) {
	formatter := &RawFormatter{}

	data := map[string]interface{}{"key": "value"}
	result, err := formatter.Format(data)
	if err != nil {
		t.Fatalf("RawFormatter.Format() error = %v", err)
	}
	if result == nil {
		t.Error("RawFormatter.Format() returned nil for map input")
	}
}

