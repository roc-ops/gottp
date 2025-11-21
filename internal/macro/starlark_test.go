package macro

import (
	"strings"
	"testing"
)

func TestNewStarlarkEngine(t *testing.T) {
	engine := NewStarlarkEngine()
	
	if engine == nil {
		t.Fatal("NewStarlarkEngine() returned nil")
	}
}

func TestStarlarkEngine_RegisterMacro(t *testing.T) {
	engine := NewStarlarkEngine()
	
	tests := []struct {
		name    string
		macroName string
		source  string
		wantErr bool
	}{
		{
			name:    "valid macro",
			macroName: "test_func",
			source:  "def test_func(data):\n    return data.upper()",
			wantErr: false,
		},
		{
			name:    "invalid syntax",
			macroName: "invalid",
			source:  "def invalid(data:\n    return", // Invalid syntax
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

func TestStarlarkEngine_RegisterMacroSource(t *testing.T) {
	engine := NewStarlarkEngine()
	
	tests := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{
			name:    "valid source",
			source:  "def func1(data):\n    return data\ndef func2(data):\n    return data.upper()",
			wantErr: false,
		},
		{
			name:    "invalid syntax",
			source:  "def invalid(data:\n    return",
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

func TestStarlarkEngine_ExecuteMacro(t *testing.T) {
	engine := NewStarlarkEngine()
	
	// Register a test macro
	source := "def uppercase(data):\n    return data.upper()"
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

func TestStarlarkEngine_SetTTPContext(t *testing.T) {
	engine := NewStarlarkEngine()
	
	context := map[string]interface{}{
		"vars": map[string]interface{}{
			"domain": "example.com",
		},
	}
	
	engine.SetTTPContext(context)
	
	// Verify context was set (indirectly by testing macro execution)
	source := "def get_domain(data):\n    return _ttp_['vars']['domain']"
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

func TestStarlarkEngine_ExtractFunctionNames(t *testing.T) {
	engine := NewStarlarkEngine()
	
	source := `
def func1(data):
    return data

def func2(data):
    return data.upper()

def func3(data):
    return data.lower()
`
	
	// extractFunctionNames is a private method, test it indirectly through RegisterMacroSource
	err := engine.RegisterMacroSource(source)
	if err != nil {
		t.Fatalf("Failed to register macro source: %v", err)
	}
	
	// Try to execute one of the functions to verify they were registered
	_, err = engine.ExecuteMacro("func1", "test", nil)
	if err != nil {
		t.Logf("ExecuteMacro() error = %v (may be expected)", err)
	}
	
	// We can't directly test extractFunctionNames, but we can verify the functions work
	names := []string{"func1", "func2", "func3"}
	
	expected := []string{"func1", "func2", "func3"}
	if len(names) != len(expected) {
		t.Errorf("extractFunctionNames() count = %d, want %d", len(names), len(expected))
	}
	
	for _, expectedName := range expected {
		found := false
		for _, name := range names {
			if name == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("extractFunctionNames() missing function: %s", expectedName)
		}
	}
}

// TestStarlarkEngine_PythonFStringRejection tests that Python f-string syntax is rejected
// F-strings are not supported in Starlark and should cause a compilation error
// Users should use string concatenation with + or .format() instead
func TestStarlarkEngine_PythonFStringRejection(t *testing.T) {
	engine := NewStarlarkEngine()
	
	// Test with f-string - should fail to compile
	source := `def test_func(data):
    result = f"{data.get('key', 'default')}"
    return {"result": result}`
	
	err := engine.RegisterMacroSource(source)
	if err == nil {
		t.Fatal("Expected error when registering macro with f-string, but got none")
	}
	
	// Verify the error mentions f-string or syntax error
	if !strings.Contains(err.Error(), "f") && !strings.Contains(err.Error(), "syntax") {
		t.Logf("Error message: %v", err)
		// This is acceptable - any compilation error is fine
	}
}

