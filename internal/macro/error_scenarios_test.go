package macro

import (
	"testing"
)

func TestStarlarkEngine_ErrorScenarios(t *testing.T) {
	engine := NewStarlarkEngine()

	tests := []struct {
		name    string
		setup   func() error
		execute func() error
		wantErr bool
	}{
		{
			name: "execute non-existent macro",
			setup: func() error {
				return nil
			},
			execute: func() error {
				_, err := engine.ExecuteMacro("nonexistent", "data", nil)
				return err
			},
			wantErr: true,
		},
		{
			name: "execute macro with runtime error",
			setup: func() error {
				source := "def error_func(data):\n    return data.nonexistent_attribute"
				return engine.RegisterMacro("error_func", source)
			},
			execute: func() error {
				_, err := engine.ExecuteMacro("error_func", "test", nil)
				return err
			},
			wantErr: true,
		},
		{
			name: "execute macro with type error",
			setup: func() error {
				source := "def type_error(data):\n    return data + 1" // String + int
				return engine.RegisterMacro("type_error", source)
			},
			execute: func() error {
				_, err := engine.ExecuteMacro("type_error", "test", nil)
				return err
			},
			wantErr: true,
		},
		{
			name: "register macro with syntax error",
			setup: func() error {
				source := "def invalid(data:\n    return" // Invalid syntax
				return engine.RegisterMacro("invalid", source)
			},
			execute: func() error {
				return nil
			},
			wantErr: true,
		},
		{
			name: "register macro with undefined function",
			setup: func() error {
				source := "def uses_undefined(data):\n    return undefined_function(data)"
				return engine.RegisterMacro("uses_undefined", source)
			},
			execute: func() error {
				_, err := engine.ExecuteMacro("uses_undefined", "test", nil)
				return err
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup()
			// If setup fails and we expected an error, that's OK (test passes)
			if tt.wantErr && err != nil {
				return
			}
			// If setup fails and we didn't expect an error, that's a problem
			if err != nil && !tt.wantErr {
				t.Errorf("Setup error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Setup succeeded, now test execute
			if err == nil {
				err = tt.execute()
				if (err != nil) != tt.wantErr {
					t.Errorf("Execute error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestJavaScriptEngine_ErrorScenarios(t *testing.T) {
	engine := NewJavaScriptEngine()

	tests := []struct {
		name    string
		setup   func() error
		execute func() error
		wantErr bool
	}{
		{
			name: "execute non-existent macro",
			setup: func() error {
				return nil
			},
			execute: func() error {
				_, err := engine.ExecuteMacro("nonexistent", "data", nil)
				return err
			},
			wantErr: true,
		},
		{
			name: "execute macro with runtime error",
			setup: func() error {
				// In JavaScript, accessing a property on null/undefined throws an error
				source := "function error_func(data) { return data.nonexistent.property; }"
				return engine.RegisterMacro("error_func", source)
			},
			execute: func() error {
				// Pass null/undefined to trigger error
				_, err := engine.ExecuteMacro("error_func", nil, nil)
				return err
			},
			wantErr: true,
		},
		{
			name: "execute macro with type error",
			setup: func() error {
				source := "function type_error(data) { return data + 1; }" // String + number
				return engine.RegisterMacro("type_error", source)
			},
			execute: func() error {
				_, err := engine.ExecuteMacro("type_error", "test", nil)
				return err
			},
			wantErr: false, // JavaScript handles type coercion
		},
		{
			name: "register macro with syntax error",
			setup: func() error {
				source := "function invalid(data { return" // Invalid syntax
				return engine.RegisterMacro("invalid", source)
			},
			execute: func() error {
				return nil
			},
			wantErr: true,
		},
		{
			name: "register macro with undefined function",
			setup: func() error {
				source := "function uses_undefined(data) { return undefinedFunction(data); }"
				return engine.RegisterMacro("uses_undefined", source)
			},
			execute: func() error {
				_, err := engine.ExecuteMacro("uses_undefined", "test", nil)
				return err
			},
			wantErr: true,
		},
		{
			name: "register macro with reference error",
			setup: func() error {
				source := "function ref_error(data) { return undefinedVariable; }"
				return engine.RegisterMacro("ref_error", source)
			},
			execute: func() error {
				_, err := engine.ExecuteMacro("ref_error", "test", nil)
				return err
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setup()
			// If setup fails and we expected an error, that's OK (test passes)
			if tt.wantErr && err != nil {
				return
			}
			// If setup fails and we didn't expect an error, that's a problem
			if err != nil && !tt.wantErr {
				t.Errorf("Setup error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Setup succeeded, now test execute
			if err == nil {
				err = tt.execute()
				if (err != nil) != tt.wantErr {
					t.Errorf("Execute error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestStarlarkEngine_InvalidMacroSource(t *testing.T) {
	engine := NewStarlarkEngine()

	tests := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{
			name:    "empty source",
			source:  "",
			wantErr: false, // Empty source might be valid
		},
		{
			name:    "invalid indentation",
			source:  "def func(data):\nreturn data", // Missing indentation
			wantErr: true,
		},
		{
			name:    "missing colon",
			source:  "def func(data)\n    return data",
			wantErr: true,
		},
		{
			name:    "invalid keyword",
			source:  "defin func(data):\n    return data",
			wantErr: true,
		},
		{
			name:    "unclosed string",
			source:  "def func(data):\n    return \"unclosed",
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

func TestJavaScriptEngine_InvalidMacroSource(t *testing.T) {
	engine := NewJavaScriptEngine()

	tests := []struct {
		name    string
		source  string
		wantErr bool
	}{
		{
			name:    "empty source",
			source:  "",
			wantErr: false, // Empty source might be valid
		},
		{
			name:    "missing closing brace",
			source:  "function func(data) { return data;",
			wantErr: true,
		},
		{
			name:    "invalid keyword",
			source:  "functio func(data) { return data; }",
			wantErr: true,
		},
		{
			name:    "unclosed string",
			source:  "function func(data) { return \"unclosed; }",
			wantErr: true,
		},
		{
			name:    "invalid operator",
			source:  "function func(data) { return data @ invalid; }",
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

func TestStarlarkEngine_MissingMacro(t *testing.T) {
	engine := NewStarlarkEngine()

	// Try to execute a macro that was never registered
	_, err := engine.ExecuteMacro("never_registered", "data", nil)
	if err == nil {
		t.Error("ExecuteMacro() expected error for missing macro, got nil")
	}
}

func TestJavaScriptEngine_MissingMacro(t *testing.T) {
	engine := NewJavaScriptEngine()

	// Try to execute a macro that was never registered
	_, err := engine.ExecuteMacro("never_registered", "data", nil)
	if err == nil {
		t.Error("ExecuteMacro() expected error for missing macro, got nil")
	}
}

func TestStarlarkEngine_ContextErrors(t *testing.T) {
	engine := NewStarlarkEngine()

	// Register a macro that uses _ttp_ context
	source := "def use_context(data):\n    return _ttp_['vars']['domain']"
	err := engine.RegisterMacro("use_context", source)
	if err != nil {
		t.Fatalf("Failed to register macro: %v", err)
	}

	// Execute without setting context
	_, err = engine.ExecuteMacro("use_context", "test", nil)
	if err == nil {
		t.Logf("ExecuteMacro() succeeded without context (may handle gracefully)")
	} else {
		t.Logf("ExecuteMacro() error = %v (expected for missing context)", err)
	}
}

func TestJavaScriptEngine_ContextErrors(t *testing.T) {
	engine := NewJavaScriptEngine()

	// Register a macro that uses _ttp_ context
	source := "function use_context(data) { return _ttp_.vars.domain; }"
	err := engine.RegisterMacro("use_context", source)
	if err != nil {
		t.Fatalf("Failed to register macro: %v", err)
	}

	// Execute without setting context
	_, err = engine.ExecuteMacro("use_context", "test", nil)
	if err == nil {
		t.Logf("ExecuteMacro() succeeded without context (may handle gracefully)")
	} else {
		t.Logf("ExecuteMacro() error = %v (expected for missing context)", err)
	}
}
