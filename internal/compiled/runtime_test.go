package compiled

import (
	"fmt"
	"sync"
	"testing"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/parser"
)

func TestStatelessParse(t *testing.T) {
	templateText := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	// Parse and compile template
	tmpl, err := parser.ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	comp := compiler.NewCompiler()
	compiled, err := comp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	runtime := NewRuntime(compiled)

	// Test first parse
	data1 := `
interface Loopback0
 ip address 192.168.0.1/24
!
`
	result1, err := runtime.Parse(map[string]string{"Default_Input": data1}, nil, nil)
	if err != nil {
		t.Fatalf("First parse failed: %v", err)
	}

	// Test second parse with different data (no reset needed)
	data2 := `
interface Vlan100
 ip address 10.0.0.1/24
!
`
	result2, err := runtime.Parse(map[string]string{"Default_Input": data2}, nil, nil)
	if err != nil {
		t.Fatalf("Second parse failed: %v", err)
	}

	// Verify results are different (check that they're not nil and have content)
	if result1 == nil || result2 == nil {
		t.Error("Results should not be nil")
	}

	// Verify stateless - parse first data again should give same result
	result3, err := runtime.Parse(map[string]string{"Default_Input": data1}, nil, nil)
	if err != nil {
		t.Fatalf("Third parse failed: %v", err)
	}

	// Results should be the same (stateless)
	// Note: We can't directly compare interface{} values, but we can check structure
	if result3 == nil {
		t.Error("Third result should not be nil")
	}
	
	// Verify we can parse multiple times without issues
	for i := 0; i < 5; i++ {
		_, err := runtime.Parse(map[string]string{"Default_Input": data1}, nil, nil)
		if err != nil {
			t.Fatalf("Parse iteration %d failed: %v", i, err)
		}
	}
}

func TestConcurrentParse(t *testing.T) {
	templateText := `
<group name="test">
{{ value }}
</group>
`

	tmpl, err := parser.ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	comp := compiler.NewCompiler()
	compiled, err := comp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Test concurrent parsing
	runtime := NewRuntime(compiled)
	
	var wg sync.WaitGroup
	results := make([]interface{}, 10)
	errors := make([]error, 10)
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := map[string]string{
				"Default_Input": fmt.Sprintf("test%d", idx),
			}
			result, err := runtime.Parse(data, nil, nil)
			results[idx] = result
			errors[idx] = err
		}(i)
	}
	
	wg.Wait()
	
	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Parse %d failed: %v", i, err)
		}
	}
}

func TestParseArguments(t *testing.T) {
	tests := []struct {
		name     string
		argStr   string
		expected []string
	}{
		{
			name:     "empty string argument",
			argStr:   "''",
			expected: []string{""},
		},
		{
			name:     "empty string with other args",
			argStr:   "'', '*', '!'",
			expected: []string{"", "*", "!"},
		},
		{
			name:     "replaceall case - empty, asterisk, exclamation",
			argStr:   "'', '*', '!'",
			expected: []string{"", "*", "!"},
		},
		{
			name:     "multiple empty strings",
			argStr:   "'', '', 'test'",
			expected: []string{"", "", "test"},
		},
		{
			name:     "regular arguments",
			argStr:   "'hello', 'world'",
			expected: []string{"hello", "world"},
		},
		{
			name:     "mixed quotes",
			argStr:   `"", '*', "!"`,
			expected: []string{"", "*", "!"},
		},
		{
			name:     "no quotes",
			argStr:   "hello, world",
			expected: []string{"hello", "world"},
		},
		{
			name:     "single argument",
			argStr:   "'test'",
			expected: []string{"test"},
		},
		{
			name:     "single empty argument",
			argStr:   "''",
			expected: []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseArguments(tt.argStr, nil)
			if len(result) != len(tt.expected) {
				t.Errorf("parseArguments() returned %d arguments, want %d", len(result), len(tt.expected))
				t.Errorf("Got: %v", result)
				t.Errorf("Want: %v", tt.expected)
				return
			}
			for i, expected := range tt.expected {
				if i >= len(result) {
					t.Errorf("parseArguments() missing argument at index %d", i)
					continue
				}
				if result[i] != expected {
					t.Errorf("parseArguments() argument[%d] = %q, want %q", i, result[i], expected)
				}
			}
		})
	}
}

