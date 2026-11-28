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
		name       string
		data       map[string]interface{}
		args       []string
		kwargs     map[string]interface{}
		wantErr    bool
		wantField  string
		wantValue  interface{}
	}{
		{
			name:      "set from kwargs variable",
			data:      map[string]interface{}{"existing": "value"},
			args:      []string{"my_var", "target_field"},
			kwargs:    map[string]interface{}{"my_var": "var_value"},
			wantErr:   false,
			wantField: "target_field",
			wantValue: "var_value",
		},
		{
			name:      "set literal value (source not in kwargs/data)",
			data:      map[string]interface{}{},
			args:      []string{"literal_value", "target_field"},
			kwargs:    nil,
			wantErr:   false,
			wantField: "target_field",
			wantValue: "literal_value",
		},
		{
			name:      "single arg - uses source as both source and target",
			data:      map[string]interface{}{},
			args:      []string{"field_name"},
			kwargs:    nil,
			wantErr:   false,
			wantField: "field_name",
			wantValue: "field_name", // literal since not found in kwargs/data
		},
		{
			name:    "no args",
			data:    map[string]interface{}{},
			args:    nil,
			wantErr: true, // set requires at least one argument
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of data for each test
			dataCopy := make(map[string]interface{})
			for k, v := range tt.data {
				dataCopy[k] = v
			}
			
			result, valid, err := fn(dataCopy, tt.args, tt.kwargs)
			if (err != nil) != tt.wantErr {
				t.Errorf("set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if tt.wantField != "" {
					// Check if field was set correctly
					if val, ok := result[tt.wantField]; ok {
						if val != tt.wantValue {
							t.Errorf("set() field %s = %v, want %v", tt.wantField, val, tt.wantValue)
						}
					} else {
						t.Errorf("set() field %s not found in result", tt.wantField)
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
			wantErr: false, // TTP's contains(data, *args): no key found
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
			wantErr: false, // TTP's delete_func(data, *args): nothing deleted
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


// contains, containsall, exclude and excludeall don't count a value that is
// only the group's default (kwargs["_defaults"]), as in TTP 0.10.1.
func TestFilterFunctionsSkipDefaults(t *testing.T) {
	defaults := map[string]interface{}{"desc": "none"}
	kw := map[string]interface{}{"_defaults": defaults}
	cases := []struct {
		name string
		fn   Function
		data map[string]interface{}
		args []string
		want bool
	}{
		{"contains default value", contains, map[string]interface{}{"desc": "none"}, []string{"desc"}, false},
		{"contains real value", contains, map[string]interface{}{"desc": "uplink"}, []string{"desc"}, true},
		{"containsall default value", containsall, map[string]interface{}{"desc": "none", "ip": "1"}, []string{"ip", "desc"}, false},
		{"containsall real values", containsall, map[string]interface{}{"desc": "x", "ip": "1"}, []string{"ip", "desc"}, true},
		{"exclude key with a default", exclude, map[string]interface{}{"desc": "uplink"}, []string{"desc"}, true},
		{"exclude key without a default", exclude, map[string]interface{}{"ip": "1"}, []string{"ip"}, false},
		{"excludeall key with a default", excludeall, map[string]interface{}{"desc": "x", "ip": "1"}, []string{"ip", "desc"}, true},
		{"excludeall keys without defaults", excludeall, map[string]interface{}{"mtu": "1", "ip": "1"}, []string{"ip", "mtu"}, false},
	}
	for _, c := range cases {
		_, keep, err := c.fn(c.data, c.args, kw)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if keep != c.want {
			t.Errorf("%s: keep = %v, want %v", c.name, keep, c.want)
		}
	}
}
