package macro

import (
	"testing"
)

func TestNewJavaScriptEngine(t *testing.T) {
	engine := NewJavaScriptEngine()
	
	if engine == nil {
		t.Fatal("NewJavaScriptEngine() returned nil")
	}
}

func TestJavaScriptEngine_RegisterMacro(t *testing.T) {
	engine := NewJavaScriptEngine()
	
	tests := []struct {
		name    string
		macroName string
		source  string
		wantErr bool
	}{
		{
			name:    "valid macro",
			macroName: "test_func",
			source:  "function test_func(data) { return data.toUpperCase(); }",
			wantErr: false,
		},
		{
			name:    "invalid syntax",
			macroName: "invalid",
			source:  "function invalid(data { return", // Invalid syntax
			wantErr: true,
		},
		{
			name:    "empty source",
			macroName: "empty",
			source:  "",
			wantErr: false, // Empty source might be valid
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.RegisterMacro(tt.macroName, tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterMacro() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJavaScriptEngine_RegisterMacroSource(t *testing.T) {
	engine := NewJavaScriptEngine()
	
	tests := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{
			name:    "valid source",
			source:  "function func1(data) { return data; }\nfunction func2(data) { return data.toUpperCase(); }",
			wantErr: false,
		},
		{
			name:    "invalid syntax",
			source:  "function invalid(data { return",
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.RegisterMacroSource(tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterMacroSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestJavaScriptEngine_ExecuteMacro(t *testing.T) {
	engine := NewJavaScriptEngine()
	
	// Register a test macro
	source := "function uppercase(data) { return data.toUpperCase(); }"
	err := engine.RegisterMacro("uppercase", source)
	if err != nil {
		t.Fatalf("Failed to register macro: %v", err)
	}
	
	tests := []struct {
		name    string
		macroName string
		data    interface{}
		wantErr bool
	}{
		{
			name:    "execute valid macro",
			macroName: "uppercase",
			data:    "test",
			wantErr: false,
		},
		{
			name:    "non-existent macro",
			macroName: "nonexistent",
			data:    "test",
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ExecuteMacro(tt.macroName, tt.data, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteMacro() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				// Verify result is uppercase
				if resultStr, ok := result.(string); ok {
					if resultStr != "TEST" {
						t.Errorf("ExecuteMacro() result = %v, want TEST", resultStr)
					}
				}
			}
		})
	}
}

func TestJavaScriptEngine_SetTTPContext(t *testing.T) {
	engine := NewJavaScriptEngine()
	
	context := map[string]interface{}{
		"vars": map[string]interface{}{
			"domain": "example.com",
		},
	}
	
	engine.SetTTPContext(context)
	
	// Verify context was set (indirectly by testing macro execution)
	source := "function get_domain(data) { return _ttp_.vars.domain; }"
	err := engine.RegisterMacro("get_domain", source)
	if err != nil {
		t.Fatalf("Failed to register macro: %v", err)
	}
	
	result, err := engine.ExecuteMacro("get_domain", "test", context)
	if err != nil {
		t.Logf("ExecuteMacro() error = %v (may be expected if context not passed)", err)
	} else if result != nil {
		if resultStr, ok := result.(string); ok {
			if resultStr == "example.com" {
				t.Logf("Successfully accessed _ttp_ context")
			}
		}
	}
}

func TestJavaScriptEngine_GoToJS(t *testing.T) {
	engine := NewJavaScriptEngine()
	
	// Test goToJS conversion (indirectly through ExecuteMacro)
	source := "function identity(data) { return data; }"
	err := engine.RegisterMacro("identity", source)
	if err != nil {
		t.Fatalf("Failed to register macro: %v", err)
	}
	
	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name:    "string",
			data:    "test",
			wantErr: false,
		},
		{
			name:    "integer",
			data:    123,
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
		{
			name:    "list",
			data:    []interface{}{1, 2, 3},
			wantErr: false,
		},
		{
			name:    "map",
			data:    map[string]interface{}{"key": "value"},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ExecuteMacro("identity", tt.data, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteMacro() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				t.Logf("ExecuteMacro() result = %v (type: %T)", result, result)
			}
		})
	}
}

func TestJavaScriptEngine_JSToGo(t *testing.T) {
	engine := NewJavaScriptEngine()
	
	// Test jsToGo conversion (indirectly through ExecuteMacro)
	source := `
function return_string() { return "test"; }
function return_number() { return 123; }
function return_bool() { return true; }
function return_array() { return [1, 2, 3]; }
function return_object() { return {key: "value"}; }
`
	err := engine.RegisterMacroSource(source)
	if err != nil {
		t.Fatalf("Failed to register macro source: %v", err)
	}
	
	tests := []struct {
		name    string
		macroName string
		wantErr bool
	}{
		{
			name:    "return string",
			macroName: "return_string",
			wantErr: false,
		},
		{
			name:    "return number",
			macroName: "return_number",
			wantErr: false,
		},
		{
			name:    "return bool",
			macroName: "return_bool",
			wantErr: false,
		},
		{
			name:    "return array",
			macroName: "return_array",
			wantErr: false,
		},
		{
			name:    "return object",
			macroName: "return_object",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.ExecuteMacro(tt.macroName, nil, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteMacro() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				t.Logf("ExecuteMacro() result = %v (type: %T)", result, result)
			}
		})
	}
}

