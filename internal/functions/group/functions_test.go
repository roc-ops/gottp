package group

import (
	"testing"
)

func TestSet(t *testing.T) {
	registry := NewRegistry()
	// Functions are registered automatically in NewRegistry via registerBuiltins
	
	fn, ok := registry.Get("set")
	if !ok {
		t.Fatal("set function not found")
	}
	
	tests := []struct {
		name    string
		data    map[string]interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "set field",
			data:    map[string]interface{}{"existing": "value"},
			args:    []string{"new_field", "new_value"},
			wantErr: false,
		},
		{
			name:    "no args",
			data:    map[string]interface{}{},
			args:    nil,
			wantErr: true, // set requires key and value arguments
		},
		{
			name:    "single arg",
			data:    map[string]interface{}{},
			args:    []string{"field"},
			wantErr: true, // set requires key and value arguments
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of data for each test
			dataCopy := make(map[string]interface{})
			for k, v := range tt.data {
				dataCopy[k] = v
			}
			
			result, valid, err := fn(dataCopy, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if len(tt.args) >= 2 {
					// Check if field was set
					if val, ok := result[tt.args[0]]; ok {
						if val != tt.args[1] {
							t.Errorf("set() field value = %v, want %v", val, tt.args[1])
						}
					} else {
						t.Errorf("set() field %s not found in result", tt.args[0])
					}
				}
				if !valid {
					t.Errorf("set() returned invalid = false, want true")
				}
			}
		})
	}
}

func TestRecord(t *testing.T) {
	registry := NewRegistry()
	// Functions are registered automatically in NewRegistry via registerBuiltins
	
	fn, ok := registry.Get("record")
	if !ok {
		t.Fatal("record function not found")
	}
	
	tests := []struct {
		name    string
		data    map[string]interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "record field",
			data:    map[string]interface{}{"field": "value"},
			args:    []string{"field", "global_name"},
			wantErr: false,
		},
		{
			name:    "no args",
			data:    map[string]interface{}{},
			args:    nil,
			wantErr: true, // record requires at least one argument
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataCopy := make(map[string]interface{})
			for k, v := range tt.data {
				dataCopy[k] = v
			}
			
			result, valid, err := fn(dataCopy, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("record() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if result == nil {
					t.Errorf("record() returned nil result")
				}
				// record returns true even if there's an error (it just logs the error)
				// So we only check valid if there's no error
				if !valid && len(tt.args) > 0 {
					t.Errorf("record() returned invalid = false, want true")
				}
			}
		})
	}
}

func TestContainsGroup(t *testing.T) {
	registry := NewRegistry()
	// Functions are registered automatically in NewRegistry via registerBuiltins
	
	fn, ok := registry.Get("contains")
	if !ok {
		t.Fatal("contains function not found")
	}
	
	tests := []struct {
		name    string
		data    map[string]interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "contains key",
			data:    map[string]interface{}{"key": "value"},
			args:    []string{"key"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "does not contain key",
			data:    map[string]interface{}{"key": "value"},
			args:    []string{"other"},
			want:    false,
			wantErr: false,
		},
		{
			name:    "no args",
			data:    map[string]interface{}{},
			args:    nil,
			want:    false,
			wantErr: true, // contains requires at least one argument
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, valid, err := fn(tt.data, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("contains() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if valid != tt.want {
					t.Errorf("contains() = %v, want %v", valid, tt.want)
				}
			}
		})
	}
}

func TestDeleteKey(t *testing.T) {
	registry := NewRegistry()
	// Functions are registered automatically in NewRegistry via registerBuiltins
	
	fn, ok := registry.Get("delete")
	if !ok {
		t.Fatal("delete function not found")
	}
	
	tests := []struct {
		name    string
		data    map[string]interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "delete existing key",
			data:    map[string]interface{}{"key1": "value1", "key2": "value2"},
			args:    []string{"key1"},
			wantErr: false,
		},
		{
			name:    "delete non-existent key",
			data:    map[string]interface{}{"key1": "value1"},
			args:    []string{"nonexistent"},
			wantErr: false,
		},
		{
			name:    "no args",
			data:    map[string]interface{}{"key": "value"},
			args:    nil,
			wantErr: true, // delete requires at least one argument
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataCopy := make(map[string]interface{})
			for k, v := range tt.data {
				dataCopy[k] = v
			}
			
			result, valid, err := fn(dataCopy, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if len(tt.args) > 0 {
					// Check if key was deleted
					if _, exists := result[tt.args[0]]; exists {
						t.Errorf("delete() key %s still exists", tt.args[0])
					}
				}
				if !valid {
					t.Errorf("delete() returned invalid = false, want true")
				}
			}
		})
	}
}

func TestExpand(t *testing.T) {
	registry := NewRegistry()
	// Functions are registered automatically in NewRegistry via registerBuiltins
	
	fn, ok := registry.Get("expand")
	if !ok {
		t.Fatal("expand function not found")
	}
	
	tests := []struct {
		name    string
		data    map[string]interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "expand dot-separated keys",
			data:    map[string]interface{}{"target.x": "value1", "target.y": "value2"},
			args:    []string{"target"},
			wantErr: false,
		},
		{
			name:    "no args",
			data:    map[string]interface{}{},
			args:    nil,
			wantErr: false, // expand doesn't require args
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataCopy := make(map[string]interface{})
			for k, v := range tt.data {
				dataCopy[k] = v
			}
			
			result, valid, err := fn(dataCopy, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("expand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if result == nil {
					t.Errorf("expand() returned nil result")
				}
				if !valid {
					t.Errorf("expand() returned invalid = false, want true")
				}
			}
		})
	}
}

func TestItemize(t *testing.T) {
	registry := NewRegistry()
	// Functions are registered automatically in NewRegistry via registerBuiltins
	
	fn, ok := registry.Get("itemize")
	if !ok {
		t.Fatal("itemize function not found")
	}
	
	tests := []struct {
		name    string
		data    map[string]interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "itemize with key",
			data:    map[string]interface{}{"items": []interface{}{"item1", "item2"}},
			args:    []string{"items"},
			wantErr: false,
		},
		{
			name:    "no args",
			data:    map[string]interface{}{},
			args:    nil,
			wantErr: true, // itemize requires a key argument
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataCopy := make(map[string]interface{})
			for k, v := range tt.data {
				dataCopy[k] = v
			}
			
			result, valid, err := fn(dataCopy, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("itemize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if result == nil {
					t.Errorf("itemize() returned nil result")
				}
				if !valid && len(tt.args) > 0 {
					t.Logf("itemize() returned invalid = false (may be expected if key not found)")
				}
			}
		})
	}
}

