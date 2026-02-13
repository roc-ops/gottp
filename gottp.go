package gottp

import (
	"fmt"

	"github.com/roc-ops/gottp/api/python"
	"github.com/roc-ops/gottp/internal/compiled"
	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/functions/group"
	"github.com/roc-ops/gottp/internal/functions/input"
	"github.com/roc-ops/gottp/internal/functions/match"
	"github.com/roc-ops/gottp/internal/functions/output"
	"github.com/roc-ops/gottp/internal/macro"
	"github.com/roc-ops/gottp/internal/parser"
	"github.com/roc-ops/gottp/internal/validator"
	"github.com/roc-ops/gottp/internal/yang"
)

// ValidateTemplate validates a template string before compilation.
// Returns an error if the template has validation issues.
func ValidateTemplate(templateStr string) error {
	v := validator.NewValidator()
	result := v.ValidateTemplateString(templateStr, "")
	if !result.Valid {
		if len(result.Errors) > 0 {
			return result.Errors[0]
		}
		return fmt.Errorf("template validation failed")
	}
	return nil
}

// CompileTemplate compiles a TTP template text into a CompiledTemplate.
//
// The compiled template is immutable and stateless, allowing it to be used
// repeatedly without reset. It can be safely shared across goroutines.
//
// Example:
//
//	template := `<group name="test">{{ value }}</group>`
//	compiled, err := gottp.CompileTemplate(template)
//	if err != nil {
//		// handle error
//	}
func CompileTemplate(templateText string) (*CompiledTemplate, error) {
	// Parse template
	tmpl, err := parser.ParseTemplate(templateText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Compile template
	comp := compiler.NewCompiler()
	compiled, err := comp.CompileTemplate(tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to compile template: %w", err)
	}

	return &CompiledTemplate{compiled: compiled}, nil
}

// CompileOptions contains options for template compilation.
// Functions registered here are baked into the compiled template and available
// for all subsequent Parse() calls without needing to pass them in ParseOptions.
//
// Precedence: built-in functions < CompileOptions functions < ParseOptions functions
type CompileOptions struct {
	// Functions provides custom functions baked into the compiled template,
	// organized by scope. These functions are available for all Parse() calls
	// and can be overridden per-parse via ParseOptions.Functions.
	Functions *Functions
}

// CompileTemplateWithOptions compiles a TTP template text with compile-time options.
//
// Functions provided in CompileOptions are baked into the compiled template
// and available for all subsequent Parse() calls. They can be overridden
// per-parse via ParseOptions.Functions.
//
// Precedence for function resolution:
//   - built-in functions (lowest)
//   - CompileOptions.Functions (middle)
//   - ParseOptions.Functions (highest, per-parse)
//
// Example:
//
//	compiled, err := gottp.CompileTemplateWithOptions(template, &gottp.CompileOptions{
//		Functions: &gottp.Functions{
//			Match: map[string]gottp.MatchFunc{
//				"custom_transform": myTransformFunc,
//			},
//		},
//	})
func CompileTemplateWithOptions(templateText string, options *CompileOptions) (*CompiledTemplate, error) {
	// Parse template
	tmpl, err := parser.ParseTemplate(templateText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Compile template
	comp := compiler.NewCompiler()
	ct, err := comp.CompileTemplate(tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to compile template: %w", err)
	}

	result := &CompiledTemplate{compiled: ct}

	// Convert and store compile-time functions
	if options != nil && options.Functions != nil {
		result.compileFunctions = convertFunctions(options.Functions)
	}

	return result, nil
}

// CompiledTemplate is a stateless, immutable compiled template.
//
// Once compiled, a template can be used to parse multiple inputs without
// any state resets. The Parse() method is safe for concurrent use from
// multiple goroutines, though each goroutine should create its own Runtime
// instance (which happens automatically in Parse()).
//
// Compiled templates can be serialized and saved for later use, or embedded
// in Go code using the gottp-gen code generation tool.
type CompiledTemplate struct {
	compiled         *compiler.CompiledTemplate
	compileFunctions *compiled.RuntimeFunctions // compile-time custom function overrides from CompileOptions
}

// GetWarnings returns compilation warnings (non-fatal issues like Python-specific syntax in Starlark macros)
func (ct *CompiledTemplate) GetWarnings() []string {
	if ct.compiled == nil {
		return []string{}
	}
	return ct.compiled.Warnings
}

// ParseResult contains the parsed results and validation results
type ParseResult struct {
	Data             interface{}                          // Parsed data
	ValidationResults map[string]*yang.ValidationResult // YANG validation results by group name
	SourceMap        *SourceMap                          // Source map tracking input positions to results (optional)
}

// SourceMap tracks which parts of input text matched which template patterns
type SourceMap struct {
	Inputs map[string]*InputSourceMap // input name -> source map
}

// InputSourceMap contains source mapping for a single input
type InputSourceMap struct {
	Lines []*LineMapping // one entry per input line
}

// LineMapping represents the mapping for a single line of input
type LineMapping struct {
	LineNumber int            // 0-indexed line number
	Matched    bool           // whether this line matched
	Matches    []*MatchMapping // matches on this line
}

// MatchMapping represents a single match on a line
type MatchMapping struct {
	StartCol     int                    // start column (0-indexed)
	EndCol       int                    // end column (exclusive)
	GroupName    string                 // group name that matched
	PatternIndex int                    // pattern index within group
	Variables    map[string]*VarRange   // variable name -> character range
	ResultPath   string                 // path in result structure (e.g., "interfaces[0]")
}

// VarRange represents the character range for a variable within a match
type VarRange struct {
	StartCol int // start column (0-indexed)
	EndCol   int // end column (exclusive)
}

// Parse executes the compiled template with given inputs and variables.
//
// This method is stateless and can be called repeatedly without reset.
// It does not modify the CompiledTemplate instance, making it safe for
// concurrent use. However, for best performance with concurrent parsing,
// create a new CompiledTemplate instance for each goroutine or use the
// parallel parsing utilities.
//
// Parameters:
//   - inputs: Map of input names to input data strings
//   - vars: Map of variable names to variable values (can be nil)
//   - options: Parse options (can be nil for defaults)
//
// Returns:
//   - Parsed results as interface{} (typically map[string]interface{} or []interface{})
//   - error if parsing fails
//
// Example:
//
//	result, err := compiled.Parse(
//		gottp.Inputs{"Default_Input": "interface Loopback0\n ip address 1.1.1.1/24"},
//		gottp.Vars{"site": "datacenter1"},
//		nil,
//	)
func (ct *CompiledTemplate) Parse(inputs Inputs, vars Vars, options *ParseOptions) (interface{}, error) {
	result, err := ct.ParseWithValidation(inputs, vars, options)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// ParseWithValidation parses and returns both data and validation results
func (ct *CompiledTemplate) ParseWithValidation(inputs Inputs, vars Vars, options *ParseOptions) (*ParseResult, error) {
	runtime := compiled.NewRuntimeWithFunctions(ct.compiled, ct.compileFunctions)

	// Convert Inputs to map[string]string
	inputMap := make(map[string]string)
	for k, v := range inputs {
		inputMap[k] = v
	}

	// Convert Vars to map[string]interface{}
	varMap := make(map[string]interface{})
	for k, v := range vars {
		varMap[k] = v
	}

	// Handle YANG modules and source map option
	var opts *compiled.ParseOptions
	if options != nil {
		opts = &compiled.ParseOptions{
			EnableSourceMap: options.EnableSourceMap,
			Lookups:         options.Lookups,
			Vars:            options.Vars,
			Functions:       convertFunctions(options.Functions),
		}

		// Load YANG modules if provided
		if options.YANGModules != nil {
			moduleSet, err := loadYANGModules(options.YANGModules)
			if err != nil {
				return nil, fmt.Errorf("failed to load YANG modules: %w", err)
			}
			opts.YANGModuleSet = moduleSet
		}
	}

	data, internalSourceMap, err := runtime.ParseWithSourceMap(inputMap, varMap, opts)
	if err != nil {
		return nil, err
	}

	// Get validation results
	validationResults := runtime.GetValidationResults()
	if validationResults == nil {
		validationResults = make(map[string]*yang.ValidationResult)
	}

	// Convert internal source map to public source map
	var sourceMap *SourceMap
	if internalSourceMap != nil {
		sourceMap = convertSourceMap(internalSourceMap)
	}

	return &ParseResult{
		Data:             data,
		ValidationResults: validationResults,
		SourceMap:        sourceMap,
	}, nil
}

// Inputs represents input data for parsing
type Inputs map[string]string

// Vars represents template variables
type Vars map[string]interface{}

// loadYANGModules loads YANG modules from YANGModules structure
// All modules are parsed first, then processed together to resolve dependencies
func loadYANGModules(yangMods *YANGModules) (*yang.ModuleSet, error) {
	moduleSet := yang.NewModuleSet()
	
	// First, parse all modules (don't process yet to allow dependencies to be added)
	// We'll collect parse errors but continue to parse all modules
	var parseErrors []error
	
	// Load modules from strings
	if yangMods.Modules != nil {
		for name, content := range yangMods.Modules {
			if err := moduleSet.AddModule(name, content); err != nil {
				// Continue parsing other modules even if one fails
				// We'll process all together at the end
				parseErrors = append(parseErrors, fmt.Errorf("failed to add module '%s': %w", name, err))
			}
		}
	}
	
	// Load modules from files
	if yangMods.Files != nil {
		for _, filePath := range yangMods.Files {
			content, err := yang.LoadModuleFromFile(filePath)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("failed to load module from file '%s': %w", filePath, err))
				continue
			}
			// Extract module name from content
			moduleName, err := yang.ExtractModuleName(content)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("failed to extract module name from file '%s': %w", filePath, err))
				continue
			}
			if err := moduleSet.AddModule(moduleName, content); err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("failed to add module '%s' from file '%s': %w", moduleName, filePath, err))
			}
		}
	}
	
	// Load modules from URLs
	if yangMods.URLs != nil {
		for _, url := range yangMods.URLs {
			content, err := yang.LoadModuleFromURL(url)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("failed to load module from URL '%s': %w", url, err))
				continue
			}
			// Extract module name from content
			moduleName, err := yang.ExtractModuleName(content)
			if err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("failed to extract module name from URL '%s': %w", url, err))
				continue
			}
			if err := moduleSet.AddModule(moduleName, content); err != nil {
				parseErrors = append(parseErrors, fmt.Errorf("failed to add module '%s' from URL '%s': %w", moduleName, url, err))
			}
		}
	}
	
	// If we had parse errors, return them
	if len(parseErrors) > 0 {
		return nil, fmt.Errorf("failed to parse some YANG modules: %v", parseErrors)
	}
	
	// Now process all modules together to resolve dependencies
	// This ensures dependencies are available regardless of the order modules were added
	if err := moduleSet.ProcessAllModules(); err != nil {
		return nil, err
	}
	
	return moduleSet, nil
}

