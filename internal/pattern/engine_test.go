package pattern

import (
	"regexp"
	"strings"
	"testing"
)

func TestExtractVariables(t *testing.T) {
	engine := NewEngine()

	line := "interface {{ interface }} ip address {{ ip }}/{{ mask }}"
	variables := engine.ExtractVariables(line)

	if len(variables) != 3 {
		t.Fatalf("Expected 3 variables, got %d", len(variables))
	}

	if variables[0].Name != "interface" {
		t.Errorf("Expected variable name 'interface', got '%s'", variables[0].Name)
	}

	if variables[1].Name != "ip" {
		t.Errorf("Expected variable name 'ip', got '%s'", variables[1].Name)
	}

	if variables[2].Name != "mask" {
		t.Errorf("Expected variable name 'mask', got '%s'", variables[2].Name)
	}
}

func TestGenerateRegex(t *testing.T) {
	engine := NewEngine()

	line := "interface {{ interface }} ip address {{ ip }}/{{ mask }}"
	regexStr, variables, err := engine.GenerateRegex(line, false, false)
	if err != nil {
		t.Fatalf("Failed to generate regex: %v", err)
	}

	if len(variables) != 3 {
		t.Fatalf("Expected 3 variables, got %d", len(variables))
	}

	// Check that regex starts with ^ and ends with $
	if !strings.HasPrefix(regexStr, "^") {
		t.Error("Expected regex to start with ^")
	}
	if !strings.HasSuffix(regexStr, "$") {
		t.Error("Expected regex to end with $")
	}

	// Test that the regex matches expected input
	compiled, err := regexp.Compile(regexStr)
	if err != nil {
		t.Fatalf("Failed to compile generated regex: %v", err)
	}

	testLine := "interface Loopback0 ip address 192.168.0.1/24"
	if !compiled.MatchString(testLine) {
		t.Errorf("Generated regex does not match test line: %s", testLine)
	}
}

func TestCompilePattern(t *testing.T) {
	engine := NewEngine()

	line := "interface {{ interface }}"
	pattern, err := engine.CompilePattern(line, false, false)
	if err != nil {
		t.Fatalf("Failed to compile pattern: %v", err)
	}

	if pattern.Regex == nil {
		t.Error("Expected compiled regex to be non-nil")
	}

	if len(pattern.Variables) != 1 {
		t.Fatalf("Expected 1 variable, got %d", len(pattern.Variables))
	}

	if _, ok := pattern.Variables["interface"]; !ok {
		t.Error("Expected variable 'interface' to be in variables map")
	}
}

func TestBuiltinPatterns(t *testing.T) {
	engine := NewEngine()

	testCases := []struct {
		name     string
		expected string
	}{
		{"IP", PatternIP},
		{"WORD", PatternWORD},
		{"PHRASE", PatternPHRASE},
	}

	for _, tc := range testCases {
		pattern, ok := engine.GetBuiltinPattern(tc.name)
		if !ok {
			t.Errorf("Built-in pattern '%s' not found", tc.name)
			continue
		}
		if pattern != tc.expected {
			t.Errorf("Expected pattern '%s' to be '%s', got '%s'", tc.name, tc.expected, pattern)
		}
	}
}

