package macro

import (
	"fmt"
	"regexp"
	"sync"

	"go.starlark.net/starlark"
)

// StarlarkEngine executes Starlark macros
type StarlarkEngine struct {
	predeclared starlark.StringDict
	mu          sync.RWMutex
	cache       map[string]*starlark.Program // source hash -> compiled program
	macros      map[string]string            // macro name -> source code
	sourceCache map[string]*macroSourceInfo  // source hash -> function names and metadata
	funcToSource map[string]string           // function name -> source hash (for fast lookup)
	funcCache   map[string]starlark.Callable // function name -> cached callable (for performance)
}

// macroSourceInfo stores information about a macro source block
type macroSourceInfo struct {
	source      string
	functionNames []string
	program     *starlark.Program
}

// NewStarlarkEngine creates a new Starlark macro engine
func NewStarlarkEngine() *StarlarkEngine {
	predeclared := make(starlark.StringDict)
	// Add _ttp_ dictionary
	predeclared["_ttp_"] = starlark.NewDict(0)
	
	return &StarlarkEngine{
		predeclared:  predeclared,
		cache:        make(map[string]*starlark.Program),
		macros:       make(map[string]string),
		sourceCache:  make(map[string]*macroSourceInfo),
		funcToSource: make(map[string]string),
		funcCache:    make(map[string]starlark.Callable),
	}
}

// RegisterMacro registers a Starlark macro function
func (e *StarlarkEngine) RegisterMacro(name, source string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Parse and compile the Starlark code to validate it
	thread := &starlark.Thread{Name: "macro"}
	globals, err := starlark.ExecFile(thread, name, source, e.predeclared)
	if err != nil {
		return fmt.Errorf("failed to compile Starlark macro %s: %w", name, err)
	}

	// Extract function names from source
	functionNames := e.extractFunctionNames(source)
	
	// Note: Starlark doesn't have a separate Compile function that returns a Program
	// We'll cache the source and compile on execution, but extract function names now
	// The actual compilation happens in ExecuteMacro using ExecFile
	var program *starlark.Program
	// For now, we'll set program to nil and use source-based caching
	// The cache will store the source, and we'll compile on demand
	
	// Create source hash for caching
	sourceHash := e.hashSource(source)
	
	// Store source info
	e.sourceCache[sourceHash] = &macroSourceInfo{
		source:        source,
		functionNames: functionNames,
		program:       program,
	}
	
	// Map function names to source hash for fast lookup
	// Also cache the compiled functions for fast execution
	for _, fnName := range functionNames {
		e.funcToSource[fnName] = sourceHash
		// Cache the compiled function for fast execution
		if fn, ok := globals[fnName]; ok {
			if callable, ok := fn.(starlark.Callable); ok {
				e.funcCache[fnName] = callable
			}
		}
	}
	
	// Store the source for backward compatibility
	e.macros[name] = source
	
	// Store compiled program in cache
	e.cache[sourceHash] = program
	
	// Verify the function exists in globals
	if _, ok := globals[name]; !ok && len(functionNames) > 0 {
		// If name doesn't match, check if it's in the extracted function names
		found := false
		for _, fnName := range functionNames {
			if fnName == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("function %s not found in macro source", name)
		}
	}

	return nil
}

// RegisterMacroSource registers macro source code directly (used when we have the source)
func (e *StarlarkEngine) RegisterMacroSource(source string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate the source
	thread := &starlark.Thread{Name: "macro"}
	globals, err := starlark.ExecFile(thread, "<macro>", source, e.predeclared)
	if err != nil {
		return fmt.Errorf("failed to validate Starlark macro source: %w", err)
	}

	// Extract function names from source
	functionNames := e.extractFunctionNames(source)
	
	// Note: Starlark doesn't have a separate Compile function that returns a Program
	// We'll cache the source and compile on execution, but extract function names now
	// The actual compilation happens in ExecuteMacro using ExecFile
	var program *starlark.Program
	// For now, we'll set program to nil and use source-based caching
	// The cache will store the source, and we'll compile on demand
	
	// Create source hash for caching
	sourceHash := e.hashSource(source)
	
	// Store source info
	e.sourceCache[sourceHash] = &macroSourceInfo{
		source:        source,
		functionNames: functionNames,
		program:       program,
	}
	
	// Map function names to source hash for fast lookup
	for _, fnName := range functionNames {
		e.funcToSource[fnName] = sourceHash
	}
	
	// Store the source for backward compatibility
	e.macros["_source_"] = source
	
	// Store compiled program in cache
	e.cache[sourceHash] = program
	
	// Store globals for reference
	_ = globals

	return nil
}

