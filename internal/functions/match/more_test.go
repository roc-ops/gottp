package match

import (
	"testing"
)

func TestToInt(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("to_int")
	if !ok {
		t.Fatal("to_int function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    interface{}
		wantErr bool
	}{
		{
			name:    "string integer",
			value:   "123",
			args:    nil,
			want:    int64(123),
			wantErr: false,
		},
		{
			name:    "string with spaces",
			value:   "  456  ",
			args:    nil,
			want:    int64(456),
			wantErr: false,
		},
		{
			name:    "integer value",
			value:   789,
			args:    nil,
			want:    int64(789),
			wantErr: false,
		},
		{
			name:    "large counter (Counter64)",
			value:   "642717570007",
			args:    nil,
			want:    int64(642717570007),
			wantErr: false,
		},
		{
			name:    "counter beyond int64 max (wrapped Counter64)",
			value:   "18000000000000000000",
			args:    nil,
			want:    uint64(18000000000000000000),
			wantErr: false,
		},
		{
			name:    "max uint64 (Counter64 max)",
			value:   "18446744073709551615",
			args:    nil,
			want:    uint64(18446744073709551615),
			wantErr: false,
		},
		{
			name:    "negative value",
			value:   "-42",
			args:    nil,
			want:    int64(-42),
			wantErr: false,
		},
		{
			name:    "invalid string",
			value:   "not a number",
			args:    nil,
			want:    "not a number", // Returns original value on error
			wantErr: false,
		},
		{
			name:    "float string",
			value:   "123.45",
			args:    nil,
			want:    "123.45", // Returns original value (can't convert float to int)
			wantErr: false,
		},
		{
			name:    "empty string",
			value:   "",
			args:    nil,
			want:    "", // Returns original value
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("toInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.want {
				t.Errorf("toInt() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestToStr(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("to_str")
	if !ok {
		t.Fatal("to_str function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "string value",
			value:   "test",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "integer value",
			value:   123,
			args:    nil,
			wantErr: false,
		},
		{
			name:    "float value",
			value:   3.14,
			args:    nil,
			wantErr: false,
		},
		{
			name:    "boolean value",
			value:   true,
			args:    nil,
			wantErr: false,
		},
		{
			name:    "nil value",
			value:   nil,
			args:    nil,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("toStr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				// Result should be a string
				if _, ok := result.(string); !ok && result != nil {
					t.Errorf("toStr() result type = %T, want string", result)
				}
			}
		})
	}
}

func TestToFloat(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("to_float")
	if !ok {
		t.Fatal("to_float function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "string float",
			value:   "123.45",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "string integer",
			value:   "123",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "float value",
			value:   3.14,
			args:    nil,
			wantErr: false,
		},
		{
			name:    "invalid string",
			value:   "not a number",
			args:    nil,
			wantErr: false, // Returns original value
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("toFloat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				// Result should be a float64 or original value
				if _, ok := result.(float64); !ok {
					// Might be original value if conversion failed
					t.Logf("toFloat() result = %v (type: %T)", result, result)
				}
			}
		})
	}
}

func TestToIP(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("to_ip")
	if !ok {
		t.Fatal("to_ip function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "valid IPv4",
			value:   "192.168.1.1",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "valid IPv4 with CIDR",
			value:   "192.168.1.1/24",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "invalid IP",
			value:   "not an ip",
			args:    nil,
			wantErr: false, // Returns original value
		},
		{
			name:    "empty string",
			value:   "",
			args:    nil,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("toIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				// Result should be a string
				if _, ok := result.(string); !ok {
					t.Errorf("toIP() result type = %T, want string", result)
				}
			}
		})
	}
}

func TestResub(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("resub")
	if !ok {
		t.Fatal("resub function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    string
		wantErr bool
	}{
		{
			name:    "basic substitution",
			value:   "hello world",
			args:    []string{"world", "universe"},
			want:    "hello universe",
			wantErr: false,
		},
		{
			name:    "regex substitution",
			value:   "test123",
			args:    []string{"\\d+", "NUM"},
			want:    "testNUM",
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    "test",
			wantErr: false,
		},
		{
			name:    "single arg",
			value:   "test",
			args:    []string{"pattern"},
			want:    "test",
			wantErr: false,
		},
		{
			name:    "multiple matches - replaces first only",
			value:   "a1b2c3",
			args:    []string{"\\d", "X"},
			want:    "aXb2c3", // resub only replaces first occurrence (use resuball for all)
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("resub() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if resultStr, ok := result.(string); ok {
					if resultStr != tt.want {
						t.Errorf("resub() = %v, want %v", resultStr, tt.want)
					}
				} else {
					t.Errorf("resub() result type = %T, want string", result)
				}
			}
		})
	}
}

func TestPrepend(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("prepend")
	if !ok {
		t.Fatal("prepend function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    string
		wantErr bool
	}{
		{
			name:    "prepend string",
			value:   "world",
			args:    []string{"hello "},
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "prepend to integer",
			value:   123,
			args:    []string{"prefix_"},
			want:    "prefix_123",
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    "test",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("prepend() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if resultStr, ok := result.(string); ok {
					if resultStr != tt.want {
						t.Errorf("prepend() = %v, want %v", resultStr, tt.want)
					}
				} else {
					t.Errorf("prepend() result type = %T, want string", result)
				}
			}
		})
	}
}

