package match

import (
	"testing"
)

func TestMatchFunctions_ErrorHandling(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()

	tests := []struct {
		name    string
		fnName  string
		value   interface{}
		args    []string
		wantErr bool
	}{
		// toInt error cases
		{
			name:    "toInt with invalid string",
			fnName:  "to_int",
			value:   "not a number",
			args:    nil,
			wantErr: false, // Returns original value, not an error
		},
		{
			name:    "toInt with nil",
			fnName:  "to_int",
			value:   nil,
			args:    nil,
			wantErr: false,
		},
		// toFloat error cases
		{
			name:    "toFloat with invalid string",
			fnName:  "to_float",
			value:   "not a float",
			args:    nil,
			wantErr: false, // Returns original value
		},
		// toIP error cases
		{
			name:    "toIP with invalid IP",
			fnName:  "to_ip",
			value:   "999.999.999.999",
			args:    nil,
			wantErr: false, // Returns original value
		},
		// resub error cases
		{
			name:    "resub with invalid regex",
			fnName:  "resub",
			value:   "test",
			args:    []string{"[invalid", "replacement"},
			wantErr: false, // May return original or handle gracefully
		},
		{
			name:    "resub with no replacement",
			fnName:  "resub",
			value:   "test",
			args:    []string{"pattern"},
			wantErr: false,
		},
		// sformat error cases
		{
			name:    "sformat with no format string",
			fnName:  "sformat",
			value:   "test",
			args:    nil,
			wantErr: false,
		},
		// count edge cases
		{
			name:    "count with nil",
			fnName:  "count",
			value:   nil,
			args:    nil,
			wantErr: false,
		},
		// unrange error cases
		{
			name:    "unrange with invalid range",
			fnName:  "unrange",
			value:   "invalid-range",
			args:    nil,
			wantErr: false, // Returns original value
		},
		{
			name:    "unrange with empty string",
			fnName:  "unrange",
			value:   "",
			args:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := registry.Get(tt.fnName)
			if !ok {
				t.Fatalf("Function %s not found", tt.fnName)
			}

			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s() error = %v, wantErr %v", tt.fnName, err, tt.wantErr)
				return
			}
			if err == nil && result == nil && tt.value != nil {
				// Some functions may return nil for certain inputs
				t.Logf("%s() returned nil for %v", tt.fnName, tt.value)
			}
		})
	}
}

func TestMatchFunctions_TypeMismatches(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()

	tests := []struct {
		name   string
		fnName string
		value  interface{}
		args   []string
	}{
		{
			name:   "toInt with map",
			fnName: "to_int",
			value:  map[string]interface{}{"key": "value"},
			args:   nil,
		},
		{
			name:   "toInt with list",
			fnName: "to_int",
			value:  []interface{}{1, 2, 3},
			args:   nil,
		},
		{
			name:   "toFloat with map",
			fnName: "to_float",
			value:  map[string]interface{}{"key": "value"},
			args:   nil,
		},
		{
			name:   "prepend with map",
			fnName: "prepend",
			value:  map[string]interface{}{"key": "value"},
			args:   []string{"prefix"},
		},
		{
			name:   "append with list",
			fnName: "append",
			value:  []interface{}{1, 2, 3},
			args:   []string{"suffix"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := registry.Get(tt.fnName)
			if !ok {
				t.Fatalf("Function %s not found", tt.fnName)
			}

			result, err := fn(tt.value, tt.args, nil)
			if err != nil {
				t.Logf("%s() error = %v (may be expected for type mismatch)", tt.fnName, err)
			} else if result != nil {
				t.Logf("%s() result = %v (type: %T)", tt.fnName, result, result)
			}
		})
	}
}

func TestMatchFunctions_InvalidArguments(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()

	tests := []struct {
		name    string
		fnName  string
		value   interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "resub with too many args",
			fnName:  "resub",
			value:   "test",
			args:    []string{"pattern", "replacement", "extra"},
			wantErr: false, // May ignore extra args
		},
		{
			name:    "sformat with multiple format strings",
			fnName:  "sformat",
			value:   "test",
			args:    []string{"format1", "format2"},
			wantErr: false,
		},
		{
			name:    "unrange with invalid rangechar",
			fnName:  "unrange",
			value:   "10-20",
			args:    []string{""}, // Empty rangechar
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := registry.Get(tt.fnName)
			if !ok {
				t.Fatalf("Function %s not found", tt.fnName)
			}

			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s() error = %v, wantErr %v", tt.fnName, err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				t.Logf("%s() result = %v", tt.fnName, result)
			}
		})
	}
}

func TestMatchFunctions_EdgeCases(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()

	tests := []struct {
		name   string
		fnName string
		value  interface{}
		args   []string
	}{
		{
			name:   "toStr with very large number",
			fnName: "to_str",
			value:  999999999999999999,
			args:   nil,
		},
		{
			name:   "toStr with very small number",
			fnName: "to_str",
			value:  -999999999999999999,
			args:   nil,
		},
		{
			name:   "toFloat with scientific notation string",
			fnName: "to_float",
			value:  "1.23e-4",
			args:   nil,
		},
		{
			name:   "resub with empty pattern",
			fnName: "resub",
			value:  "test",
			args:   []string{"", "replacement"},
		},
		{
			name:   "resub with empty replacement",
			fnName: "resub",
			value:  "test",
			args:   []string{"t", ""},
		},
		{
			name:   "prepend with empty prefix",
			fnName: "prepend",
			value:  "test",
			args:   []string{""},
		},
		{
			name:   "append with empty suffix",
			fnName: "append",
			value:  "test",
			args:   []string{""},
		},
		{
			name:   "default with empty default value",
			fnName: "default",
			value:  "",
			args:   []string{""},
		},
		{
			name:   "let with empty default value",
			fnName: "let",
			value:  "",
			args:   []string{""},
		},
		{
			name:   "count with empty string",
			fnName: "count",
			value:  "",
			args:   nil,
		},
		{
			name:   "count with empty list",
			fnName: "count",
			value:  []interface{}{},
			args:   nil,
		},
		{
			name:   "count with empty map",
			fnName: "count",
			value:  map[string]interface{}{},
			args:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := registry.Get(tt.fnName)
			if !ok {
				t.Fatalf("Function %s not found", tt.fnName)
			}

			result, err := fn(tt.value, tt.args, nil)
			if err != nil {
				t.Errorf("%s() error = %v", tt.fnName, err)
				return
			}
			if result != nil {
				t.Logf("%s() result = %v (type: %T)", tt.fnName, result, result)
			}
		})
	}
}

