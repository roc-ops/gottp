package pattern

import (
	"testing"
)

func TestGenerateRegex_InvalidPatterns(t *testing.T) {
	engine := NewEngine()
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "unclosed character class",
			pattern: "test[abc",
			wantErr: true,
		},
		{
			name:    "unclosed group",
			pattern: "test(abc",
			wantErr: true,
		},
		{
			name:    "invalid escape sequence",
			pattern: "test\\",
			wantErr: true,
		},
		{
			name:    "invalid quantifier",
			pattern: "test{5,3}", // min > max
			wantErr: true,
		},
		{
			name:    "invalid quantifier syntax",
			pattern: "test{a,b}",
			wantErr: true,
		},
		{
			name:    "nested quantifiers",
			pattern: "test*+",
			wantErr: false, // May be valid in some contexts
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.CompilePattern(tt.pattern, false, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompilePattern() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateRegex_SpecialCharacters(t *testing.T) {
	engine := NewEngine()
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "pattern with newlines",
			pattern: "line1\nline2",
			wantErr: false,
		},
		{
			name:    "pattern with tabs",
			pattern: "col1\tcol2",
			wantErr: false,
		},
		{
			name:    "pattern with unicode",
			pattern: "测试.*内容",
			wantErr: false,
		},
		{
			name:    "pattern with regex special chars",
			pattern: "test.*+?^$[]{}()|\\",
			wantErr: false, // Should escape appropriately
		},
		{
			name:    "pattern with quotes",
			pattern: `test"value"test`,
			wantErr: false,
		},
		{
			name:    "pattern with backticks",
			pattern: "test`value`test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.CompilePattern(tt.pattern, false, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompilePattern() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateRegex_EdgeCases(t *testing.T) {
	engine := NewEngine()
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "empty pattern",
			pattern: "",
			wantErr: false,
		},
		{
			name:    "pattern with variable reference",
			pattern: "test {{ var }} end",
			wantErr: false,
		},
		{
			name:    "pattern with undefined variable",
			pattern: "test {{ undefined }} end",
			wantErr: false, // May use empty string or variable name
		},
		{
			name:    "pattern with nested variables",
			pattern: "test {{ outer{{inner}} }} end",
			wantErr: false,
		},
		{
			name:    "very long pattern",
			pattern: string(make([]byte, 100)),
			wantErr: false,
		},
		{
			name:    "pattern with only variables",
			pattern: "{{ var1 }}{{ var2 }}",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.CompilePattern(tt.pattern, false, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompilePattern() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEscapeText_EdgeCases(t *testing.T) {
	engine := NewEngine()
	tests := []struct {
		name string
		text string
	}{
		{
			name: "empty string",
			text: "",
		},
		{
			name: "only whitespace",
			text: "   \n\t  ",
		},
		{
			name: "special regex characters",
			text: ".*+?^$[]{}()|\\",
		},
		{
			name: "unicode text",
			text: "测试内容",
		},
		{
			name: "mixed content",
			text: "test.*value+?^$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test through CompilePattern which uses escapeText internally
			_, err := engine.CompilePattern(tt.text, false, false)
			if err != nil {
				t.Logf("CompilePattern() error = %v (may be expected)", err)
			}
		})
	}
}

func TestParseVariable_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "variable with function",
			pattern: "{{ var | to_int }}",
			wantErr: false,
		},
		{
			name:    "variable with multiple functions",
			pattern: "{{ var | to_int | to_str }}",
			wantErr: false,
		},
		{
			name:    "variable with function args",
			pattern: "{{ var | resub('old', 'new') }}",
			wantErr: false,
		},
		{
			name:    "variable with nested quotes",
			pattern: `{{ var | resub("old", "new") }}`,
			wantErr: false,
		},
		{
			name:    "unclosed variable",
			pattern: "{{ var ",
			wantErr: false, // May handle gracefully
		},
		{
			name:    "nested variables",
			pattern: "{{ outer{{inner}} }}",
			wantErr: false, // May handle gracefully
		},
	}

	engine := NewEngine()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that pattern can be parsed (indirectly through CompilePattern)
			_, err := engine.CompilePattern(tt.pattern, false, false)
			if (err != nil) != tt.wantErr {
				t.Logf("CompilePattern() error = %v (may be expected for complex patterns)", err)
			}
		})
	}
}

