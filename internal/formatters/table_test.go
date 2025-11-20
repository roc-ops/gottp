package formatters

import (
	"testing"
)

func TestTableFormatter_Format(t *testing.T) {
	formatter := NewTableFormatter()

	tests := []struct {
		name    string
		data    interface{}
		options *TableOptions
		wantErr bool
	}{
		{
			name:    "nil data",
			data:    nil,
			options: nil,
			wantErr: false,
		},
		{
			name:    "list of maps",
			data:    []map[string]interface{}{{"name": "John", "age": 30}, {"name": "Jane", "age": 25}},
			options: nil,
			wantErr: false,
		},
		{
			name:    "single map",
			data:    map[string]interface{}{"name": "John", "age": 30},
			options: nil,
			wantErr: false,
		},
		{
			name:    "empty list",
			data:    []interface{}{},
			options: nil,
			wantErr: false,
		},
		{
			name:    "list with path option",
			data:    map[string]interface{}{"users": []map[string]interface{}{{"name": "John"}}},
			options: &TableOptions{Path: "users"},
			wantErr: false,
		},
		{
			name:    "list with headers option",
			data:    []map[string]interface{}{{"name": "John", "age": 30}},
			options: &TableOptions{Headers: []string{"Name", "Age"}},
			wantErr: false,
		},
		{
			name:    "list with missing option",
			data:    []map[string]interface{}{{"name": "John", "age": 30}},
			options: &TableOptions{Missing: "N/A"},
			wantErr: false,
		},
		{
			name:    "list with key option",
			data:    []map[string]interface{}{{"name": "John", "age": 30}},
			options: &TableOptions{Key: "name"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.data, tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("TableFormatter.Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result == nil && tt.data != nil {
				t.Errorf("TableFormatter.Format() returned nil for non-nil data")
			}
		})
	}
}

func TestTableFormatter_WithPath(t *testing.T) {
	formatter := NewTableFormatter()

	data := map[string]interface{}{
		"users": []map[string]interface{}{
			{"name": "John", "age": 30},
			{"name": "Jane", "age": 25},
		},
	}
	options := &TableOptions{Path: "users"}
	result, err := formatter.Format(data, options)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}
	if result == nil {
		t.Error("TableFormatter.Format() returned nil")
	}
	// Result should be a list of lists
	if len(result) == 0 {
		t.Error("TableFormatter.Format() returned empty table")
	}
}

func TestTableFormatter_WithHeaders(t *testing.T) {
	formatter := NewTableFormatter()

	data := []map[string]interface{}{{"name": "John", "age": 30}}
	options := &TableOptions{Headers: []string{"Name", "Age"}}
	result, err := formatter.Format(data, options)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}
	if result == nil {
		t.Error("TableFormatter.Format() returned nil")
	}
	// Result should include headers as first row
	if len(result) == 0 {
		t.Error("TableFormatter.Format() returned empty table")
	}
	if len(result) > 0 {
		// First row should be headers
		if len(result[0]) != 2 {
			t.Errorf("TableFormatter.Format() headers row has wrong length: %d", len(result[0]))
		}
	}
}

func TestTableFormatter_WithMissing(t *testing.T) {
	formatter := NewTableFormatter()

	data := []map[string]interface{}{
		{"name": "John", "age": 30},
		{"name": "Jane"}, // Missing age
	}
	options := &TableOptions{Missing: "N/A"}
	result, err := formatter.Format(data, options)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}
	if result == nil {
		t.Error("TableFormatter.Format() returned nil")
	}
}

func TestTableFormatter_WithKey(t *testing.T) {
	formatter := NewTableFormatter()

	data := []map[string]interface{}{{"name": "John", "age": 30}}
	options := &TableOptions{Key: "name"}
	result, err := formatter.Format(data, options)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}
	if result == nil {
		t.Error("TableFormatter.Format() returned nil")
	}
}

func TestTableFormatter_ErrorHandling(t *testing.T) {
	formatter := NewTableFormatter()

	// Test with invalid data types
	tests := []struct {
		name    string
		data    interface{}
		options *TableOptions
		wantErr bool
	}{
		{
			name:    "channel type",
			data:    make(chan int),
			options: nil,
			wantErr: false, // Table formatter handles this gracefully
		},
		{
			name:    "function type",
			data:    func() {},
			options: nil,
			wantErr: false, // Table formatter handles this gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatter.Format(tt.data, tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("TableFormatter.Format() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && result != nil {
				t.Logf("Result: %v", result)
			}
		})
	}
}

func TestTableFormatter_EmptyData(t *testing.T) {
	formatter := NewTableFormatter()

	data := []interface{}{}
	result, err := formatter.Format(data, nil)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}
	if result == nil {
		t.Error("TableFormatter.Format() returned nil for empty list")
	}
	if len(result) != 0 {
		t.Errorf("TableFormatter.Format() returned non-empty table for empty list: %v", result)
	}
}

func TestTableFormatter_ListInterface(t *testing.T) {
	formatter := NewTableFormatter()

	// Test with []interface{} containing maps
	data := []interface{}{
		map[string]interface{}{"name": "John", "age": 30},
		map[string]interface{}{"name": "Jane", "age": 25},
	}
	result, err := formatter.Format(data, nil)
	if err != nil {
		t.Fatalf("TableFormatter.Format() error = %v", err)
	}
	if result == nil {
		t.Error("TableFormatter.Format() returned nil")
	}
	if len(result) == 0 {
		t.Error("TableFormatter.Format() returned empty table")
	}
}