func TestAppendFunc(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("append")
	if !ok {
		t.Fatal("append function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    string
		wantErr bool
	}{
		{
			name:    "append string",
			value:   "hello",
			args:    []string{" world"},
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "append to integer",
			value:   123,
			args:    []string{"_suffix"},
			want:    "123_suffix",
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    "test",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("append() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if resultStr, ok := result.(string); ok {
					if resultStr != tt.want {
						t.Errorf("append() = %v, want %v", resultStr, tt.want)
					}
				} else {
					t.Errorf("append() result type = %T, want string", result)
				}
			}
		})
	}
}

func TestCopyFunc(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("copy")
	if !ok {
		t.Fatal("copy function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "copy string",
			value:   "test",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "copy integer",
			value:   123,
			args:    nil,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("copy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != tt.value {
				t.Errorf("copy() = %v, want %v", result, tt.value)
			}
		})
	}
}

func TestDefaultFunc(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("default")
	if !ok {
		t.Fatal("default function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    interface{}
		wantErr bool
	}{
		{
			name:    "empty string",
			value:   "",
			args:    []string{"default_value"},
			want:    "default_value",
			wantErr: false,
		},
		{
			name:    "non-empty string",
			value:   "actual_value",
			args:    []string{"default_value"},
			want:    "actual_value",
			wantErr: false,
		},
		{
			name:    "nil value",
			value:   nil,
			args:    []string{"default_value"},
			want:    "default_value",
			wantErr: false,
		},
		{
			name:    "whitespace only",
			value:   "   ",
			args:    []string{"default_value"},
			want:    "default_value",
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    "test",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("default() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != tt.want {
				t.Errorf("default() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestCount(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("count")
	if !ok {
		t.Fatal("count function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    int
		wantErr bool
	}{
		{
			name:    "string value",
			value:   "test",
			args:    nil,
			want:    4,
			wantErr: false,
		},
		{
			name:    "list value",
			value:   []interface{}{1, 2, 3},
			args:    nil,
			want:    3,
			wantErr: false,
		},
		{
			name:    "map value",
			value:   map[string]interface{}{"a": 1, "b": 2},
			args:    nil,
			want:    2,
			wantErr: false,
		},
		{
			name:    "other type",
			value:   123,
			args:    nil,
			want:    1,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("count() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if resultInt, ok := result.(int); ok {
					if resultInt != tt.want {
						t.Errorf("count() = %v, want %v", resultInt, tt.want)
					}
				} else {
					t.Errorf("count() result type = %T, want int", result)
				}
			}
		})
	}
}

func TestSformat(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("sformat")
	if !ok {
		t.Fatal("sformat function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    string
		wantErr bool
	}{
		{
			name:    "basic format",
			value:   "test",
			args:    []string{"prefix {} suffix"},
			want:    "prefix test suffix",
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    "test",
			wantErr: false,
		},
		{
			name:    "integer value",
			value:   123,
			args:    []string{"value: {}"},
			want:    "value: 123",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("sformat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if resultStr, ok := result.(string); ok {
					if resultStr != tt.want {
						t.Errorf("sformat() = %v, want %v", resultStr, tt.want)
					}
				} else {
					t.Errorf("sformat() result type = %T, want string", result)
				}
			}
		})
	}
}

func TestLet(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("let")
	if !ok {
		t.Fatal("let function not found")
	}
	
	tests := []struct {
		name      string
		value     interface{}
		args      []string
		kwargs    map[string]interface{}
		want      interface{}
		wantField map[string]interface{} // Expected field to be set in match data
		wantErr   bool
	}{
		{
			name:    "single arg - replace value",
			value:   "original",
			args:    []string{"new_value"},
			kwargs:  nil,
			want:    "new_value",
			wantErr: false,
		},
		{
			name:    "two args - set field with string value",
			value:   "test",
			args:    []string{"field_name", "field_value"},
			kwargs:  map[string]interface{}{"_match_data": make(map[string]interface{})},
			want:    "test", // Original value should be returned
			wantField: map[string]interface{}{"field_name": "field_value"},
			wantErr: false,
		},
		{
			name:    "two args - set field with True boolean",
			value:   "test",
			args:    []string{"us-bonded", "True"},
			kwargs:  map[string]interface{}{"_match_data": make(map[string]interface{})},
			want:    "test",
			wantField: map[string]interface{}{"us-bonded": true},
			wantErr: false,
		},
		{
			name:    "two args - set field with False boolean",
			value:   "test",
			args:    []string{"enabled", "False"},
			kwargs:  map[string]interface{}{"_match_data": make(map[string]interface{})},
			want:    "test",
			wantField: map[string]interface{}{"enabled": false},
			wantErr: false,
		},
		{
			name:    "two args - set field with integer",
			value:   "test",
			args:    []string{"count", "42"},
			kwargs:  map[string]interface{}{"_match_data": make(map[string]interface{})},
			want:    "test",
			wantField: map[string]interface{}{"count": 42},
			wantErr: false,
		},
		{
			name:    "two args - set field with float",
			value:   "test",
			args:    []string{"price", "3.14"},
			kwargs:  map[string]interface{}{"_match_data": make(map[string]interface{})},
			want:    "test",
			wantField: map[string]interface{}{"price": 3.14},
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			kwargs:  nil,
			want:    "test",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh match data map for each test
			var matchData map[string]interface{}
			if tt.kwargs != nil {
				if md, ok := tt.kwargs["_match_data"].(map[string]interface{}); ok {
					matchData = md
				} else {
					matchData = make(map[string]interface{})
					tt.kwargs["_match_data"] = matchData
				}
			}
			
			result, err := fn(tt.value, tt.args, tt.kwargs)
			if (err != nil) != tt.wantErr {
				t.Errorf("let() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if result != tt.want {
					t.Errorf("let() = %v, want %v", result, tt.want)
				}
				
				// Check if field was set correctly
				if tt.wantField != nil && matchData != nil {
					for fieldName, expectedValue := range tt.wantField {
						if actualValue, ok := matchData[fieldName]; !ok {
							t.Errorf("let() did not set field %s", fieldName)
						} else if actualValue != expectedValue {
							t.Errorf("let() set field %s = %v (type %T), want %v (type %T)", 
								fieldName, actualValue, actualValue, expectedValue, expectedValue)
						}
					}
				}
			}
		})
	}
}

func TestUnrange(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("unrange")
	if !ok {
		t.Fatal("unrange function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "simple range",
			value:   "10-13",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "range with list",
			value:   "8,10-13,20",
			args:    nil,
			wantErr: false,
		},
		{
			name:    "custom rangechar",
			value:   "10..13",
			args:    []string{".."},
			wantErr: false,
		},
		{
			name:    "custom joinchar",
			value:   "10-13",
			args:    []string{"-", ":"},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("unrange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				// Result should be a string with expanded range
				if resultStr, ok := result.(string); ok {
					t.Logf("unrange() result = %v", resultStr)
				}
			}
		})
	}
}

func TestLookup(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("lookup")
	if !ok {
		t.Fatal("lookup function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "lookup with args",
			value:   "key1",
			args:    []string{"table_name"},
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("lookup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Lookup currently returns original value (not implemented)
			if err == nil && result != tt.value {
				t.Logf("lookup() result = %v (expected original value for now)", result)
			}
		})
	}
}

func TestReplaceall(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("replaceall")
	if !ok {
		t.Fatal("replaceall function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    string
		wantErr bool
	}{
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    "test",
			wantErr: false,
		},
		{
			name:    "single arg - replace with empty",
			value:   "hello world",
			args:    []string{"world"},
			want:    "hello ",
			wantErr: false,
		},
		{
			name:    "multiple args - replace multiple with first arg",
			value:   "a*b*c",
			args:    []string{"-", "a", "b"},
			want:    "-*-*c",
			wantErr: false,
		},
		{
			name:    "replace asterisk and exclamation with empty",
			value:   "*20.0",
			args:    []string{"", "*", "!"},
			want:    "20.0",
			wantErr: false,
		},
		{
			name:    "replace exclamation and asterisk with empty",
			value:   "!20.0",
			args:    []string{"", "!", "*"},
			want:    "20.0",
			wantErr: false,
		},
		{
			name:    "replace both asterisk and exclamation in same string",
			value:   "*!20.0",
			args:    []string{"", "*", "!"},
			want:    "20.0",
			wantErr: false,
		},
		{
			name:    "replace asterisk only",
			value:   "*20.0",
			args:    []string{"", "*"},
			want:    "20.0",
			wantErr: false,
		},
		{
			name:    "replace exclamation only",
			value:   "!20.0",
			args:    []string{"", "!"},
			want:    "20.0",
			wantErr: false,
		},
		{
			name:    "replace multiple characters with same replacement",
			value:   "a*b!c",
			args:    []string{"X", "*", "!"},
			want:    "aXbXc",
			wantErr: false,
		},
		{
			name:    "replace special regex characters",
			value:   "test.123",
			args:    []string{"", ".", "1"},
			want:    "test23",
			wantErr: false,
		},
		{
			name:    "replace with non-empty string",
			value:   "hello world",
			args:    []string{"X", "hello", "world"},
			want:    "X X",
			wantErr: false,
		},
		{
			name:    "no matches",
			value:   "hello world",
			args:    []string{"", "x", "y"},
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "empty string value",
			value:   "",
			args:    []string{"", "*", "!"},
			want:    "",
			wantErr: false,
		},
		{
			name:    "integer value",
			value:   123,
			args:    []string{"", "1", "2"},
			want:    "3",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("replaceall() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if resultStr, ok := result.(string); ok {
					if resultStr != tt.want {
						t.Errorf("replaceall() = %q, want %q", resultStr, tt.want)
					}
				} else {
					t.Errorf("replaceall() result type = %T, want string", result)
				}
			}
		})
	}
}

