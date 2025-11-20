package macro

import (
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

// JavaScriptEngine executes JavaScript macros
type JavaScriptEngine struct {
	vm        *goja.Runtime
	mu        sync.RWMutex
	macros    map[string]string // macro name -> source code
	ttpContext map[string]interface{}
}

// NewJavaScriptEngine creates a new JavaScript macro engine
func NewJavaScriptEngine() *JavaScriptEngine {
	vm := goja.New()
	return &JavaScriptEngine{
		vm:        vm,
		macros:    make(map[string]string),
		ttpContext: make(map[string]interface{}),
	}
}

// RegisterMacro registers a JavaScript macro function
func (e *JavaScriptEngine) RegisterMacro(name, source string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate the JavaScript code
	_, err := e.vm.RunString(source)
	if err != nil {
		return fmt.Errorf("failed to validate JavaScript macro %s: %w", name, err)
	}

	// Store the source
	e.macros[name] = source
	return nil
}

// RegisterMacroSource registers macro source code directly
func (e *JavaScriptEngine) RegisterMacroSource(source string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate the source
	_, err := e.vm.RunString(source)
	if err != nil {
		return fmt.Errorf("failed to validate JavaScript macro source: %w", err)
	}

	// Store source
	e.macros["_source_"] = source
	return nil
}

// ExecuteMacro executes a JavaScript macro function
func (e *JavaScriptEngine) ExecuteMacro(name string, data interface{}, ttpContext map[string]interface{}) (interface{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Create a new VM for this execution (thread-safe)
	vm := goja.New()

	// Set up _ttp_ context
	if ttpContext != nil {
		ttpObj := vm.NewObject()
		for k, v := range ttpContext {
			if err := ttpObj.Set(k, e.goToJS(vm, v)); err != nil {
				return nil, fmt.Errorf("failed to set _ttp_ context: %w", err)
			}
		}
		vm.Set("_ttp_", ttpObj)
	} else if e.ttpContext != nil {
		ttpObj := vm.NewObject()
		for k, v := range e.ttpContext {
			if err := ttpObj.Set(k, e.goToJS(vm, v)); err != nil {
				return nil, fmt.Errorf("failed to set _ttp_ context: %w", err)
			}
		}
		vm.Set("_ttp_", ttpObj)
	}

	// Find macro source
	var source string
	var found bool

	if s, ok := e.macros[name]; ok {
		source = s
		found = true
	} else if s, ok := e.macros["_source_"]; ok {
		source = s
		found = true
	}

	if !found {
		return nil, fmt.Errorf("macro %s not found", name)
	}

	// Execute the macro source
	_, err := vm.RunString(source)
	if err != nil {
		return nil, fmt.Errorf("failed to execute JavaScript macro: %w", err)
	}

	// Get the function
	fn, ok := goja.AssertFunction(vm.Get(name))
	if !ok {
		return nil, fmt.Errorf("function %s not found in macro", name)
	}

	// Convert data to JS value
	dataVal := e.goToJS(vm, data)

	// Call the function
	result, err := fn(goja.Undefined(), dataVal)
	if err != nil {
		return nil, fmt.Errorf("failed to call macro function %s: %w", name, err)
	}

	// Convert result back to Go value
	return e.jsToGo(result), nil
}

// goToJS converts a Go value to a JavaScript value
func (e *JavaScriptEngine) goToJS(vm *goja.Runtime, v interface{}) goja.Value {
	switch val := v.(type) {
	case string:
		return vm.ToValue(val)
	case int:
		return vm.ToValue(val)
	case int64:
		return vm.ToValue(val)
	case float64:
		return vm.ToValue(val)
	case bool:
		return vm.ToValue(val)
	case []interface{}:
		arr := vm.NewArray()
		for i, item := range val {
			arr.Set(fmt.Sprintf("%d", i), e.goToJS(vm, item))
		}
		return arr
	case []string:
		arr := vm.NewArray()
		for i, item := range val {
			arr.Set(fmt.Sprintf("%d", i), vm.ToValue(item))
		}
		return arr
	case map[string]interface{}:
		obj := vm.NewObject()
		for k, v := range val {
			obj.Set(k, e.goToJS(vm, v))
		}
		return obj
	case nil:
		return goja.Undefined()
	default:
		return vm.ToValue(fmt.Sprintf("%v", v))
	}
}

// jsToGo converts a JavaScript value to a Go value
func (e *JavaScriptEngine) jsToGo(v goja.Value) interface{} {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}

	exported := v.Export()
	
	// Handle common types
	switch val := exported.(type) {
	case string, int, int64, float64, bool, nil:
		return val
	case []interface{}:
		return val
	case map[string]interface{}:
		return val
	case map[interface{}]interface{}:
		// Convert map[interface{}]interface{} to map[string]interface{}
		result := make(map[string]interface{})
		for k, v := range val {
			result[fmt.Sprintf("%v", k)] = v
		}
		return result
	default:
		return exported
	}
}

// SetTTPContext sets the _ttp_ context dictionary
func (e *JavaScriptEngine) SetTTPContext(context map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ttpContext = context
}