// YANGModules represents YANG modules for validation
type YANGModules struct {
	Modules map[string]string // name -> content
	Files   []string          // file paths
	URLs    []string          // URLs to fetch
}

// MatchFunc is the signature for custom match functions.
// Matches internal/functions/match.Function.
type MatchFunc func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)

// GroupFunc is the signature for custom group functions.
// Matches internal/functions/group.Function.
type GroupFunc func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)

// OutputFunc is the signature for custom output functions.
// Matches internal/functions/output.Function.
type OutputFunc func(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)

// InputFunc is the signature for custom input functions.
// Matches internal/functions/input.Function.
type InputFunc func(data string, args []string, kwargs map[string]interface{}) (string, bool, error)

// MacroFunc is the signature for custom Go macro functions.
// Matches internal/macro.GoMacroFunc.
type MacroFunc func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)

// Functions contains custom functions organized by scope.
type Functions struct {
	Match  map[string]MatchFunc
	Group  map[string]GroupFunc
	Output map[string]OutputFunc
	Input  map[string]InputFunc
	Macro  map[string]MacroFunc
}

// ParseOptions represents options for parsing
type ParseOptions struct {
	YANGModules     *YANGModules // YANG modules for validation
	EnableSourceMap bool          // Enable source map collection (zero overhead when false)

	// Lookups provides runtime lookup tables merged with compiled lookups at parse time.
	// Runtime lookups override compiled lookups with the same name.
	Lookups map[string]map[string]interface{}

	// Vars provides runtime variables merged with compiled template vars and Parse() vars.
	// Precedence: compiled vars < Parse() vars < ParseOptions.Vars.
	Vars map[string]interface{}

	// Functions provides custom functions injected at parse time, organized by scope.
	// Custom functions override built-in functions with the same name.
	Functions *Functions
}