func TestResuball(t *testing.T) {
	registry := NewRegistry()
	registry.registerMoreFunctions()
	
	fn, ok := registry.Get("resuball")
	if !ok {
		t.Fatal("resuball function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    string
		wantErr bool
	}{
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    "test",
			wantErr: false,
		},
		{
			name:    "single arg - replace pattern with empty",
			value:   "test123",
			args:    []string{"\\d+"},
			want:    "test",
			wantErr: false,
		},
		{
			name:    "multiple args - replace multiple patterns with first arg",
			value:   "a1b2c3",
			args:    []string{"X", "\\d", "[a-c]"},
			want:    "XXXXXX", // First replaces digits, then replaces letters
			wantErr: false,
		},
		{
			name:    "replace asterisk and exclamation with empty (regex)",
			value:   "*20.0",
			args:    []string{"", "\\*", "!"},
			want:    "20.0",
			wantErr: false,
		},
		{
			name:    "replace exclamation and asterisk with empty (regex)",
			value:   "!20.0",
			args:    []string{"", "!", "\\*"},
			want:    "20.0",
			wantErr: false,
		},
		{
			name:    "replace both asterisk and exclamation in same string (regex)",
			value:   "*!20.0",
			args:    []string{"", "\\*", "!"},
			want:    "20.0",
			wantErr: false,
		},
		{
			name:    "replace digits with X",
			value:   "a1b2c3",
			args:    []string{"X", "\\d"},
			want:    "aXbXcX",
			wantErr: false,
		},
		{
			name:    "replace multiple patterns",
			value:   "test.123",
			args:    []string{"", "\\.", "\\d+"},
			want:    "test",
			wantErr: false,
		},
		{
			name:    "no matches",
			value:   "hello world",
			args:    []string{"", "\\d+", "[0-9]"},
			want:    "hello world",
			wantErr: false,
		},
		{
			name:    "empty string value",
			value:   "",
			args:    []string{"", "\\*", "!"},
			want:    "",
			wantErr: false,
		},
		{
			name:    "integer value",
			value:   123,
			args:    []string{"", "1", "2"},
			want:    "3",
			wantErr: false,
		},
		{
			name:    "invalid regex pattern - should skip",
			value:   "test123",
			args:    []string{"", "[invalid", "\\d+"},
			want:    "test",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("resuball() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if resultStr, ok := result.(string); ok {
					if resultStr != tt.want {
						t.Errorf("resuball() = %q, want %q", resultStr, tt.want)
					}
				} else {
					t.Errorf("resuball() result type = %T, want string", result)
				}
			}
		})
	}
}