// ExecuteMacroStarlark executes a Starlark macro on already-converted Starlark data
// This avoids Go↔Starlark conversion overhead when chaining macros
func (e *StarlarkEngine) ExecuteMacroStarlark(name string, dataVal starlark.Value, ttpContext map[string]interface{}) (starlark.Value, error) {
	e.mu.RLock()
	
	// Update _ttp_ context if provided
	if ttpContext != nil {
		ttpDict := starlark.NewDict(len(ttpContext))
		for k, v := range ttpContext {
			key := starlark.String(k)
			value := e.goToStarlark(v)
			ttpDict.SetKey(key, value)
		}
		e.predeclared["_ttp_"] = ttpDict
	}
	
	// Find macro source using cached lookup
	var source string
	var found bool
	
	// First try direct lookup by name
	if s, ok := e.macros[name]; ok {
		source = s
		found = true
	}
	
	// If not found, try function name to source mapping (for source blocks)
	if !found {
		if sourceHash, ok := e.funcToSource[name]; ok {
			if info, ok := e.sourceCache[sourceHash]; ok {
				source = info.source
				found = true
			}
		}
	}
	
	// Fallback to _source_ lookup
	if !found {
		if s, ok := e.macros["_source_"]; ok {
			source = s
			found = true
		}
	}
	
	e.mu.RUnlock()
	
	if !found {
		return nil, fmt.Errorf("macro %s not found", name)
	}

	// Try to use cached function first (major performance optimization)
	e.mu.RLock()
	cachedFunc, hasCache := e.funcCache[name]
	e.mu.RUnlock()
	
	var callable starlark.Callable
	var err error
	
	if hasCache {
		// Use cached function - no compilation needed!
		callable = cachedFunc
	} else {
		// Fallback: compile and execute (shouldn't happen if macro was registered properly)
		thread := &starlark.Thread{Name: "macro"}
		var globals starlark.StringDict
		
		// Execute the macro source (Starlark compiles on-the-fly with ExecFile)
		globals, err = starlark.ExecFile(thread, "<macro>", source, e.predeclared)
		if err != nil {
			return nil, fmt.Errorf("failed to execute Starlark macro: %w", err)
		}

		// Look for the function with the given name
		fn, ok := globals[name]
		if !ok {
			return nil, fmt.Errorf("function %s not found in macro", name)
		}

		// Get callable
		var ok2 bool
		callable, ok2 = fn.(starlark.Callable)
		if !ok2 {
			return nil, fmt.Errorf("%s is not callable", name)
		}
		
		// Cache it for next time
		e.mu.Lock()
		e.funcCache[name] = callable
		e.mu.Unlock()
	}

	// Create thread for function call (reuse if possible, but thread is lightweight)
	thread := &starlark.Thread{Name: "macro"}
	
	// Call the function with already-converted Starlark data
	result, err := starlark.Call(thread, callable, starlark.Tuple{dataVal}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call macro function %s: %w", name, err)
	}

	return result, nil
}

