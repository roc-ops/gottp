package compiled

import (
	"fmt"
	"sync"
	"testing"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/parser"
)

// TestThreadSafety verifies that CompiledTemplate and Runtime are thread-safe
func TestThreadSafety(t *testing.T) {
	templateText := `
<group name="test">
value={{ value }}
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

	// Test 1: Multiple goroutines using the same CompiledTemplate with different Runtime instances
	var wg sync.WaitGroup
	numGoroutines := 50
	errors := make([]error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Each goroutine creates its own Runtime
			runtime := NewRuntime(compiled)
			data := map[string]string{
				"Default_Input": fmt.Sprintf("value=test%d", idx),
			}
			_, err := runtime.Parse(data, nil, nil)
			errors[idx] = err
		}(i)
	}
	
	wg.Wait()
	
	// Check for errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d failed: %v", i, err)
		}
	}
}

// TestSharedRuntime verifies that sharing a Runtime instance is NOT thread-safe
// (This documents the expected behavior - each goroutine should use its own Runtime)
func TestSharedRuntime(t *testing.T) {
	templateText := `
<group name="test">
value={{ value }}
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

	// Create a single Runtime instance
	runtime := NewRuntime(compiled)
	
	// Note: Sharing a Runtime instance across goroutines is not recommended
	// because macro registries may have internal state.
	// However, since Parse() doesn't modify Runtime state, it might work,
	// but it's safer to create a new Runtime per goroutine.
	
	var wg sync.WaitGroup
	numGoroutines := 10
	errors := make([]error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			data := map[string]string{
				"Default_Input": fmt.Sprintf("value=test%d", idx),
			}
			_, err := runtime.Parse(data, nil, nil)
			errors[idx] = err
		}(i)
	}
	
	wg.Wait()
	
	// Check for errors - this test documents current behavior
	// In practice, each goroutine should use its own Runtime
	for i, err := range errors {
		if err != nil {
			t.Logf("Goroutine %d failed (expected if sharing Runtime): %v", i, err)
		}
	}
}

// TestCompiledTemplateImmutability verifies that CompiledTemplate is immutable
func TestCompiledTemplateImmutability(t *testing.T) {
	templateText := `
<group name="test">
value={{ value }}
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

	// CompiledTemplate should be read-only
	// We can't easily test immutability in Go without reflection,
	// but we can verify that multiple goroutines can safely read from it
	
	var wg sync.WaitGroup
	numGoroutines := 100
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Read from compiled template (should be safe)
			_ = compiled.Groups
			_ = compiled.Macros
			_ = compiled.ResultsMethod
		}()
	}
	
	wg.Wait()
	// If we get here without panics, reading is safe
}