// Runtime provides access to the underlying runtime for advanced operations
// such as registering native Go macros before parsing
type Runtime struct {
	runtime *compiled.Runtime
}

// MacroRegistry provides access to register native Go macros
type MacroRegistry struct {
	registry *macro.MacroRegistry
}

// RegisterGoMacro registers a native Go macro function.
// This allows high-performance macro execution without conversion overhead.
// The function signature matches group functions:
//
//	func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)
//
// Deprecated: Use CompileTemplateWithOptions with CompileOptions.Functions.Macro instead,
// which provides a cleaner API and proper precedence handling. RegisterGoMacro continues
// to work for backward compatibility.
func (mr *MacroRegistry) RegisterGoMacro(name string, fn func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)) {
	mr.registry.RegisterGoMacro(name, macro.GoMacroFunc(fn))
}

// GetMacroRegistry returns the macro registry for registering Go macros
// This allows you to register native Go macros for high-performance execution
func (r *Runtime) GetMacroRegistry() *MacroRegistry {
	return &MacroRegistry{
		registry: r.runtime.GetMacroRegistry(),
	}
}

// Parse executes the runtime with given inputs and variables
func (r *Runtime) Parse(inputs Inputs, vars Vars, options *ParseOptions) (interface{}, error) {
	result, err := r.ParseWithValidation(inputs, vars, options)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// ParseWithValidation parses and returns both data and validation results
func (r *Runtime) ParseWithValidation(inputs Inputs, vars Vars, options *ParseOptions) (*ParseResult, error) {
	// Convert Inputs to map[string]string
	inputMap := make(map[string]string)
	for k, v := range inputs {
		inputMap[k] = v
	}

	// Convert Vars to map[string]interface{}
	varMap := make(map[string]interface{})
	for k, v := range vars {
		varMap[k] = v
	}

	// Handle YANG modules and source map option
	var opts *compiled.ParseOptions
	if options != nil {
		opts = &compiled.ParseOptions{
			EnableSourceMap: options.EnableSourceMap,
			Lookups:         options.Lookups,
			Vars:            options.Vars,
			Functions:       convertFunctions(options.Functions),
		}

		// Load YANG modules if provided
		if options.YANGModules != nil {
			moduleSet, err := loadYANGModules(options.YANGModules)
			if err != nil {
				return nil, fmt.Errorf("failed to load YANG modules: %w", err)
			}
			opts.YANGModuleSet = moduleSet
		}
	}

	data, internalSourceMap, err := r.runtime.ParseWithSourceMap(inputMap, varMap, opts)
	if err != nil {
		return nil, err
	}

	// Get validation results
	validationResults := r.runtime.GetValidationResults()
	if validationResults == nil {
		validationResults = make(map[string]*yang.ValidationResult)
	}

	// Convert internal source map to public source map
	var sourceMap *SourceMap
	if internalSourceMap != nil {
		sourceMap = convertSourceMap(internalSourceMap)
	}

	return &ParseResult{
		Data:             data,
		ValidationResults: validationResults,
		SourceMap:        sourceMap,
	}, nil
}

// NewRuntime creates a reusable Runtime instance for a compiled template.
// This allows you to register Go macros and reuse the runtime for multiple parses.
//
// Deprecated: For registering custom macros, use CompileTemplateWithOptions with
// CompileOptions.Functions.Macro instead, which provides a cleaner API and proper
// precedence handling. The RegisterGoMacro API continues to work for backward
// compatibility.
func (ct *CompiledTemplate) NewRuntime() *Runtime {
	return &Runtime{
		runtime: compiled.NewRuntimeWithFunctions(ct.compiled, ct.compileFunctions),
	}
}

// NewParser creates a new Python-compatible parser (stateful API)
func NewParser() *PythonParser {
	return &PythonParser{
		parser: python.NewParser(),
	}
}

// PythonParser provides Python-compatible API
type PythonParser struct {
	parser *python.Parser
}

// AddTemplate adds a template
func (p *PythonParser) AddTemplate(template string) error {
	return p.parser.AddTemplate(template)
}

// AddInput adds input data
func (p *PythonParser) AddInput(data, inputName string) {
	p.parser.AddInput(data, inputName)
}

// AddVars adds variables
func (p *PythonParser) AddVars(vars Vars) {
	p.parser.AddVars(map[string]interface{}(vars))
}

// Parse parses all inputs
func (p *PythonParser) Parse() error {
	return p.parser.Parse()
}

// Result returns results
func (p *PythonParser) Result() []interface{} {
	return p.parser.Result()
}

// ClearInput clears inputs
func (p *PythonParser) ClearInput() {
	p.parser.ClearInput()
}

// ClearResult clears results
func (p *PythonParser) ClearResult() {
	p.parser.ClearResult()
}

// SaveCompiledTemplate saves a compiled template to bytes in the specified format
// Supported formats: "gob", "json", "yaml"
func SaveCompiledTemplate(ct *CompiledTemplate, format string) ([]byte, error) {
	var f compiler.SerializationFormat
	switch format {
	case "gob":
		f = compiler.FormatGob
	case "json":
		f = compiler.FormatJSON
	case "yaml":
		f = compiler.FormatYAML
	default:
		return nil, fmt.Errorf("unsupported format: %s (supported: gob, json, yaml)", format)
	}
	return compiler.SaveCompiledTemplateToBytes(ct.compiled, f)
}

// LoadCompiledTemplate loads a compiled template from bytes in the specified format
func LoadCompiledTemplate(data []byte, format string) (*CompiledTemplate, error) {
	var f compiler.SerializationFormat
	switch format {
	case "gob":
		f = compiler.FormatGob
	case "json":
		f = compiler.FormatJSON
	case "yaml":
		f = compiler.FormatYAML
	default:
		return nil, fmt.Errorf("unsupported format: %s (supported: gob, json, yaml)", format)
	}
	compiled, err := compiler.LoadCompiledTemplateFromBytes(data, f)
	if err != nil {
		return nil, err
	}
	return &CompiledTemplate{compiled: compiled}, nil
}

// SerializationFormat represents the format for serialization
type SerializationFormat = compiler.SerializationFormat

const (
	FormatGob  SerializationFormat = compiler.FormatGob
	FormatJSON SerializationFormat = compiler.FormatJSON
	FormatYAML SerializationFormat = compiler.FormatYAML
)

// SaveCompiledTemplateToBytes saves a compiled template to bytes
func SaveCompiledTemplateToBytes(ct *CompiledTemplate, format SerializationFormat) ([]byte, error) {
	return compiler.SaveCompiledTemplateToBytes(ct.compiled, format)
}

// LoadCompiledTemplateFromBytes loads a compiled template from bytes
func LoadCompiledTemplateFromBytes(data []byte, format SerializationFormat) (*CompiledTemplate, error) {
	compiled, err := compiler.LoadCompiledTemplateFromBytes(data, format)
	if err != nil {
		return nil, err
	}
	return &CompiledTemplate{compiled: compiled}, nil
}

// convertFunctions converts public Functions to internal RuntimeFunctions.
func convertFunctions(fns *Functions) *compiled.RuntimeFunctions {
	if fns == nil {
		return nil
	}
	rf := &compiled.RuntimeFunctions{}
	if fns.Match != nil {
		rf.Match = make(map[string]match.Function, len(fns.Match))
		for name, fn := range fns.Match {
			rf.Match[name] = match.Function(fn)
		}
	}
	if fns.Group != nil {
		rf.Group = make(map[string]group.Function, len(fns.Group))
		for name, fn := range fns.Group {
			rf.Group[name] = group.Function(fn)
		}
	}
	if fns.Output != nil {
		rf.Output = make(map[string]output.Function, len(fns.Output))
		for name, fn := range fns.Output {
			rf.Output[name] = output.Function(fn)
		}
	}
	if fns.Input != nil {
		rf.Input = make(map[string]input.Function, len(fns.Input))
		for name, fn := range fns.Input {
			rf.Input[name] = input.Function(fn)
		}
	}
	if fns.Macro != nil {
		rf.Macro = make(map[string]macro.GoMacroFunc, len(fns.Macro))
		for name, fn := range fns.Macro {
			rf.Macro[name] = macro.GoMacroFunc(fn)
		}
	}
	return rf
}

// convertSourceMap converts internal source map to public source map
func convertSourceMap(internal *compiled.SourceMap) *SourceMap {
	if internal == nil {
		return nil
	}

	public := &SourceMap{
		Inputs: make(map[string]*InputSourceMap),
	}

	for inputName, internalInput := range internal.Inputs {
		publicInput := &InputSourceMap{
			Lines: make([]*LineMapping, len(internalInput.Lines)),
		}

		for i, internalLine := range internalInput.Lines {
			publicLine := &LineMapping{
				LineNumber: internalLine.LineNumber,
				Matched:    internalLine.Matched,
				Matches:    make([]*MatchMapping, len(internalLine.Matches)),
			}

			for j, internalMatch := range internalLine.Matches {
				publicMatch := &MatchMapping{
					StartCol:     internalMatch.StartCol,
					EndCol:       internalMatch.EndCol,
					GroupName:    internalMatch.GroupName,
					PatternIndex: internalMatch.PatternIndex,
					ResultPath:   internalMatch.ResultPath,
					Variables:    make(map[string]*VarRange),
				}

				for varName, internalVarRange := range internalMatch.Variables {
					publicMatch.Variables[varName] = &VarRange{
						StartCol: internalVarRange.StartCol,
						EndCol:   internalVarRange.EndCol,
					}
				}

				publicLine.Matches[j] = publicMatch
			}

			publicInput.Lines[i] = publicLine
		}

		public.Inputs[inputName] = publicInput
	}

	return public
}