// ExecuteMacroStarlarkBatch executes multiple Starlark macros in sequence on the same data
// This is optimized for chaining macros without recreating threads
func (e *StarlarkEngine) ExecuteMacroStarlarkBatch(names []string, dataVal starlark.Value, ttpContext map[string]interface{}) (starlark.Value, error) {
	e.mu.RLock()
	
	// Update _ttp_ context once if provided
	if ttpContext != nil {
		ttpDict := starlark.NewDict(len(ttpContext))
		for k, v := range ttpContext {
			key := starlark.String(k)
			value := e.goToStarlark(v)
			ttpDict.SetKey(key, value)
		}
		e.predeclared["_ttp_"] = ttpDict
	}
	
	// Pre-load all callables to avoid repeated lookups
	callables := make([]starlark.Callable, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		
		// Try cached function first
		cachedFunc, hasCache := e.funcCache[name]
		if hasCache {
			callables = append(callables, cachedFunc)
			continue
		}
		
		// Find macro source
		var source string
		var found bool
		
		if s, ok := e.macros[name]; ok {
			source = s
			found = true
		} else if sourceHash, ok := e.funcToSource[name]; ok {
			if info, ok := e.sourceCache[sourceHash]; ok {
				source = info.source
				found = true
			}
		} else if s, ok := e.macros["_source_"]; ok {
			source = s
			found = true
		}
		
		if !found {
			e.mu.RUnlock()
			return nil, fmt.Errorf("macro %s not found", name)
		}
		
		// Compile and cache
		thread := &starlark.Thread{Name: "macro"}
		globals, err := starlark.ExecFile(thread, "<macro>", source, e.predeclared)
		if err != nil {
			e.mu.RUnlock()
			return nil, fmt.Errorf("failed to execute Starlark macro %s: %w", name, err)
		}
		
		fn, ok := globals[name]
		if !ok {
			e.mu.RUnlock()
			return nil, fmt.Errorf("function %s not found in macro", name)
		}
		
		callable, ok := fn.(starlark.Callable)
		if !ok {
			e.mu.RUnlock()
			return nil, fmt.Errorf("%s is not callable", name)
		}
		
		// Cache it
		e.mu.RUnlock()
		e.mu.Lock()
		e.funcCache[name] = callable
		e.mu.Unlock()
		e.mu.RLock()
		
		callables = append(callables, callable)
	}
	
	e.mu.RUnlock()
	
	// Reuse thread for all calls
	thread := &starlark.Thread{Name: "macro"}
	currentData := dataVal
	
	// Track which name index we're on (skip empty names)
	nameIdx := 0
	
	// Execute all macros in sequence
	for _, callable := range callables {
		// Find the corresponding name (skip empty ones)
		for nameIdx < len(names) && names[nameIdx] == "" {
			nameIdx++
		}
		macroName := "unknown"
		if nameIdx < len(names) {
			macroName = names[nameIdx]
			nameIdx++
		}
		
		result, err := starlark.Call(thread, callable, starlark.Tuple{currentData}, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to call macro function %s: %w", macroName, err)
		}
		
		// Check if result is a bool (filter)
		if boolVal, ok := result.(starlark.Bool); ok {
			if !bool(boolVal) {
				return starlark.None, nil // Signal filtering
			}
			// Keep original data, continue with next macro
			continue
		}
		
		currentData = result
	}
	
	return currentData, nil
}

// ExecuteMacro executes a Starlark macro function
func (e *StarlarkEngine) ExecuteMacro(name string, data interface{}, ttpContext map[string]interface{}) (interface{}, error) {
	e.mu.RLock()
	
	// Update _ttp_ context if provided
	if ttpContext != nil {
		ttpDict := starlark.NewDict(len(ttpContext))
		for k, v := range ttpContext {
			key := starlark.String(k)
			value := e.goToStarlark(v)
			ttpDict.SetKey(key, value)
		}
		e.predeclared["_ttp_"] = ttpDict
	}
	
	// Find macro source using cached lookup
	var source string
	var found bool
	
	// First try direct lookup by name
	if s, ok := e.macros[name]; ok {
		source = s
		found = true
	}
	
	// If not found, try function name to source mapping (for source blocks)
	if !found {
		if sourceHash, ok := e.funcToSource[name]; ok {
			if info, ok := e.sourceCache[sourceHash]; ok {
				source = info.source
				found = true
			}
		}
	}
	
	// Fallback to _source_ lookup
	if !found {
		if s, ok := e.macros["_source_"]; ok {
			source = s
			found = true
		}
	}
	
	e.mu.RUnlock()
	
	if !found {
		return nil, fmt.Errorf("macro %s not found", name)
	}

	// Try to use cached function first (major performance optimization)
	e.mu.RLock()
	cachedFunc, hasCache := e.funcCache[name]
	e.mu.RUnlock()
	
	var callable starlark.Callable
	var err error
	
	if hasCache {
		// Use cached function - no compilation needed!
		callable = cachedFunc
	} else {
		// Fallback: compile and execute (shouldn't happen if macro was registered properly)
		thread := &starlark.Thread{Name: "macro"}
		var globals starlark.StringDict
		
		// Execute the macro source (Starlark compiles on-the-fly with ExecFile)
		globals, err = starlark.ExecFile(thread, "<macro>", source, e.predeclared)
		if err != nil {
			return nil, fmt.Errorf("failed to execute Starlark macro: %w", err)
		}

		// Look for the function with the given name
		fn, ok := globals[name]
		if !ok {
			return nil, fmt.Errorf("function %s not found in macro", name)
		}

		// Get callable
		var ok2 bool
		callable, ok2 = fn.(starlark.Callable)
		if !ok2 {
			return nil, fmt.Errorf("%s is not callable", name)
		}
		
		// Cache it for next time
		e.mu.Lock()
		e.funcCache[name] = callable
		e.mu.Unlock()
	}

	// Convert data to Starlark value
	dataVal := e.goToStarlark(data)
	
	// Create thread for function call
	thread := &starlark.Thread{Name: "macro"}
	
	// Call the function
	result, err := starlark.Call(thread, callable, starlark.Tuple{dataVal}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call macro function %s: %w", name, err)
	}

	// Convert result back to Go value
	return e.starlarkToGo(result), nil
}

// SetTTPContext sets the _ttp_ context dictionary
func (e *StarlarkEngine) SetTTPContext(context map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Convert Go map to Starlark dict
	ttpDict := starlark.NewDict(len(context))
	for k, v := range context {
		key := starlark.String(k)
		value := e.goToStarlark(v)
		ttpDict.SetKey(key, value)
	}
	e.predeclared["_ttp_"] = ttpDict
}

// extractFunctionNames extracts function names from Starlark source code
func (e *StarlarkEngine) extractFunctionNames(source string) []string {
	var functionNames []string
	
	// Use regex to find function definitions
	// Pattern: def function_name( or def function_name (
	funcPattern := regexp.MustCompile(`\bdef\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	matches := funcPattern.FindAllStringSubmatch(source, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			functionNames = append(functionNames, match[1])
		}
	}
	
	return functionNames
}

// hashSource creates a simple hash of the source code for caching
func (e *StarlarkEngine) hashSource(source string) string {
	// Use a simple hash based on source content
	// In production, you might want to use a proper hash function like SHA256
	// For now, we'll use the source itself as the key (works for small sources)
	// This could be improved with crypto/sha256 for better performance with large sources
	return source
}

// GoToStarlark converts a Go value to a Starlark value (public for optimization)
func (e *StarlarkEngine) GoToStarlark(v interface{}) starlark.Value {
	return e.goToStarlark(v)
}

// StarlarkToGo converts a Starlark value to a Go value (public for optimization)
func (e *StarlarkEngine) StarlarkToGo(v starlark.Value) interface{} {
	return e.starlarkToGo(v)
}

// goToStarlark converts a Go value to a Starlark value
func (e *StarlarkEngine) goToStarlark(v interface{}) starlark.Value {
	switch val := v.(type) {
	case string:
		return starlark.String(val)
	case int:
		return starlark.MakeInt(val)
	case int64:
		return starlark.MakeInt64(val)
	case float64:
		return starlark.Float(val)
	case bool:
		return starlark.Bool(val)
	case []interface{}:
		list := make([]starlark.Value, len(val))
		for i, item := range val {
			list[i] = e.goToStarlark(item)
		}
		return starlark.NewList(list)
	case map[string]interface{}:
		dict := starlark.NewDict(len(val))
		for k, v := range val {
			dict.SetKey(starlark.String(k), e.goToStarlark(v))
		}
		return dict
	case nil:
		return starlark.None
	default:
		return starlark.String(fmt.Sprintf("%v", v))
	}
}

// starlarkToGo converts a Starlark value to a Go value
func (e *StarlarkEngine) starlarkToGo(v starlark.Value) interface{} {
	switch val := v.(type) {
	case starlark.String:
		return string(val)
	case starlark.Int:
		if i, ok := val.Int64(); ok {
			return i
		}
		return val.String()
	case starlark.Float:
		return float64(val)
	case starlark.Bool:
		return bool(val)
	case *starlark.List:
		result := make([]interface{}, val.Len())
		iter := val.Iterate()
		defer iter.Done()
		var item starlark.Value
		i := 0
		for iter.Next(&item) {
			result[i] = e.starlarkToGo(item)
			i++
		}
		return result
	case *starlark.Dict:
		result := make(map[string]interface{})
		for _, item := range val.Items() {
			key, ok := item[0].(starlark.String)
			if !ok {
				key = starlark.String(item[0].String())
			}
			result[string(key)] = e.starlarkToGo(item[1])
		}
		return result
	case starlark.NoneType:
		return nil
	default:
		return val.String()
	}
}

// GoMacroFunc represents a native Go macro function
// Uses the same signature as group functions for consistency:
// - data: the match data to process
// - args: function arguments (currently unused for macros, but available for future use)
// - kwargs: keyword arguments including template variables (available via _ttp_ context)
// Returns: (processed data, keep flag, error)
//   - keep: false means filter out this match, true means keep it
type GoMacroFunc func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)

// MacroRegistry manages macros for different languages
type MacroRegistry struct {
	starlark   *StarlarkEngine
	javascript *JavaScriptEngine
	python     PythonEngineInterface
	goMacros   map[string]GoMacroFunc // native Go macros
	mu         sync.RWMutex
}

// PythonEngineInterface defines the interface for Python macro engines
// This allows for optional Python support via build tags
type PythonEngineInterface interface {
	RegisterMacro(name, source string) error
	RegisterMacroSource(source string) error
	ExecuteMacro(name string, data interface{}, ttpContext map[string]interface{}) (interface{}, error)
	SetTTPContext(context map[string]interface{})
}

// NewMacroRegistry creates a new macro registry
func NewMacroRegistry() *MacroRegistry {
	reg := &MacroRegistry{
		starlark:   NewStarlarkEngine(),
		javascript: NewJavaScriptEngine(),
		goMacros:   make(map[string]GoMacroFunc),
	}
	reg.initPythonEngine()
	return reg
}

// initPythonEngine initializes the Python engine if available
func (r *MacroRegistry) initPythonEngine() {
	// This will be set by build tag-specific code
	// For now, use stub if Python support not compiled in
	r.python = NewPythonEngineStub()
}

// RegisterMacro registers a macro in the specified language
func (r *MacroRegistry) RegisterMacro(language, name, source string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch language {
	case "starlark", "":
		// If name is empty, register as source block
		if name == "" {
			return r.starlark.RegisterMacroSource(source)
		}
		return r.starlark.RegisterMacro(name, source)
	case "javascript", "js":
		// If name is empty, register as source block
		if name == "" {
			return r.javascript.RegisterMacroSource(source)
		}
		return r.javascript.RegisterMacro(name, source)
	case "python", "py":
		// If name is empty, register as source block
		if name == "" {
			return r.python.RegisterMacroSource(source)
		}
		return r.python.RegisterMacro(name, source)
	default:
		return fmt.Errorf("unsupported macro language: %s", language)
	}
}

// RegisterGoMacro registers a native Go macro function
// This allows users to register Go functions as macros for high-performance execution
func (r *MacroRegistry) RegisterGoMacro(name string, fn GoMacroFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.goMacros[name] = fn
}

// HasGoMacro checks if a native Go macro exists
func (r *MacroRegistry) HasGoMacro(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.goMacros[name]
	return ok
}

// ExecuteMacro executes a macro in the specified language
func (r *MacroRegistry) ExecuteMacro(language, name string, data interface{}, ttpContext map[string]interface{}) (interface{}, error) {
	r.mu.RLock()

	// PRIORITY: Check native Go macros first (fastest, no conversion overhead)
	if goMacro, ok := r.goMacros[name]; ok {
		r.mu.RUnlock()

		// Convert data to map[string]interface{} if needed
		dataMap, ok := data.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("Go macro %s requires map[string]interface{} data", name)
		}

		// Execute native Go macro (no conversion overhead!)
		result, keep, err := goMacro(dataMap, nil, ttpContext)
		if err != nil {
			return nil, fmt.Errorf("Go macro %s failed: %w", name, err)
		}

		// Handle keep flag (false means filter out this match)
		// Return false (bool) to signal filtering, matching Starlark macro behavior
		if !keep {
			return false, nil
		}

		return result, nil
	}

	r.mu.RUnlock()

	// Fall back to language-based macros (Starlark/Python/JS)
	switch language {
	case "starlark", "":
		return r.starlark.ExecuteMacro(name, data, ttpContext)
	case "javascript", "js":
		return r.javascript.ExecuteMacro(name, data, ttpContext)
	case "python", "py":
		return r.python.ExecuteMacro(name, data, ttpContext)
	default:
		return nil, fmt.Errorf("unsupported macro language: %s", language)
	}
}

// GetStarlarkEngine returns the Starlark engine for optimized macro chaining
func (r *MacroRegistry) GetStarlarkEngine() *StarlarkEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.starlark
}

// SetTTPContext sets the _ttp_ context for all macro engines
func (r *MacroRegistry) SetTTPContext(context map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.starlark.SetTTPContext(context)
	r.javascript.SetTTPContext(context)
	if r.python != nil {
		r.python.SetTTPContext(context)
	}
}

