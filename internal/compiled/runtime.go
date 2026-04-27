package compiled

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"go.starlark.net/starlark"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/functions/group"
	"github.com/roc-ops/gottp/internal/functions/input"
	"github.com/roc-ops/gottp/internal/functions/match"
	"github.com/roc-ops/gottp/internal/functions/output"
	"github.com/roc-ops/gottp/internal/macro"
	"github.com/roc-ops/gottp/internal/pattern"
	"github.com/roc-ops/gottp/internal/yang"
)

// Runtime executes compiled templates
type Runtime struct {
	compiled          *compiler.CompiledTemplate
	matchRegistry     *match.Registry
	groupRegistry     *group.Registry
	inputRegistry     *input.Registry
	outputRegistry    *output.Registry
	macroRegistry     *macro.MacroRegistry
	pathResolver      *PathResolver
	matchCollector    *MatchCollector
	yangValidator     *yang.Validator
	validationResults map[string]*yang.ValidationResult // group name -> validation result
	keyFields         map[string][]string              // group name -> key field names (from keys= attribute)
	recordedVars      map[string]interface{}            // global recorded variables (from record() function)
	runtimeLookups    map[string]map[string]interface{} // per-parse runtime lookup tables from ParseOptions
	runtimeFunctions  *RuntimeFunctions                 // per-parse custom function overrides from ParseOptions
	compileFunctions  *RuntimeFunctions                 // compile-time custom function overrides from CompileOptions
}

// NewRuntime creates a new runtime for a compiled template
func NewRuntime(compiled *compiler.CompiledTemplate) *Runtime {
	return NewRuntimeWithFunctions(compiled, nil)
}

// NewRuntimeWithFunctions creates a new runtime for a compiled template with compile-time functions.
// Compile-time functions sit between built-in functions and per-parse (runtime) functions in precedence:
//
//	built-ins < compileFunctions < runtimeFunctions (ParseOptions)
func NewRuntimeWithFunctions(compiled *compiler.CompiledTemplate, compileFns *RuntimeFunctions) *Runtime {
	rt := &Runtime{
		compiled:          compiled,
		matchRegistry:     match.NewRegistry(),
		groupRegistry:     group.NewRegistry(),
		inputRegistry:     input.NewRegistry(),
		outputRegistry:    output.NewRegistry(),
		macroRegistry:     macro.NewMacroRegistry(),
		pathResolver:      NewPathResolver(),
		matchCollector:    NewMatchCollector(),
		validationResults: make(map[string]*yang.ValidationResult),
		keyFields:         make(map[string][]string),
		compileFunctions:  compileFns,
	}

	// Register macros from compiled template
	for _, m := range compiled.Macros {
		// Register macro source (functions will be extracted on execution)
		// Don't ignore errors - if macro registration fails, we should know about it
		if err := rt.macroRegistry.RegisterMacro(m.Language, "", m.Source); err != nil {
			// Log the error but don't fail - allow template to continue
			// This matches Python TTP behavior where macro errors are handled gracefully
			// The macro simply won't be available if registration fails
			// TODO: Consider adding a logger here for debugging
			_ = err
		}
	}

	// Register compile-time macro functions
	if compileFns != nil && compileFns.Macro != nil {
		for name, fn := range compileFns.Macro {
			rt.macroRegistry.RegisterGoMacro(name, fn)
		}
	}

	return rt
}

// SetYANGModuleSet sets the YANG module set for validation
func (r *Runtime) SetYANGModuleSet(moduleSet *yang.ModuleSet) {
	if moduleSet != nil {
		r.yangValidator = yang.NewValidator(moduleSet)
	} else {
		r.yangValidator = nil
	}
}

// GetValidationResults returns all validation results
func (r *Runtime) GetValidationResults() map[string]*yang.ValidationResult {
	return r.validationResults
}

// GetKeyFields returns declared key fields for each group (from keys= attribute)
func (r *Runtime) GetKeyFields() map[string][]string {
	return r.keyFields
}

// collectGroupKeys recursively collects key fields from a group and all its
// nested sub-groups. This ensures DH's converter can resolve keys for nested
// lists (e.g., channel-entry inside qam-entry) via GetKeyFields().
func (r *Runtime) collectGroupKeys(group *compiler.CompiledGroup) {
	if keysAttr, ok := group.Attributes["keys"]; ok && keysAttr != "" {
		keys := strings.Split(keysAttr, ",")
		for i := range keys {
			keys[i] = strings.TrimSpace(keys[i])
		}
		groupName := strings.TrimSuffix(group.Name, "*")
		r.keyFields[groupName] = keys
	}
	for _, nested := range group.Groups {
		r.collectGroupKeys(nested)
	}
}

// GetMacroRegistry returns the macro registry for registering Go macros
func (r *Runtime) GetMacroRegistry() *macro.MacroRegistry {
	return r.macroRegistry
}

// getMatchFunc checks runtime overrides first, then compile-time overrides, then falls back to registry.
// Precedence: runtimeFunctions (ParseOptions) > compileFunctions (CompileOptions) > registry (built-ins)
func (r *Runtime) getMatchFunc(name string) (match.Function, bool) {
	if r.runtimeFunctions != nil && r.runtimeFunctions.Match != nil {
		if fn, ok := r.runtimeFunctions.Match[name]; ok {
			return fn, true
		}
	}
	if r.compileFunctions != nil && r.compileFunctions.Match != nil {
		if fn, ok := r.compileFunctions.Match[name]; ok {
			return fn, true
		}
	}
	return r.matchRegistry.Get(name)
}

// getGroupFunc checks runtime overrides first, then compile-time overrides, then falls back to registry.
// Precedence: runtimeFunctions (ParseOptions) > compileFunctions (CompileOptions) > registry (built-ins)
func (r *Runtime) getGroupFunc(name string) (group.Function, bool) {
	if r.runtimeFunctions != nil && r.runtimeFunctions.Group != nil {
		if fn, ok := r.runtimeFunctions.Group[name]; ok {
			return fn, true
		}
	}
	if r.compileFunctions != nil && r.compileFunctions.Group != nil {
		if fn, ok := r.compileFunctions.Group[name]; ok {
			return fn, true
		}
	}
	return r.groupRegistry.Get(name)
}

// getOutputFunc checks runtime overrides first, then compile-time overrides, then falls back to registry.
// Precedence: runtimeFunctions (ParseOptions) > compileFunctions (CompileOptions) > registry (built-ins)
func (r *Runtime) getOutputFunc(name string) (output.Function, bool) {
	if r.runtimeFunctions != nil && r.runtimeFunctions.Output != nil {
		if fn, ok := r.runtimeFunctions.Output[name]; ok {
			return fn, true
		}
	}
	if r.compileFunctions != nil && r.compileFunctions.Output != nil {
		if fn, ok := r.compileFunctions.Output[name]; ok {
			return fn, true
		}
	}
	return r.outputRegistry.Get(name)
}

// getInputFunc checks runtime overrides first, then compile-time overrides, then falls back to registry.
// Precedence: runtimeFunctions (ParseOptions) > compileFunctions (CompileOptions) > registry (built-ins)
func (r *Runtime) getInputFunc(name string) (input.Function, bool) {
	if r.runtimeFunctions != nil && r.runtimeFunctions.Input != nil {
		if fn, ok := r.runtimeFunctions.Input[name]; ok {
			return fn, true
		}
	}
	if r.compileFunctions != nil && r.compileFunctions.Input != nil {
		if fn, ok := r.compileFunctions.Input[name]; ok {
			return fn, true
		}
	}
	return r.inputRegistry.Get(name)
}

// RuntimeFunctions holds per-parse custom function overrides from ParseOptions.
type RuntimeFunctions struct {
	Match  map[string]match.Function
	Group  map[string]group.Function
	Output map[string]output.Function
	Input  map[string]input.Function
	Macro  map[string]macro.GoMacroFunc
}

// ParseOptions contains options for parsing
type ParseOptions struct {
	YANGModuleSet   *yang.ModuleSet                  // YANG modules for validation
	EnableSourceMap bool                              // Enable source map collection
	Lookups         map[string]map[string]interface{} // Runtime lookup tables
	Vars            map[string]interface{}            // Runtime variables
	Functions       *RuntimeFunctions                 // Custom function overrides
}

// Parse executes the compiled template with given inputs and variables.
//
// This method is stateless and can be called repeatedly without reset.
// It does not modify the Runtime instance or the CompiledTemplate.
//
// Thread-safety: This method is safe to call from a single goroutine.
// For concurrent parsing, create a new Runtime instance for each goroutine.
func (r *Runtime) Parse(inputs map[string]string, vars map[string]interface{}, options *ParseOptions) (interface{}, error) {
	// Clear previous validation results
	r.validationResults = make(map[string]*yang.ValidationResult)
	// Clear recorded vars (from record() function)
	r.recordedVars = make(map[string]interface{})

	// Clear runtime lookups
	r.runtimeLookups = nil

	// Clear runtime functions
	r.runtimeFunctions = nil

	// Set YANG module set if provided
	if options != nil && options.YANGModuleSet != nil {
		r.SetYANGModuleSet(options.YANGModuleSet)
	}

	// Set runtime lookups if provided
	if options != nil && options.Lookups != nil {
		r.runtimeLookups = options.Lookups
	}

	// Set runtime functions if provided
	if options != nil && options.Functions != nil {
		r.runtimeFunctions = options.Functions
	}

	// Re-register compile-time macro functions (restore baseline after previous Parse)
	if r.compileFunctions != nil && r.compileFunctions.Macro != nil {
		for name, fn := range r.compileFunctions.Macro {
			r.macroRegistry.RegisterGoMacro(name, fn)
		}
	}

	// Register runtime macro overrides (highest precedence)
	if r.runtimeFunctions != nil && r.runtimeFunctions.Macro != nil {
		for name, fn := range r.runtimeFunctions.Macro {
			r.macroRegistry.RegisterGoMacro(name, fn)
		}
	}

	// Merge template vars (from <vars> tag) with passed vars
	// Template vars are the base, passed vars override them
	if r.compiled.Vars != nil || vars != nil {
		mergedVars := make(map[string]interface{})
		// First add template vars
		if r.compiled.Vars != nil {
			for k, v := range r.compiled.Vars {
				mergedVars[k] = v
			}
		}
		// Then add passed vars (override template vars)
		for k, v := range vars {
			mergedVars[k] = v
		}
		vars = mergedVars
	}

	// Merge ParseOptions.Vars (highest precedence)
	if options != nil && options.Vars != nil {
		if vars == nil {
			vars = make(map[string]interface{})
		}
		for k, v := range options.Vars {
			vars[k] = v
		}
	}

	// Track all input names for per_input results method
	// Use a slice to preserve order (inputs from template first, then passed inputs)
	allInputNames := make(map[string]bool)
	inputOrder := make([]string, 0)

	// Track which inputs are referenced by groups (via input attribute)
	// Inputs that are only referenced by groups shouldn't create separate result entries
	inputsReferencedByGroups := make(map[string]bool)

	// First, identify which inputs are referenced by groups
	// Only count inputs that actually exist in the inputs map
	for _, group := range r.compiled.Groups {
		if !group.IsNested {
			inputName := group.Input
			if inputName == "" {
				inputName = "Default_Input"
			}
			// Only mark as referenced if the input actually exists
			if _, exists := inputs[inputName]; exists {
				inputsReferencedByGroups[inputName] = true
			}
		}
	}

	// Process input tags from template first (preserve template order)
	// This allows <input> tags in templates to provide data even when data is passed separately
	if r.compiled.Inputs != nil {
		for _, input := range r.compiled.Inputs {
			// Only process inputs with load="text" (embedded data)
			// The data is stored in the input's Data field after compilation
			if input.Load == "text" && input.Data != "" {
				// If input name already exists in inputs map, don't overwrite (passed data takes precedence)
				if _, exists := inputs[input.Name]; !exists {
					inputs[input.Name] = input.Data
				}
			}
		}
	}

	// Build inputOrder: preserve order from template inputs first, then passed inputs
	// Python TTP behavior: inputs are ordered as they appear in template, then passed inputs
	// All inputs are included in results, even if not referenced by groups (they get empty results)

	// First, add template inputs in the order they appear in the template
	if r.compiled.Inputs != nil {
		for _, input := range r.compiled.Inputs {
			// Check if this input exists (either from template or passed)
			if _, exists := inputs[input.Name]; exists {
				if !allInputNames[input.Name] {
					allInputNames[input.Name] = true
					inputOrder = append(inputOrder, input.Name)
				}
			}
		}
	}

	// Then add passed inputs that weren't already added (in the order they were passed)
	// We need to preserve the order from the inputs map, but Go maps don't preserve order
	// So we'll add them in a deterministic way: sorted by name for non-template inputs
	passedInputNames := make([]string, 0)
	for name := range inputs {
		if !allInputNames[name] {
			passedInputNames = append(passedInputNames, name)
		}
	}
	// Sort to ensure deterministic order (Python TTP may have different ordering, but this is consistent)
	sort.Strings(passedInputNames)
	for _, name := range passedInputNames {
		allInputNames[name] = true
		inputOrder = append(inputOrder, name)
	}

	// For per_input results method, track results per input
	// For per_template, use a single results map
	var results map[string]interface{}
	var resultsPerInput map[string]map[string]interface{} // input name -> results map

	if r.compiled.ResultsMethod == "per_input" {
		resultsPerInput = make(map[string]map[string]interface{})
		// Initialize results map for each input
		for inputName := range allInputNames {
			resultsPerInput[inputName] = make(map[string]interface{})
		}
	} else {
		results = make(map[string]interface{})
	}

	// Helper function to get the appropriate results map for storing group results
	getResultsMap := func(inputName string) map[string]interface{} {
		if r.compiled.ResultsMethod == "per_input" {
			// Get or create results map for this input
			if resultsMap, exists := resultsPerInput[inputName]; exists {
				return resultsMap
			}
			// Create new results map if input wasn't tracked yet
			resultsMap := make(map[string]interface{})
			resultsPerInput[inputName] = resultsMap
			return resultsMap
		}
		return results
	}

	// Process each group
	for _, group := range r.compiled.Groups {
		// Skip nested groups - they're processed within their parent group
		if group.IsNested {
			continue
		}

		// Collect key fields from keys= attribute (recursively includes nested groups)
		r.collectGroupKeys(group)

		// Determine which inputs to process
		// If group specifies input attribute, only process that input
		// If group doesn't specify input attribute, process all inputs
		var inputsToProcess []string
		if group.Input != "" {
			// Group specifies specific input(s) - parse comma-separated list
			inputNames := strings.Split(group.Input, ",")
			for _, name := range inputNames {
				name = strings.TrimSpace(name)
				if name != "" {
					inputsToProcess = append(inputsToProcess, name)
				}
			}
		} else {
			// Group doesn't specify input - process all inputs in order
			inputsToProcess = inputOrder
		}

		// Process group for each input
		for _, inputName := range inputsToProcess {
			// Get input data
			inputData, ok := inputs[inputName]
			if !ok {
				// Input not found - skip (Python TTP behavior)
				continue
			}

			// Process input functions if any
			processedInputData, err := r.processInputFunctions(inputName, inputData, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process input functions for %s: %w", inputName, err)
			}

			// Parse the input data with this group
			groupResults, err := r.parseGroup(group, processedInputData, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to parse group %s: %w", group.Name, err)
			}

			// Python TTP quirk: when ignore() uses a template variable, return one empty result
			hasIgnoreWithTemplateVar := false
			for _, compiledPattern := range group.Patterns {
				for _, variable := range compiledPattern.Variables {
					if variable.Name == "ignore" && variable.IgnoreUsesTemplateVar {
						hasIgnoreWithTemplateVar = true
						break
					}
				}
				if hasIgnoreWithTemplateVar {
					break
				}
			}

			if hasIgnoreWithTemplateVar {
				// Return one empty result to match Python TTP's behavior
				groupResults = []map[string]interface{}{{}}
			}

			// Check if group has void attribute - if so, skip saving results
			hasVoid := false
			if voidVal, ok := group.Attributes["void"]; ok {
				// void attribute is present - check if it's "true" or just present (empty string also means void)
				hasVoid = voidVal == "" || voidVal == "true" || voidVal == "True" || voidVal == "TRUE"
			}

			// Get the appropriate results map for this input
			resultsMap := getResultsMap(inputName)

			// Store results with dynamic path resolution (but skip if void attribute is set)
			if groupResults != nil && !hasVoid {
				if group.Name != "" {
					// Check if path is dynamic (contains {{ }})
					isDynamicPath := strings.Contains(group.Name, "{{")

					// Extract variables used in dynamic path (if any)
					// These variables should be removed from match results
					pathVars := r.pathResolver.ExtractVariablesFromPath(group.Name)
					pathVarSet := make(map[string]bool)
					for _, v := range pathVars {
						pathVarSet[v] = true
					}

					// Helper function to remove path variables from a match
					removePathVars := func(match map[string]interface{}) map[string]interface{} {
						if len(pathVarSet) == 0 {
							return match
						}
						cleaned := make(map[string]interface{})
						for k, v := range match {
							if !pathVarSet[k] {
								cleaned[k] = v
							}
						}
						return cleaned
					}

					// For dynamic group names, we need to handle them per match
					// If groupResults is a list, each item may need its own resolved path
					if matches, ok := groupResults.([]map[string]interface{}); ok {
						// Group results by resolved path
						groupedResults := make(map[string][]map[string]interface{})
						for _, match := range matches {
							// Resolve dynamic group name using match values
							resolvedName, err := r.pathResolver.ResolvePath(group.Name, match, vars)
							if err == nil && resolvedName != "" && resolvedName != group.Name {
								// Remove variables used in path from match
								cleanedMatch := removePathVars(match)
								groupedResults[resolvedName] = append(groupedResults[resolvedName], cleanedMatch)
							} else {
								// Fallback to original name
								cleanedMatch := removePathVars(match)
								groupedResults[group.Name] = append(groupedResults[group.Name], cleanedMatch)
							}
						}
						// Store grouped results using path-based storage
						// Python TTP: top-level groups - single match = map, multiple matches = list
						for resolvedName, matches := range groupedResults {
							var valueToStore interface{}
							if len(matches) == 1 {
								// Single match - store as map (matches Python TTP behavior for top-level groups)
								valueToStore = matches[0]
							} else {
								// Multiple matches - store as list
								// Convert to []interface{} for storage
								resultList := make([]interface{}, len(matches))
								for i, m := range matches {
									resultList[i] = m
								}
								valueToStore = resultList
							}

							// Store at path (handles dotted paths like "show.cable.modem*")
							// For groups without * or **, if we have multiple matches, we need to ensure they're stored as a list
							// Check if path has * or ** - if not and we have multiple matches, append * to make it a list
							pathToUse := resolvedName
							if len(matches) > 1 && !strings.HasSuffix(pathToUse, "*") && !strings.HasSuffix(pathToUse, "**") {
								// Multiple matches but no * formatter - append * to ensure list storage
								pathToUse = pathToUse + "*"
							}
							r.storeAtPath(resultsMap, pathToUse, valueToStore)
						}
					} else if singleMatch, ok := groupResults.(map[string]interface{}); ok {
						// Single match - resolve path using match values
						resolvedName, err := r.pathResolver.ResolvePath(group.Name, singleMatch, vars)
						cleanedMatch := removePathVars(singleMatch)
						if err == nil && resolvedName != "" && resolvedName != group.Name {
							// Dynamic path resolved - store as map (matches Python TTP behavior for top-level groups)
							r.storeAtPath(resultsMap, resolvedName, cleanedMatch)
						} else {
							// Non-dynamic path - store as map (single match)
							r.storeAtPath(resultsMap, group.Name, cleanedMatch)
						}
					} else {
						// Other types - just use original name
						resolvedName, err := r.pathResolver.ResolvePath(group.Name, nil, vars)
						if err == nil && resolvedName != "" && resolvedName != group.Name {
							// Dynamic path resolved - always store as list for dynamic paths
							if isDynamicPath {
								if valueList, ok := groupResults.([]interface{}); ok {
									r.storeAtPath(resultsMap, resolvedName, valueList)
								} else {
									r.storeAtPath(resultsMap, resolvedName, []interface{}{groupResults})
								}
							} else {
								r.storeAtPath(resultsMap, resolvedName, groupResults)
							}
						} else {
							r.storeAtPath(resultsMap, group.Name, groupResults)
						}
					}
				} else {
					// Anonymous group (no name) - Python TTP treats this as "_anonymous_*"
					// The * formatter ensures results are always a list
					// Store results using the _anonymous_* path
					// Store using _anonymous_* path (the * ensures it's stored as a list)
					r.storeAtPath(resultsMap, "_anonymous_*", groupResults)
				}
			}
		}
	}

	// Store vars with name attribute in result structure
	// These vars should be stored in each input's results
	for inputName, inputResults := range resultsPerInput {
		// Get the input data for this input to resolve getter functions
		var inputDataForGetter string
		if inputData, ok := inputs[inputName]; ok {
			inputDataForGetter = inputData
		}

		for _, varsWithName := range r.compiled.VarsWithName {
			// Resolve the path (may contain dynamic variables)
			// For vars with name, we need to resolve the path using the vars from this vars tag
			// The path can contain {{ }} for dynamic resolution (e.g., {{ hostname }})
			// Merge template vars with vars from this vars tag (vars from this tag take precedence)
			resolutionVars := make(map[string]interface{})
			// First add template vars
			for k, v := range vars {
				resolutionVars[k] = v
			}
			// Then add vars from this vars tag (override template vars)
			for k, v := range varsWithName.Vars {
				resolutionVars[k] = v
			}

			// Extract variables used in the path (these should be removed from stored vars)
			pathVars := r.pathResolver.ExtractVariablesFromPath(varsWithName.Name)
			pathVarSet := make(map[string]bool)
			for _, v := range pathVars {
				pathVarSet[v] = true
			}

			resolvedPath, err := r.pathResolver.ResolvePath(varsWithName.Name, nil, resolutionVars)
			if err != nil {
				// If resolution fails, use the original path
				resolvedPath = varsWithName.Name
			}
			if resolvedPath == "" {
				resolvedPath = varsWithName.Name
			}

			// Remove path variables from vars before storing (similar to group path variables)
			// Also resolve special getter functions like "gethostname"
			varsToStore := make(map[string]interface{})
			for k, v := range varsWithName.Vars {
				if !pathVarSet[k] {
					// Resolve special getter functions
					if strVal, ok := v.(string); ok {
						resolved := r.resolveGetterFunction(strVal, inputDataForGetter)
						varsToStore[k] = resolved
					} else {
						varsToStore[k] = v
					}
				}
			}

			// Store vars at the resolved path in the input's results
			r.storeAtPath(inputResults, resolvedPath, varsToStore)
		}
	}

	// Handle results method first to get the correct structure
	// Also handle anonymous groups - merge _anonymous_ results into root
	// Python TTP: anonymous groups are merged with the rest of the groups' results
	var resultsToFormat interface{}
	if r.compiled.ResultsMethod == "per_template" {
		if anonymousResults, ok := results["_anonymous_"]; ok {
			// If _anonymous_ is a list, that becomes the root result
			// Otherwise, merge it into results
			if anonymousList, ok := anonymousResults.([]interface{}); ok {
				// Anonymous group with multiple matches - the list is the result
				resultsToFormat = anonymousList
			} else {
				// Single match or other type - merge into results
				delete(results, "_anonymous_")
				if anonymousMap, ok := anonymousResults.(map[string]interface{}); ok {
					for k, v := range anonymousMap {
						results[k] = v
					}
				}
				resultsToFormat = results
			}
		} else {
			resultsToFormat = results
		}
	} else {
		// per_input - merge _anonymous_ into each input's results
		for _, inputResults := range resultsPerInput {
			if anonymousResults, ok := inputResults["_anonymous_"]; ok {
				// If _anonymous_ is a list, that becomes the input's result
				// Otherwise, merge it into inputResults
				if anonymousList, ok := anonymousResults.([]interface{}); ok {
					// Anonymous group with multiple matches - store the list for later use
					// We'll handle this when creating resultsList
					// Store it back in the map temporarily so we can access it
					inputResults["_anonymous_list"] = anonymousList
				} else {
					// Single match or other type - merge into inputResults
					delete(inputResults, "_anonymous_")
					if anonymousMap, ok := anonymousResults.(map[string]interface{}); ok {
						for k, v := range anonymousMap {
							inputResults[k] = v
						}
					}
				}
			}
		}

		// per_input - create a result for each input, even if empty
		// Create results list with one entry per input
		resultsList := make([]interface{}, 0)

		// Use inputOrder to preserve the order inputs appear in template
		// If we don't have order info, fall back to sorted names
		inputNamesList := inputOrder
		if len(inputNamesList) == 0 {
			// Fallback: collect and sort if we don't have order
			inputNamesList = make([]string, 0, len(allInputNames))
			for name := range allInputNames {
				inputNamesList = append(inputNamesList, name)
			}
			sort.Strings(inputNamesList)
		}

		for _, inputName := range inputNamesList {
			if inputResults, exists := resultsPerInput[inputName]; exists {
				// Check if this input has anonymous group results stored as a list
				// Anonymous groups with multiple matches are stored at "_anonymous_list" key
				if anonymousList, ok := inputResults["_anonymous_list"]; ok {
					if list, ok := anonymousList.([]interface{}); ok {
						// Anonymous group results - these should BE the entire result list
						// Don't append, just use the list directly as the result
						resultsToFormat = list
						goto skipResultsListBuild
					}
				}

				// This input had results - use its results map
				resultsList = append(resultsList, inputResults)
			} else {
				// Empty result for this input (matches Python TTP behavior)
				resultsList = append(resultsList, make(map[string]interface{}))
			}
		}

		resultsToFormat = resultsList
	skipResultsListBuild:
	}

	// Process output functions if any
	var finalResults interface{} = resultsToFormat
	if len(r.compiled.Outputs) > 0 {
		var err error
		finalResults, err = r.processOutputFunctions(resultsToFormat, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process output functions: %w", err)
		}
	}

	// Handle results method for return structure
	if r.compiled.ResultsMethod == "per_template" {
		return finalResults, nil
	}

	// per_input - return as list (already wrapped if formatted, or wrap if not)
	if _, isList := finalResults.([]interface{}); !isList {
		return []interface{}{finalResults}, nil
	}
	return finalResults, nil
}

// errStreamGate is the sentinel matched by errors.Is when the gate fires.
// The public TemplateNotStreamableError wraps this.
var errStreamGate = errors.New("template not streamable (internal sentinel)")

// streamGateError carries the per-group reasons through the package
// boundary. The public gottp wrapper translates this into
// *gottp.TemplateNotStreamableError in the next task.
type streamGateError struct {
	Reasons []string
}

func (e *streamGateError) Error() string        { return errStreamGate.Error() }
func (e *streamGateError) Is(target error) bool { return target == errStreamGate }
func (e *streamGateError) Unwrap() error        { return errStreamGate }

func wrapNotStreamableError(reasons []string) error {
	return &streamGateError{Reasons: reasons}
}

// StreamGateReasons returns the per-group reasons if err is a streamGateError,
// or nil otherwise. Used by the public wrapper to translate the internal
// sentinel into *gottp.TemplateNotStreamableError.
func StreamGateReasons(err error) []string {
	var e *streamGateError
	if errors.As(err, &e) {
		return e.Reasons
	}
	return nil
}

// ParseStream is the streaming counterpart to Parse. For each top-level
// group in the compiled template, it drives the streaming runtime and
// invokes fn once per completed record, in (group, scan) order.
//
// Returns a streamGateError (which the public wrapper translates to
// *gottp.TemplateNotStreamableError) without invoking fn if any top-level
// group is not streamable. Returning a non-nil error from fn aborts the
// parse and returns that error.
func (r *Runtime) ParseStream(
	inputs map[string]string,
	vars map[string]interface{},
	options *ParseOptions,
	fn func(record map[string]interface{}, srcRange [2]int, groupPath string) error,
) error {
	if r.compiled == nil {
		return fmt.Errorf("ParseStream: compiled template is nil")
	}
	if !r.compiled.Streamable {
		var reasons []string
		for _, g := range r.compiled.Groups {
			if !g.Streamable {
				reasons = append(reasons, g.NonStreamableReasons...)
			}
		}
		return wrapNotStreamableError(reasons)
	}

	// --- Setup: mirror Parse exactly ---

	// Clear previous validation results
	r.validationResults = make(map[string]*yang.ValidationResult)
	// Clear recorded vars (from record() function)
	r.recordedVars = make(map[string]interface{})

	// Clear runtime lookups
	r.runtimeLookups = nil

	// Clear runtime functions
	r.runtimeFunctions = nil

	// Set YANG module set if provided
	if options != nil && options.YANGModuleSet != nil {
		r.SetYANGModuleSet(options.YANGModuleSet)
	}

	// Set runtime lookups if provided
	if options != nil && options.Lookups != nil {
		r.runtimeLookups = options.Lookups
	}

	// Set runtime functions if provided
	if options != nil && options.Functions != nil {
		r.runtimeFunctions = options.Functions
	}

	// Re-register compile-time macro functions (restore baseline after previous Parse)
	if r.compileFunctions != nil && r.compileFunctions.Macro != nil {
		for name, fn := range r.compileFunctions.Macro {
			r.macroRegistry.RegisterGoMacro(name, fn)
		}
	}

	// Register runtime macro overrides (highest precedence)
	if r.runtimeFunctions != nil && r.runtimeFunctions.Macro != nil {
		for name, fn := range r.runtimeFunctions.Macro {
			r.macroRegistry.RegisterGoMacro(name, fn)
		}
	}

	// Merge template vars (from <vars> tag) with passed vars
	// Template vars are the base, passed vars override them
	if r.compiled.Vars != nil || vars != nil {
		mergedVars := make(map[string]interface{})
		if r.compiled.Vars != nil {
			for k, v := range r.compiled.Vars {
				mergedVars[k] = v
			}
		}
		for k, v := range vars {
			mergedVars[k] = v
		}
		vars = mergedVars
	}

	// Merge ParseOptions.Vars (highest precedence)
	if options != nil && options.Vars != nil {
		if vars == nil {
			vars = make(map[string]interface{})
		}
		for k, v := range options.Vars {
			vars[k] = v
		}
	}

	// Build inputOrder: same logic as Parse.
	allInputNames := make(map[string]bool)
	inputOrder := make([]string, 0)

	// Process input tags from template first (provide embedded text data)
	if r.compiled.Inputs != nil {
		for _, input := range r.compiled.Inputs {
			if input.Load == "text" && input.Data != "" {
				if _, exists := inputs[input.Name]; !exists {
					inputs[input.Name] = input.Data
				}
			}
		}
	}

	// Add template inputs in template order
	if r.compiled.Inputs != nil {
		for _, input := range r.compiled.Inputs {
			if _, exists := inputs[input.Name]; exists {
				if !allInputNames[input.Name] {
					allInputNames[input.Name] = true
					inputOrder = append(inputOrder, input.Name)
				}
			}
		}
	}

	// Add passed inputs not already in order (sorted for determinism)
	passedInputNames := make([]string, 0)
	for name := range inputs {
		if !allInputNames[name] {
			passedInputNames = append(passedInputNames, name)
		}
	}
	sort.Strings(passedInputNames)
	for _, name := range passedInputNames {
		allInputNames[name] = true
		inputOrder = append(inputOrder, name)
	}

	// --- Drive parseGroupStream for each top-level group ---

	for _, g := range r.compiled.Groups {
		// Skip nested groups — they're processed within their parent group.
		// (Streamable templates have none, but guard for safety.)
		if g.IsNested {
			continue
		}

		r.collectGroupKeys(g)

		// Determine which inputs to process (mirrors Parse exactly).
		var inputsToProcess []string
		if g.Input != "" {
			inputNames := strings.Split(g.Input, ",")
			for _, name := range inputNames {
				name = strings.TrimSpace(name)
				if name != "" {
					inputsToProcess = append(inputsToProcess, name)
				}
			}
		} else {
			inputsToProcess = inputOrder
		}

		for _, inputName := range inputsToProcess {
			inputData, ok := inputs[inputName]
			if !ok {
				continue
			}

			processedInputData, err := r.processInputFunctions(inputName, inputData, vars)
			if err != nil {
				return fmt.Errorf("failed to process input functions for %s: %w", inputName, err)
			}

			if err := r.parseGroupStream(g, processedInputData, vars, fn); err != nil {
				return err
			}
		}
	}

	return nil
}

// ParseWithSourceMap executes the compiled template and returns both data and source map
func (r *Runtime) ParseWithSourceMap(inputs map[string]string, vars map[string]interface{}, options *ParseOptions) (interface{}, *SourceMap, error) {
	// If source map is not enabled, just call Parse
	if options == nil || !options.EnableSourceMap {
		data, err := r.Parse(inputs, vars, options)
		return data, nil, err
	}

	// Create source map collector
	sourceMap := &SourceMap{
		Inputs: make(map[string]*InputSourceMap),
	}

	// Clear previous validation results
	r.validationResults = make(map[string]*yang.ValidationResult)
	// Clear recorded vars (from record() function)
	r.recordedVars = make(map[string]interface{})

	// Clear runtime lookups
	r.runtimeLookups = nil

	// Clear runtime functions
	r.runtimeFunctions = nil

	// Set YANG module set if provided
	if options != nil && options.YANGModuleSet != nil {
		r.SetYANGModuleSet(options.YANGModuleSet)
	}

	// Set runtime lookups if provided
	if options != nil && options.Lookups != nil {
		r.runtimeLookups = options.Lookups
	}

	// Set runtime functions if provided
	if options != nil && options.Functions != nil {
		r.runtimeFunctions = options.Functions
	}

	// Re-register compile-time macro functions (restore baseline after previous Parse)
	if r.compileFunctions != nil && r.compileFunctions.Macro != nil {
		for name, fn := range r.compileFunctions.Macro {
			r.macroRegistry.RegisterGoMacro(name, fn)
		}
	}

	// Register runtime macro overrides (highest precedence)
	if r.runtimeFunctions != nil && r.runtimeFunctions.Macro != nil {
		for name, fn := range r.runtimeFunctions.Macro {
			r.macroRegistry.RegisterGoMacro(name, fn)
		}
	}

	// Merge template vars (from <vars> tag) with passed vars
	// Template vars are the base, passed vars override them
	if r.compiled.Vars != nil || vars != nil {
		mergedVars := make(map[string]interface{})
		// First add template vars
		if r.compiled.Vars != nil {
			for k, v := range r.compiled.Vars {
				mergedVars[k] = v
			}
		}
		// Then add passed vars (override template vars)
		for k, v := range vars {
			mergedVars[k] = v
		}
		vars = mergedVars
	}

	// Merge ParseOptions.Vars (highest precedence)
	if options != nil && options.Vars != nil {
		if vars == nil {
			vars = make(map[string]interface{})
		}
		for k, v := range options.Vars {
			vars[k] = v
		}
	}

	// Track all input names for per_input results method
	// Use a slice to preserve order (inputs from template first, then passed inputs)
	allInputNames := make(map[string]bool)
	inputOrder := make([]string, 0)

	// Track which inputs are referenced by groups (via input attribute)
	// Inputs that are only referenced by groups shouldn't create separate result entries
	inputsReferencedByGroups := make(map[string]bool)

	// First, identify which inputs are referenced by groups
	// Only count inputs that actually exist in the inputs map
	for _, group := range r.compiled.Groups {
		if !group.IsNested {
			inputName := group.Input
			if inputName == "" {
				inputName = "Default_Input"
			}
			// Only mark as referenced if the input actually exists
			if _, exists := inputs[inputName]; exists {
				inputsReferencedByGroups[inputName] = true
			}
		}
	}

	// Process input tags from template first (preserve template order)
	// This allows <input> tags in templates to provide data even when data is passed separately
	if r.compiled.Inputs != nil {
		for _, input := range r.compiled.Inputs {
			// Only process inputs with load="text" (embedded data)
			// The data is stored in the input's Data field after compilation
			if input.Load == "text" && input.Data != "" {
				// If input name already exists in inputs map, don't overwrite (passed data takes precedence)
				if _, exists := inputs[input.Name]; !exists {
					inputs[input.Name] = input.Data
				}
			}
		}
	}

	// Build inputOrder: preserve order from template inputs first, then passed inputs
	// Python TTP behavior: inputs are ordered as they appear in template, then passed inputs
	// All inputs are included in results, even if not referenced by groups (they get empty results)

	// First, add template inputs in the order they appear in the template
	if r.compiled.Inputs != nil {
		for _, input := range r.compiled.Inputs {
			// Check if this input exists (either from template or passed)
			if _, exists := inputs[input.Name]; exists {
				if !allInputNames[input.Name] {
					allInputNames[input.Name] = true
					inputOrder = append(inputOrder, input.Name)
				}
			}
		}
	}

	// Then add passed inputs that weren't already added (in the order they were passed)
	// We need to preserve the order from the inputs map, but Go maps don't preserve order
	// So we'll add them in a deterministic way: sorted by name for non-template inputs
	passedInputNames := make([]string, 0)
	for name := range inputs {
		if !allInputNames[name] {
			passedInputNames = append(passedInputNames, name)
		}
	}
	// Sort to ensure deterministic order (Python TTP may have different ordering, but this is consistent)
	sort.Strings(passedInputNames)
	for _, name := range passedInputNames {
		allInputNames[name] = true
		inputOrder = append(inputOrder, name)
	}

	// Initialize source maps for all inputs
	for inputName := range allInputNames {
		inputData := inputs[inputName]
		lines := strings.Split(inputData, "\n")
		inputSourceMap := &InputSourceMap{
			Lines: make([]*LineMapping, len(lines)),
		}
		for i := range lines {
			inputSourceMap.Lines[i] = &LineMapping{
				LineNumber: i,
				Matched:    false,
				Matches:    make([]*MatchMapping, 0),
			}
		}
		sourceMap.Inputs[inputName] = inputSourceMap
	}

	// For per_input results method, track results per input
	// For per_template, use a single results map
	var results map[string]interface{}
	var resultsPerInput map[string]map[string]interface{} // input name -> results map

	if r.compiled.ResultsMethod == "per_input" {
		resultsPerInput = make(map[string]map[string]interface{})
		// Initialize results map for each input
		for inputName := range allInputNames {
			resultsPerInput[inputName] = make(map[string]interface{})
		}
	} else {
		results = make(map[string]interface{})
	}

	// Helper function to get the appropriate results map for storing group results
	getResultsMap := func(inputName string) map[string]interface{} {
		if r.compiled.ResultsMethod == "per_input" {
			// Get or create results map for this input
			if resultsMap, exists := resultsPerInput[inputName]; exists {
				return resultsMap
			}
			// Create new results map if input wasn't tracked yet
			resultsMap := make(map[string]interface{})
			resultsPerInput[inputName] = resultsMap
			return resultsMap
		}
		return results
	}

	// Process each group
	for _, group := range r.compiled.Groups {
		// Skip nested groups - they're processed within their parent group
		if group.IsNested {
			// Nested groups should never be processed as top-level groups
			// They are only processed within their parent group's context
			// If we see a nested group here, it shouldn't be in the top-level list
			// This is a bug - nested groups should only be in parent's Groups field
			continue
		}

		// Determine which input this group should parse
		inputName := group.Input
		if inputName == "" {
			inputName = "Default_Input"
		}

		// Get input data
		inputData, exists := inputs[inputName]
		if !exists {
			// Input doesn't exist - skip this group
			continue
		}

		// Parse group with source map collection
		groupResults, err := r.parseGroupWithSourceMap(group, inputData, vars, inputName, sourceMap, getResultsMap(inputName))
		if err != nil {
			return nil, nil, err
		}

		// Store group results using the same logic as parseGroup
		// Track the actual path strings used when storing
		resultsMap := getResultsMap(inputName)
		var actualResultPaths []string // Track paths that were actually used
		
		// Store results using the same logic as parseGroup (from Parse method)
		if groupResults != nil && group.Name != "" {
			// Check if path is dynamic (contains {{ }})
			isDynamicPath := strings.Contains(group.Name, "{{")
			
			// Extract variables used in dynamic path (if any)
			pathVars := r.pathResolver.ExtractVariablesFromPath(group.Name)
			pathVarSet := make(map[string]bool)
			for _, v := range pathVars {
				pathVarSet[v] = true
			}
			
			// Helper function to remove path variables from a match
			removePathVars := func(match map[string]interface{}) map[string]interface{} {
				if len(pathVarSet) == 0 {
					return match
				}
				cleaned := make(map[string]interface{})
				for k, v := range match {
					if !pathVarSet[k] {
						cleaned[k] = v
					}
				}
				return cleaned
			}
			
			// Store results using storeAtPath (same as parseGroup does)
			if matches, ok := groupResults.([]map[string]interface{}); ok {
				// Multiple matches - group by resolved path
				groupedResults := make(map[string][]map[string]interface{})
				for _, match := range matches {
					resolvedName, err := r.pathResolver.ResolvePath(group.Name, match, vars)
					if err == nil && resolvedName != "" && resolvedName != group.Name {
						cleanedMatch := removePathVars(match)
						groupedResults[resolvedName] = append(groupedResults[resolvedName], cleanedMatch)
					} else {
						cleanedMatch := removePathVars(match)
						groupedResults[group.Name] = append(groupedResults[group.Name], cleanedMatch)
					}
				}
				for resolvedName, matches := range groupedResults {
					var valueToStore interface{}
					if len(matches) == 1 {
						valueToStore = matches[0]
					} else {
						resultList := make([]interface{}, len(matches))
						for i, m := range matches {
							resultList[i] = m
						}
						valueToStore = resultList
					}
					pathToUse := resolvedName
					if len(matches) > 1 && !strings.HasSuffix(pathToUse, "*") && !strings.HasSuffix(pathToUse, "**") {
						pathToUse = pathToUse + "*"
					}
					r.storeAtPath(resultsMap, pathToUse, valueToStore)
					actualResultPaths = append(actualResultPaths, pathToUse)
				}
			} else if singleMatch, ok := groupResults.(map[string]interface{}); ok {
				// Single match
				resolvedName, err := r.pathResolver.ResolvePath(group.Name, singleMatch, vars)
				cleanedMatch := removePathVars(singleMatch)
				if err == nil && resolvedName != "" && resolvedName != group.Name {
					r.storeAtPath(resultsMap, resolvedName, cleanedMatch)
					actualResultPaths = append(actualResultPaths, resolvedName)
				} else {
					r.storeAtPath(resultsMap, group.Name, cleanedMatch)
					actualResultPaths = append(actualResultPaths, group.Name)
				}
			} else {
				// Other types
				resolvedName, err := r.pathResolver.ResolvePath(group.Name, nil, vars)
				if err == nil && resolvedName != "" && resolvedName != group.Name {
					if isDynamicPath {
						if valueList, ok := groupResults.([]interface{}); ok {
							r.storeAtPath(resultsMap, resolvedName, valueList)
						} else {
							r.storeAtPath(resultsMap, resolvedName, []interface{}{groupResults})
						}
					} else {
						r.storeAtPath(resultsMap, resolvedName, groupResults)
					}
					actualResultPaths = append(actualResultPaths, resolvedName)
				} else {
					r.storeAtPath(resultsMap, group.Name, groupResults)
					actualResultPaths = append(actualResultPaths, group.Name)
				}
			}
		} else if groupResults != nil && group.Name == "" {
			// Anonymous group
			r.storeAtPath(resultsMap, "_anonymous_*", groupResults)
			actualResultPaths = append(actualResultPaths, "_anonymous_*")
		}
		
		// If no paths tracked, use group name as fallback
		if len(actualResultPaths) == 0 && group.Name != "" {
			actualResultPaths = append(actualResultPaths, group.Name)
		} else if len(actualResultPaths) == 0 && group.Name == "" {
			actualResultPaths = append(actualResultPaths, "_anonymous_")
		}
		
		// Update result paths in source map with actual paths used
		if len(actualResultPaths) > 0 {
			inputSourceMap, exists := sourceMap.Inputs[inputName]
			if exists && inputSourceMap != nil {
				for _, lineMapping := range inputSourceMap.Lines {
					for _, matchMapping := range lineMapping.Matches {
						if matchMapping.GroupName == group.Name {
							// Use the first actual path (most common case)
							matchMapping.ResultPath = actualResultPaths[0]
						}
					}
				}
			}
		}

		// YANG validation
		if r.yangValidator != nil && groupResults != nil {
			if yangPath, ok := group.Attributes["yang"]; ok && yangPath != "" {
				validationResult := r.yangValidator.Validate(groupResults, yangPath, group.Name)
				r.validationResults[group.Name] = validationResult
			}
		}
	}

	// Process vars with name attribute (store in result structure)
	if r.compiled.VarsWithName != nil {
		for _, varsWithName := range r.compiled.VarsWithName {
			// Store vars in result structure at the specified path
			// For per_input, store in each input's results
			if r.compiled.ResultsMethod == "per_input" {
				for _, inputName := range inputOrder {
					resultsMap := getResultsMap(inputName)
					// Use storeAtPath to set nested values
					r.storeAtPath(resultsMap, varsWithName.Name, varsWithName.Vars)
				}
			} else {
				// For per_template, store in main results map
				r.storeAtPath(results, varsWithName.Name, varsWithName.Vars)
			}
		}
	}

	// Format results based on results method
	var resultsToFormat interface{}

	if r.compiled.ResultsMethod == "per_template" {
		// per_template - return single results map
		resultsToFormat = results
	} else {
		// per_input - handle anonymous groups
		// Anonymous groups with multiple matches need special handling
		for _, inputResults := range resultsPerInput {
			if anonymousResults, ok := inputResults["_anonymous_"]; ok {
				// If _anonymous_ is a list, that becomes the input's result
				// Otherwise, merge it into inputResults
				if anonymousList, ok := anonymousResults.([]interface{}); ok {
					// Anonymous group with multiple matches - store the list for later use
					// We'll handle this when creating resultsList
					// Store it back in the map temporarily so we can access it
					inputResults["_anonymous_list"] = anonymousList
				} else {
					// Single match or other type - merge into inputResults
					delete(inputResults, "_anonymous_")
					if anonymousMap, ok := anonymousResults.(map[string]interface{}); ok {
						for k, v := range anonymousMap {
							inputResults[k] = v
						}
					}
				}
			}
		}

		// per_input - create a result for each input, even if empty
		// Create results list with one entry per input
		resultsList := make([]interface{}, 0)

		// Use inputOrder to preserve the order inputs appear in template
		// If we don't have order info, fall back to sorted names
		inputNamesList := inputOrder
		if len(inputNamesList) == 0 {
			// Fallback: collect and sort if we don't have order
			inputNamesList = make([]string, 0, len(allInputNames))
			for name := range allInputNames {
				inputNamesList = append(inputNamesList, name)
			}
			sort.Strings(inputNamesList)
		}

		for _, inputName := range inputNamesList {
			if inputResults, exists := resultsPerInput[inputName]; exists {
				// Check if this input has anonymous group results stored as a list
				// Anonymous groups with multiple matches are stored at "_anonymous_list" key
				if anonymousList, ok := inputResults["_anonymous_list"]; ok {
					if list, ok := anonymousList.([]interface{}); ok {
						// Anonymous group results - these should BE the entire result list
						// Don't append, just use the list directly as the result
						resultsToFormat = list
						goto skipResultsListBuild
					}
				}

				// This input had results - use its results map
				resultsList = append(resultsList, inputResults)
			} else {
				// Empty result for this input (matches Python TTP behavior)
				resultsList = append(resultsList, make(map[string]interface{}))
			}
		}

		resultsToFormat = resultsList
	skipResultsListBuild:
	}

	// Process output functions if any
	var finalResults interface{} = resultsToFormat
	if len(r.compiled.Outputs) > 0 {
		var err error
		finalResults, err = r.processOutputFunctions(resultsToFormat, vars)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process output functions: %w", err)
		}
	}

	// Handle results method for return structure
	if r.compiled.ResultsMethod == "per_template" {
		return finalResults, sourceMap, nil
	}

	// per_input - return as list (already wrapped if formatted, or wrap if not)
	if _, isList := finalResults.([]interface{}); !isList {
		return []interface{}{finalResults}, sourceMap, nil
	}
	return finalResults, sourceMap, nil
}

// patternMatch represents a single regex match against the input with the
// metadata needed by the merge state machine. It used to be a function-local
// type inside parseGroup; lifted to package scope so the shared merge helper
// (stepMerge) can reference it.
type patternMatch struct {
	patternIdx int
	spanStart  int
	spanEnd    int
	lineIdx    int // line number (0-based) for line-based gap comparison
	result     map[string]interface{}
}

// mergeState carries the cross-match state that the parseGroup merge phase
// tracks while walking sorted matches. Streaming and non-streaming variants
// share the same state machine via stepMerge, so this struct holds every
// field that survives across iterations of the merge loop.
//
// The original mergeState owned a running mergedMatches accumulator and the
// state machine read len(mergedMatches) mid-iteration to drive decisions.
// To support streaming (where there is no accumulator at all), every append
// now flows through the flush callback and len(mergedMatches) reads have
// been replaced by recordCount. Batch mode sets flush to a closure that
// appends to a local slice; streaming mode sets flush to a closure that
// invokes the user callback.
type mergeState struct {
	currentMatch               map[string]interface{}
	currentStartPos            int
	currentStartLineIdx        int
	currentStartPatternIdx     int
	currentMatchHasEnd         bool
	patternMatchCount          map[int]int
	parentMatchToAllMatches    map[int][]int
	currentParentMatchStartIdx int

	// recordCount mirrors the count of records flushed so far. Always
	// incremented at every flush point. Replaces every place stepMerge
	// previously read len(mergedMatches) to drive a decision.
	recordCount int

	// flush is invoked at every place the original code did
	//   mergedMatches = append(mergedMatches, currentMatch)
	// Batch mode sets it to a closure that appends to a local slice.
	// Streaming mode sets it to a closure that invokes the user callback
	// (with srcRange computed at call time). Caller MUST set flush before
	// invoking stepMerge.
	flush func(record map[string]interface{}, srcRange [2]int) error

	// skipParentBookkeeping disables appending to parentMatchToAllMatches.
	// Streaming mode (parseGroupStream) sets this to true: streamable groups
	// have no nested children, so the parent->matches map is never read, and
	// keeping it would grow O(records). Setting this flag bounds heap usage.
	skipParentBookkeeping bool
}

// newMergeState constructs a fresh mergeState with the same initial values
// the inline merge loop used to set on its local variables. The caller MUST
// set state.flush before invoking stepMerge.
func newMergeState() *mergeState {
	return &mergeState{
		currentStartPos:            -1,
		currentStartLineIdx:        -1,
		currentStartPatternIdx:     -1,
		patternMatchCount:          make(map[int]int),
		parentMatchToAllMatches:    make(map[int][]int),
		currentParentMatchStartIdx: -1,
		recordCount:                0,
	}
}

// stepMerge advances the merge state machine by one match. It is a verbatim
// extraction of the per-iteration body of the original parseGroup merge loop:
// same conditions, same branch order, same edge cases, same side effects on
// r.pathResolver / r.matchCollector.
//
// Append-points: every place the original code did
// `mergedMatches = append(mergedMatches, ...)` now invokes state.flush with
// the record and srcRange = [currentStartPos, allMatches[matchIdx].spanStart]
// (i.e. the new match's start position is the end of the just-flushed record).
// Length reads: every place the original code read len(mergedMatches) for a
// decision now reads state.recordCount, which is incremented immediately
// after each successful flush. parentIdx assignments use state.recordCount
// captured BEFORE the flush, matching the pre-increment len() semantics.
//
// Read this function as the original loop body with `currentMatch` etc.
// rewritten to `state.currentMatch`, append-to-mergedMatches rewritten to
// state.flush, and pre-computed booleans like `hasLineIndicator` passed as
// arguments. Each `return nil` corresponds to a `continue` in the original
// loop.
func (r *Runtime) stepMerge(
	state *mergeState,
	allMatches []patternMatch,
	matchIdx int,
	group *compiler.CompiledGroup,
	joinMatchesVars map[string]bool,
	joinMatchesHasToList map[string]bool,
	startPatterns map[int]bool,
	endPatterns map[int]bool,
	hasLineIndicator bool,
	hasAnyStartIndicator bool,
	hasEmptyStartPattern bool,
) error {
	match := allMatches[matchIdx]
	const maxGapLines = 100

	isStartPattern := startPatterns[match.patternIdx]
	isEndPattern := endPatterns[match.patternIdx]

	// Declare shouldMerge here so it's available in both branches
	shouldMerge := false

	if isStartPattern {
		// Start pattern - save previous match and start new one
		shouldStartNewMatch := true
		if hasLineIndicator && match.patternIdx == 0 && state.currentMatch != nil {
			shouldStartNewMatch = false
		} else if hasAnyStartIndicator && state.currentMatch != nil {
			hasEndPatterns := len(endPatterns) > 0
			isStartPatternVar := false
			if match.patternIdx >= 0 && match.patternIdx < len(group.Patterns) {
				pattern := group.Patterns[match.patternIdx]
				for varName, variable := range pattern.Variables {
					if varName == "_start_" {
						isStartPatternVar = true
						break
					}
					for _, funcStr := range variable.Functions {
						if funcStr == "_start_" {
							isStartPatternVar = true
							break
						}
					}
					if isStartPatternVar {
						break
					}
				}
			}
			currentMatchStartedByStart := false
			if state.currentStartPatternIdx >= 0 && state.currentStartPatternIdx < len(group.Patterns) {
				startPattern := group.Patterns[state.currentStartPatternIdx]
				for varName, variable := range startPattern.Variables {
					if varName == "_start_" {
						currentMatchStartedByStart = true
						break
					}
					for _, funcStr := range variable.Functions {
						if funcStr == "_start_" {
							currentMatchStartedByStart = true
							break
						}
					}
					if currentMatchStartedByStart {
						break
					}
				}
				if !currentMatchStartedByStart {
					for _, variable := range startPattern.Variables {
						for _, funcStr := range variable.Functions {
							if funcStr == "_start_" {
								currentMatchStartedByStart = true
								break
							}
						}
						if currentMatchStartedByStart {
							break
						}
					}
				}
			}
			if isStartPatternVar && (hasEndPatterns || hasAnyStartIndicator) {
				hasPatternsToMergeOnSameLine := false
				for i := matchIdx + 1; i < len(allMatches); i++ {
					nextMatch := allMatches[i]
					if nextMatch.spanStart != match.spanStart {
						break
					}
					nextIsEnd := endPatterns[nextMatch.patternIdx]
					nextIsStartPatternVar := false
					if nextMatch.patternIdx >= 0 && nextMatch.patternIdx < len(group.Patterns) {
						nextPattern := group.Patterns[nextMatch.patternIdx]
						for varName := range nextPattern.Variables {
							if varName == "_start_" {
								nextIsStartPatternVar = true
								break
							}
						}
					}
					if !nextIsEnd && !nextIsStartPatternVar {
						hasPatternsToMergeOnSameLine = true
						break
					}
				}
				if state.currentMatch == nil {
					shouldStartNewMatch = true
				} else if hasPatternsToMergeOnSameLine {
					shouldStartNewMatch = true
				} else if !state.currentMatchHasEnd {
					shouldStartNewMatch = true
				}
			} else if !isStartPatternVar && hasEndPatterns && !state.currentMatchHasEnd {
				shouldStartNewMatch = false
			} else if !isStartPatternVar && currentMatchStartedByStart {
				shouldStartNewMatch = false
			} else if match.patternIdx == state.currentStartPatternIdx {
				count := state.patternMatchCount[match.patternIdx]
				isStartPatternVar := false
				isStartPattern := false
				if match.patternIdx >= 0 && match.patternIdx < len(group.Patterns) {
					pattern := group.Patterns[match.patternIdx]
					for varName := range pattern.Variables {
						if varName == "_start_" {
							isStartPatternVar = true
							isStartPattern = true
							break
						}
					}
					if !isStartPattern {
						for _, variable := range pattern.Variables {
							for _, funcStr := range variable.Functions {
								if funcStr == "_start_" {
									isStartPattern = true
									break
								}
							}
							if isStartPattern {
								break
							}
						}
					}
				}
				_ = isStartPattern
				if isStartPatternVar {
					if hasEndPatterns && state.currentMatch != nil && !state.currentMatchHasEnd && count == 0 {
						shouldStartNewMatch = false
					} else {
						shouldStartNewMatch = true
					}
				} else if count == 0 {
					if hasEndPatterns && !state.currentMatchHasEnd {
						shouldStartNewMatch = false
					} else {
						shouldStartNewMatch = true
					}
				} else if hasEndPatterns && !state.currentMatchHasEnd {
					shouldStartNewMatch = false
				} else {
					shouldStartNewMatch = true
				}
			} else {
				if currentMatchStartedByStart || (hasEndPatterns && !state.currentMatchHasEnd) {
					shouldStartNewMatch = false
				} else if hasLineIndicator {
					if match.lineIdx-state.currentStartLineIdx >= 0 && match.lineIdx-state.currentStartLineIdx < maxGapLines {
						shouldStartNewMatch = false
					} else {
						shouldStartNewMatch = true
					}
				} else if match.lineIdx-state.currentStartLineIdx >= 0 && match.lineIdx-state.currentStartLineIdx < maxGapLines {
					shouldStartNewMatch = false
				} else if state.currentStartPos == -1 {
					shouldStartNewMatch = false
				}
			}
		} else if hasLineIndicator && state.currentMatch != nil {
			if match.patternIdx == state.currentStartPatternIdx {
				shouldStartNewMatch = false
			} else if isStartPattern {
				if match.lineIdx-state.currentStartLineIdx >= 0 && match.lineIdx-state.currentStartLineIdx < maxGapLines {
					shouldStartNewMatch = false
				} else {
					shouldStartNewMatch = true
				}
			} else {
				count := state.patternMatchCount[match.patternIdx]
				patternHasJoinMatches := false
				for varName := range match.result {
					if joinMatchesVars[varName] {
						patternHasJoinMatches = true
						break
					}
				}
				if count > 0 && !patternHasJoinMatches {
					shouldStartNewMatch = true
				} else if match.lineIdx-state.currentStartLineIdx >= 0 && match.lineIdx-state.currentStartLineIdx < maxGapLines {
					shouldStartNewMatch = false
				} else {
					if state.recordCount > 0 {
						shouldStartNewMatch = true
					} else {
						shouldStartNewMatch = true
					}
				}
			}
		}

		// Check if this start pattern has joinmatches
		patternHasJoinMatches := false
		for varName := range match.result {
			if joinMatchesVars[varName] {
				patternHasJoinMatches = true
				break
			}
		}

		if patternHasJoinMatches && state.currentMatch != nil {
			isSameInstance := true
			hasNonJoinMatchesVars := false
			for k, v := range match.result {
				if !joinMatchesVars[k] {
					hasNonJoinMatchesVars = true
					if existing, ok := state.currentMatch[k]; !ok || existing != v {
						isSameInstance = false
						break
					}
				}
			}

			if !hasNonJoinMatchesVars && len(state.currentMatch) > 0 {
				isSameInstance = true
			} else if match.patternIdx == state.currentStartPatternIdx {
				isSameInstance = true
			}

			if isSameInstance {
				state.patternMatchCount[match.patternIdx]++
				for k, v := range match.result {
					if joinMatchesVars[k] {
						if existing, ok := state.currentMatch[k]; ok {
							var list []interface{}
							if existingList, ok := existing.([]interface{}); ok {
								list = existingList
							} else {
								list = []interface{}{existing}
							}
							if vList, ok := v.([]interface{}); ok {
								list = append(list, vList...)
							} else {
								list = append(list, v)
							}
							state.currentMatch[k] = list
						} else {
							if vList, ok := v.([]interface{}); ok {
								state.currentMatch[k] = vList
							} else {
								state.currentMatch[k] = []interface{}{v}
							}
						}
					}
				}
				return nil // continue - skip rest of step
			} else {
				if patternHasJoinMatches {
					state.patternMatchCount[match.patternIdx]++
				}
			}
		}

		if !shouldStartNewMatch && state.currentMatch != nil {
			if !patternHasJoinMatches || state.patternMatchCount[match.patternIdx] == 0 {
				shouldMerge = true
			}
		} else if group.Method == "table" && shouldStartNewMatch {
			if state.currentMatch != nil {
				hasNonSpecialVars := false
				for k := range state.currentMatch {
					if k != "ignore" && k != "_start_" && k != "_end_" && k != "_line_" {
						hasNonSpecialVars = true
						break
					}
				}

				ignoreUsesTemplateVar := false
				for _, compiledPattern := range group.Patterns {
					if compiledPattern.IgnoreUsesTemplateVar {
						ignoreUsesTemplateVar = true
						break
					}
				}
				if ignoreUsesTemplateVar {
					state.currentMatch = make(map[string]interface{})
				}

				if hasNonSpecialVars {
					srcRange := [2]int{state.currentStartPos, match.spanStart}
					if err := state.flush(state.currentMatch, srcRange); err != nil {
						return err
					}
					state.recordCount++
					r.pathResolver.UpdateCache(state.currentMatch)
					r.matchCollector.Clear()
				}
			}
			state.currentMatch = make(map[string]interface{})
			state.currentStartPatternIdx = match.patternIdx
			state.currentMatchHasEnd = false

			ignoreUsesTemplateVar := false
			for _, compiledPattern := range group.Patterns {
				if compiledPattern.IgnoreUsesTemplateVar {
					ignoreUsesTemplateVar = true
					break
				}
			}

			if !ignoreUsesTemplateVar {
				for k, v := range match.result {
					if joinMatchesVars[k] {
						if vList, ok := v.([]interface{}); ok {
							state.currentMatch[k] = vList
						} else {
							state.currentMatch[k] = []interface{}{v}
						}
					} else {
						if _, exists := state.currentMatch[k]; !exists {
							state.currentMatch[k] = v
						}
					}
				}
			}
			state.currentStartPos = match.spanStart
			state.currentStartLineIdx = match.lineIdx
			state.patternMatchCount = make(map[int]int)
			state.patternMatchCount[match.patternIdx] = 1
			return nil // continue
		}

		if shouldStartNewMatch && !shouldMerge {
			shouldFinalizePrevious := true
			hasEndPatterns := len(endPatterns) > 0
			isStartPatternVar := false
			if match.patternIdx >= 0 && match.patternIdx < len(group.Patterns) {
				pattern := group.Patterns[match.patternIdx]
				for varName, variable := range pattern.Variables {
					if varName == "_start_" {
						isStartPatternVar = true
						break
					}
					for _, funcStr := range variable.Functions {
						if funcStr == "_start_" {
							isStartPatternVar = true
							break
						}
					}
					if isStartPatternVar {
						break
					}
				}
			}
			isRepeatStartMatch := isStartPatternVar && match.patternIdx == state.currentStartPatternIdx
			hasPatternsToMergeOnSameLine := false
			if isStartPatternVar && hasEndPatterns {
				for i := matchIdx + 1; i < len(allMatches); i++ {
					nextMatch := allMatches[i]
					if nextMatch.spanStart != match.spanStart {
						break
					}
					nextIsEnd := endPatterns[nextMatch.patternIdx]
					nextIsStartPatternVar := false
					if nextMatch.patternIdx >= 0 && nextMatch.patternIdx < len(group.Patterns) {
						nextPattern := group.Patterns[nextMatch.patternIdx]
						for varName := range nextPattern.Variables {
							if varName == "_start_" {
								nextIsStartPatternVar = true
								break
							}
						}
					}
					if !nextIsEnd && !nextIsStartPatternVar {
						count := state.patternMatchCount[nextMatch.patternIdx]
						if count == 0 {
							hasPatternsToMergeOnSameLine = true
							break
						}
					}
				}
			}
			if hasAnyStartIndicator {
				if hasPatternsToMergeOnSameLine {
					shouldFinalizePrevious = false
				} else if state.currentMatchHasEnd {
					shouldFinalizePrevious = true
				} else if hasEndPatterns && state.currentMatch != nil && !isRepeatStartMatch {
					shouldFinalizePrevious = false
				}
			}
			if !shouldMerge {
				if state.currentMatch != nil && shouldFinalizePrevious {
					hasNonSpecialVars := false
					for k := range state.currentMatch {
						if k != "ignore" && k != "_start_" && k != "_end_" && k != "_line_" {
							hasNonSpecialVars = true
							break
						}
					}
					shouldSave := hasNonSpecialVars || isRepeatStartMatch
					if shouldSave {
						matchCopy := make(map[string]interface{})
						for k, v := range state.currentMatch {
							matchCopy[k] = v
						}
						parentIdx := state.recordCount
						srcRange := [2]int{state.currentStartPos, match.spanStart}
						if err := state.flush(matchCopy, srcRange); err != nil {
							return err
						}
						state.recordCount++
						if state.currentParentMatchStartIdx >= 0 && !state.skipParentBookkeeping {
							for i := state.currentParentMatchStartIdx; i <= matchIdx; i++ {
								state.parentMatchToAllMatches[parentIdx] = append(state.parentMatchToAllMatches[parentIdx], i)
							}
						}
						r.pathResolver.UpdateCache(matchCopy)
						r.matchCollector.Clear()
					}
				}
			}
			if shouldFinalizePrevious || state.currentMatch == nil {
				state.patternMatchCount = make(map[int]int)
				state.currentMatch = make(map[string]interface{})
				state.currentStartPatternIdx = match.patternIdx
				state.currentMatchHasEnd = false
				state.currentParentMatchStartIdx = matchIdx
				state.currentStartPos = match.spanStart
				state.currentStartLineIdx = match.lineIdx
				for k, v := range match.result {
					if joinMatchesVars[k] {
						if vList, ok := v.([]interface{}); ok {
							state.currentMatch[k] = vList
						} else {
							state.currentMatch[k] = []interface{}{v}
						}
					} else {
						state.currentMatch[k] = v
					}
				}
				state.patternMatchCount[match.patternIdx] = 1
			} else {
				shouldMerge = true
				state.currentStartPos = match.spanStart
				state.currentStartLineIdx = match.lineIdx
				state.patternMatchCount[match.patternIdx]++
			}

			if isEndPattern {
				state.currentMatchHasEnd = true
				srcRange := [2]int{state.currentStartPos, match.spanEnd}
				if err := state.flush(state.currentMatch, srcRange); err != nil {
					return err
				}
				state.recordCount++
				r.pathResolver.UpdateCache(state.currentMatch)
				r.matchCollector.Clear()
				state.currentMatch = nil
				state.currentStartPos = -1
				state.currentStartLineIdx = -1
				state.currentMatchHasEnd = false
				state.patternMatchCount = make(map[int]int)
				return nil // continue
			}
		}
	} else {
		// Normal pattern - merge into current match if close enough
		shouldFinalizeAndStartNew := false
		currentMatchStartedByStart := false
		if state.currentStartPatternIdx >= 0 && state.currentStartPatternIdx < len(group.Patterns) {
			startPattern := group.Patterns[state.currentStartPatternIdx]
			for varName, variable := range startPattern.Variables {
				if varName == "_start_" {
					currentMatchStartedByStart = true
					break
				}
				for _, funcStr := range variable.Functions {
					if funcStr == "_start_" {
						currentMatchStartedByStart = true
						break
					}
				}
				if currentMatchStartedByStart {
					break
				}
			}
		}
		if state.currentMatch != nil {
			hasEndPatterns := len(endPatterns) > 0

			if hasLineIndicator && !isStartPattern {
				count := state.patternMatchCount[match.patternIdx]
				if count > 0 {
					shouldFinalizeAndStartNew = true
				}
			}

			if (hasEmptyStartPattern && !hasEndPatterns) && state.currentStartPatternIdx >= 0 && match.patternIdx == state.currentStartPatternIdx {
				count := state.patternMatchCount[match.patternIdx]
				if count > 0 {
					shouldFinalizeAndStartNew = true
				}
			}

			if state.currentMatchHasEnd && !isStartPattern {
				isFirstPattern := match.patternIdx == 0
				if state.currentMatch == nil {
					if isFirstPattern {
						shouldFinalizeAndStartNew = true
						shouldMerge = false
					} else {
						shouldFinalizeAndStartNew = false
						shouldMerge = false
						return nil // continue
					}
				} else {
					if isFirstPattern {
						shouldFinalizeAndStartNew = true
						shouldMerge = false
					} else {
						shouldFinalizeAndStartNew = false
						shouldMerge = false
						return nil // continue
					}
				}
			} else if currentMatchStartedByStart || hasEndPatterns {
				if !shouldFinalizeAndStartNew {
					if hasEndPatterns && state.currentMatchHasEnd && !isStartPattern {
						shouldMerge = false
						for j := matchIdx - 1; j >= 0; j-- {
							prevMatch := allMatches[j]
							if endPatterns[prevMatch.patternIdx] {
								shouldMerge = false
								break
							}
							if startPatterns[prevMatch.patternIdx] {
								shouldMerge = true
								break
							}
						}
					} else {
						shouldMerge = true
					}
				}
			} else if hasLineIndicator {
				if !shouldFinalizeAndStartNew {
					shouldMerge = true
				}
			} else if match.lineIdx-state.currentStartLineIdx >= 0 && match.lineIdx-state.currentStartLineIdx < maxGapLines {
				if !shouldFinalizeAndStartNew {
					shouldMerge = true
				}
			}
		}

		if shouldFinalizeAndStartNew {
			hasNonSpecialVars := false
			for k := range state.currentMatch {
				if k != "ignore" && k != "_start_" && k != "_end_" && k != "_line_" {
					hasNonSpecialVars = true
					break
				}
			}
			if hasNonSpecialVars {
				matchCopy := make(map[string]interface{})
				for k, v := range state.currentMatch {
					matchCopy[k] = v
				}
				parentIdx := state.recordCount
				srcRange := [2]int{state.currentStartPos, match.spanStart}
				if err := state.flush(matchCopy, srcRange); err != nil {
					return err
				}
				state.recordCount++
				if state.currentParentMatchStartIdx >= 0 && !state.skipParentBookkeeping {
					for i := state.currentParentMatchStartIdx; i < matchIdx; i++ {
						state.parentMatchToAllMatches[parentIdx] = append(state.parentMatchToAllMatches[parentIdx], i)
					}
				}
				r.pathResolver.UpdateCache(matchCopy)
				r.matchCollector.Clear()
			}
			state.currentMatch = make(map[string]interface{})
			state.currentStartPos = match.spanStart
			state.currentStartLineIdx = match.lineIdx
			state.currentStartPatternIdx = match.patternIdx
			state.currentMatchHasEnd = false
			state.currentParentMatchStartIdx = matchIdx
			state.patternMatchCount = make(map[int]int)
			state.patternMatchCount[match.patternIdx] = 1
			shouldMerge = true
		}

		if shouldMerge {
			patternHasJoinMatches := false
			for varName := range match.result {
				if joinMatchesVars[varName] {
					patternHasJoinMatches = true
					break
				}
			}

			joinMatchesAlreadyInitialized := false
			if patternHasJoinMatches && state.patternMatchCount[match.patternIdx] == 1 {
				for varName := range match.result {
					if joinMatchesVars[varName] {
						if _, exists := state.currentMatch[varName]; exists {
							joinMatchesAlreadyInitialized = true
							break
						}
					}
				}
			}

			state.patternMatchCount[match.patternIdx]++

			if patternHasJoinMatches && !joinMatchesAlreadyInitialized {
				for k, v := range match.result {
					if joinMatchesVars[k] {
						existing, exists := state.currentMatch[k]
						if exists {
							var list []interface{}
							if existingList, ok := existing.([]interface{}); ok {
								if len(existingList) > 0 {
									if _, isNestedList := existingList[0].([]interface{}); isNestedList {
										list = existingList
									} else {
										list = []interface{}{existingList}
									}
								} else {
									list = existingList
								}
							} else {
								list = []interface{}{existing}
							}

							if vList, ok := v.([]interface{}); ok {
								if joinMatchesHasToList[k] {
									list = append(list, vList)
								} else {
									list = append(list, vList...)
								}
							} else {
								list = append(list, v)
							}

							state.currentMatch[k] = list
						} else {
							if joinMatchesHasToList[k] {
								if vList, ok := v.([]interface{}); ok {
									state.currentMatch[k] = []interface{}{vList}
								} else {
									state.currentMatch[k] = []interface{}{[]interface{}{v}}
								}
							} else {
								if _, exists := state.currentMatch[k]; !exists {
									state.currentMatch[k] = v
								}
							}
						}
					} else {
						if _, exists := state.currentMatch[k]; !exists {
							state.currentMatch[k] = v
						}
					}
				}
			} else {
				for k, v := range match.result {
					if k == "ignore" || k == "_start_" || k == "_end_" || k == "_line_" {
						continue
					}
					if !joinMatchesVars[k] {
						if _, exists := state.currentMatch[k]; !exists {
							state.currentMatch[k] = v
						}
					}
				}
			}

			if isEndPattern {
				state.currentMatchHasEnd = true
			}
		} else if state.currentMatch == nil {
			hasEndPatterns := len(endPatterns) > 0
			shouldStart := false

			if hasLineIndicator {
				shouldStart = false
			} else if len(startPatterns) == 0 {
				shouldStart = true
			} else if state.recordCount == 0 {
				shouldStart = isStartPattern
				if !shouldStart && len(startPatterns) > 0 {
					// Debug: non-start pattern trying to match when no matches yet
				}
			} else if hasEndPatterns && state.recordCount > 0 {
				if len(startPatterns) > 0 {
					shouldStart = isStartPattern
				} else {
					shouldStart = true
				}
			} else if hasEmptyStartPattern {
				shouldStart = true
			}

			if shouldStart {
				state.patternMatchCount = make(map[int]int)
				state.currentMatch = make(map[string]interface{})
				state.currentStartPatternIdx = match.patternIdx
				state.currentMatchHasEnd = false
				state.currentParentMatchStartIdx = matchIdx
				for k, v := range match.result {
					if joinMatchesVars[k] {
						if vList, ok := v.([]interface{}); ok {
							state.currentMatch[k] = vList
						} else {
							state.currentMatch[k] = []interface{}{v}
						}
					} else {
						if _, exists := state.currentMatch[k]; !exists {
							state.currentMatch[k] = v
						}
					}
				}
				state.currentStartPos = match.spanStart
				state.currentStartLineIdx = match.lineIdx
				state.patternMatchCount[match.patternIdx] = 1
				shouldMerge = false
			} else {
				isFirstPattern := match.patternIdx == 0
				if len(startPatterns) > 0 && !isStartPattern && !isFirstPattern {
					if !(hasEndPatterns && state.recordCount > 0) {
						if group.Name == "ipv4_afi" {
							// Debug placeholder preserved from original
						}
						return nil // continue
					}
				}
			}
		}
	}

	// If shouldMerge is true, execute merge logic (shared for both start and non-start patterns)
	if shouldMerge {
		patternHasJoinMatches := false
		for varName := range match.result {
			if joinMatchesVars[varName] {
				patternHasJoinMatches = true
				break
			}
		}

		joinMatchesAlreadyProcessed := false
		currentCount := state.patternMatchCount[match.patternIdx]
		if patternHasJoinMatches {
			for varName := range match.result {
				if joinMatchesVars[varName] {
					if _, exists := state.currentMatch[varName]; exists {
						if currentCount >= 1 {
							joinMatchesAlreadyProcessed = true
							break
						}
					}
				}
			}
		}

		state.patternMatchCount[match.patternIdx]++

		if patternHasJoinMatches && !joinMatchesAlreadyProcessed {
			for k, v := range match.result {
				if joinMatchesVars[k] {
					existing, exists := state.currentMatch[k]
					if exists {
						var list []interface{}
						if existingList, ok := existing.([]interface{}); ok {
							if len(existingList) > 0 {
								if _, isNestedList := existingList[0].([]interface{}); isNestedList {
									list = existingList
								} else {
									list = []interface{}{existingList}
								}
							} else {
								list = existingList
							}
						} else {
							list = []interface{}{existing}
						}

						if vList, ok := v.([]interface{}); ok {
							list = append(list, vList...)
						} else {
							list = append(list, v)
						}
						state.currentMatch[k] = list
					} else {
						if vList, ok := v.([]interface{}); ok {
							state.currentMatch[k] = vList
						} else {
							state.currentMatch[k] = []interface{}{v}
						}
					}
				} else {
					if _, exists := state.currentMatch[k]; !exists {
						state.currentMatch[k] = v
					}
				}
			}
		} else {
			for k, v := range match.result {
				if k == "ignore" || k == "_start_" || k == "_end_" {
					continue
				}
				if _, exists := state.currentMatch[k]; !exists {
					state.currentMatch[k] = v
				}
			}
		}

		if isEndPattern {
			state.currentMatchHasEnd = true
		}
	}
	return nil
}

// parseGroup parses input data against a compiled group
func (r *Runtime) parseGroup(group *compiler.CompiledGroup, inputData string, vars map[string]interface{}) (interface{}, error) {
	if len(group.Patterns) == 0 {
		// If the group has no direct patterns but has unnamed nested groups,
		// parse the nested groups and return their results directly.
		// This matches Python TTP behavior where unnamed inner groups are
		// transparent wrappers that merge into their parent.
		if len(group.Groups) > 0 {
			for _, nestedGroup := range group.Groups {
				if nestedGroup.Name == "" || nestedGroup.Name == "_" {
					return r.parseGroup(nestedGroup, inputData, vars)
				}
			}
		}
		return nil, nil
	}

	// patternMatch is now defined at package scope so stepMerge can reference it.
	var allMatches []patternMatch

	// Process each pattern and collect matches with positions
	// For patterns with ^ and $ anchors, we need to match line by line
	// For patterns without anchors, we can match against the entire input
	lines := strings.Split(inputData, "\n")
	lineOffsets := make([]int, len(lines)+1) // Track byte offsets for each line
	offset := 0
	for i, line := range lines {
		lineOffsets[i] = offset
		offset += len(line) + 1 // +1 for newline
	}
	lineOffsets[len(lines)] = offset

	for patternIdx, compiledPattern := range group.Patterns {

		if compiledPattern.HasAnchors {
			// Match line by line
			for lineIdx, line := range lines {
				// Trim \r and trailing spaces, but preserve leading spaces for regex matching
				// The regex was generated from the template which may have leading spaces
				line = strings.TrimRight(line, "\r \t")
				trimmedLine := strings.TrimSpace(line)

				// Check if this pattern has _start_ or _end_ indicator - if so, allow matching empty lines
				hasStartIndicator := false
				hasEndIndicator := false
				for varName := range compiledPattern.Variables {
					if varName == "_start_" {
						hasStartIndicator = true
					}
					if varName == "_end_" {
						hasEndIndicator = true
					}
					// Also check functions
					for _, funcStr := range compiledPattern.Variables[varName].Functions {
						if funcStr == "_start_" {
							hasStartIndicator = true
						}
						if funcStr == "_end_" {
							hasEndIndicator = true
						}
					}
					if hasStartIndicator && hasEndIndicator {
						break
					}
				}

				// Skip empty lines unless this pattern has _start_ or _end_ indicator
				if trimmedLine == "" && !hasStartIndicator && !hasEndIndicator {
					continue
				}

				// Use the line (with leading spaces but trailing spaces trimmed) for matching
				// For _start_ or _end_ patterns on empty lines, use empty string
				matchLine := line
				if trimmedLine == "" && (hasStartIndicator || hasEndIndicator) {
					matchLine = ""
				}

				match := compiledPattern.Regex.FindStringSubmatch(matchLine)

				if match != nil {
					result := r.extractMatchResult(match, compiledPattern, vars)
					if result != nil && (len(result) > 0 || compiledPattern.HasOnlySpecialIndicators || compiledPattern.IgnoreUsesTemplateVar || compiledPattern.HasJoinMatches) {
						spanStart := lineOffsets[lineIdx]
						spanEnd := lineOffsets[lineIdx] + len(line)
						allMatches = append(allMatches, patternMatch{
							patternIdx: patternIdx,
							spanStart:  spanStart,
							spanEnd:    spanEnd,
							lineIdx:    lineIdx,
							result:     result,
						})
					}
				}
			}
		} else {
			// Match against entire input
			// Find all matches in the input
			allIndices := compiledPattern.Regex.FindAllStringSubmatchIndex(inputData, -1)

			for _, indices := range allIndices {
				if len(indices) < 2 {
					continue
				}

				// Extract the full match groups
				matchGroups := make([]string, len(indices)/2)
				for i := 0; i < len(indices); i += 2 {
					if indices[i] >= 0 && indices[i+1] >= 0 {
						matchGroups[i/2] = inputData[indices[i]:indices[i+1]]
					}
				}

				// Extract result
				result := r.extractMatchResult(matchGroups, compiledPattern, vars)
				if result != nil && (len(result) > 0 || compiledPattern.IgnoreUsesTemplateVar) {
					allMatches = append(allMatches, patternMatch{
						patternIdx: patternIdx,
						spanStart:  indices[0],
						spanEnd:    indices[1],
						lineIdx:    sort.SearchInts(lineOffsets, indices[0]+1) - 1,
						result:     result,
					})
				}
			}
		}
	}

	// Sort matches by position (spanStart)
	// Use Go's sort.Slice for better performance (O(n log n) vs O(n²))
	sort.Slice(allMatches, func(i, j int) bool {
		if allMatches[i].spanStart == allMatches[j].spanStart {
			// If same position, sort by pattern index (first pattern wins)
			// This ensures that when _line_ matches on the same line as other patterns,
			// _line_ comes first so it can start the match before others merge into it
			return allMatches[i].patternIdx < allMatches[j].patternIdx
		}
		return allMatches[i].spanStart < allMatches[j].spanStart
	})

	// DEBUG: Log all matches after sorting
	// for i, m := range allMatches {
	// 	fmt.Printf("  Match %d: patternIdx=%d, spanStart=%d, spanEnd=%d, result=%v\n", i, m.patternIdx, m.spanStart, m.spanEnd, m.result)
	// }

	// For method="table", only keep the first pattern that matches each line
	// This ensures that if multiple patterns match the same line, only the first one is used
	if group.Method == "table" {
		filteredMatches := make([]patternMatch, 0, len(allMatches))
		seenPositions := make(map[int]bool) // Track which positions (spanStart) we've already matched
		for _, match := range allMatches {
			if !seenPositions[match.spanStart] {
				seenPositions[match.spanStart] = true
				filteredMatches = append(filteredMatches, match)
			}
		}
		allMatches = filteredMatches
	} else {
		// For non-table method, filter redundant pure indicator matches
		// If a line is matched by a normal pattern, ignore pure indicator matches (_start_, _end_) on the same line
		// This prevents _start_/_end_ from matching every line and triggering unwanted logic
		filteredMatches := make([]patternMatch, 0, len(allMatches))
		i := 0
		for i < len(allMatches) {
			currentStart := allMatches[i].spanStart
			// Find all matches at this position
			j := i
			hasNormalMatch := false
			for j < len(allMatches) && allMatches[j].spanStart == currentStart {
				// Check if this is a "normal" match (not just pure _start_/_end_)
				isPureIndicator := true
				for v := range allMatches[j].result {
					if v != "_start_" && v != "_end_" && v != "_line_" {
						isPureIndicator = false
						break
					}
				}
				if !isPureIndicator {
					hasNormalMatch = true
				}
				j++
			}

			// Process matches at this position
			for k := i; k < j; k++ {
				isPureIndicator := true
				for v := range allMatches[k].result {
					if v != "_start_" && v != "_end_" && v != "_line_" {
						isPureIndicator = false
						break
					}
				}
				// Check if this is a _start_ or _line_ pattern by checking the pattern index
				// These patterns are needed to start matches even if redundant
				isStartPatternMatch := false
				isLinePatternMatch := false
				if allMatches[k].patternIdx < len(group.Patterns) {
					pattern := group.Patterns[allMatches[k].patternIdx]
					for varName := range pattern.Variables {
						if varName == "_start_" {
							isStartPatternMatch = true
							break
						}
						if varName == "_line_" {
							// Check if _line_ is on its own line (not used with joinmatches)
							// If pattern has only _line_ variable, it's on its own line and should start matches
							if len(pattern.Variables) == 1 {
								isLinePatternMatch = true
								break
							}
						}
					}
				}

				if hasNormalMatch && isPureIndicator {
					// Skip pure indicator match if we have a normal match
					// EXCEPT _start_ and _line_ (on its own line), which we need to start matches even if redundant
					// Filter out _end_ and _line_ (with joinmatches) when redundant, but keep _start_ and standalone _line_
					if isStartPatternMatch || isLinePatternMatch {
						// Keep _start_ or standalone _line_ even if redundant - they're needed to start matches
						filteredMatches = append(filteredMatches, allMatches[k])
						continue
					}
					// Skip _end_ and _line_ (with joinmatches) when redundant
					continue
				}
				filteredMatches = append(filteredMatches, allMatches[k])
			}
			i = j
		}
		allMatches = filteredMatches
	}

	// Check if any pattern has joinmatches variables and extract join characters
	joinMatchesVars := make(map[string]bool)      // Track which variables have joinmatches
	joinMatchesChars := make(map[string]string)   // Track join characters for each variable
	joinMatchesHasToList := make(map[string]bool) // Track if to_list was used before joinmatches
	for _, pattern := range group.Patterns {
		for varName, variable := range pattern.Variables {
			hasToList := false
			joinChar := "\n" // default join character
			for _, funcStr := range variable.Functions {
				if funcStr == "to_list" {
					hasToList = true
				}
				if strings.HasPrefix(funcStr, "joinmatches") {
					joinMatchesVars[varName] = true
					joinMatchesHasToList[varName] = hasToList
					// Extract join character from joinmatches function call
					// Format: "joinmatches" or "joinmatches(',')" or "joinmatches('char')"
					if strings.Contains(funcStr, "(") {
						// Extract argument between parentheses
						start := strings.Index(funcStr, "(")
						end := strings.Index(funcStr, ")")
						if start >= 0 && end > start {
							arg := funcStr[start+1 : end]
							// Remove quotes if present
							arg = strings.Trim(arg, "'\"")
							if arg != "" {
								joinChar = arg
							}
						}
					}
					joinMatchesChars[varName] = joinChar
					break // Only need first joinmatches
				}
			}
		}
	}

	// Merge matches that belong to the same group instance
	// First pattern (index 0) is the "start" pattern
	// Subsequent patterns add to the match if they're close enough
	// If joinmatches is used, multiple matches of the same pattern should be collected
	//
	// All cross-iteration merge state lives on mergeState so the merge state
	// machine can be shared with the streaming variant (parseGroupStream).
	// Batch mode collects flushed records into a local slice via the flush
	// callback; srcRange is ignored here (it's only meaningful for streaming).
	state := newMergeState()
	var mergedMatches []map[string]interface{}
	state.flush = func(record map[string]interface{}, _ [2]int) error {
		mergedMatches = append(mergedMatches, record)
		return nil
	}

	// Detect which patterns have _start_ indicator
	// Start pattern detection rules:
	// - When _start_ AND _end_ are present: Only patterns with _start_ are start patterns
	// - When _start_ is present but NO _end_: 
	//   * Patterns with _start_ are always start patterns
	//   * Patterns BEFORE the first _start_ pattern can also be start patterns (if they have non-special variables)
	//   * Patterns AFTER the first _start_ pattern are NOT start patterns (they only merge)
	startPatterns := make(map[int]bool) // pattern index -> is start pattern
	hasEmptyStartPattern := false       // true if we have a _start_ pattern that only matches empty lines
	hasLineIndicator := false           // true if we have a _line_ indicator (matches any line)
	hasAnyStartIndicator := false       // true if ANY pattern has _start_ indicator

	// If method is "table", all patterns are start patterns
	if group.Method == "table" {
		for patternIdx := range group.Patterns {
			startPatterns[patternIdx] = true
		}
	} else {
		// Otherwise, check for _start_ or _line_ indicator
		// First pass: detect if any pattern has _start_ indicator
		// Python TTP behavior: if ANY pattern has _start_, ALL patterns can serve as start patterns
		// Note: Use = not := to assign to the function-level variable, not create a new local one
		hasAnyStartIndicator = false
		for _, compiledPattern := range group.Patterns {
			for _, variable := range compiledPattern.Variables {
				if variable.Name == "_start_" {
					hasAnyStartIndicator = true
					break
				}
				// Also check if _start_ is in functions (e.g., {{ name | _start_ }})
				for _, funcStr := range variable.Functions {
					if funcStr == "_start_" {
						hasAnyStartIndicator = true
						break
					}
				}
				if hasAnyStartIndicator {
					break
				}
			}
			if hasAnyStartIndicator {
				break
			}
		}

		// Second pass: mark start patterns
		// First, check if we have _end_ patterns (needed for special handling)
		hasAnyEndIndicator := false
		for _, compiledPattern := range group.Patterns {
			for _, variable := range compiledPattern.Variables {
				if variable.Name == "_end_" {
					hasAnyEndIndicator = true
					break
				}
				for _, funcStr := range variable.Functions {
					if funcStr == "_end_" {
						hasAnyEndIndicator = true
						break
					}
				}
				if hasAnyEndIndicator {
					break
				}
			}
			if hasAnyEndIndicator {
				break
			}
		}

		for patternIdx, compiledPattern := range group.Patterns {
			// Check if this pattern has joinmatches - if so, _line_ in functions shouldn't make it a start pattern
			patternHasJoinMatches := false
			for _, variable := range compiledPattern.Variables {
				for _, funcStr := range variable.Functions {
					if strings.HasPrefix(funcStr, "joinmatches") {
						patternHasJoinMatches = true
						break
					}
				}
				if patternHasJoinMatches {
					break
				}
			}

			// SPECIAL: If we have BOTH _start_ and _end_ patterns, only patterns with _start_ should be start patterns.
			// Other patterns should merge into the match started by _start_ until _end_ is hit.
			// This is different from the general Python TTP behavior where "if any pattern has _start_, all patterns are start patterns".
			// When _end_ is present, we need more precise control: only _start_ patterns actually start matches.
			if hasAnyStartIndicator && hasAnyEndIndicator {
				// Check if this specific pattern has _start_
				patternHasStart := false
				for _, variable := range compiledPattern.Variables {
					if variable.Name == "_start_" {
						patternHasStart = true
						break
					}
					for _, funcStr := range variable.Functions {
						if funcStr == "_start_" {
							patternHasStart = true
							break
						}
					}
					if patternHasStart {
						break
					}
				}
				// Only mark as start pattern if it has _start_
				if patternHasStart {
					startPatterns[patternIdx] = true
				}
			} else if hasAnyStartIndicator {
				// If any pattern has _start_ but NO _end_, patterns with _start_ are start patterns.
				// Additionally, patterns BEFORE the first _start_ pattern can also be start patterns,
				// allowing them to match independently (e.g., "interface Tunnel" can start a match
				// even though only "interface GigabitEthernet" has _start_).
				// However, patterns AFTER the _start_ pattern should NOT be start patterns - they should
				// only merge into matches started by _start_ patterns.
				// Find the first pattern with _start_
				firstStartPatternIdx := -1
				for idx, pattern := range group.Patterns {
					for _, variable := range pattern.Variables {
						if variable.Name == "_start_" {
							firstStartPatternIdx = idx
							break
						}
						for _, funcStr := range variable.Functions {
							if funcStr == "_start_" {
								firstStartPatternIdx = idx
								break
							}
						}
						if firstStartPatternIdx >= 0 {
							break
						}
					}
					if firstStartPatternIdx >= 0 {
						break
					}
				}
				
				// Check if this specific pattern has _start_
				patternHasStart := false
				for _, variable := range compiledPattern.Variables {
					if variable.Name == "_start_" {
						patternHasStart = true
						break
					}
					for _, funcStr := range variable.Functions {
						if funcStr == "_start_" {
							patternHasStart = true
							break
						}
					}
					if patternHasStart {
						break
					}
				}
				if patternHasStart {
					// Pattern has _start_ - mark as start pattern
					startPatterns[patternIdx] = true
				} else if firstStartPatternIdx >= 0 && patternIdx < firstStartPatternIdx {
					// Pattern comes BEFORE the first _start_ pattern - allow it to be a start pattern
					// This allows "interface Tunnel" to start a match even though only "interface GigabitEthernet" has _start_
					hasNonSpecialVars := false
					for varName := range compiledPattern.Variables {
						if varName != "_start_" && varName != "_end_" && varName != "_line_" && varName != "ignore" {
							hasNonSpecialVars = true
							break
						}
					}
					if hasNonSpecialVars {
						startPatterns[patternIdx] = true
					}
				}
				// Patterns AFTER the first _start_ pattern are NOT start patterns - they only merge
			}

			// Check if any variable in this pattern has _start_ or _line_ indicator
			for _, variable := range compiledPattern.Variables {
				if variable.Name == "_start_" || variable.Name == "_line_" {
					// If _line_ is the variable name and pattern has joinmatches, don't treat as start pattern
					// (it should merge with existing match)
					if variable.Name == "_line_" && patternHasJoinMatches {
						// Don't mark as start pattern - it should merge
						if !hasAnyStartIndicator {
							// Only unmark if we're not in "all patterns are start patterns" mode
							startPatterns[patternIdx] = false
							startPatterns[patternIdx] = true
						}
					} else {
						startPatterns[patternIdx] = true
					}
					// Check if this pattern only has _line_ (on its own line)
					if len(compiledPattern.Variables) == 1 && variable.Name == "_line_" {
						hasLineIndicator = true
					}
					// Check if this pattern only has _start_ (on its own line)
					if len(compiledPattern.Variables) == 1 && variable.Name == "_start_" {
						// Check if pattern regex matches empty/whitespace-only lines
						// (used when _start_ is on its own line)
						// The regex string might be "^[\t ]*$" (current) or "^.*$" (legacy)
						if compiledPattern.Regex != nil {
							regexStr := compiledPattern.Regex.String()
							if regexStr == "^.*$" || regexStr == "(?m)^.*$" || regexStr == "^$" || regexStr == "(?m)^$" || regexStr == `^[\t ]*$` || regexStr == `(?m)^[\t ]*$` {
								hasEmptyStartPattern = true
							}
						}
					}
					break
				}
				// Also check if _start_ or _line_ is in functions (e.g., {{ name | _start_ }})
				for _, funcStr := range variable.Functions {
					if funcStr == "_start_" || funcStr == "_line_" {
						// If _line_ is in functions and pattern has joinmatches, don't treat as start pattern
						// (it should merge with existing match)
						if funcStr == "_line_" && patternHasJoinMatches {
							// Don't mark as start pattern - it should merge
							if !hasAnyStartIndicator {
								// Only unmark if we're not in "all patterns are start patterns" mode
								startPatterns[patternIdx] = false
							}
						} else {
							startPatterns[patternIdx] = true
						}
						break
					}
				}
			}
		}

		// If no patterns have _start_, default to pattern 0 being the start
		if len(startPatterns) == 0 {
			startPatterns[0] = true
		}
	}

	// Detect which patterns have _end_ indicator
	endPatterns := make(map[int]bool) // pattern index -> is end pattern

	// If we have _end_ patterns but no _start_ patterns,
	// make the first pattern a start pattern to initiate match collection
	hasAnyEndIndicator := false
	for patternIdx, compiledPattern := range group.Patterns {
		for varName := range compiledPattern.Variables {
			if varName == "_end_" {
				endPatterns[patternIdx] = true
				hasAnyEndIndicator = true
			}
			// Check if _end_ is in functions
			for _, funcStr := range compiledPattern.Variables[varName].Functions {
				if funcStr == "_end_" {
					endPatterns[patternIdx] = true
					hasAnyEndIndicator = true
				}
			}
		}
	}

	// If we have _end_ but no _start_, mark first pattern as start
	// This must happen AFTER we've checked for _end_ patterns
	if hasAnyEndIndicator && !hasAnyStartIndicator {
		// Check if startPatterns is empty or if pattern 0 is not marked as start
		if len(startPatterns) == 0 || !startPatterns[0] {
			startPatterns[0] = true
		}
	}

	for patternIdx, compiledPattern := range group.Patterns {
		// Check if any variable in this pattern has _end_ indicator
		for _, variable := range compiledPattern.Variables {
			if variable.Name == "_end_" {
				endPatterns[patternIdx] = true
				break
			}
			// Also check if _end_ is in functions
			for _, funcStr := range variable.Functions {
				if funcStr == "_end_" {
					endPatterns[patternIdx] = true
					break
				}
			}
		}
	}

	for matchIdx := range allMatches {
		if err := r.stepMerge(state, allMatches, matchIdx, group,
			joinMatchesVars, joinMatchesHasToList,
			startPatterns, endPatterns,
			hasLineIndicator, hasAnyStartIndicator, hasEmptyStartPattern); err != nil {
			return nil, err
		}
	}

	// Re-bind locals from state for the post-loop logic that was originally
	// written against function-local merge variables. mergedMatches is the
	// closure-captured local populated via state.flush above; the rest are
	// rebound from state for readability.
	currentMatch := state.currentMatch
	parentMatchToAllMatches := state.parentMatchToAllMatches
	currentParentMatchStartIdx := state.currentParentMatchStartIdx

	// Add final match
	// IMPORTANT: With _start_/_end_ patterns, if currentMatch has been ended by _end_,
	// we should save it even if a new _start_ hasn't finalized it yet
	// This ensures that the last match is saved even if there's no subsequent _start_ pattern
	if currentMatch != nil {
		// Check if currentMatch has any non-special variables
		hasNonSpecialVars := false
		for k := range currentMatch {
			if k != "ignore" && k != "_start_" && k != "_end_" && k != "_line_" {
				hasNonSpecialVars = true
				break
			}
		}
		// Save if it has non-special variables
		// For _start_/_end_ patterns, we should save even if currentMatchHasEnd is true
		// because the match was ended by _end_ and should be saved
		if hasNonSpecialVars {
			// Create a copy to avoid reference issues
			matchCopy := make(map[string]interface{})
			for k, v := range currentMatch {
				matchCopy[k] = v
			}
			parentIdx := len(mergedMatches)
			mergedMatches = append(mergedMatches, matchCopy)
			// Track which matches from allMatches belong to this parent
			// All matches from currentParentMatchStartIdx to the last match index belong to this parent
			if currentParentMatchStartIdx >= 0 && len(allMatches) > 0 {
				for i := currentParentMatchStartIdx; i < len(allMatches); i++ {
					parentMatchToAllMatches[parentIdx] = append(parentMatchToAllMatches[parentIdx], i)
				}
			}
			// Update path resolver cache with final match values
			r.pathResolver.UpdateCache(matchCopy)
		}
	}

	// Track parent match input ranges for nested group context
	// Store the input data range (start and end positions) for each parent match
	parentMatchRanges := make([]struct {
		start int
		end   int
	}, len(mergedMatches))

	// Determine input ranges for each parent match based on actual match positions
	// Find the start and end positions of matches that belong to each parent match
	if len(mergedMatches) > 0 && len(allMatches) > 0 {
		// Group matches by parent match index
		// We need to track which matches belong to which parent match
		// For now, we'll use a simple approach: find start patterns and group subsequent matches
		parentMatchIndices := make([][]int, len(mergedMatches)) // parent index -> match indices

		// Find start patterns to identify parent match boundaries
		startPatterns := make(map[int]bool)
		for patternIdx, pattern := range group.Patterns {
			for varName := range pattern.Variables {
				if varName == "_start_" {
					startPatterns[patternIdx] = true
					break
				}
			}
		}

		// If no start patterns, use first pattern as start
		if len(startPatterns) == 0 && len(group.Patterns) > 0 {
			startPatterns[0] = true
		}

		// Group matches by parent match using the tracked mapping
		// Use parentMatchToAllMatches to get the correct matches for each parent
		for parentIdx := 0; parentIdx < len(mergedMatches); parentIdx++ {
			if matchIndices, ok := parentMatchToAllMatches[parentIdx]; ok {
				for _, matchIdx := range matchIndices {
					if matchIdx < len(allMatches) {
						parentMatchIndices[parentIdx] = append(parentMatchIndices[parentIdx], allMatches[matchIdx].spanStart)
					}
				}
			}
		}

		// Calculate ranges based on match positions
		// Check if we have _end_ patterns - if so, use _end_ position as range end
		hasEndPatterns := false
		endPatternIndices := make(map[int]bool)
		for patternIdx, pattern := range group.Patterns {
			for varName := range pattern.Variables {
				if varName == "_end_" {
					endPatternIndices[patternIdx] = true
					hasEndPatterns = true
					break
				}
			}
		}

		for i := range parentMatchRanges {
			if i < len(parentMatchIndices) && len(parentMatchIndices[i]) > 0 {
				// Use first match position as start
				parentMatchRanges[i].start = parentMatchIndices[i][0]
				// Use last match position + some buffer as end
				lastPos := parentMatchIndices[i][len(parentMatchIndices[i])-1]
				// Find the end of the last match
				// If we have _end_ patterns, look for the _end_ pattern match for this parent
				if hasEndPatterns {
					// Find the _end_ pattern match that belongs to this parent
					// It should be after the start but before the next parent's start
					nextParentStart := len(inputData)
					if i < len(parentMatchIndices)-1 && len(parentMatchIndices[i+1]) > 0 {
						nextParentStart = parentMatchIndices[i+1][0]
					}
					// Look for _end_ pattern matches in this parent's range
					for _, match := range allMatches {
						if match.spanStart >= parentMatchRanges[i].start && match.spanStart < nextParentStart {
							if endPatternIndices[match.patternIdx] {
								// Found _end_ pattern - use its end position
								parentMatchRanges[i].end = match.spanEnd
								break
							}
						}
					}
				}
				// If no _end_ found or no _end_ patterns, check if we have nested groups
				// If we have nested groups, extend range to end of input (or next parent) so nested groups can match
				if parentMatchRanges[i].end == 0 {
					if len(group.Groups) > 0 {
						// We have nested groups - extend range to end of input or next parent
						if i < len(mergedMatches)-1 && len(parentMatchIndices[i+1]) > 0 {
							parentMatchRanges[i].end = parentMatchIndices[i+1][0]
						} else {
							parentMatchRanges[i].end = len(inputData)
						}
					} else {
						// No nested groups - use last match end
						for _, match := range allMatches {
							if match.spanStart == lastPos {
								parentMatchRanges[i].end = match.spanEnd
								break
							}
						}
					}
				}
				// If still no end found, use next parent's start or end of input
				if parentMatchRanges[i].end == 0 {
					if i < len(mergedMatches)-1 && len(parentMatchIndices[i+1]) > 0 {
						parentMatchRanges[i].end = parentMatchIndices[i+1][0]
					} else {
						parentMatchRanges[i].end = len(inputData)
					}
				}
			} else {
				// Fallback: divide input evenly
				inputLen := len(inputData)
				rangeSize := inputLen / len(mergedMatches)
				parentMatchRanges[i].start = i * rangeSize
				if i < len(mergedMatches)-1 {
					parentMatchRanges[i].end = (i + 1) * rangeSize
				} else {
					parentMatchRanges[i].end = inputLen
				}
			}
		}
	} else if len(mergedMatches) > 0 {
		// No matches but we have parent matches - divide input evenly
		inputLen := len(inputData)
		rangeSize := inputLen / len(mergedMatches)
		for i := range parentMatchRanges {
			parentMatchRanges[i].start = i * rangeSize
			if i < len(mergedMatches)-1 {
				parentMatchRanges[i].end = (i + 1) * rangeSize
			} else {
				parentMatchRanges[i].end = inputLen
			}
		}
	}

	// Apply joinmatches to all merged matches
	// If to_list was used, keep as list; otherwise join as string
	for _, match := range mergedMatches {
		for varName := range joinMatchesVars {
			if value, ok := match[varName]; ok {
				hasToList := joinMatchesHasToList[varName]
				joinChar := joinMatchesChars[varName]

				// Convert value to list if not already
				var list []interface{}
				if vList, ok := value.([]interface{}); ok {
					list = vList
				} else {
					list = []interface{}{value}
				}

				if hasToList {
					// Keep as list - each item should be a string
					// Each item from to_list is ["value"], we need to unwrap to just "value"
					resultList := make([]interface{}, 0)
					for _, item := range list {
						// Item should be a single-element list like ["138,166,173"] from to_list
						// Unwrap it to get the string value
						if itemList, ok := item.([]interface{}); ok && len(itemList) == 1 {
							// Unwrap single-element list
							resultList = append(resultList, itemList[0])
						} else {
							// Not a single-element list, append as-is
							resultList = append(resultList, item)
						}
					}
					match[varName] = resultList
				} else {
					// Join as string
					strs := make([]string, len(list))
					for i, item := range list {
						if itemList, ok := item.([]interface{}); ok {
							// If item is a list, join its elements first
							itemStrs := make([]string, len(itemList))
							for j, subItem := range itemList {
								itemStrs[j] = fmt.Sprintf("%v", subItem)
							}
							strs[i] = strings.Join(itemStrs, joinChar)
						} else {
							strs[i] = fmt.Sprintf("%v", item)
						}
					}
					match[varName] = strings.Join(strs, joinChar)
				}
			}
		}
	}

	// Handle nested groups
	// For nested groups, we need to parse them within the context of each parent match
	// This means parsing nested groups on the input data range that belongs to each parent match
	if len(group.Groups) > 0 {
		// For now, we'll parse nested groups on the entire input
		// But we need to associate nested matches with parent matches based on position
		// This is a simplified approach - a full implementation would track input ranges per parent match

		// Check if parent group uses record attribute - if so, preserve those variables
		recordVar := ""
		if recordAttr, ok := group.Attributes["record"]; ok && recordAttr != "" {
			recordVar = strings.TrimSpace(recordAttr)
		}
		
		for _, nestedGroup := range group.Groups {
			// DEBUG: Trace nested group processing for issue #13
			if nestedGroup.Name == "io" {
			}
			// Parse nested group for each parent match within its input range
			// This ensures nested matches are associated with the correct parent
			for i, parentMatch := range mergedMatches {
				// Extract input data range for this parent match
				rangeStart := parentMatchRanges[i].start
				rangeEnd := parentMatchRanges[i].end
				parentInputData := inputData[rangeStart:rangeEnd]

				// Parse nested group on this parent's input range
				// Merge parent match variables with template vars so nested group functions can access them
				nestedVars := make(map[string]interface{})
				// First add template vars
				for k, v := range vars {
					nestedVars[k] = v
				}
				// Then add parent match vars (override template vars)
				// IMPORTANT: Deep copy to avoid sharing references with parentMatch
				for k, v := range parentMatch {
					if vMap, ok := v.(map[string]interface{}); ok {
						copiedMap := make(map[string]interface{})
						for k2, v2 := range vMap {
							copiedMap[k2] = v2
						}
						nestedVars[k] = copiedMap
					} else if vSlice, ok := v.([]interface{}); ok {
						copiedSlice := make([]interface{}, len(vSlice))
						copy(copiedSlice, vSlice)
						nestedVars[k] = copiedSlice
					} else {
						nestedVars[k] = v
					}
				}
				// Also add recorded vars (override parent match vars)
				if r.recordedVars != nil {
					for k, v := range r.recordedVars {
						nestedVars[k] = v
					}
			}
			nestedResults, err := r.parseGroup(nestedGroup, parentInputData, nestedVars)
			if err != nil {
				return nil, err
			}

			// YANG validation for nested group (before merging into parent)
				if r.yangValidator != nil && nestedResults != nil {
					if yangPath, ok := nestedGroup.Attributes["yang"]; ok && yangPath != "" {
						validationResult := r.yangValidator.Validate(nestedResults, yangPath, nestedGroup.Name)
						// Store validation result (use group name with index to avoid collisions)
						validationKey := fmt.Sprintf("%s[%d]", nestedGroup.Name, i)
						r.validationResults[validationKey] = validationResult
					}
				}

				// Check if nested group has void attribute - if so, skip saving results
				hasVoid := false
				if voidVal, ok := nestedGroup.Attributes["void"]; ok {
					hasVoid = voidVal == "" || voidVal == "true" || voidVal == "True" || voidVal == "TRUE"
				}

				// Merge nested results into this specific parent match (but skip if void attribute is set)
				// Also skip if nested results are empty (no matches)
				if nestedResults != nil && !hasVoid {
					// Check if nested results are empty
					isEmpty := false
					if nestedList, ok := nestedResults.([]map[string]interface{}); ok {
						// Check if list is empty or contains only empty maps
						if len(nestedList) == 0 {
							isEmpty = true
						} else {
							// Check if all matches are empty
							allEmpty := true
							for _, match := range nestedList {
								if len(match) > 0 {
									allEmpty = false
									break
								}
							}
							isEmpty = allEmpty
						}
					} else if nestedMap, ok := nestedResults.(map[string]interface{}); ok {
						isEmpty = len(nestedMap) == 0
					} else if nestedList, ok := nestedResults.([]interface{}); ok {
						// Handle []interface{} case
						isEmpty = len(nestedList) == 0
					}

					if isEmpty {
						// Skip storing empty nested results (matches Python TTP behavior)
						continue
					}

					// Check if nested group has void attribute
					nestedHasVoid := false
					if voidVal, ok := nestedGroup.Attributes["void"]; ok {
						nestedHasVoid = voidVal == "" || voidVal == "true" || voidVal == "True" || voidVal == "TRUE"
					}

					if !nestedHasVoid {
						// If nested group name is "_", merge into parent match
						if nestedGroup.Name == "_" || nestedGroup.Name == "" {
							switch v := nestedResults.(type) {
							case []map[string]interface{}:
								// Merge all nested matches into parent match
								for _, nestedMatch := range v {
									for k, val := range nestedMatch {
										parentMatch[k] = val
									}
								}
							case map[string]interface{}:
								// Merge single nested match into parent match
								for k, val := range v {
									parentMatch[k] = val
								}
							}
						} else {
							// Named nested group - resolve path and store using storeAtPath
							// This handles dynamic paths and path formatters (*, **) correctly
							// For nested groups, we need to resolve the path per match and store accordingly
							// Track all variables that were stored in nested groups to remove from parent
							allNestedVars := make(map[string]bool)
							switch v := nestedResults.(type) {
							case []map[string]interface{}:
								// Multiple nested matches - resolve path for each and group by resolved path
								// Use a map to store resolved path parts with matches
								type pathMatch struct {
									parts []struct {
										key       string
										formatter string
									}
									match map[string]interface{}
								}
								groupedNestedResults := make(map[string][]pathMatch)
								for _, nestedMatch := range v {
									// Resolve dynamic path using nested match values and parent match context
									// Merge parent match vars with nested match for path resolution
									// IMPORTANT: Deep copy parentMatch vars to avoid sharing references
									resolutionVars := make(map[string]interface{})
									for k, val := range parentMatch {
										if vMap, ok := val.(map[string]interface{}); ok {
											copiedMap := make(map[string]interface{})
											for k2, v2 := range vMap {
												copiedMap[k2] = v2
											}
											resolutionVars[k] = copiedMap
										} else if vSlice, ok := val.([]interface{}); ok {
											copiedSlice := make([]interface{}, len(vSlice))
											copy(copiedSlice, vSlice)
											resolutionVars[k] = copiedSlice
										} else {
											resolutionVars[k] = val
										}
									}
									for k, val := range nestedMatch {
										resolutionVars[k] = val
									}
									// Also include template vars
									for k, val := range vars {
										resolutionVars[k] = val
									}

									// Resolve path segment-by-segment to preserve formatters
									// Split path by dots first, then resolve each segment separately
									resolvedParts := r.resolvePathSegments(nestedGroup.Name, nestedMatch, resolutionVars)
									if len(resolvedParts) == 0 {
										// Fallback to original name
										resolvedParts = r.resolvePathSegments(nestedGroup.Name, nestedMatch, resolutionVars)
										if len(resolvedParts) == 0 {
											continue
										}
									}
									// Build a key for grouping (convert parts to string for map key)
									// Use a special delimiter that won't appear in resolved values
									pathKeyParts := make([]string, len(resolvedParts))
									for i, part := range resolvedParts {
										pathKeyParts[i] = part.key + part.formatter
									}
									pathKey := strings.Join(pathKeyParts, "\x00") // Use null byte as delimiter

									// Remove path variables from nested match
									pathVars := r.pathResolver.ExtractVariablesFromPath(nestedGroup.Name)
									pathVarSet := make(map[string]bool)
									for _, pv := range pathVars {
										pathVarSet[pv] = true
									}
									cleanedNestedMatch := make(map[string]interface{})
									for k, val := range nestedMatch {
										if !pathVarSet[k] {
											cleanedNestedMatch[k] = val
										}
									}

									// Store resolved parts with the match for later processing
									if _, exists := groupedNestedResults[pathKey]; !exists {
										groupedNestedResults[pathKey] = make([]pathMatch, 0)
									}
									groupedNestedResults[pathKey] = append(groupedNestedResults[pathKey], pathMatch{
										parts: resolvedParts,
										match: cleanedNestedMatch,
									})
								}

								// Store grouped results using storeAtPathSegments to handle formatters
								for _, pathMatches := range groupedNestedResults {
									if len(pathMatches) == 0 {
										continue
									}

									// All matches in this group have the same resolved path parts
									// Make a deep copy of resolvedParts so we can modify the formatter
									resolvedParts := make([]struct {
										key       string
										formatter string
									}, len(pathMatches[0].parts))
									copy(resolvedParts, pathMatches[0].parts)

									// Python TTP: nested groups with dynamic paths follow normal rule:
									// single match = map, multiple matches = list
									var valueToStore interface{}
									if len(pathMatches) == 1 {
										// Single match - store as map
										valueToStore = pathMatches[0].match
									} else {
										// Multiple matches - store as list
										resultList := make([]interface{}, len(pathMatches))
										for i, pm := range pathMatches {
											resultList[i] = pm.match
										}
										valueToStore = resultList
									}

									// Ensure final segment has * if multiple matches
									if len(pathMatches) > 1 {
										if len(resolvedParts) > 0 && resolvedParts[len(resolvedParts)-1].formatter == "" {
											resolvedParts[len(resolvedParts)-1].formatter = "*"
										}
									}
									// Store at the path
									r.storeAtPathSegments(parentMatch, resolvedParts, valueToStore)
									
									// Track variables that were stored in nested groups to remove from parent
									if valueMap, ok := valueToStore.(map[string]interface{}); ok {
										for k := range valueMap {
											allNestedVars[k] = true
										}
									} else if valueList, ok := valueToStore.([]interface{}); ok {
										for _, item := range valueList {
											if itemMap, ok := item.(map[string]interface{}); ok {
												for k := range itemMap {
													allNestedVars[k] = true
												}
											}
										}
									}
								}
								// IMPORTANT: Python TTP removes variables from parent match that are also in nested groups
								// BUT only if they have the same value (to avoid removing variables that are different)
								// AND only if they're not used by group attributes like record (which need them for grouping)
								// This prevents duplication - if a variable is matched by nested group with the same value,
								// it should only exist in the nested group, not in the parent
								// Note: For multiple nested matches, we check each match individually
								for k := range allNestedVars {
									// Don't remove if this variable is used by record attribute (needed for grouping)
									if recordVar != "" && k == recordVar {
										continue
									}
									// Check if this variable exists in parent and has the same value in any nested match
									if parentVal, exists := parentMatch[k]; exists {
										// Check all nested matches to see if any have the same value
										shouldRemove := false
										for _, pathMatches := range groupedNestedResults {
											for _, pm := range pathMatches {
												if nestedVal, ok := pm.match[k]; ok {
													if reflect.DeepEqual(parentVal, nestedVal) {
														shouldRemove = true
														break
													}
												}
											}
											if shouldRemove {
												break
											}
										}
										if shouldRemove {
											delete(parentMatch, k)
										}
									}
								}
							case map[string]interface{}:
								// Single nested match - resolve path and store
								// Merge parent match vars with nested match for path resolution
								// IMPORTANT: Deep copy to avoid sharing references
								resolutionVars := make(map[string]interface{})
								for k, val := range parentMatch {
									if vMap, ok := val.(map[string]interface{}); ok {
										copiedMap := make(map[string]interface{})
										for k2, v2 := range vMap {
											copiedMap[k2] = v2
										}
										resolutionVars[k] = copiedMap
									} else if vSlice, ok := val.([]interface{}); ok {
										copiedSlice := make([]interface{}, len(vSlice))
										copy(copiedSlice, vSlice)
										resolutionVars[k] = copiedSlice
									} else {
										resolutionVars[k] = val
									}
								}
								for k, val := range v {
									resolutionVars[k] = val
								}
								for k, val := range vars {
									resolutionVars[k] = val
								}

								// Resolve path segment-by-segment to preserve formatters
								resolvedParts := r.resolvePathSegments(nestedGroup.Name, v, resolutionVars)
								if len(resolvedParts) == 0 {
									// Fallback - use original name
									resolvedParts = r.resolvePathSegments(nestedGroup.Name, v, resolutionVars)
									if len(resolvedParts) == 0 {
										continue
									}
								}

								// Remove path variables from nested match
								pathVars := r.pathResolver.ExtractVariablesFromPath(nestedGroup.Name)
								pathVarSet := make(map[string]bool)
								for _, pv := range pathVars {
									pathVarSet[pv] = true
								}
								cleanedNestedMatch := make(map[string]interface{})
								for k, val := range v {
									if !pathVarSet[k] {
										cleanedNestedMatch[k] = val
									}
								}

								// Python TTP: nested groups with dynamic paths follow normal rule:
								// single match = map, multiple matches = list
								// Store as map (single match) - matches Python TTP behavior
								r.storeAtPathSegments(parentMatch, resolvedParts, cleanedNestedMatch)
								
								// IMPORTANT: Python TTP removes variables from parent match that are also in nested group
								// BUT only if they have the same value (to avoid removing variables that are different)
								// AND only if they're not used by group attributes like record (which need them for grouping)
								// This prevents duplication - if a variable is matched by nested group with the same value,
								// it should only exist in the nested group, not in the parent
								for k, nestedVal := range cleanedNestedMatch {
									if parentVal, exists := parentMatch[k]; exists {
										// Don't remove if this variable is used by record attribute (needed for grouping)
										if recordVar != "" && k == recordVar {
											continue
										}
										// Only remove if values are the same (Python TTP behavior)
										if reflect.DeepEqual(parentVal, nestedVal) {
											delete(parentMatch, k)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Special handling for record attribute - group matches by recorded variable value
	// This must happen BEFORE other group functions, as it affects how matches are grouped
	// Python TTP: when record="vrf" is used, matches are grouped by the vrf variable value
	// Matches with the same vrf value are grouped together, different vrf values are separate entries
	if recordAttr, ok := group.Attributes["record"]; ok && recordAttr != "" {
		recordVar := strings.TrimSpace(recordAttr)
		
		groupedByRecord := make(map[string][]map[string]interface{})
		
		for _, match := range mergedMatches {
			// Get the value of the record variable from the match
			recordValue := ""
			if val, exists := match[recordVar]; exists {
				// Convert to string for grouping
				recordValue = fmt.Sprintf("%v", val)
			}
			groupedByRecord[recordValue] = append(groupedByRecord[recordValue], match)
		}
		
		// Convert grouped matches to a list (Python TTP behavior: record groups matches as a list)
		// Each unique record value becomes a separate entry in the list
		var groupedMatches []map[string]interface{}
		for _, matches := range groupedByRecord {
			// For each group of matches with the same record value, merge them into one match
			// Python TTP: when record is used, matches with the same record value are grouped together
			if len(matches) > 0 {
				// Merge all matches with the same record value into one match
				mergedMatch := make(map[string]interface{})
				for _, m := range matches {
					for k, v := range m {
						// If key already exists and values differ, convert to list
						if existing, exists := mergedMatch[k]; exists {
							if !reflect.DeepEqual(existing, v) {
								// Convert to list if not already a list
								if existingList, ok := existing.([]interface{}); ok {
									mergedMatch[k] = append(existingList, v)
								} else {
									mergedMatch[k] = []interface{}{existing, v}
								}
							}
						} else {
							mergedMatch[k] = v
						}
					}
				}
				groupedMatches = append(groupedMatches, mergedMatch)
			}
		}
		
		// Replace mergedMatches with grouped matches
		mergedMatches = groupedMatches
	}

	// Process group function attributes (containsall, contains, del, exclude, excludeall, etc.)
	// These are group attributes that act as functions to filter/transform matches
	// They should be processed before the functions/chain attribute
	// Note: record attribute is handled above, so we skip it here
	groupFunctionAttrs := []string{"containsall", "contains", "del", "delete", "exclude", "excludeall", "equal", "to_int", "contains_val", "exclude_val", "sformat", "itemize", "set", "expand", "items2dict", "to_ip"}
	for _, attrName := range groupFunctionAttrs {
		// Special handling for expand - empty string means expand all dot-separated keys
		if attrName == "expand" {
			if _, ok := group.Attributes[attrName]; ok {
				// expand="" or expand="target" - both should work
				attrValue := group.Attributes[attrName]
				// If empty string, expand all dot-separated keys
				// Otherwise, expand only specified keys
				// Always process expand if the attribute exists
				// Get the function from registry (checks runtime overrides first)
				fn, ok := r.getGroupFunc(attrName)
				if !ok {
					continue
				}

				// Prepare kwargs
				kwargs := make(map[string]interface{})
				for k, v := range vars {
					kwargs[k] = v
				}
				if r.recordedVars != nil {
					for k, v := range r.recordedVars {
						kwargs[k] = v
					}
				}

				// Apply expand to all matches
				var expandedMatches []map[string]interface{}
				for _, match := range mergedMatches {
					// Make a copy
					data := make(map[string]interface{})
					for k, v := range match {
						data[k] = v
					}

					// Execute expand function
					// If attrValue is empty, expand all; otherwise use as args
					args := []string{}
					if attrValue != "" {
						args = strings.Split(attrValue, ",")
						for i := range args {
							args[i] = strings.TrimSpace(args[i])
						}
					}

					newData, keep, err := fn(data, args, kwargs)
					if err != nil {
						return nil, fmt.Errorf("group function %s failed: %w", attrName, err)
					}

					if keep {
						expandedMatches = append(expandedMatches, newData)
					}
				}
				mergedMatches = expandedMatches
				continue // Continue to next attribute in the loop
			}
		}

		if attrValue, ok := group.Attributes[attrName]; ok && attrValue != "" {
			var args []string
			kwargs := make(map[string]interface{})

			// Special handling for sformat which uses keyword arguments
			if attrName == "sformat" {
				// Parse keyword arguments: string='...', add_field='...'
				// Format: string='{hostname}.{fqdn},{domain}', add_field='fqdn'
				// We need to parse this manually to handle commas inside quoted strings
				var current strings.Builder
				inQuotes := false
				quoteChar := byte(0)
				parts := []string{}

				for i := 0; i < len(attrValue); i++ {
					char := attrValue[i]
					if !inQuotes && (char == '"' || char == '\'') {
						inQuotes = true
						quoteChar = char
						current.WriteByte(char)
					} else if inQuotes && char == quoteChar {
						inQuotes = false
						quoteChar = 0
						current.WriteByte(char)
					} else if !inQuotes && char == ',' {
						// Split on comma only if not in quotes
						part := strings.TrimSpace(current.String())
						if part != "" {
							parts = append(parts, part)
						}
						current.Reset()
					} else {
						current.WriteByte(char)
					}
				}
				// Add last part
				part := strings.TrimSpace(current.String())
				if part != "" {
					parts = append(parts, part)
				}

				// Extract string and add_field from keyword args
				for _, part := range parts {
					if strings.HasPrefix(part, "string=") {
						// Extract value after string=
						value := strings.TrimPrefix(part, "string=")
						value = strings.Trim(value, `"'`)
						args = append(args, value)
					} else if strings.HasPrefix(part, "add_field=") {
						// Extract value after add_field=
						value := strings.TrimPrefix(part, "add_field=")
						value = strings.Trim(value, `"'`)
						args = append(args, value)
					}
				}
			} else {
				// Parse comma-separated arguments for other functions
				args = strings.Split(attrValue, ",")
				for i := range args {
					args[i] = strings.TrimSpace(args[i])
					// Remove quotes if present
					args[i] = strings.Trim(args[i], `"'`)
				}
			}

			// Get the function from registry (checks runtime overrides first)
			fn, ok := r.getGroupFunc(attrName)
			if !ok {
				continue // Function not found, skip
			}

			// Prepare kwargs (include template variables and recorded vars)
			// Order: 1) compiled template vars (from <vars> tag), 2) runtime vars, 3) recorded vars
			// Recorded vars override runtime vars, which override template vars
			// First add compiled template vars (from <vars> tag)
			if r.compiled.Vars != nil {
				for k, v := range r.compiled.Vars {
					kwargs[k] = v
				}
			}
			// Then add runtime vars (override template vars)
			for k, v := range vars {
				kwargs[k] = v
			}
			// Finally add recorded vars (from record() function) - these override everything
			if r.recordedVars != nil {
				for k, v := range r.recordedVars {
					kwargs[k] = v
				}
			}
			// Pass r.recordedVars to record() function so it can store values globally
			kwargs["_recorded_vars"] = r.recordedVars

			// Special handling for itemize - it needs to collect values and return a list
			if attrName == "itemize" {
				// Collect all itemized values
				var itemizedValues []interface{}
				for _, match := range mergedMatches {
					// Make a copy to avoid modifying the original
					data := make(map[string]interface{})
					for k, v := range match {
						data[k] = v
					}

					// Execute function
					newData, keep, err := fn(data, args, kwargs)
					if err != nil {
						return nil, fmt.Errorf("group function %s failed: %w", attrName, err)
					}

					// If function returns true and has _itemize_value, collect it
					if keep {
						if itemizeValue, ok := newData["_itemize_value"]; ok {
							itemizedValues = append(itemizedValues, itemizeValue)
						}
					}
				}
				// Return the list of itemized values directly
				if len(itemizedValues) > 0 {
					return itemizedValues, nil
				}
				// If no values, return empty list
				return []interface{}{}, nil
			}

			// Apply function to filter matches
			var filteredMatches []map[string]interface{}
			for _, match := range mergedMatches {
				// Make a copy to avoid modifying the original
				data := make(map[string]interface{})
				for k, v := range match {
					data[k] = v
				}

				// Execute function
				newData, keep, err := fn(data, args, kwargs)
				if err != nil {
					return nil, fmt.Errorf("group function %s failed: %w", attrName, err)
				}

				// If function returns true, keep this match
				if keep {
					filteredMatches = append(filteredMatches, newData)
				}
			}
			mergedMatches = filteredMatches
		}
	}

	// Process custom group functions specified as group attributes
	// These are not in the hardcoded list above, so we check runtime and compile-time overrides
	if group.Attributes != nil && (r.runtimeFunctions != nil && r.runtimeFunctions.Group != nil || r.compileFunctions != nil && r.compileFunctions.Group != nil) {
		for attrName, attrValue := range group.Attributes {
			// Skip attributes already handled by the hardcoded list above
			alreadyHandled := false
			for _, builtinAttr := range groupFunctionAttrs {
				if attrName == builtinAttr {
					alreadyHandled = true
					break
				}
			}
			if alreadyHandled {
				continue
			}
			// Skip known non-function attributes
			if attrName == "name" || attrName == "input" || attrName == "method" || attrName == "output" ||
				attrName == "default" || attrName == "chain" || attrName == "functions" || attrName == "macro" ||
				attrName == "record" {
				continue
			}
			// Check if this attribute name matches a custom group function
			// Uses getGroupFunc for proper precedence: runtime > compile-time > built-in
			fn, ok := r.getGroupFunc(attrName)
			if !ok {
				continue
			}
			// Parse args from the attribute value
			var args []string
			if attrValue != "" {
				args = strings.Split(attrValue, ",")
				for i := range args {
					args[i] = strings.TrimSpace(args[i])
					args[i] = strings.Trim(args[i], `"'`)
				}
			}
			// Prepare kwargs
			kwargs := make(map[string]interface{})
			for k, v := range vars {
				kwargs[k] = v
			}
			if r.recordedVars != nil {
				for k, v := range r.recordedVars {
					kwargs[k] = v
				}
			}
			// Apply function to each match
			var filteredMatches []map[string]interface{}
			for _, matchData := range mergedMatches {
				dataCopy := make(map[string]interface{})
				for k, v := range matchData {
					dataCopy[k] = v
				}
				newData, keep, err := fn(dataCopy, args, kwargs)
				if err != nil {
					return nil, fmt.Errorf("custom group function %s failed: %w", attrName, err)
				}
				if keep {
					filteredMatches = append(filteredMatches, newData)
				}
			}
			mergedMatches = filteredMatches
		}
	}

	// Process group functions (chain or functions attribute)
	// chain and functions are the same - pipe-separated function calls
	chainStr := group.Chain
	if chainStr == "" {
		chainStr = group.Functions
	}

	if chainStr != "" {
		// Parse and apply chain functions
		processedMatches, err := r.processGroupFunctions(mergedMatches, chainStr, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process group functions: %w", err)
		}
		mergedMatches = processedMatches

		// Check if itemize was used - if so, return the value directly
		if len(mergedMatches) > 0 {
			if itemizeValue, ok := mergedMatches[0]["_itemize_value"]; ok {
				// itemize returns the value directly, not as a map
				// Return the value as-is (it's already wrapped in a list)
				return itemizeValue, nil
			}
		}
	}

	// Process group macro if specified
	if group.Macro != "" {
		processedMatches, err := r.processGroupMacro(mergedMatches, group.Macro, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process group macro: %w", err)
		}
		mergedMatches = processedMatches
	}

	// Apply group defaults (from unconditional set() functions)
	// These are set during compilation when a variable with set() is on a line with no other text
	// Python TTP: when set() is used without text to match, it sets skip_regex_dict = True
	// and adds the value to group.defaults, which is applied to all matches
	// This should be applied BEFORE the default attribute, so defaults from set() can be overridden by default attribute
	if len(group.Defaults) > 0 {
		for _, match := range mergedMatches {
			for varName, defaultValue := range group.Defaults {
				// Only set if key doesn't exist in match (matches Python TTP behavior)
				if _, exists := match[varName]; !exists {
					match[varName] = defaultValue
				}
			}
		}
	}

	// Apply default attribute if specified
	if defaultAttr, ok := group.Attributes["default"]; ok && defaultAttr != "" {
		// Get default value - could be a literal string or a template variable name
		var defaultValue interface{}

		// Check if it's a template variable - check both runtime vars and compiled template vars
		// Runtime vars take precedence over template vars
		var varValue interface{} = defaultAttr // default to literal string
		foundVar := false
		if vars != nil {
			if val, exists := vars[defaultAttr]; exists {
				varValue = val
				foundVar = true
			} else if r.compiled.Vars != nil {
				// Check compiled template vars (from <vars> tag)
				if val, exists := r.compiled.Vars[defaultAttr]; exists {
					varValue = val
					foundVar = true
				}
			}
		} else if r.compiled.Vars != nil {
			// No runtime vars, check compiled template vars
			if val, exists := r.compiled.Vars[defaultAttr]; exists {
				varValue = val
				foundVar = true
			}
		}

		// If we found a variable value, use it; otherwise use as literal string
		if foundVar {
			defaultValue = varValue
		} else {
			defaultValue = defaultAttr
		}

		// Apply default to each match
		if defaultValue != nil {
			// If default is a dictionary/map, merge it into each match
			if defaultMap, isMap := defaultValue.(map[string]interface{}); isMap {
				// Merge default dictionary into each match result
				for _, match := range mergedMatches {
					// Merge default values into match (don't overwrite existing values)
					for k, v := range defaultMap {
						// Only set if key doesn't exist in match (matches Python TTP behavior)
						if _, exists := match[k]; !exists {
							match[k] = v
						}
					}
				}
			} else if defaultStr, isStr := defaultValue.(string); isStr {
				// If default is a string, apply it to all unmatched variables in each match
				// Collect all variable names from all patterns in the group
				allVarNames := make(map[string]bool)
				for _, pattern := range group.Patterns {
					for varName := range pattern.Variables {
						// Skip special variables that don't get defaults
						if varName != "ignore" && varName != "_start_" && varName != "_end_" && varName != "_line_" {
							allVarNames[varName] = true
						}
					}
				}

				// Apply default to unmatched variables in each match
				for _, match := range mergedMatches {
					for varName := range allVarNames {
						if _, exists := match[varName]; !exists {
							match[varName] = defaultStr
						}
					}
				}

				// If group has no matches but has a default string, create a result with all variables set to default
				// This matches Python TTP behavior when a group with default has no matches
				if len(mergedMatches) == 0 && len(allVarNames) > 0 {
					defaultMatch := make(map[string]interface{})
					for varName := range allVarNames {
						defaultMatch[varName] = defaultStr
					}
					mergedMatches = append(mergedMatches, defaultMatch)
				}
			}
		}
	}

	// YANG validation for parent group (after functions and macros, before returning)
	if r.yangValidator != nil && mergedMatches != nil {
		if yangPath, ok := group.Attributes["yang"]; ok && yangPath != "" {
			validationResult := r.yangValidator.Validate(mergedMatches, yangPath, group.Name)
			// Store validation result
			r.validationResults[group.Name] = validationResult
		}
	}

	// Check if group has no matches but has variables with default() function
	// If the start pattern has variables with default(), create a result with defaults
	// This matches Python TTP behavior: if no matches found for start REs but has_start_re_default,
	// create a result structure populated with default values
	if len(mergedMatches) == 0 && len(group.Patterns) > 0 {
		// Check if first pattern (start pattern) has variables with default()
		firstPattern := group.Patterns[0]
		hasStartDefault := false
		defaultValues := make(map[string]interface{})

		for varName, variable := range firstPattern.Variables {
			// Skip special variables
			if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_line_" {
				continue
			}

			// Check if this variable has default() function
			for _, funcStr := range variable.Functions {
				if strings.HasPrefix(funcStr, "default(") {
					hasStartDefault = true
					// Extract default value from default("value")
					start := strings.Index(funcStr, "(")
					end := strings.LastIndex(funcStr, ")")
					if start >= 0 && end > start {
						defaultVal := funcStr[start+1 : end]
						defaultVal = strings.TrimSpace(defaultVal)
						defaultVal = strings.Trim(defaultVal, `"'`)
						defaultValues[varName] = defaultVal
					}
					break
				}
			}
		}

		// If start pattern has defaults, create a result with all variables that have defaults
		if hasStartDefault {
			// Collect all variables with defaults from all patterns
			for _, pattern := range group.Patterns {
				for varName, variable := range pattern.Variables {
					// Skip special variables
					if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_line_" {
						continue
					}

					// Check if this variable has default() function
					for _, funcStr := range variable.Functions {
						if strings.HasPrefix(funcStr, "default(") {
							// Extract default value from default("value")
							start := strings.Index(funcStr, "(")
							end := strings.LastIndex(funcStr, ")")
							if start >= 0 && end > start {
								defaultVal := funcStr[start+1 : end]
								defaultVal = strings.TrimSpace(defaultVal)
								defaultVal = strings.Trim(defaultVal, `"'`)
								// Only set if not already set (first pattern takes precedence)
								if _, exists := defaultValues[varName]; !exists {
									defaultValues[varName] = defaultVal
								}
							}
							break
						}
					}
				}
			}

			// Create a result with default values
			if len(defaultValues) > 0 {
				defaultMatch := make(map[string]interface{})
				for k, v := range defaultValues {
					defaultMatch[k] = v
				}
				mergedMatches = append(mergedMatches, defaultMatch)
			}
		}
	}

	// Python TTP quirk: when ignore() uses a template variable, return only one empty result
	// Check if any pattern has ignore with template variable
	hasIgnoreWithTemplateVar := false
	for _, compiledPattern := range group.Patterns {
		for _, variable := range compiledPattern.Variables {
			if variable.Name == "ignore" && variable.IgnoreUsesTemplateVar {
				hasIgnoreWithTemplateVar = true
				break
			}
		}
		if hasIgnoreWithTemplateVar {
			break
		}
	}

	// If ignore uses template variable and we have empty results, return only one empty result
	if hasIgnoreWithTemplateVar {
		emptyResults := 0
		for _, match := range mergedMatches {
			if len(match) == 0 {
				emptyResults++
			}
		}
		// If all results are empty, return only one empty result
		if emptyResults == len(mergedMatches) && len(mergedMatches) > 0 {
			return []map[string]interface{}{{}}, nil
		}
	}

	// Filter out empty matches from mergedMatches
	// This can happen when a match is started but not populated (e.g., _start_ pattern with no other variables)
	filteredMatches := make([]map[string]interface{}, 0, len(mergedMatches))
	for _, match := range mergedMatches {
		if len(match) > 0 {
			filteredMatches = append(filteredMatches, match)
		}
	}
	mergedMatches = filteredMatches

	// Return results based on method
	switch group.Method {
	case "table":
		return mergedMatches, nil
	default:
		if len(mergedMatches) == 1 {
			return mergedMatches[0], nil
		}
		return mergedMatches, nil
	}
}

// applyGroupFilterStreaming runs the group's chain/functions attribute against
// a single record. Returns nil to indicate the record should be dropped (e.g.
// containsall failed). Implemented by wrapping the per-record value in a
// single-element slice and calling processGroupFunctions, which is the same
// path batch mode uses (preserving behavior).
//
// For streamable groups the allowlist guarantees these functions are
// per-record (no aggregation), so a single-record invocation is well-defined.
func (r *Runtime) applyGroupFilterStreaming(
	record map[string]interface{},
	group *compiler.CompiledGroup,
	vars map[string]interface{},
) (map[string]interface{}, error) {
	chainStr := group.Chain
	if chainStr == "" {
		chainStr = group.Functions
	}
	if chainStr == "" {
		return record, nil
	}
	out, err := r.processGroupFunctions([]map[string]interface{}{record}, chainStr, vars)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out[0], nil
}

// applyGroupMacroStreaming applies the group's macro chain to a single record.
// Implemented by wrapping in a single-element slice and calling
// processGroupMacro, which preserves the macro semantics used by batch mode.
func (r *Runtime) applyGroupMacroStreaming(
	record map[string]interface{},
	group *compiler.CompiledGroup,
	vars map[string]interface{},
) (map[string]interface{}, error) {
	if group.Macro == "" {
		return record, nil
	}
	out, err := r.processGroupMacro([]map[string]interface{}{record}, group.Macro, vars)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out[0], nil
}

// parseGroupStream is the streaming variant of parseGroup. It drives the same
// merge state machine (stepMerge) as parseGroup but invokes fn for each
// completed record instead of accumulating into a slice. Phase 1+2 (pattern
// dispatch and match collection into allMatches) is shared with parseGroup
// via prepareGroupMerge — allMatches itself is bounded and not the source of
// the heap pressure we're solving; the win is bypassing the post-merge
// mergedMatches accumulation and intermediate slices in
// processGroupFunctions / processGroupMacro / extractMatchResult.
//
// Caller must verify group.Streamable == true. parseGroupStream does not
// re-check (entry gate is in Runtime.ParseStream).
//
// Streamability guarantees from Phase A simplify the body:
//   - No nested groups → no recursion needed
//   - No joinmatches → joinMatchesVars / joinMatchesHasToList are empty
//   - No itemize → no aggregating group functions
//   - Group chain functions are from the per-record allowlist
func (r *Runtime) parseGroupStream(
	group *compiler.CompiledGroup,
	inputData string,
	vars map[string]interface{},
	fn func(record map[string]interface{}, srcRange [2]int, groupPath string) error,
) error {
	if len(group.Patterns) == 0 {
		return nil
	}

	// Compute indicator/joinmatches metadata once. This is cheap and does not
	// scan input. We deliberately do NOT call prepareGroupMerge — that would
	// fully materialize allMatches with extracted results (~256K entries on
	// the phy fixture, ~500 MB peak heap). Instead we scan line-by-line below
	// and feed stepMerge incrementally so each match's result map can be
	// flushed and freed as we go.
	indicator := computeStreamIndicatorMeta(group)

	// For streamable groups, joinmatches is forbidden by the streamability
	// rule (rule 3), so joinMatchesVars and joinMatchesHasToList are
	// guaranteed empty. We still pass them through stepMerge for signature
	// parity.
	joinMatchesVars := indicator.joinMatchesVars
	joinMatchesHasToList := indicator.joinMatchesHasToList

	state := newMergeState()
	// Streamable groups have no nested children — the parent->matches map is
	// never read in streaming mode, so disable the bookkeeping. Otherwise
	// the map grows O(records) and dominates the peak heap on large inputs.
	state.skipParentBookkeeping = true
	state.flush = func(record map[string]interface{}, srcRange [2]int) error {
		// Apply per-record group filter and macro that parseGroup applies
		// post-merge. For streamable groups the allowlist guarantees these
		// are per-record (no aggregation).
		filtered, err := r.applyGroupFilterStreaming(record, group, vars)
		if err != nil {
			return err
		}
		if filtered == nil {
			return nil // dropped by filter
		}
		filtered, err = r.applyGroupMacroStreaming(filtered, group, vars)
		if err != nil {
			return err
		}
		if filtered == nil {
			return nil // dropped by macro
		}
		return fn(filtered, srcRange, group.NormalizedPath)
	}

	// Pre-extract per-pattern indicator booleans (mirrors prepareGroupMerge's
	// per-line-per-pattern reads of pattern.Variables for _start_/_end_).
	type patInfo struct {
		hasStartIndicator bool
		hasEndIndicator   bool
	}
	patInfos := make([]patInfo, len(group.Patterns))
	for i, p := range group.Patterns {
		var info patInfo
		for varName, v := range p.Variables {
			if varName == "_start_" {
				info.hasStartIndicator = true
			}
			if varName == "_end_" {
				info.hasEndIndicator = true
			}
			for _, funcStr := range v.Functions {
				if funcStr == "_start_" {
					info.hasStartIndicator = true
				}
				if funcStr == "_end_" {
					info.hasEndIndicator = true
				}
			}
		}
		patInfos[i] = info
	}

	// Streamability rule 4: every pattern must be line-anchored (or the group
	// must have _start_; in practice the engine auto-anchors). Enforce
	// HasAnchors here so we can iterate line-major. If a non-anchored pattern
	// slips through, treat it as a streamability classifier bug.
	for patternIdx, p := range group.Patterns {
		if !p.HasAnchors {
			return fmt.Errorf(
				"parseGroupStream: pattern %d in group %q is not line-anchored; "+
					"streamability classifier bug",
				patternIdx, group.Name)
		}
	}

	// Line-major, pattern-major iteration. All matches at the same line share
	// the same spanStart (= line start offset), and stepMerge's forward
	// lookahead at matchIdx+1 walks only while spanStart == match.spanStart,
	// so passing per-line slices preserves lookahead semantics. method="table"
	// is not in the streamable allowlist, so the dedup logic for that branch
	// is irrelevant here.
	//
	// We walk the input as a single string with manual newline scanning
	// (instead of strings.Split) to avoid materializing a slice of ~256K
	// string headers (~4 MB for the phy fixture).
	lineMatches := make([]patternMatch, 0, len(group.Patterns))
	totalLen := len(inputData)

	pos := 0
	lineIdx := 0
	for pos <= totalLen {
		// Find next newline (or end of input).
		nl := strings.IndexByte(inputData[pos:], '\n')
		var rawLine string
		var lineStartOffset = pos
		if nl < 0 {
			rawLine = inputData[pos:]
			pos = totalLen + 1 // terminate loop
		} else {
			rawLine = inputData[pos : pos+nl]
			pos = pos + nl + 1
		}
		curLineIdx := lineIdx
		lineIdx++

		// Trim \r and trailing spaces (mirror prepareGroupMerge).
		line := strings.TrimRight(rawLine, "\r \t")
		trimmedLine := strings.TrimSpace(line)

		// Build per-line matches. Reset slice (keep backing array).
		lineMatches = lineMatches[:0]

		for patternIdx, compiledPattern := range group.Patterns {
			info := patInfos[patternIdx]

			// Skip empty lines unless this pattern has _start_/_end_.
			if trimmedLine == "" && !info.hasStartIndicator && !info.hasEndIndicator {
				continue
			}

			matchLine := line
			if trimmedLine == "" && (info.hasStartIndicator || info.hasEndIndicator) {
				matchLine = ""
			}

			match := compiledPattern.Regex.FindStringSubmatch(matchLine)
			if match == nil {
				continue
			}

			result := r.extractMatchResult(match, compiledPattern, vars)
			if result == nil {
				continue
			}
			if len(result) == 0 && !compiledPattern.HasOnlySpecialIndicators &&
				!compiledPattern.IgnoreUsesTemplateVar && !compiledPattern.HasJoinMatches {
				continue
			}

			spanStart := lineStartOffset
			spanEnd := lineStartOffset + len(line)
			lineMatches = append(lineMatches, patternMatch{
				patternIdx: patternIdx,
				spanStart:  spanStart,
				spanEnd:    spanEnd,
				lineIdx:    curLineIdx,
				result:     result,
			})
		}

		if len(lineMatches) == 0 {
			continue
		}

		// Apply the same redundant-pure-indicator filter that prepareGroupMerge
		// applies for non-table groups. Within a single line all matches share
		// the same spanStart, so the filter is local to the line.
		lineMatches = filterRedundantIndicatorsInLine(lineMatches, group)

		// Feed each match through stepMerge. allMatches passed here is the
		// per-line buffer; matchIdx is the position within the line. The
		// forward lookahead at matchIdx+1 only walks while spanStart matches,
		// which is exactly the line boundary, so this is correct.
		for k := range lineMatches {
			if err := r.stepMerge(state, lineMatches, k, group,
				joinMatchesVars, joinMatchesHasToList,
				indicator.startPatterns, indicator.endPatterns,
				indicator.hasLineIndicator, indicator.hasAnyStartIndicator,
				indicator.hasEmptyStartPattern); err != nil {
				return err
			}
		}
	}

	// Final flush — the last in-flight currentMatch. End position is end of
	// input. Mirrors the post-loop "Add final match" logic in parseGroup but
	// without the parentMatchToAllMatches bookkeeping (no nested groups in
	// streamable templates).
	if state.currentMatch != nil {
		hasNonSpecialVars := false
		for k := range state.currentMatch {
			if k != "ignore" && k != "_start_" && k != "_end_" && k != "_line_" {
				hasNonSpecialVars = true
				break
			}
		}
		if hasNonSpecialVars {
			matchCopy := make(map[string]interface{})
			for k, v := range state.currentMatch {
				matchCopy[k] = v
			}
			srcRange := [2]int{state.currentStartPos, totalLen}
			if err := state.flush(matchCopy, srcRange); err != nil {
				return err
			}
			state.recordCount++
			r.pathResolver.UpdateCache(matchCopy)
			r.matchCollector.Clear()
		}
	}

	return nil
}

// streamIndicatorMeta bundles the indicator/joinmatches metadata that
// stepMerge consumes. Computed once at the top of parseGroupStream.
type streamIndicatorMeta struct {
	joinMatchesVars      map[string]bool
	joinMatchesHasToList map[string]bool
	startPatterns        map[int]bool
	endPatterns          map[int]bool
	hasLineIndicator     bool
	hasAnyStartIndicator bool
	hasEmptyStartPattern bool
}

// computeStreamIndicatorMeta extracts the indicator/joinmatches metadata
// that prepareGroupMerge computes — without scanning input. This lifts only
// the cheap, per-group, per-pattern-metadata work; the per-line/per-match
// extraction is left to the streaming loop in parseGroupStream.
func computeStreamIndicatorMeta(group *compiler.CompiledGroup) streamIndicatorMeta {
	meta := streamIndicatorMeta{
		joinMatchesVars:      make(map[string]bool),
		joinMatchesHasToList: make(map[string]bool),
		startPatterns:        make(map[int]bool),
		endPatterns:          make(map[int]bool),
	}

	// joinmatches metadata. Streamability rule 3 forbids joinmatches, but we
	// compute defensively for parity with prepareGroupMerge.
	for _, pattern := range group.Patterns {
		for varName, variable := range pattern.Variables {
			hasToList := false
			for _, funcStr := range variable.Functions {
				if funcStr == "to_list" {
					hasToList = true
				}
				if strings.HasPrefix(funcStr, "joinmatches") {
					meta.joinMatchesVars[varName] = true
					meta.joinMatchesHasToList[varName] = hasToList
					break
				}
			}
		}
	}

	// Detect hasAnyStartIndicator and hasAnyEndIndicator.
	if group.Method == "table" {
		for patternIdx := range group.Patterns {
			meta.startPatterns[patternIdx] = true
		}
	} else {
		hasAnyStartIndicator := false
		for _, compiledPattern := range group.Patterns {
			for _, variable := range compiledPattern.Variables {
				if variable.Name == "_start_" {
					hasAnyStartIndicator = true
					break
				}
				for _, funcStr := range variable.Functions {
					if funcStr == "_start_" {
						hasAnyStartIndicator = true
						break
					}
				}
				if hasAnyStartIndicator {
					break
				}
			}
			if hasAnyStartIndicator {
				break
			}
		}
		meta.hasAnyStartIndicator = hasAnyStartIndicator

		hasAnyEndIndicator := false
		for _, compiledPattern := range group.Patterns {
			for _, variable := range compiledPattern.Variables {
				if variable.Name == "_end_" {
					hasAnyEndIndicator = true
					break
				}
				for _, funcStr := range variable.Functions {
					if funcStr == "_end_" {
						hasAnyEndIndicator = true
						break
					}
				}
				if hasAnyEndIndicator {
					break
				}
			}
			if hasAnyEndIndicator {
				break
			}
		}

		for patternIdx, compiledPattern := range group.Patterns {
			patternHasJoinMatches := false
			for _, variable := range compiledPattern.Variables {
				for _, funcStr := range variable.Functions {
					if strings.HasPrefix(funcStr, "joinmatches") {
						patternHasJoinMatches = true
						break
					}
				}
				if patternHasJoinMatches {
					break
				}
			}

			if hasAnyStartIndicator && hasAnyEndIndicator {
				patternHasStart := false
				for _, variable := range compiledPattern.Variables {
					if variable.Name == "_start_" {
						patternHasStart = true
						break
					}
					for _, funcStr := range variable.Functions {
						if funcStr == "_start_" {
							patternHasStart = true
							break
						}
					}
					if patternHasStart {
						break
					}
				}
				if patternHasStart {
					meta.startPatterns[patternIdx] = true
				}
			} else if hasAnyStartIndicator {
				firstStartPatternIdx := -1
				for idx, pattern := range group.Patterns {
					for _, variable := range pattern.Variables {
						if variable.Name == "_start_" {
							firstStartPatternIdx = idx
							break
						}
						for _, funcStr := range variable.Functions {
							if funcStr == "_start_" {
								firstStartPatternIdx = idx
								break
							}
						}
						if firstStartPatternIdx >= 0 {
							break
						}
					}
					if firstStartPatternIdx >= 0 {
						break
					}
				}

				patternHasStart := false
				for _, variable := range compiledPattern.Variables {
					if variable.Name == "_start_" {
						patternHasStart = true
						break
					}
					for _, funcStr := range variable.Functions {
						if funcStr == "_start_" {
							patternHasStart = true
							break
						}
					}
					if patternHasStart {
						break
					}
				}
				if patternHasStart {
					meta.startPatterns[patternIdx] = true
				} else if firstStartPatternIdx >= 0 && patternIdx < firstStartPatternIdx {
					hasNonSpecialVars := false
					for varName := range compiledPattern.Variables {
						if varName != "_start_" && varName != "_end_" && varName != "_line_" && varName != "ignore" {
							hasNonSpecialVars = true
							break
						}
					}
					if hasNonSpecialVars {
						meta.startPatterns[patternIdx] = true
					}
				}
			}

			for _, variable := range compiledPattern.Variables {
				if variable.Name == "_start_" || variable.Name == "_line_" {
					if variable.Name == "_line_" && patternHasJoinMatches {
						if !hasAnyStartIndicator {
							meta.startPatterns[patternIdx] = false
							meta.startPatterns[patternIdx] = true
						}
					} else {
						meta.startPatterns[patternIdx] = true
					}
					if len(compiledPattern.Variables) == 1 && variable.Name == "_line_" {
						meta.hasLineIndicator = true
					}
					if len(compiledPattern.Variables) == 1 && variable.Name == "_start_" {
						if compiledPattern.Regex != nil {
							regexStr := compiledPattern.Regex.String()
							if regexStr == "^.*$" || regexStr == "(?m)^.*$" || regexStr == "^$" || regexStr == "(?m)^$" || regexStr == `^[\t ]*$` || regexStr == `(?m)^[\t ]*$` {
								meta.hasEmptyStartPattern = true
							}
						}
					}
					break
				}
				for _, funcStr := range variable.Functions {
					if funcStr == "_start_" || funcStr == "_line_" {
						if funcStr == "_line_" && patternHasJoinMatches {
							if !hasAnyStartIndicator {
								meta.startPatterns[patternIdx] = false
							}
						} else {
							meta.startPatterns[patternIdx] = true
						}
						break
					}
				}
			}
		}

		if len(meta.startPatterns) == 0 {
			meta.startPatterns[0] = true
		}
	}

	// endPatterns + start-default when only _end_ patterns exist.
	hasAnyEndIndicator := false
	for patternIdx, compiledPattern := range group.Patterns {
		for varName := range compiledPattern.Variables {
			if varName == "_end_" {
				meta.endPatterns[patternIdx] = true
				hasAnyEndIndicator = true
			}
			for _, funcStr := range compiledPattern.Variables[varName].Functions {
				if funcStr == "_end_" {
					meta.endPatterns[patternIdx] = true
					hasAnyEndIndicator = true
				}
			}
		}
	}
	if hasAnyEndIndicator && !meta.hasAnyStartIndicator {
		if len(meta.startPatterns) == 0 || !meta.startPatterns[0] {
			meta.startPatterns[0] = true
		}
	}
	for patternIdx, compiledPattern := range group.Patterns {
		for _, variable := range compiledPattern.Variables {
			if variable.Name == "_end_" {
				meta.endPatterns[patternIdx] = true
				break
			}
			for _, funcStr := range variable.Functions {
				if funcStr == "_end_" {
					meta.endPatterns[patternIdx] = true
					break
				}
			}
		}
	}

	return meta
}

// filterRedundantIndicatorsInLine mirrors the per-position redundant
// pure-indicator filter prepareGroupMerge applies for non-table groups.
// Within a single line all matches share the same spanStart, so the filter
// reduces to the same logic over the line's match slice.
func filterRedundantIndicatorsInLine(lineMatches []patternMatch, group *compiler.CompiledGroup) []patternMatch {
	if group.Method == "table" {
		// Table mode: keep only the first pattern that matched the line.
		if len(lineMatches) > 0 {
			return lineMatches[:1]
		}
		return lineMatches
	}

	hasNormalMatch := false
	for _, m := range lineMatches {
		isPureIndicator := true
		for v := range m.result {
			if v != "_start_" && v != "_end_" && v != "_line_" {
				isPureIndicator = false
				break
			}
		}
		if !isPureIndicator {
			hasNormalMatch = true
			break
		}
	}

	if !hasNormalMatch {
		return lineMatches
	}

	out := lineMatches[:0]
	for _, m := range lineMatches {
		isPureIndicator := true
		for v := range m.result {
			if v != "_start_" && v != "_end_" && v != "_line_" {
				isPureIndicator = false
				break
			}
		}
		isStartPatternMatch := false
		isLinePatternMatch := false
		if m.patternIdx < len(group.Patterns) {
			pattern := group.Patterns[m.patternIdx]
			for varName := range pattern.Variables {
				if varName == "_start_" {
					isStartPatternMatch = true
					break
				}
				if varName == "_line_" {
					if len(pattern.Variables) == 1 {
						isLinePatternMatch = true
						break
					}
				}
			}
		}

		if isPureIndicator {
			// hasNormalMatch is true here; keep _start_ and standalone _line_.
			if isStartPatternMatch || isLinePatternMatch {
				out = append(out, m)
				continue
			}
			// Skip _end_ and _line_-with-joinmatches when redundant.
			continue
		}
		out = append(out, m)
	}
	return out
}

// parseGroupWithSourceMap parses input data against a compiled group and collects source map data
func (r *Runtime) parseGroupWithSourceMap(group *compiler.CompiledGroup, inputData string, vars map[string]interface{}, inputName string, sourceMap *SourceMap, resultsMap map[string]interface{}) (interface{}, error) {
	if len(group.Patterns) == 0 {
		// If the group has no direct patterns but has unnamed nested groups,
		// parse the nested groups and return their results directly.
		// This matches Python TTP behavior where unnamed inner groups are
		// transparent wrappers that merge into their parent.
		if len(group.Groups) > 0 {
			for _, nestedGroup := range group.Groups {
				if nestedGroup.Name == "" || nestedGroup.Name == "_" {
					return r.parseGroupWithSourceMap(nestedGroup, inputData, vars, inputName, sourceMap, resultsMap)
				}
			}
		}
		return nil, nil
	}

	// Get input source map
	inputSourceMap, exists := sourceMap.Inputs[inputName]
	if !exists {
		// Should not happen - input source map should be initialized
		// Fall back to parseGroup without source map
		return r.parseGroup(group, inputData, vars)
	}

	// Collect all matches with their positions and source map data
	type patternMatch struct {
		patternIdx int
		spanStart  int
		spanEnd    int
		result     map[string]interface{}
		lineIdx    int                    // line index for source map
		varRanges  map[string]*VarRange // variable name -> character range
	}

	var allMatches []patternMatch

	// Process each pattern and collect matches with positions
	lines := strings.Split(inputData, "\n")
	lineOffsets := make([]int, len(lines)+1) // Track byte offsets for each line
	offset := 0
	for i, line := range lines {
		lineOffsets[i] = offset
		offset += len(line) + 1 // +1 for newline
	}
	lineOffsets[len(lines)] = offset

	for patternIdx, compiledPattern := range group.Patterns {
		if compiledPattern.HasAnchors {
			// Match line by line
			for lineIdx, line := range lines {
				line = strings.TrimRight(line, "\r \t")
				trimmedLine := strings.TrimSpace(line)

				hasStartIndicator := false
				hasEndIndicator := false
				for varName := range compiledPattern.Variables {
					if varName == "_start_" {
						hasStartIndicator = true
					}
					if varName == "_end_" {
						hasEndIndicator = true
					}
					for _, funcStr := range compiledPattern.Variables[varName].Functions {
						if funcStr == "_start_" {
							hasStartIndicator = true
						}
						if funcStr == "_end_" {
							hasEndIndicator = true
						}
					}
					if hasStartIndicator && hasEndIndicator {
						break
					}
				}

				if trimmedLine == "" && !hasStartIndicator && !hasEndIndicator {
					continue
				}

				matchLine := line
				if trimmedLine == "" && (hasStartIndicator || hasEndIndicator) {
					matchLine = ""
				}

				match := compiledPattern.Regex.FindStringSubmatch(matchLine)
				if match != nil {
					// Find character positions for the match
					matchIndices := compiledPattern.Regex.FindStringSubmatchIndex(matchLine)
					if matchIndices == nil || len(matchIndices) < 2 {
						continue
					}

					result := r.extractMatchResult(match, compiledPattern, vars)

					if result != nil && (len(result) > 0 || compiledPattern.HasOnlySpecialIndicators || compiledPattern.IgnoreUsesTemplateVar || compiledPattern.HasJoinMatches) {
						spanStart := lineOffsets[lineIdx] + matchIndices[0]
						spanEnd := lineOffsets[lineIdx] + matchIndices[1]
						
						// Extract variable ranges from match indices
						varRanges := make(map[string]*VarRange)
						varIndex := 1 // First capture group is at index 1
						for _, varName := range compiledPattern.VariableOrder {
							_, ok := compiledPattern.Variables[varName]
							if !ok {
								continue
							}
							
							// Skip special variables
							if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_exact_" || varName == "_exact_space_" {
								varIndex++
								continue
							}
							
							// Get variable range from match indices
							if varIndex*2 < len(matchIndices) {
								varStart := matchIndices[varIndex*2]
								varEnd := matchIndices[varIndex*2+1]
								if varStart >= 0 && varEnd >= 0 {
									varRanges[varName] = &VarRange{
										StartCol: varStart,
										EndCol:   varEnd,
									}
								}
							}
							varIndex++
						}

						allMatches = append(allMatches, patternMatch{
							patternIdx: patternIdx,
							spanStart:  spanStart,
							spanEnd:    spanEnd,
							result:     result,
							lineIdx:    lineIdx,
							varRanges:  varRanges,
						})
					}
				}
			}
		} else {
			// Match against entire input
			allIndices := compiledPattern.Regex.FindAllStringSubmatchIndex(inputData, -1)

			for _, indices := range allIndices {
				if len(indices) < 2 {
					continue
				}

				matchGroups := make([]string, len(indices)/2)
				for i := 0; i < len(indices); i += 2 {
					if indices[i] >= 0 && indices[i+1] >= 0 {
						matchGroups[i/2] = inputData[indices[i]:indices[i+1]]
					}
				}

				result := r.extractMatchResult(matchGroups, compiledPattern, vars)
				if result != nil && (len(result) > 0 || compiledPattern.IgnoreUsesTemplateVar) {
					// Find which line this match is on
					lineIdx := 0
					for i, offset := range lineOffsets {
						if indices[0] < offset {
							lineIdx = i - 1
							break
						}
					}
					if lineIdx < 0 {
						lineIdx = 0
					}
					if lineIdx >= len(lines) {
						lineIdx = len(lines) - 1
					}

					// Extract variable ranges (relative to line start)
					varRanges := make(map[string]*VarRange)
					varIndex := 1
					lineStart := lineOffsets[lineIdx]
					for _, varName := range compiledPattern.VariableOrder {
						_, ok := compiledPattern.Variables[varName]
						if !ok {
							continue
						}
						
						if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_exact_" || varName == "_exact_space_" {
							varIndex++
							continue
						}
						
						if varIndex*2 < len(indices) {
							// Calculate absolute positions, then convert to relative to line
							varStartAbs := indices[varIndex*2]
							varEndAbs := indices[varIndex*2+1]
							if varStartAbs >= 0 && varEndAbs >= 0 {
								varStart := varStartAbs - lineStart
								varEnd := varEndAbs - lineStart
								if varStart >= 0 && varEnd >= 0 {
									varRanges[varName] = &VarRange{
										StartCol: varStart,
										EndCol:   varEnd,
									}
								}
							}
						}
						varIndex++
					}

					allMatches = append(allMatches, patternMatch{
						patternIdx: patternIdx,
						spanStart:  indices[0],
						spanEnd:    indices[1],
						result:     result,
						lineIdx:    lineIdx,
						varRanges:  varRanges,
					})
				}
			}
		}
	}

	// Sort matches by position
	sort.Slice(allMatches, func(i, j int) bool {
		if allMatches[i].spanStart == allMatches[j].spanStart {
			return allMatches[i].patternIdx < allMatches[j].patternIdx
		}
		return allMatches[i].spanStart < allMatches[j].spanStart
	})

	// Filter matches for table method
	if group.Method == "table" {
		filteredMatches := make([]patternMatch, 0, len(allMatches))
		seenPositions := make(map[int]bool)
		for _, match := range allMatches {
			if !seenPositions[match.spanStart] {
				seenPositions[match.spanStart] = true
				filteredMatches = append(filteredMatches, match)
			}
		}
		allMatches = filteredMatches
	}

	// Now delegate to parseGroup for the complex merging logic
	// But first, record matches in source map
	for _, match := range allMatches {
		if match.lineIdx >= 0 && match.lineIdx < len(inputSourceMap.Lines) {
			lineMapping := inputSourceMap.Lines[match.lineIdx]
			lineMapping.Matched = true
			
			// Calculate column positions relative to line start
			lineStart := lineOffsets[match.lineIdx]
			startCol := match.spanStart - lineStart
			endCol := match.spanEnd - lineStart
			
			// Adjust variable ranges to be relative to line start
			adjustedVarRanges := make(map[string]*VarRange)
			for varName, varRange := range match.varRanges {
				adjustedVarRanges[varName] = &VarRange{
					StartCol: varRange.StartCol,
					EndCol:   varRange.EndCol,
				}
			}
			
			matchMapping := &MatchMapping{
				StartCol:     startCol,
				EndCol:       endCol,
				GroupName:    group.Name,
				PatternIndex: match.patternIdx,
				Variables:    adjustedVarRanges,
				ResultPath:   "", // Will be set when we know the result path
			}
			
			lineMapping.Matches = append(lineMapping.Matches, matchMapping)
		}
	}

	// Call parseGroup to do the actual parsing and merging
	// This ensures we get the same results as parseGroup
	result, err := r.parseGroup(group, inputData, vars)
	if err != nil {
		return nil, err
	}

	// Update result paths in source map based on how results are stored
	// Try to determine the actual result path from the resultsMap
	if result != nil && group.Name != "" {
		// Check if group name exists in results map (might be resolved path)
		resultPath := group.Name
		
		// Check if resultsMap has this group
		if resultsMap != nil {
			if _, exists := resultsMap[group.Name]; exists {
				resultPath = group.Name
			} else {
				// Try to find resolved path - check all keys in resultsMap
				for key := range resultsMap {
					// Check if key starts with group name (for dynamic paths)
					if strings.HasPrefix(key, group.Name) || group.Name == "" {
						resultPath = key
						break
					}
				}
			}
		}
		
		// Update all matches for this group with the result path
		for _, lineMapping := range inputSourceMap.Lines {
			for _, matchMapping := range lineMapping.Matches {
				if matchMapping.GroupName == group.Name {
					matchMapping.ResultPath = resultPath
				}
			}
		}
	} else if result != nil && group.Name == "" {
		// Anonymous group - use "_anonymous_" as path
		for _, lineMapping := range inputSourceMap.Lines {
			for _, matchMapping := range lineMapping.Matches {
				if matchMapping.GroupName == "" {
					matchMapping.ResultPath = "_anonymous_"
				}
			}
		}
	}

	// Process nested groups with source map support
	// Nested groups are already processed by parseGroup, but we need to:
	// 1. Record their matches in the source map (they weren't recorded because parseGroup doesn't have source map)
	// 2. Update their ResultPath to reflect the full path (e.g., "parent.nested" instead of just "nested")
	if len(group.Groups) > 0 && result != nil {
		// Determine parent result path
		var parentResultPath string
		if group.Name != "" {
			parentResultPath = group.Name
			if resultsMap != nil {
				if _, exists := resultsMap[group.Name]; exists {
					parentResultPath = group.Name
				} else {
					// Try to find resolved path
					for key := range resultsMap {
						if strings.HasPrefix(key, group.Name) || group.Name == "" {
							parentResultPath = key
							break
						}
					}
				}
			}
		} else {
			parentResultPath = "_anonymous_"
		}

		// Reconstruct parent match ranges from allMatches
		// Group matches by parent match instance (pattern index 0 typically starts a new parent match)
		type parentRange struct {
			startLine int
			endLine   int
			startPos  int
			endPos    int
		}
		parentRanges := make([]parentRange, 0)
		
		// Get merged matches from result to determine how many parent matches we have
		var mergedMatches []map[string]interface{}
		if resultList, ok := result.([]map[string]interface{}); ok {
			mergedMatches = resultList
		} else if resultSingle, ok := result.(map[string]interface{}); ok {
			mergedMatches = []map[string]interface{}{resultSingle}
		}
		
		// Reconstruct parent match ranges by grouping matches
		// IMPORTANT: Parent ranges must extend to include nested group content, not just parent patterns
		// Nested groups are parsed on content AFTER the parent's direct pattern matches
		if len(allMatches) > 0 && len(mergedMatches) > 0 {
			// First, find where each parent match starts (pattern index 0)
			parentStarts := make([]int, 0)
			for i, match := range allMatches {
				if match.patternIdx == 0 {
					parentStarts = append(parentStarts, i)
				}
			}
			
			// Build ranges where each parent extends to the start of the next parent (or end of input)
			for pi, startIdx := range parentStarts {
				firstMatch := allMatches[startIdx]
				
				// Determine where this parent's range ends
				var endPos int
				var endLine int
				
				if pi+1 < len(parentStarts) {
					// There's another parent after this one - extend to just before it
					nextStartIdx := parentStarts[pi+1]
					nextMatch := allMatches[nextStartIdx]
					endPos = nextMatch.spanStart
					endLine = nextMatch.lineIdx
					if endLine > 0 {
						endLine-- // Include up to the line before the next parent starts
					}
				} else {
					// This is the last parent - extend to end of input
					endPos = len(inputData)
					endLine = len(lines) - 1
				}
				
				parentRanges = append(parentRanges, parentRange{
					startLine: firstMatch.lineIdx,
					endLine:   endLine,
					startPos:  firstMatch.spanStart,
					endPos:    endPos,
				})
			}
			
			// Ensure we have enough ranges for all merged matches
			for len(parentRanges) < len(mergedMatches) {
				if len(parentRanges) == 0 {
					parentRanges = append(parentRanges, parentRange{
						startLine: 0,
						endLine:   len(lines) - 1,
						startPos:  0,
						endPos:    len(inputData),
					})
				} else {
					// Duplicate last range
					lastRange := parentRanges[len(parentRanges)-1]
					parentRanges = append(parentRanges, lastRange)
				}
			}
		}

		// First pass: Find where each nested group's pattern 0 matches within each parent range
		// This lets us determine the actual range for each nested group
		type nestedGroupRange struct {
			groupName  string
			startLine  int
			endLine    int  // exclusive
			parentIdx  int
		}
		nestedRanges := make([]nestedGroupRange, 0)
		
		
		for _, nestedGroup := range group.Groups {
			if nestedGroup.Name == "" || nestedGroup.Name == "_" {
				continue
			}
			if len(nestedGroup.Patterns) == 0 {
				continue
			}
			
			for parentIdx, parentRange := range parentRanges {
				if parentIdx >= len(mergedMatches) {
					break
				}
				
				parentMatch := mergedMatches[parentIdx]
				existsInResult := r.nestedGroupExistsInResult(parentMatch, nestedGroup.Name)
				if parentMatch == nil || !existsInResult {
					continue
				}
				
				if parentRange.endPos <= parentRange.startPos || parentRange.endPos > len(inputData) {
					continue
				}
				
				parentInputData := inputData[parentRange.startPos:parentRange.endPos]
				rangeStartLine := parentRange.startLine
				parentLines := strings.Split(parentInputData, "\n")
				
			
			// Find ALL start patterns (pattern 0 + any patterns with _start_ attribute)
			// Multiple start patterns allow different formats to begin a new group instance
			startPatternIdxs := make([]int, 0)
			for i := range nestedGroup.Patterns {
				if i == 0 {
					startPatternIdxs = append(startPatternIdxs, i)
				} else {
					// Check if this pattern has _start_ indicator
					// It can be a variable named _start_ or a function called _start_
					patternHasStart := false
					for _, variable := range nestedGroup.Patterns[i].Variables {
						if variable.Name == "_start_" {
							patternHasStart = true
							break
						}
						// Also check if _start_ is in functions (e.g., {{ name | _start_ }})
						for _, funcStr := range variable.Functions {
							if funcStr == "_start_" {
								patternHasStart = true
								break
							}
						}
						if patternHasStart {
							break
						}
					}
					if patternHasStart {
						startPatternIdxs = append(startPatternIdxs, i)
					}
				}
			}
			
			// Find ALL instances where ANY start pattern matches
			// Each match represents a new instance of the nested group
			instanceStartLines := make([]int, 0)
			
			// Check if any start pattern uses anchors (most do)
			hasAnchoredStart := false
			for _, spIdx := range startPatternIdxs {
				spRegex := nestedGroup.Patterns[spIdx].Regex.String()
				if strings.Contains(spRegex, "^") || strings.Contains(spRegex, "$") {
					hasAnchoredStart = true
					break
				}
			}
			
		if hasAnchoredStart {
			for localLineIdx, line := range parentLines {
				line = strings.TrimRight(line, "\r \t")
				if strings.TrimSpace(line) == "" {
					continue
				}
				// Check ALL start patterns
				matchedAnyStart := false
				for _, spIdx := range startPatternIdxs {
					if nestedGroup.Patterns[spIdx].Regex.MatchString(line) {
						matchedAnyStart = true
						break
					}
				}
				if matchedAnyStart {
						absoluteLine := rangeStartLine + localLineIdx
						instanceStartLines = append(instanceStartLines, absoluteLine)
					}
				}
		} else {
				// For non-anchored patterns, find all matches from ALL start patterns
				parentLineOffsets := make([]int, 0)
				offset := 0
				for _, line := range parentLines {
					parentLineOffsets = append(parentLineOffsets, offset)
					offset += len(line) + 1
				}
				parentLineOffsets = append(parentLineOffsets, offset)
				
				// Check all start patterns for non-anchored matches
				for _, spIdx := range startPatternIdxs {
					allMatchIndices := nestedGroup.Patterns[spIdx].Regex.FindAllStringIndex(parentInputData, -1)
					for _, indices := range allMatchIndices {
						if len(indices) >= 2 {
							// Find which line this match is on
							localLineIdx := 0
							for i, off := range parentLineOffsets {
								if indices[0] < off {
									localLineIdx = i - 1
									break
								}
							}
							if localLineIdx >= 0 {
								absoluteLine := rangeStartLine + localLineIdx
								// Avoid duplicates
								alreadyHave := false
								for _, existing := range instanceStartLines {
									if existing == absoluteLine {
										alreadyHave = true
										break
									}
								}
								if !alreadyHave {
									instanceStartLines = append(instanceStartLines, absoluteLine)
								}
							}
						}
					}
				}
			}
			
			// Create a nestedRange entry for each instance
			for _, startLine := range instanceStartLines {
				nestedRanges = append(nestedRanges, nestedGroupRange{
					groupName: nestedGroup.Name,
					startLine: startLine,
					endLine:   parentRange.endLine + 1, // Will be adjusted below
					parentIdx: parentIdx,
				})
			}
		}
	}
		
		// Sort nested ranges by startLine within each parent
		// Then adjust endLine to be the start of the next instance (could be same group or different group)
		for i := range nestedRanges {
			for j := range nestedRanges {
				// Check if range j starts after range i within the same parent
				if i != j && nestedRanges[i].parentIdx == nestedRanges[j].parentIdx {
					if nestedRanges[j].startLine > nestedRanges[i].startLine {
						// Range j starts after range i - use j's start as i's end
						if nestedRanges[j].startLine < nestedRanges[i].endLine {
							nestedRanges[i].endLine = nestedRanges[j].startLine
						}
					}
				}
			}
	}
	
	// Second pass: Process each nested group range and record matches
		// Now we iterate over ALL ranges (not just one per nested group)
		for rangeIdx, nr := range nestedRanges {
			// Find the corresponding nestedGroup for this range
			var nestedGroup *compiler.CompiledGroup
			for i := range group.Groups {
				if group.Groups[i].Name == nr.groupName {
					nestedGroup = group.Groups[i]
					break
				}
			}
			if nestedGroup == nil {
				continue
			}
			if len(nestedGroup.Patterns) == 0 {
				continue
			}
			
			parentIdx := nr.parentIdx
			if parentIdx >= len(parentRanges) || parentIdx >= len(mergedMatches) {
				continue
			}
			parentRange := parentRanges[parentIdx]
			
			nestedGroupStartLine := nr.startLine
			nestedGroupEndLine := nr.endLine
			

			// Extract input data for this parent match range
			if parentRange.endPos > parentRange.startPos && parentRange.endPos <= len(inputData) {
				parentInputData := inputData[parentRange.startPos:parentRange.endPos]
				rangeStartLine := parentRange.startLine

				// Process nested group patterns on this parent's input range
				// But only record matches within the nested group's specific range
				parentLines := strings.Split(parentInputData, "\n")
				
				for patternIdx, compiledPattern := range nestedGroup.Patterns {
					if compiledPattern.HasAnchors {
						// Match line by line within parent range
						for localLineIdx, line := range parentLines {
							absoluteLineIdx := rangeStartLine + localLineIdx
							if absoluteLineIdx < nestedGroupStartLine || absoluteLineIdx >= nestedGroupEndLine {
								continue
							}
							if absoluteLineIdx < 0 || absoluteLineIdx >= len(inputSourceMap.Lines) {
								continue
							}
							line = strings.TrimRight(line, "\r \t")
							if strings.TrimSpace(line) == "" {
								continue
							}
							match := compiledPattern.Regex.FindStringSubmatch(line)
							if match != nil {
								matchIndices := compiledPattern.Regex.FindStringSubmatchIndex(line)
								if matchIndices == nil || len(matchIndices) < 2 {
									continue
								}
								lineMapping := inputSourceMap.Lines[absoluteLineIdx]
								lineMapping.Matched = true
								startCol := matchIndices[0]
								endCol := matchIndices[1]
								adjustedVarRanges := make(map[string]*VarRange)
								varIndex := 1
								for _, varName := range compiledPattern.VariableOrder {
									if _, ok := compiledPattern.Variables[varName]; !ok {
										continue
									}
									if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_exact_" || varName == "_exact_space_" {
										varIndex++
										continue
									}
									if varIndex*2 < len(matchIndices) {
										varStart := matchIndices[varIndex*2]
										varEnd := matchIndices[varIndex*2+1]
										if varStart >= 0 && varEnd >= 0 {
											adjustedVarRanges[varName] = &VarRange{StartCol: varStart, EndCol: varEnd}
										}
									}
									varIndex++
								}
								matchMapping := &MatchMapping{
									StartCol: startCol, EndCol: endCol,
									GroupName: nestedGroup.Name, PatternIndex: patternIdx,
									Variables: adjustedVarRanges, ResultPath: "",
								}
								lineMapping.Matches = append(lineMapping.Matches, matchMapping)
							}
						}
					} else {
						// Match against parent input range (non-anchored)
						allIndices := compiledPattern.Regex.FindAllStringSubmatchIndex(parentInputData, -1)
						parentLineOffsets := make([]int, 0)
						offset := 0
						for _, line := range parentLines {
							parentLineOffsets = append(parentLineOffsets, offset)
							offset += len(line) + 1
						}
						parentLineOffsets = append(parentLineOffsets, offset)

						for _, indices := range allIndices {
							if len(indices) < 2 {
								continue
							}
							localLineIdx := 0
							for i, off := range parentLineOffsets {
								if indices[0] < off {
									localLineIdx = i - 1
									break
								}
							}
							if localLineIdx < 0 {
								localLineIdx = 0
							}
							if localLineIdx >= len(parentLines) {
								localLineIdx = len(parentLines) - 1
							}
							absoluteLineIdx := rangeStartLine + localLineIdx
							if absoluteLineIdx < nestedGroupStartLine || absoluteLineIdx >= nestedGroupEndLine {
								continue
							}
							if absoluteLineIdx < 0 || absoluteLineIdx >= len(inputSourceMap.Lines) {
								continue
							}
							lineMapping := inputSourceMap.Lines[absoluteLineIdx]
							lineMapping.Matched = true
							lineStart := parentLineOffsets[localLineIdx]
							startCol := indices[0] - lineStart
							endCol := indices[1] - lineStart
							adjustedVarRanges := make(map[string]*VarRange)
							varIndex := 1
							for _, varName := range compiledPattern.VariableOrder {
								if _, ok := compiledPattern.Variables[varName]; !ok {
									continue
								}
								if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_exact_" || varName == "_exact_space_" {
									varIndex++
									continue
								}
								if varIndex*2 < len(indices) {
									varStartAbs := indices[varIndex*2]
									varEndAbs := indices[varIndex*2+1]
									if varStartAbs >= 0 && varEndAbs >= 0 {
										varStart := varStartAbs - lineStart
										varEnd := varEndAbs - lineStart
										if varStart >= 0 && varEnd >= 0 {
											adjustedVarRanges[varName] = &VarRange{StartCol: varStart, EndCol: varEnd}
										}
									}
								}
								varIndex++
							}
							matchMapping := &MatchMapping{
								StartCol: startCol, EndCol: endCol,
								GroupName: nestedGroup.Name, PatternIndex: patternIdx,
								Variables: adjustedVarRanges, ResultPath: "",
							}
							lineMapping.Matches = append(lineMapping.Matches, matchMapping)
						}
					}
				}
			}

			// Find where this nested group is stored in the result and update ResultPath
			// for matches within this specific range
			
			// Get the base path for the nested group (without array suffix)
			baseNestedPath := ""
			nestedNameWithoutStar := strings.TrimSuffix(strings.TrimSuffix(nestedGroup.Name, "**"), "*")
			if parentResultPath != "" && parentResultPath != "_anonymous_" {
				baseNestedPath = parentResultPath + "." + nestedNameWithoutStar
			} else {
				baseNestedPath = nestedNameWithoutStar
			}
			
			// Determine the array index for this specific range
			// Count how many ranges of the same nested group within the same parent appear before this one
			instanceIdx := 0
			for i := 0; i < rangeIdx; i++ {
				if nestedRanges[i].groupName == nr.groupName && nestedRanges[i].parentIdx == nr.parentIdx {
					instanceIdx++
				}
			}
			
			// Build the instance path (e.g., "show_system_detail.smms[0]")
			instancePath := baseNestedPath
			if strings.HasSuffix(nestedGroup.Name, "*") || strings.HasSuffix(nestedGroup.Name, "**") {
				instancePath = fmt.Sprintf("%s[%d]", baseNestedPath, instanceIdx)
			}
			
			
		// Update ResultPath for matches within this nested group's range
		for lineIdx := nr.startLine; lineIdx < nr.endLine && lineIdx < len(inputSourceMap.Lines); lineIdx++ {
			lineMapping := inputSourceMap.Lines[lineIdx]
			for _, matchMapping := range lineMapping.Matches {
				if matchMapping.GroupName == nestedGroup.Name && matchMapping.ResultPath == "" {
					matchMapping.ResultPath = instancePath
				}
			}
		}
		
		// Process deeply nested groups (children of this nested group)
		// For example, if nestedGroup is "smms*" and it has a child "io", process "io" within this SMM's range
		if len(nestedGroup.Groups) > 0 {
			// Get the result data for this specific nested group instance
			nestedResultData := mergedMatches[parentIdx]
			nestedNameWithoutStars := strings.TrimSuffix(strings.TrimSuffix(nestedGroup.Name, "**"), "*")
			
			// Find this nested group's result in the parent
			var nestedInstanceResults []map[string]interface{}
			if nestedValue, exists := nestedResultData[nestedNameWithoutStars]; exists {
				if nestedList, ok := nestedValue.([]interface{}); ok {
					for _, item := range nestedList {
						if itemMap, ok := item.(map[string]interface{}); ok {
							nestedInstanceResults = append(nestedInstanceResults, itemMap)
						}
					}
				} else if nestedMap, ok := nestedValue.(map[string]interface{}); ok {
					nestedInstanceResults = []map[string]interface{}{nestedMap}
				}
			}
			
			// If we have results for this nested group instance, process its children
			if instanceIdx < len(nestedInstanceResults) {
				nestedInstanceData := nestedInstanceResults[instanceIdx]
				
				for _, childGroup := range nestedGroup.Groups {
					if childGroup.Name == "" || childGroup.Name == "_" {
						continue
					}
					if len(childGroup.Patterns) == 0 {
						continue
					}
					
					// Check if this child group exists in the nested instance's result
					childNameWithoutStars := strings.TrimSuffix(strings.TrimSuffix(childGroup.Name, "**"), "*")
					if _, childExists := nestedInstanceData[childNameWithoutStars]; !childExists {
						continue
					}
					
					
					// Process child group patterns within this nested group's range
					for patternIdx, compiledPattern := range childGroup.Patterns {
						if compiledPattern.HasAnchors {
							// Match line by line within nested group range
							for lineIdx := nr.startLine; lineIdx < nr.endLine && lineIdx < len(inputSourceMap.Lines); lineIdx++ {
								if lineIdx < 0 || lineIdx >= len(lines) {
									continue
								}
								
								// Skip lines that already have a match from the parent group
								// This prevents child group patterns from overriding parent matches
								lineMapping := inputSourceMap.Lines[lineIdx]
								hasParentMatch := false
								for _, existingMatch := range lineMapping.Matches {
									if existingMatch.GroupName == nestedGroup.Name {
										hasParentMatch = true
										break
									}
								}
								if hasParentMatch {
									continue
								}
								
								line := strings.TrimRight(lines[lineIdx], "\r \t")
								if strings.TrimSpace(line) == "" {
									continue
								}
								match := compiledPattern.Regex.FindStringSubmatch(line)
								if match != nil {
									matchIndices := compiledPattern.Regex.FindStringSubmatchIndex(line)
									if matchIndices == nil || len(matchIndices) < 2 {
										continue
									}
									lineMapping.Matched = true
									startCol := matchIndices[0]
									endCol := matchIndices[1]
									adjustedVarRanges := make(map[string]*VarRange)
									varIndex := 1
									for _, varName := range compiledPattern.VariableOrder {
										if _, ok := compiledPattern.Variables[varName]; !ok {
											continue
										}
										if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_exact_" || varName == "_exact_space_" {
											varIndex++
											continue
										}
										if varIndex*2 < len(matchIndices) {
											varStart := matchIndices[varIndex*2]
											varEnd := matchIndices[varIndex*2+1]
											if varStart >= 0 && varEnd >= 0 {
												adjustedVarRanges[varName] = &VarRange{StartCol: varStart, EndCol: varEnd}
											}
										}
										varIndex++
									}
									
									// Build result path for this deeply nested group
									childResultPath := instancePath + "." + childNameWithoutStars
									
									matchMapping := &MatchMapping{
										StartCol: startCol, EndCol: endCol,
										GroupName: childGroup.Name, PatternIndex: patternIdx,
										Variables: adjustedVarRanges, ResultPath: childResultPath,
									}
									lineMapping.Matches = append(lineMapping.Matches, matchMapping)
								}
							}
						} else {
							// For non-anchored patterns, match within the nested group's range
							rangeText := ""
							lineOffsets := make([]int, 0)
							offset := 0
							for lineIdx := nr.startLine; lineIdx < nr.endLine && lineIdx < len(lines); lineIdx++ {
								lineOffsets = append(lineOffsets, offset)
								rangeText += lines[lineIdx] + "\n"
								offset += len(lines[lineIdx]) + 1
							}
							
							allIndices := compiledPattern.Regex.FindAllStringSubmatchIndex(rangeText, -1)
							for _, indices := range allIndices {
								if len(indices) < 2 {
									continue
								}
								// Find which line this match is on
								localLineIdx := 0
								for i, off := range lineOffsets {
									if indices[0] < off {
										localLineIdx = i - 1
										break
									}
									localLineIdx = i
								}
								if localLineIdx < 0 {
									localLineIdx = 0
								}
								absoluteLineIdx := nr.startLine + localLineIdx
								if absoluteLineIdx < 0 || absoluteLineIdx >= len(inputSourceMap.Lines) {
									continue
								}
								
								// Skip lines that already have a match from the parent group
								lineMapping := inputSourceMap.Lines[absoluteLineIdx]
								hasParentMatch := false
								for _, existingMatch := range lineMapping.Matches {
									if existingMatch.GroupName == nestedGroup.Name {
										hasParentMatch = true
										break
									}
								}
								if hasParentMatch {
									continue
								}
								
								lineMapping.Matched = true
								lineStart := lineOffsets[localLineIdx]
								startCol := indices[0] - lineStart
								endCol := indices[1] - lineStart
								
								adjustedVarRanges := make(map[string]*VarRange)
								varIndex := 1
								for _, varName := range compiledPattern.VariableOrder {
									if _, ok := compiledPattern.Variables[varName]; !ok {
										continue
									}
									if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_exact_" || varName == "_exact_space_" {
										varIndex++
										continue
									}
									if varIndex*2 < len(indices) {
										varStartAbs := indices[varIndex*2]
										varEndAbs := indices[varIndex*2+1]
										if varStartAbs >= 0 && varEndAbs >= 0 {
											varStart := varStartAbs - lineStart
											varEnd := varEndAbs - lineStart
											if varStart >= 0 && varEnd >= 0 {
												adjustedVarRanges[varName] = &VarRange{StartCol: varStart, EndCol: varEnd}
											}
										}
									}
									varIndex++
								}
								
								// Build result path for this deeply nested group
								childResultPath := instancePath + "." + childNameWithoutStars
								
								matchMapping := &MatchMapping{
									StartCol: startCol, EndCol: endCol,
									GroupName: childGroup.Name, PatternIndex: patternIdx,
									Variables: adjustedVarRanges, ResultPath: childResultPath,
								}
								lineMapping.Matches = append(lineMapping.Matches, matchMapping)
							}
						}
					}
				}
			}
		}
	}
}

	return result, nil
}

// nestedGroupExistsInResult checks if a nested group exists in a parent match result
func (r *Runtime) nestedGroupExistsInResult(parentMatch map[string]interface{}, nestedGroupName string) bool {
	if parentMatch == nil {
		return false
	}
	
	// Check if nested group name exists directly in parent match
	if _, exists := parentMatch[nestedGroupName]; exists {
		return true
	}
	
	// Check if nested group exists as a nested path (e.g., "smms*" might be stored as "smms[0]", "smms[1]", etc.)
	// Or it might be stored with a resolved path
	for key := range parentMatch {
		// Check if key starts with nested group name (handles dynamic paths and formatters)
		keyWithoutFormatters := strings.TrimSuffix(key, "*")
		keyWithoutFormatters = strings.TrimSuffix(keyWithoutFormatters, "**")
		nestedNameWithoutFormatters := strings.TrimSuffix(nestedGroupName, "*")
		nestedNameWithoutFormatters = strings.TrimSuffix(nestedNameWithoutFormatters, "**")
		
		if keyWithoutFormatters == nestedNameWithoutFormatters || 
		   strings.HasPrefix(keyWithoutFormatters, nestedNameWithoutFormatters+"[") ||
		   strings.HasPrefix(key, nestedGroupName) {
			return true
		}
	}
	
	return false
}

// findNestedGroupPath searches for a nested group in the result map and returns its full path
func (r *Runtime) findNestedGroupPath(resultMap map[string]interface{}, nestedGroupName string, parentPath string) string {
	// First, try direct lookup: parentPath.nestedGroupName
	if parentPath != "" && parentPath != "_anonymous_" {
		directPath := parentPath + "." + nestedGroupName
		if r.pathExistsInMap(resultMap, directPath) {
			return directPath
		}
	}

	// Search recursively in the result map for the nested group
	// This handles cases where the path might be resolved differently
	return r.searchNestedGroupInMap(resultMap, nestedGroupName, parentPath, "")
}

// pathExistsInMap checks if a dot-separated path exists in the map
func (r *Runtime) pathExistsInMap(m map[string]interface{}, path string) bool {
	parts := strings.Split(path, ".")
	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - check if key exists
			_, exists := current[part]
			return exists
		}
		// Navigate deeper
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return false
		}
	}
	return false
}

// searchNestedGroupInMap recursively searches for a nested group in the map
func (r *Runtime) searchNestedGroupInMap(m map[string]interface{}, nestedGroupName string, parentPath string, currentPath string) string {
	for key, value := range m {
		var fullPath string
		if currentPath == "" {
			fullPath = key
		} else {
			fullPath = currentPath + "." + key
		}

		// Check if this key matches the nested group name
		if key == nestedGroupName {
			// Check if this path starts with the parent path (for nested groups)
			if parentPath == "" || parentPath == "_anonymous_" || strings.HasPrefix(fullPath, parentPath+".") {
				return fullPath
			}
		}

		// Recursively search in nested maps
		if nestedMap, ok := value.(map[string]interface{}); ok {
			if found := r.searchNestedGroupInMap(nestedMap, nestedGroupName, parentPath, fullPath); found != "" {
				return found
			}
		} else if nestedList, ok := value.([]interface{}); ok {
			// Check first element if it's a map (for arrays of objects)
			if len(nestedList) > 0 {
				if firstMap, ok := nestedList[0].(map[string]interface{}); ok {
					if found := r.searchNestedGroupInMap(firstMap, nestedGroupName, parentPath, fullPath); found != "" {
						return found
					}
				}
			}
		}
	}
	return ""
}

// processFunctions applies function pipeline to a value
// Returns an error with message "condition_failed" if a condition function returns false
func (r *Runtime) processFunctions(value interface{}, functions []string, vars map[string]interface{}, matchData map[string]interface{}, varName string) (interface{}, error) {
	result := value

	// Pre-allocate kwargs map only if needed (for functions that require vars)
	// Most functions don't need vars, so we avoid the allocation overhead
	var kwargs map[string]interface{}
	needsVars := false

	for _, funcStr := range functions {
		// Parse function call (e.g., "upper()", "split(',')", "macro('name')")
		funcName, args, err := parseFunction(funcStr, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to parse function %s: %w", funcStr, err)
		}

		// Check if args contain keyword arguments (key='value' format)
		// If so, parse them into kwargs and remove from args
		hasKeywordArgs := false
		var positionalArgs []string
		keywordArgs := make(map[string]interface{}) // Store keyword args separately
		for _, arg := range args {
			if strings.Contains(arg, "=") && !strings.HasPrefix(arg, "=") {
				hasKeywordArgs = true
				// This is a keyword argument
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					// Remove quotes from value
					value = strings.Trim(value, `"'`)
					// Store keyword arguments
					keywordArgs[key] = value
				} else {
					// Not a valid keyword arg, treat as positional
					positionalArgs = append(positionalArgs, arg)
				}
			} else {
				// Positional argument
				positionalArgs = append(positionalArgs, arg)
			}
		}
		if hasKeywordArgs {
			args = positionalArgs
		}

		// Get function from registry (checks runtime overrides first)
		fn, ok := r.getMatchFunc(funcName)
		if !ok {
			// Function not found, skip it (could be a macro handled by the function itself)
			continue
		}

		// Only create kwargs map if function needs it (chain, macro, or functions that use vars)
		// This avoids unnecessary map creation and copying for most functions
		needsKwargs := funcName == "chain" || funcName == "macro" || funcName == "lookup" || funcName == "gpvlookup" || funcName == "rlookup" || funcName == "let" || funcName == "record" || funcName == "set" || funcName == "replaceall" || funcName == "resuball" || funcName == "unrange" || hasKeywordArgs
		if needsKwargs {
			if !needsVars {
				kwargs = make(map[string]interface{}, len(vars)+len(keywordArgs)+3) // Pre-allocate with capacity
				// Copy template variables only when needed
				for k, v := range vars {
					kwargs[k] = v
				}
				needsVars = true
			}
			// Merge keyword arguments into kwargs (override template vars)
			for k, v := range keywordArgs {
				kwargs[k] = v
			}

			// Add recorded vars (from record() function) - these override template vars
			if r.recordedVars != nil {
				for k, v := range r.recordedVars {
					kwargs[k] = v
				}
			}
			// Pass r.recordedVars to record() function so it can store values globally
			kwargs["_recorded_vars"] = r.recordedVars

			// Pass function registry to chain function so it can call other functions
			if funcName == "chain" {
				kwargs["_match_registry"] = r.matchRegistry
				// Pass resolver that checks runtime overrides first
				kwargs["_match_func_resolver"] = func(name string) (match.Function, bool) {
					return r.getMatchFunc(name)
				}
			}

			if funcName == "macro" {
				// Pass macro registry and macros to macro function
				kwargs["_macro_registry"] = r.macroRegistry
				// Convert macros to a format the function can use
				macroList := make([]struct {
					Language string
					Name     string
				}, len(r.compiled.Macros))
				for i, m := range r.compiled.Macros {
					macroList[i] = struct {
						Language string
						Name     string
					}{
						Language: m.Language,
						Name:     "", // Macro name is extracted from source during execution
					}
				}
				kwargs["_macros"] = macroList
			}

			// Pass match result data structure to let and set functions so they can set additional fields
			if funcName == "let" || funcName == "set" {
				if matchData != nil {
					kwargs["_match_data"] = matchData
				}
				if varName != "" {
					kwargs["_var_name"] = varName
				}
			}

			// Pass lookup tables to lookup functions
			if funcName == "lookup" || funcName == "gpvlookup" || funcName == "rlookup" {
				// Build lookup tables map from compiled lookups
				lookupTables := make(map[string]map[string]interface{})
				if r.compiled.Lookups != nil {
					for _, lookup := range r.compiled.Lookups {
						if lookup.Data != nil {
							// Type assert lookup.Data to map[string]interface{}
							if dataMap, ok := lookup.Data.(map[string]interface{}); ok {
								lookupTables[lookup.Name] = dataMap
							}
						}
					}
				}
				// Merge runtime lookups (from ParseOptions) - these override compiled lookups
				if r.runtimeLookups != nil {
					for name, data := range r.runtimeLookups {
						lookupTables[name] = data
					}
				}
				kwargs["_lookup_tables"] = lookupTables
				// Also pass match data for add_field support
				if matchData != nil {
					kwargs["_match_data"] = matchData
				}
			}
		} else if !hasKeywordArgs {
			// Most functions don't need kwargs, pass nil to avoid allocation
			// But if we parsed keyword args above, we need to keep the kwargs map
			kwargs = nil
		}

		// Execute function
		newResult, err := fn(result, args, kwargs)
		if err != nil {
			return nil, fmt.Errorf("function %s failed: %w", funcName, err)
		}

		// Check if result is a ConditionResult
		if condResult, ok := newResult.(*match.ConditionResult); ok {
			// If condition is false, reject the match
			if !condResult.Condition {
				return nil, fmt.Errorf("condition_failed")
			}
			// Condition passed, use the value
			result = condResult.Value
		} else {
			result = newResult
		}
	}

	return result, nil
}

// extractMatchResult extracts match results from a regex match
// Returns nil if any condition function returns false (match rejected)
// Returns empty map if ignore() uses a template variable (Python TTP quirk)
func (r *Runtime) extractMatchResult(match []string, compiledPattern *pattern.CompiledPattern, vars map[string]interface{}) map[string]interface{} {
	// Python TTP quirk: when ignore() uses a template variable, return empty result
	// This causes the match to be added but with no fields (matches Python TTP behavior)
	if compiledPattern.IgnoreUsesTemplateVar {
		// Return empty result to match Python TTP's behavior
		// The empty result will be added to matches (Python TTP returns [{}])
		return make(map[string]interface{})
	}

	// Use the preserved variable order from the pattern
	varNames := compiledPattern.VariableOrder
	if len(varNames) == 0 {
		// Fallback: iterate through map (order not guaranteed)
		for varName := range compiledPattern.Variables {
			varNames = append(varNames, varName)
		}
	}

	// Pre-size result map: most variables produce a single key, so VariableOrder
	// length is a tight upper bound and avoids map growth/rehash for wide patterns.
	result := make(map[string]interface{}, len(varNames))
	varIndex := 1 // First capture group is at index 1 (index 0 is full match)

	// Extract values in order
	for _, varName := range varNames {
		variable, ok := compiledPattern.Variables[varName]
		if !ok {
			continue
		}

		// Skip special variables that don't save values
		// Note: _line_ should NOT be skipped - it captures the line content when used with joinmatches
		if varName == "ignore" || varName == "_start_" || varName == "_end_" || varName == "_exact_" || varName == "_exact_space_" {
			varIndex++
			continue
		}

		// For variables with set(), we need to process them even if they don't match (varIndex >= len(match))
		// The set() function will set the value regardless of what was matched
		var value interface{}
		if varIndex < len(match) {
			value = match[varIndex]
		} else if variable.HasSet {
			// Variable with set() didn't match - use empty string as value
			// The set() function will set the actual value
			value = ""
		} else {
			// Variable didn't match and doesn't have set() - skip it
			varIndex++
			continue
		}

		// Apply functions if any (but handle joinmatches specially).
		// Pass result + variable.Name as explicit matchData/varName so we
		// avoid cloning vars per variable just to inject _match_data/_var_name.
		var processedValue interface{}
		var err error
		if variable.HasJoinMatches {
			// For joinmatches, collect the value but don't apply joinmatches yet
			// We'll collect all matches and join them later
			// Apply other functions first
			otherFuncs := []string{}
			for _, funcStr := range variable.Functions {
				if !strings.HasPrefix(funcStr, "joinmatches") {
					otherFuncs = append(otherFuncs, funcStr)
				}
			}
			processedValue, err = r.processFunctions(value, otherFuncs, vars, result, variable.Name)
			if err != nil {
				// Check if it's a condition failure
				if strings.Contains(err.Error(), "condition_failed") {
					// When joinmatches is present, condition failures should skip this line,
					// not reject the entire match. This allows _line_ with contains to filter lines.
					// Skip this variable (don't add it to result) but continue processing other variables
					varIndex++
					continue
				}
				// Other error, log but continue
				continue
			}
		} else {
			processedValue, err = r.processFunctions(value, variable.Functions, vars, result, variable.Name)
			if err != nil {
				// Check if it's a condition failure
				if strings.Contains(err.Error(), "condition_failed") {
					// Reject entire match
					return nil
				}
				// Other error, log but continue
				continue
			}
		}

		// If variable has set() function, it should set a value in the match result
		// The set function should have already set it via _match_data
		// For set(), we don't save the processedValue - the set function sets the value directly
		if variable.HasSet {
			// The set function should have already set the value in result via _match_data
			// Don't overwrite it with processedValue - set() handles the value setting
			// Just skip saving processedValue - the value is already set by set()
			varIndex++
			continue
		}

		// For joinmatches, we still need to add the value to result
		// so it can be collected during merging
		// Store the processed value as-is (to_list will have already wrapped it if needed)
		// The value will be collected during the merging phase
		if variable.HasJoinMatches {
			// For joinmatches, we need to collect the value
			// The value will be joined later in the merging phase
			// Store as list to prepare for joining
			if existing, exists := result[variable.Name]; exists {
				// We already have a value - append to list
				if existingList, ok := existing.([]interface{}); ok {
					result[variable.Name] = append(existingList, processedValue)
				} else {
					result[variable.Name] = []interface{}{existing, processedValue}
				}
			} else {
				// First value - store as list
				// If processedValue is already a list (from to_list), use it directly
				// Otherwise wrap it in a list
				if pvList, ok := processedValue.([]interface{}); ok {
					result[variable.Name] = pvList
				} else {
					result[variable.Name] = []interface{}{processedValue}
				}
			}
		} else {
			// Normal variable - store the processed value
			result[variable.Name] = processedValue
		}
		varIndex++
	}

	// Note: joinmatches collection happens during match merging, not here
	// This function just extracts individual match results

	return result
}

// parseFunction parses a function call string and resolves variables from vars map
func parseFunction(funcStr string, vars map[string]interface{}) (name string, args []string, err error) {
	// Simple parsing - extract function name and arguments
	parts := strings.SplitN(funcStr, "(", 2)
	name = strings.TrimSpace(parts[0])

	if len(parts) > 1 {
		// Extract arguments
		argStr := strings.TrimSuffix(parts[1], ")")
		argStr = strings.TrimSpace(argStr)
		if argStr != "" {
			// Split by comma, handling quoted strings
			args = parseArguments(argStr, vars)
		}
	}

	return name, args, nil
}

// parseFunctionWithoutResolve parses a function call string without resolving variables
// This is used for group functions where arguments should be literal strings
func parseFunctionWithoutResolve(funcStr string) (name string, args []string, err error) {
	// Simple parsing - extract function name and arguments
	parts := strings.SplitN(funcStr, "(", 2)
	name = strings.TrimSpace(parts[0])

	if len(parts) > 1 {
		// Extract arguments
		argStr := strings.TrimSuffix(parts[1], ")")
		argStr = strings.TrimSpace(argStr)
		if argStr != "" {
			// Parse arguments without resolving variables
			args = parseArgumentsWithoutResolve(argStr)
		}
	}

	return name, args, nil
}

// parseArgumentsWithoutResolve parses function arguments without resolving variables
func parseArgumentsWithoutResolve(argStr string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)
	wasQuoted := false

	for i := 0; i < len(argStr); i++ {
		char := argStr[i]

		if !inQuotes && (char == '"' || char == '\'') {
			inQuotes = true
			quoteChar = char
			wasQuoted = true
		} else if inQuotes && char == quoteChar {
			// Check if this is an escaped quote (e.g., \" or \')
			if i > 0 && argStr[i-1] == '\\' {
				// Escaped quote - remove the backslash and add the quote
				// Remove the backslash we added
				currentStr := current.String()
				if len(currentStr) > 0 {
					current.Reset()
					current.WriteString(currentStr[:len(currentStr)-1])
				}
				current.WriteByte(char)
			} else {
				// End of quoted string
				inQuotes = false
				quoteChar = 0
			}
		} else if inQuotes && char == '\\' {
			// Handle escape sequences in quoted strings
			// Check what comes after the backslash
			if i+1 < len(argStr) {
				nextChar := argStr[i+1]
				switch nextChar {
				case '\\':
					// Escaped backslash - add single backslash
					current.WriteByte('\\')
					i++ // Skip the next backslash
				case '"', '\'':
					// Escaped quote - will be handled when we see the quote
					current.WriteByte('\\')
				case 'n':
					// Newline
					current.WriteByte('\n')
					i++ // Skip the 'n'
				case 't':
					// Tab
					current.WriteByte('\t')
					i++ // Skip the 't'
				case 'r':
					// Carriage return
					current.WriteByte('\r')
					i++ // Skip the 'r'
				default:
					// Other escape sequence - for regex patterns like \d, \w, etc.
					// Keep the backslash and the next character as-is
					// This handles cases like \\d+ where we want \d+ (single backslash)
					// But in raw strings, \\d+ is actually \\d+ (double backslash)
					// So we need to convert \\ to \ for all cases
					current.WriteByte('\\')
					current.WriteByte(nextChar)
					i++ // Skip the next character
				}
			} else {
				// Backslash at end of string - keep as is
				current.WriteByte(char)
			}
		} else if !inQuotes && char == ',' {
			arg := current.String()
			// Remove quotes from quoted arguments and trim spaces
			if wasQuoted {
				arg = strings.Trim(arg, `"'`)
				arg = strings.TrimSpace(arg) // Also trim spaces after removing quotes
			} else {
				arg = strings.TrimSpace(arg)
			}
			// Include empty strings if they were quoted
			if arg != "" || wasQuoted {
				args = append(args, arg)
			}
			current.Reset()
			wasQuoted = false
		} else {
			current.WriteByte(char)
		}
	}

	// Add last argument
	arg := current.String()
	// Remove quotes from quoted arguments and trim spaces
	if wasQuoted {
		arg = strings.Trim(arg, `"'`)
		arg = strings.TrimSpace(arg) // Also trim spaces after removing quotes
	} else {
		arg = strings.TrimSpace(arg)
	}
	// Include empty strings if they were quoted
	if arg != "" || wasQuoted {
		args = append(args, arg)
	}

	return args
}

// parseArguments parses function arguments and resolves variables from vars map
func parseArguments(argStr string, vars map[string]interface{}) []string {
	var args []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)
	wasQuoted := false // Track if current argument was quoted (to preserve empty strings)

	for i := 0; i < len(argStr); i++ {
		char := argStr[i]

		if !inQuotes && (char == '"' || char == '\'') {
			inQuotes = true
			quoteChar = char
			wasQuoted = true
			// Don't include the opening quote in the argument
		} else if inQuotes && char == quoteChar {
			// Check if this is an escaped quote (e.g., \" or \')
			if i > 0 && argStr[i-1] == '\\' {
				// Escaped quote - remove the backslash and add the quote
				currentStr := current.String()
				if len(currentStr) > 0 {
					current.Reset()
					current.WriteString(currentStr[:len(currentStr)-1])
				}
				current.WriteByte(char)
			} else {
				// End of quoted string
				inQuotes = false
				quoteChar = 0
			}
		} else if inQuotes && char == '\\' {
			// Handle escape sequences in quoted strings
			if i+1 < len(argStr) {
				nextChar := argStr[i+1]
				switch nextChar {
				case '\\':
					// Escaped backslash - add single backslash
					current.WriteByte('\\')
					i++ // Skip the next backslash
				case '"', '\'':
					// Escaped quote - will be handled when we see the quote
					current.WriteByte('\\')
				case 'n':
					// Newline
					current.WriteByte('\n')
					i++ // Skip the 'n'
				case 't':
					// Tab
					current.WriteByte('\t')
					i++ // Skip the 't'
				case 'r':
					// Carriage return
					current.WriteByte('\r')
					i++ // Skip the 'r'
				default:
					// Other escape sequence - for regex patterns like \d, \w, etc.
					// Keep the backslash and the next character as-is
					current.WriteByte('\\')
					current.WriteByte(nextChar)
					i++ // Skip the next character
				}
			} else {
				// Backslash at end of string - keep as is
				current.WriteByte(char)
			}
		} else if !inQuotes && char == ',' {
			arg := current.String()
			// Only trim spaces for unquoted arguments
			// For quoted arguments, preserve spaces (they might be meaningful, e.g., ' - non production')
			if !wasQuoted {
				arg = strings.TrimSpace(arg)
			}
			// Include empty strings if they were quoted (e.g., '' or "")
			if arg != "" || wasQuoted {
				// Resolve variable if not quoted
				arg = resolveVariable(arg, vars)
				args = append(args, arg)
			}
			current.Reset()
			wasQuoted = false
		} else if !inQuotes && char == ' ' {
			// Skip spaces when not in quotes (they're just formatting)
			continue
		} else {
			current.WriteByte(char)
		}
	}

	// Add last argument
	arg := current.String()
	// Only trim spaces for unquoted arguments
	// For quoted arguments, preserve spaces (they might be meaningful, e.g., ' - non production')
	if !wasQuoted {
		arg = strings.TrimSpace(arg)
	}
	// Include empty strings if they were quoted (e.g., '' or "")
	if arg != "" || wasQuoted {
		// Resolve variable if not quoted
		arg = resolveVariable(arg, vars)
		args = append(args, arg)
	}

	return args
}

// resolveVariable resolves a variable name from vars map, or returns the original string
func resolveVariable(arg string, vars map[string]interface{}) string {
	// If quoted, remove quotes and return
	if (strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`)) ||
		(strings.HasPrefix(arg, `'`) && strings.HasSuffix(arg, `'`)) {
		return strings.Trim(arg, `"'`)
	}

	// Check if it's a variable name
	if val, ok := vars[arg]; ok {
		// Convert to string
		return fmt.Sprintf("%v", val)
	}

	// Check for raw string prefix (r"...")
	if strings.HasPrefix(arg, "r\"") && strings.HasSuffix(arg, "\"") {
		return strings.TrimPrefix(strings.TrimSuffix(arg, "\""), "r\"")
	}
	if strings.HasPrefix(arg, "r'") && strings.HasSuffix(arg, "'") {
		return strings.TrimPrefix(strings.TrimSuffix(arg, "'"), "r'")
	}

	// Not a variable, return as-is
	return arg
}

// processGroupFunctions processes group functions from chain/functions attribute
// Functions are pipe-separated: "contains('ip') | set('key', 'value')"
// Returns filtered/processed matches
func (r *Runtime) processGroupFunctions(matches []map[string]interface{}, chainStr string, vars map[string]interface{}) ([]map[string]interface{}, error) {
	// Check if chainStr references a template variable
	if varValue, ok := vars[chainStr]; ok {
		// It's a variable reference - use its value
		if strValue, ok := varValue.(string); ok {
			chainStr = strValue
		} else if listValue, ok := varValue.([]interface{}); ok {
			// Variable is a list of function strings
			var funcStrs []string
			for _, item := range listValue {
				if strItem, ok := item.(string); ok {
					funcStrs = append(funcStrs, strItem)
				}
			}
			return r.applyGroupFunctions(matches, funcStrs, vars)
		}
	}

	// Parse pipe-separated function calls
	funcStrs := strings.Split(chainStr, "|")
	for i := range funcStrs {
		funcStrs[i] = strings.TrimSpace(funcStrs[i])
	}

	return r.applyGroupFunctions(matches, funcStrs, vars)
}

// applyGroupFunctions applies a list of group functions to matches
func (r *Runtime) applyGroupFunctions(matches []map[string]interface{}, funcStrs []string, vars map[string]interface{}) ([]map[string]interface{}, error) {
	var result []map[string]interface{}

	for _, match := range matches {
		// Make a copy to avoid modifying the original
		data := make(map[string]interface{})
		for k, v := range match {
			data[k] = v
		}

		// Apply each function in the chain
		shouldKeep := true
		for _, funcStr := range funcStrs {
			if funcStr == "" {
				continue
			}

			// Parse function call (e.g., "contains('ip')", "set('key', 'value')")
			// For group functions, we need to parse arguments without resolving variables
			// because arguments like 'ip' and 'mask' should be literal strings, not variable names
			funcName, args, err := parseFunctionWithoutResolve(funcStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse group function %s: %w", funcStr, err)
			}

			// Get function from registry (checks runtime overrides first)
			fn, ok := r.getGroupFunc(funcName)
			if !ok {
				// Function not found - skip it (could be a macro or custom function)
				continue
			}

			// Prepare kwargs for functions that need variable/schema access
			kwargs := make(map[string]interface{})
			// Always pass template variables to group functions so they can resolve variable references
			// This is needed for functions like set('key', 'var_name') where var_name should be resolved
			for k, v := range vars {
				kwargs[k] = v
			}
			// Add recorded vars (from record() function) - these override template vars
			if r.recordedVars != nil {
				for k, v := range r.recordedVars {
					kwargs[k] = v
				}
			}
			// Pass r.recordedVars to record() function so it can store values globally
			kwargs["_recorded_vars"] = r.recordedVars

			// Execute function
			newData, keep, err := fn(data, args, kwargs)
			if err != nil {
				return nil, fmt.Errorf("group function %s failed: %w", funcName, err)
			}

			// If function returns false, discard this match
			if !keep {
				shouldKeep = false
				break
			}

			// Update data with function result
			data = newData
		}

		// Only keep matches that passed all functions
		if shouldKeep {
			result = append(result, data)
		}
	}

	return result, nil
}

// processGroupMacro processes group macro on each match
// Macro can be a single macro name or comma-separated list
// Optimized to chain Starlark macros without Go↔Starlark conversions
func (r *Runtime) processGroupMacro(matches []map[string]interface{}, macroStr string, vars map[string]interface{}) ([]map[string]interface{}, error) {
	// Parse macro string (can be comma-separated list)
	macroNames := strings.Split(macroStr, ",")
	for i := range macroNames {
		macroNames[i] = strings.TrimSpace(macroNames[i])
	}

	// Check if all macros are Starlark (for optimization)
	allStarlark := true
	for _, macroName := range macroNames {
		if macroName == "" {
			continue
		}
		actualMacroName := resolveVariable(macroName, vars)
		if r.macroRegistry.HasGoMacro(actualMacroName) {
			allStarlark = false
			break
		}
	}

	// Determine macro language
	macroLang := "starlark" // default
	for _, m := range r.compiled.Macros {
		if m.Language != "" {
			macroLang = m.Language
			break
		}
	}

	var result []map[string]interface{}

	for _, match := range matches {
		processedMatch := match

		// Optimization: If all macros are Starlark, chain them without Go↔Starlark conversions
		if allStarlark && macroLang == "starlark" {
			// Convert to Starlark once
			starlarkEngine := r.macroRegistry.GetStarlarkEngine()
			if starlarkEngine != nil {
				dataVal := starlarkEngine.GoToStarlark(processedMatch)

				// Collect actual macro names (resolved from vars)
				actualMacroNames := make([]string, 0, len(macroNames))
				for _, macroName := range macroNames {
					if macroName == "" {
						continue
					}
					actualMacroNames = append(actualMacroNames, resolveVariable(macroName, vars))
				}

				// Execute all macros in batch (optimized)
				if len(actualMacroNames) > 0 {
					resultVal, err := starlarkEngine.ExecuteMacroStarlarkBatch(actualMacroNames, dataVal, nil)
					if err != nil {
						// Macro not found, continue with original data
						// Fall through to original method
					} else {
						// Check if result is None - in Python TTP, None means "continue with original data", NOT filter
						if resultVal == starlark.None {
							// Continue with original data (don't filter)
							// processedMatch stays the same
						} else {
							// Convert back to Go once
							if resultMap, ok := starlarkEngine.StarlarkToGo(resultVal).(map[string]interface{}); ok {
								processedMatch = resultMap
							}
						}
						// Skip the original method
						goto addResult
					}
				}
			}
		}

		// Original method (for Go macros, mixed languages, or if optimization failed)
		{
			// Apply each macro in sequence
			for _, macroName := range macroNames {
				if macroName == "" {
					continue
				}

				// Resolve macro name from vars if it's a variable reference
				actualMacroName := resolveVariable(macroName, vars)

				var macroResult interface{}
				var err error

				// Check if macro is registered as native Go macro first (priority)
				if r.macroRegistry.HasGoMacro(actualMacroName) {
					macroResult, err = r.macroRegistry.ExecuteMacro("go", actualMacroName, processedMatch, nil)
				} else {
					// Execute language-based macro on the match data
					macroResult, err = r.macroRegistry.ExecuteMacro(macroLang, actualMacroName, processedMatch, nil)
				}

				if err != nil {
					// If macro not found, continue with original data (don't fail)
					// This matches Python TTP behavior
					continue
				}

				// Handle macro result
				// Macro can return:
				// - nil/None: continue with original data
				// - dict/map: replace match data
				// - bool: filter (true = keep, false = discard)
				// - tuple: (data, additional_fields) - not fully supported yet

				switch v := macroResult.(type) {
				case bool:
					// Boolean result: filter the match
					if !v {
						// Discard this match
						processedMatch = nil
						break
					}
					// Keep match as-is
				case map[string]interface{}:
					// Dict result: replace match data
					processedMatch = v
				case nil:
					// None/nil: continue with original data
					// processedMatch stays the same
				default:
					// Other types: try to convert to map or keep as-is
					if mapResult, ok := v.(map[string]interface{}); ok {
						processedMatch = mapResult
					}
					// Otherwise keep original
				}

				// If match was discarded, break out of macro loop
				if processedMatch == nil {
					break
				}
			}
		}

	addResult:
		// Only add match if it wasn't discarded
		if processedMatch != nil {
			result = append(result, processedMatch)
		}
	}

	return result, nil
}

// processInputFunctions processes input functions for a given input
func (r *Runtime) processInputFunctions(inputName string, data string, vars map[string]interface{}) (string, error) {
	// Find the input configuration
	var inputConfig *compiler.CompiledInput
	for _, input := range r.compiled.Inputs {
		if input.Name == inputName {
			inputConfig = input
			break
		}
	}

	if inputConfig == nil {
		// No input config found, return data as-is
		return data, nil
	}

	result := data

	// Process extract_commands if specified
	if len(inputConfig.ExtractCommands) > 0 {
		fn, ok := r.getInputFunc("extract_commands")
		if ok {
			processed, shouldContinue, err := fn(result, inputConfig.ExtractCommands, nil)
			if err != nil {
				return "", err
			}
			if !shouldContinue {
				return "", nil // Condition failed, stop processing
			}
			result = processed
		}
	}

	// Process functions attribute (pipe-separated)
	if inputConfig.Functions != "" {
		funcStrs := strings.Split(inputConfig.Functions, "|")
		for _, funcStr := range funcStrs {
			funcStr = strings.TrimSpace(funcStr)
			if funcStr == "" {
				continue
			}

			// Parse function call
			funcName, args, err := parseFunction(funcStr, vars)
			if err != nil {
				return "", fmt.Errorf("failed to parse input function %s: %w", funcStr, err)
			}

			// Get function from registry (checks runtime overrides first)
			fn, ok := r.getInputFunc(funcName)
			if !ok {
				// Function not found, skip it
				continue
			}

			// Execute function
			processed, shouldContinue, err := fn(result, args, nil)
			if err != nil {
				return "", fmt.Errorf("input function %s failed: %w", funcName, err)
			}
			if !shouldContinue {
				return "", nil // Condition failed, stop processing
			}
			result = processed
		}
	}

	// Process macro attribute (comma-separated macro names)
	if inputConfig.Macro != "" {
		macroNames := strings.Split(inputConfig.Macro, ",")
		for _, macroName := range macroNames {
			macroName = strings.TrimSpace(macroName)
			if macroName == "" {
				continue
			}

			// Execute macro
			var macroLang string
			for _, m := range r.compiled.Macros {
				macroLang = m.Language
				break // Use first macro's language
			}
			if macroLang == "" {
				macroLang = "starlark"
			}

			macroResult, err := r.macroRegistry.ExecuteMacro(macroLang, macroName, result, nil)
			if err != nil {
				// Macro not found, continue
				continue
			}

			// Handle macro return values
			if boolResult, ok := macroResult.(bool); ok {
				if !boolResult {
					return "", nil // Condition failed, stop processing
				}
				// Condition passed, continue with original data
			} else if macroResult != nil {
				// Replace data with macro result
				if strResult, ok := macroResult.(string); ok {
					result = strResult
				} else {
					// Convert to string
					result = fmt.Sprintf("%v", macroResult)
				}
			}
		}
	}

	return result, nil
}

// processOutputFunctions processes output functions for results
func (r *Runtime) processOutputFunctions(results interface{}, vars map[string]interface{}) (interface{}, error) {
	result := results

	// Process each output configuration
	for _, outputConfig := range r.compiled.Outputs {
		// Process functions attribute FIRST (before formatting)
		// This allows functions like traverse() to extract data before formatting
		if outputConfig.Functions != "" {
			funcStrs := strings.Split(outputConfig.Functions, "|")
			for _, funcStr := range funcStrs {
				funcStr = strings.TrimSpace(funcStr)
				if funcStr == "" {
					continue
				}

				// Parse function call
				funcName, args, err := parseFunction(funcStr, vars)
				if err != nil {
					return nil, fmt.Errorf("failed to parse output function %s: %w", funcStr, err)
				}

				// Get function from registry (checks runtime overrides first)
				fn, ok := r.getOutputFunc(funcName)
				if !ok {
					// Function not found, skip it (could be a macro)
					continue
				}

				// Prepare kwargs for functions that need context
				kwargs := make(map[string]interface{})
				if funcName == "is_equal" {
					// Pass output tag content as expected data
					// This would need to be loaded from output tag content
					// For now, we'll skip this functionality
				}

				// Execute function
				processed, err := fn(result, args, kwargs)
				if err != nil {
					return nil, fmt.Errorf("output function %s failed: %w", funcName, err)
				}
				result = processed
			}
		}

		// Apply format conversion if format is specified
		if outputConfig.Format != "" && outputConfig.Format != "raw" {
			// Get format function from registry (checks runtime overrides first)
			formatFn, ok := r.getOutputFunc(outputConfig.Format)
			if !ok {
				return nil, fmt.Errorf("unknown output format: %s", outputConfig.Format)
			}

			// Prepare kwargs from output config attributes
			kwargs := make(map[string]interface{})
			if outputConfig.Attributes != nil {
				for k, v := range outputConfig.Attributes {
					kwargs[k] = v
				}
			}

			// Add path if specified
			if outputConfig.Path != "" {
				kwargs["path"] = outputConfig.Path
			}

			// Add headers if specified
			if outputConfig.Headers != "" {
				kwargs["headers"] = outputConfig.Headers
			}

			// Check if returner is "terminal"
			// When returner="terminal", Python TTP outputs formatted data to stdout
			// but still returns the original parsed data structure
			// We need to output formatted data but keep original data for return value
			returnerIsTerminal := outputConfig.Returner == "terminal"
			var originalResult interface{} // Will save original result before formatting

			// Save original result before formatting (for terminal returner)
			if returnerIsTerminal {
				originalResult = result
			}

			// For per_input results, format each input's results separately
			// Python TTP applies formatters to each input's results individually
			// Exception: JSON and some formatters format the entire result list
			if resultList, ok := result.([]interface{}); ok {
				// For JSON formatter, format the entire list (not each item)
				if outputConfig.Format == "json" || outputConfig.Format == "yaml" || outputConfig.Format == "pprint" {
					formatted, err := formatFn(result, nil, kwargs)
					if err != nil {
						return nil, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
					}
					result = formatted
				} else {
					// For other formatters, format each input's results separately
					// BUT: for table formatter with anonymous groups (list of dicts),
					// Python TTP combines all matches into a single table
					if outputConfig.Format == "table" {
						// Check if resultList is a list of dicts (anonymous group results)
						allDicts := true
						for _, item := range resultList {
							if _, ok := item.(map[string]interface{}); !ok {
								allDicts = false
								break
							}
						}
						if allDicts {
							// Anonymous group results - combine into single table
							formatted, err := formatFn(resultList, nil, kwargs)
							if err != nil {
								return nil, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
							}
							// formatTable returns [][]interface{} (table structure: [[headers], [row1], [row2], ...])
							// For per_input, Python TTP wraps it once: [[headers, row1, row2, ...]]
							// So we need to convert [][]interface{} to []interface{} and wrap it
							if tableResult, ok := formatted.([][]interface{}); ok {
								// Convert table to list of rows: [[headers], [row1], [row2], ...] -> [headers, row1, row2, ...]
								tableList := make([]interface{}, len(tableResult))
								for i, row := range tableResult {
									tableList[i] = row
								}
								result = []interface{}{tableList}
							} else {
								result = formatted
							}
						} else {
							// Not anonymous group - format each input separately
							formattedList := make([]interface{}, len(resultList))
							for i, inputResult := range resultList {
								formatted, err := formatFn(inputResult, nil, kwargs)
								if err != nil {
									return nil, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
								}
								formattedList[i] = formatted
							}
							result = formattedList
						}
					} else {
						// Other formatters - format each input separately
						formattedList := make([]interface{}, len(resultList))
						for i, inputResult := range resultList {
							formatted, err := formatFn(inputResult, nil, kwargs)
							if err != nil {
								return nil, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
							}
							formattedList[i] = formatted
						}
						result = formattedList
					}
				}
			} else {
				// Execute format function
				formatted, err := formatFn(result, nil, kwargs)
				if err != nil {
					return nil, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
				}
				result = formatted
			}

			// If returner is "terminal", output formatted data to stdout
			// but return the original parsed data structure (Python TTP behavior)
			if returnerIsTerminal {
				// Output formatted data to stdout
				if formattedStr, ok := result.(string); ok {
					fmt.Print(formattedStr)
				} else if formattedBytes, ok := result.([]byte); ok {
					fmt.Print(string(formattedBytes))
				} else {
					// Convert to string and output
					fmt.Print(fmt.Sprintf("%v", result))
				}
				// Return original parsed data structure (not formatted)
				result = originalResult
			}
		}
	}

	return result, nil
}

// resolvePathSegments resolves a path by splitting it into segments first,
// then resolving each segment separately. This preserves formatters (*, **)
// and prevents issues when resolved values contain dots (like IP addresses).
// Returns the resolved path segments as a slice, preserving formatters.
func (r *Runtime) resolvePathSegments(pathTemplate string, matchResult map[string]interface{}, vars map[string]interface{}) []struct {
	key       string
	formatter string
} {
	if pathTemplate == "" {
		return nil
	}

	// Split path by dots first
	rawSegments := strings.Split(pathTemplate, ".")
	resolvedParts := make([]struct {
		key       string
		formatter string
	}, 0, len(rawSegments))

	for _, segment := range rawSegments {
		if segment == "" {
			continue
		}

		// Extract formatter from segment (preserve * or **)
		formatter := ""
		segmentWithoutFormatter := segment
		if strings.HasSuffix(segment, "**") {
			formatter = "**"
			segmentWithoutFormatter = strings.TrimSuffix(segment, "**")
		} else if strings.HasSuffix(segment, "*") {
			formatter = "*"
			segmentWithoutFormatter = strings.TrimSuffix(segment, "*")
		}

		// Resolve variables in this segment only
		resolvedSegment, err := r.pathResolver.ResolvePath(segmentWithoutFormatter, matchResult, vars)
		if err != nil || resolvedSegment == "" {
			resolvedSegment = segmentWithoutFormatter
		}

		// Store resolved segment with formatter
		resolvedParts = append(resolvedParts, struct {
			key       string
			formatter string
		}{
			key:       resolvedSegment,
			formatter: formatter,
		})
	}

	return resolvedParts
}

// storeAtPathSegments stores a value at a path given as resolved segments.
// This prevents issues when resolved values contain dots (like IP addresses).
func (r *Runtime) storeAtPathSegments(results map[string]interface{}, parts []struct {
	key       string
	formatter string
}, value interface{}) {
	if len(parts) == 0 {
		return
	}

	// DEBUG: Trace storeAtPathSegments for issue #13
	// Navigate/create the nested structure
	current := results
	for _, part := range parts[:len(parts)-1] {
		key := part.key
		formatter := part.formatter

		// Handle formatter for this intermediate segment
		if formatter == "**" {
			// ** means this segment should be a dictionary
			// Ensure it exists as a map
			if _, exists := current[key]; !exists {
				current[key] = make(map[string]interface{})
			}
			next, ok := current[key].(map[string]interface{})
			if !ok {
				current[key] = make(map[string]interface{})
				next = current[key].(map[string]interface{})
			}
			current = next
		} else if formatter == "*" {
			// * means this segment should be a list
			// Create a list containing a single object for the nested structure
			if _, exists := current[key]; !exists {
				// Create a list with a single empty map for the nested structure
				nestedMap := make(map[string]interface{})
				current[key] = []interface{}{nestedMap}
			}
			// Navigate into the first (and only) item in the list
			list, ok := current[key].([]interface{})
			if !ok || len(list) == 0 {
				// If not a list or empty, create a new list with a single map
				nestedMap := make(map[string]interface{})
				current[key] = []interface{}{nestedMap}
				current = nestedMap
			} else {
				// Navigate into the first item
				next, ok := list[0].(map[string]interface{})
				if !ok {
					// First item is not a map, replace with a map
					next = make(map[string]interface{})
					list[0] = next
				}
				current = next
			}
		} else {
			// No formatter - create nested map
			if _, exists := current[key]; !exists {
				current[key] = make(map[string]interface{})
			}
			next, ok := current[key].(map[string]interface{})
			if !ok {
				current[key] = make(map[string]interface{})
				next = current[key].(map[string]interface{})
			}
			current = next
		}
	}

	// Set the final value
	finalPart := parts[len(parts)-1]
	finalKey := finalPart.key
	finalFormatter := finalPart.formatter

	if finalKey == "" {
		return
	}

	if finalFormatter == "**" {
		// ** on final segment means: the finalKey itself becomes a key in the parent dictionary
		// Check if previous segment had ** formatter
		if len(parts) > 1 {
			prevPart := parts[len(parts)-2]
			if prevPart.formatter == "**" {
				// Previous segment is a dict - use finalKey as key in that dict
				// current already points to the dict from previous segment
				if valueMap, ok := value.(map[string]interface{}); ok {
					// Value is a map - store it directly at finalKey
					current[finalKey] = valueMap
				} else {
					// Value is not a map - store it directly
					current[finalKey] = value
				}
				return
			}
		}

		// No previous ** segment - create dict at finalKey
		if _, exists := current[finalKey]; !exists {
			current[finalKey] = make(map[string]interface{})
		}
		dict, ok := current[finalKey].(map[string]interface{})
		if !ok {
			current[finalKey] = make(map[string]interface{})
			dict = current[finalKey].(map[string]interface{})
		}

		if valueMap, ok := value.(map[string]interface{}); ok {
			// Merge map values into dict
			for k, v := range valueMap {
				dict[k] = v
			}
		} else {
			dict[finalKey] = value
		}
	} else if finalFormatter == "*" {
		// * means store as list
		if existingList, ok := current[finalKey].([]interface{}); ok {
			// Append to existing list
			if valueList, ok := value.([]interface{}); ok {
				current[finalKey] = append(existingList, valueList...)
			} else if valueList, ok := value.([]map[string]interface{}); ok {
				// Convert []map[string]interface{} to []interface{} and append
				// This prevents double-wrapping lists of maps
				for _, v := range valueList {
					existingList = append(existingList, v)
				}
				current[finalKey] = existingList
			} else {
				current[finalKey] = append(existingList, value)
			}
		} else {
			// Create new list
			if valueList, ok := value.([]interface{}); ok {
				current[finalKey] = valueList
			} else if valueList, ok := value.([]map[string]interface{}); ok {
				// Convert []map[string]interface{} to []interface{}
				newList := make([]interface{}, len(valueList))
				for i, v := range valueList {
					newList[i] = v
				}
				current[finalKey] = newList
			} else {
				current[finalKey] = []interface{}{value}
			}
		}
	} else {
		// No formatter - store as single value (but if it's a list and key already exists as list, merge)
		if existingList, ok := current[finalKey].([]interface{}); ok {
			// Key already exists as list - this shouldn't happen for groups without * formatter
			// But it can happen if an intermediate segment had * formatter
			// Check if we should convert list back to map (if final segment has no * formatter and list has one item)
			// Also check if the list contains an empty map that should be removed
			
			
			// First, remove any empty maps from the list
			cleanedList := make([]interface{}, 0, len(existingList))
			for _, item := range existingList {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if len(itemMap) > 0 {
						cleanedList = append(cleanedList, item)
					} else {
					}
				} else {
					cleanedList = append(cleanedList, item)
				}
			}
			
			
			// If final segment has no * formatter, we should convert list to map if possible
			if finalFormatter == "" {
				// If cleaned list has only one item, convert to map
				if len(cleanedList) == 1 {
					// Convert list to map (single item) - use the item from cleaned list
					// If the new value is also a map, merge them; otherwise use the cleaned item
					if valueMap, ok := value.(map[string]interface{}); ok {
						if cleanedMap, ok := cleanedList[0].(map[string]interface{}); ok {
							// Merge maps
							for k, v := range valueMap {
								cleanedMap[k] = v
							}
							current[finalKey] = cleanedMap
						} else {
							current[finalKey] = valueMap
						}
					} else {
						current[finalKey] = cleanedList[0]
					}
					return
				}
				
				// If cleaned list is empty and value is a map, store as map
				if len(cleanedList) == 0 {
					if valueMap, ok := value.(map[string]interface{}); ok {
						current[finalKey] = valueMap
						return
					}
				}
			}
			
			
			// Otherwise, append to cleaned list (but only if finalFormatter is not "")
			// If finalFormatter is "", we should have converted above
			if finalFormatter != "" {
				if valueList, ok := value.([]interface{}); ok {
					current[finalKey] = append(cleanedList, valueList...)
				} else if valueList, ok := value.([]map[string]interface{}); ok {
					// Convert []map[string]interface{} to []interface{} and append
					for _, v := range valueList {
						cleanedList = append(cleanedList, v)
					}
					current[finalKey] = cleanedList
				} else {
					current[finalKey] = append(cleanedList, value)
				}
			} else {
				// FinalFormatter is "" but we have multiple items - this shouldn't happen
				// But if it does, just use the value (should be a map)
				if valueMap, ok := value.(map[string]interface{}); ok {
					current[finalKey] = valueMap
				} else {
					current[finalKey] = value
				}
			}
		} else {
			// Store as single value
			existingValue := current[finalKey]
			if existingValue != nil {
			}
			current[finalKey] = value
		}
	}
}

// storeAtPath stores a value at a path, creating nested structures as needed
// Paths with dots (e.g., "show.cable.modem*") are split and nested structures are created
// Path formatters (* and **) are handled per path segment, not just at the end
// This matches Python TTP's behavior where each path segment is processed separately
func (r *Runtime) storeAtPath(results map[string]interface{}, path string, value interface{}) {
	if path == "" {
		return
	}

	// Split path by dots FIRST (before handling formatters)
	// This is important because formatters apply to each segment, not the whole path
	rawParts := strings.Split(path, ".")

	// Process each part to extract formatter and value
	type pathPart struct {
		key       string
		formatter string // "", "*", or "**"
	}

	parts := make([]pathPart, 0, len(rawParts))
	for _, rawPart := range rawParts {
		if rawPart == "" {
			continue // Skip empty parts
		}

		part := pathPart{}
		// Extract formatter from the end of the part
		if strings.HasSuffix(rawPart, "**") {
			part.key = strings.TrimSuffix(rawPart, "**")
			part.formatter = "**"
		} else if strings.HasSuffix(rawPart, "*") {
			part.key = strings.TrimSuffix(rawPart, "*")
			part.formatter = "*"
		} else {
			part.key = rawPart
			part.formatter = ""
		}
		parts = append(parts, part)
	}

	if len(parts) == 0 {
		return
	}

	// Navigate/create the nested structure
	current := results
	for _, part := range parts[:len(parts)-1] {
		key := part.key
		formatter := part.formatter

		// Handle formatter for this intermediate segment
		if formatter == "**" {
			// ** means this segment should be a dictionary
			// Ensure it exists as a map
			if _, exists := current[key]; !exists {
				current[key] = make(map[string]interface{})
			}
			next, ok := current[key].(map[string]interface{})
			if !ok {
				current[key] = make(map[string]interface{})
				next = current[key].(map[string]interface{})
			}
			current = next
		} else if formatter == "*" {
			// * means this segment should be a list
			// Create a list containing a single object for the nested structure
			if _, exists := current[key]; !exists {
				// Create a list with a single empty map for the nested structure
				nestedMap := make(map[string]interface{})
				current[key] = []interface{}{nestedMap}
			}
			// Navigate into the first (and only) item in the list
			list, ok := current[key].([]interface{})
			if !ok || len(list) == 0 {
				// If not a list or empty, create a new list with a single map
				nestedMap := make(map[string]interface{})
				current[key] = []interface{}{nestedMap}
				current = nestedMap
			} else {
				// Navigate into the first item
				next, ok := list[0].(map[string]interface{})
				if !ok {
					// First item is not a map, replace with a map
					next = make(map[string]interface{})
					list[0] = next
				}
				current = next
			}
		} else {
			// No formatter - create nested map
			if _, exists := current[key]; !exists {
				current[key] = make(map[string]interface{})
			}
			next, ok := current[key].(map[string]interface{})
			if !ok {
				current[key] = make(map[string]interface{})
				next = current[key].(map[string]interface{})
			}
			current = next
		}
	}

	// Set the final value
	finalPart := parts[len(parts)-1]
	finalKey := finalPart.key
	finalFormatter := finalPart.formatter

	if finalKey == "" {
		return
	}

	if finalFormatter == "**" {
		// ** on final segment means: the finalKey itself becomes a key in the parent dictionary
		// For example: `neighbors**.{{ neighbor }}**` resolves to `neighbors**.10.100.100.212**`
		// This means: store value at `neighbors["10.100.100.212"]`
		// So we need to check if the previous segment had ** formatter
		// If so, use finalKey as the dictionary key in the parent
		if len(parts) > 1 {
			// Check if previous segment had ** formatter
			prevPart := parts[len(parts)-2]
			if prevPart.formatter == "**" {
				// Previous segment is a dict - use finalKey as key in that dict
				// current already points to the dict from previous segment
				if valueMap, ok := value.(map[string]interface{}); ok {
					// Value is a map - store it directly at finalKey
					current[finalKey] = valueMap
				} else {
					// Value is not a map - store it directly
					current[finalKey] = value
				}
				return
			}
		}

		// No previous ** segment - create dict at finalKey and store value
		// This handles cases like `interfaces.**` where we create a dict with numeric keys
		if _, exists := current[finalKey]; !exists {
			current[finalKey] = make(map[string]interface{})
		}
		dict, ok := current[finalKey].(map[string]interface{})
		if !ok {
			current[finalKey] = make(map[string]interface{})
			dict = current[finalKey].(map[string]interface{})
		}

		// For ** without previous **, we need to generate a key
		// But actually, if we're here, the finalKey should be used as the dict key
		// This is a fallback case
		if valueMap, ok := value.(map[string]interface{}); ok {
			// Merge map values into dict
			for k, v := range valueMap {
				dict[k] = v
			}
		} else {
			// Store value directly - this shouldn't happen in normal cases
			dict[finalKey] = value
		}
	} else if finalFormatter == "*" {
		// * means store as list
		if existingList, ok := current[finalKey].([]interface{}); ok {
			// Append to existing list
			if valueList, ok := value.([]interface{}); ok {
				current[finalKey] = append(existingList, valueList...)
			} else if valueMapList, ok := value.([]map[string]interface{}); ok {
				// Flatten []map[string]interface{} into []interface{}
				for _, v := range valueMapList {
					existingList = append(existingList, v)
				}
				current[finalKey] = existingList
			} else {
				current[finalKey] = append(existingList, value)
			}
		} else {
			// Create new list
			if valueList, ok := value.([]interface{}); ok {
				current[finalKey] = valueList
			} else if valueMapList, ok := value.([]map[string]interface{}); ok {
				// Flatten []map[string]interface{} into []interface{}
				newList := make([]interface{}, len(valueMapList))
				for i, v := range valueMapList {
					newList[i] = v
				}
				current[finalKey] = newList
			} else {
				current[finalKey] = []interface{}{value}
			}
		}
	} else {
		// No formatter - store as single value (but if it's a list and key already exists as list, merge)
		if existingList, ok := current[finalKey].([]interface{}); ok {
			// Key already exists as list - check if it was created by intermediate segment with *
			// If the list contains only one item and it's an empty map, replace it instead of appending
			// Also check if the list contains an empty map that should be removed
			if len(existingList) == 1 {
				if existingMap, ok := existingList[0].(map[string]interface{}); ok && len(existingMap) == 0 {
					// List contains only an empty map - this was created by intermediate segment with *
					// Replace the empty map with the actual value
					current[finalKey] = []interface{}{value}
					return
				}
			} else if len(existingList) == 2 {
				// Check if list contains one actual value and one empty map
				// This can happen if the empty map was created first, then the actual value was stored
				emptyMapIdx := -1
				hasValue := false
				for i, item := range existingList {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if len(itemMap) == 0 {
							emptyMapIdx = i
						} else {
							hasValue = true
						}
					} else {
						hasValue = true
					}
				}
				if emptyMapIdx >= 0 && hasValue {
					// List contains both an empty map and a value - remove the empty map
					newList := make([]interface{}, 0, len(existingList)-1)
					for i, item := range existingList {
						if i != emptyMapIdx {
							newList = append(newList, item)
						}
					}
					current[finalKey] = newList
					return
				}
			}
			// Otherwise, append to existing list
			if valueList, ok := value.([]interface{}); ok {
				current[finalKey] = append(existingList, valueList...)
			} else if valueMapList, ok := value.([]map[string]interface{}); ok {
				// Flatten []map[string]interface{} into []interface{}
				for _, v := range valueMapList {
					existingList = append(existingList, v)
				}
				current[finalKey] = existingList
			} else {
				current[finalKey] = append(existingList, value)
			}
		} else {
			// Store as single value
			current[finalKey] = value
		}
	}
}

// resolveGetterFunction resolves special getter function strings like "gethostname"
// to their actual values based on the input data
func (r *Runtime) resolveGetterFunction(value string, inputData string) interface{} {
	if value == "gethostname" {
		// Try to find hostname pattern: word characters followed by # or >
		re := regexp.MustCompile(`(\w+)[#>]`)
		matches := re.FindStringSubmatch(inputData)
		if len(matches) > 1 {
			return matches[1]
		}
		// If not found, try other patterns from Python TTP
		// Juniper: some.user@hostname>
		re = regexp.MustCompile(`\S+@(\S+)>`)
		matches = re.FindStringSubmatch(inputData)
		if len(matches) > 1 {
			return matches[1]
		}
		// Huawei: <hostname>
		re = regexp.MustCompile(`<(\S+)>`)
		matches = re.FindStringSubmatch(inputData)
		if len(matches) > 1 {
			return matches[1]
		}
		// Cisco IOS XR: RP/0/4/CPU0:hostname#
		re = regexp.MustCompile(`\S+:(\S+)#`)
		matches = re.FindStringSubmatch(inputData)
		if len(matches) > 1 {
			return matches[1]
		}
		// Fortigate: hostname (context) #
		re = regexp.MustCompile(`(\S+ \(\S+\)) #`)
		matches = re.FindStringSubmatch(inputData)
		if len(matches) > 1 {
			return matches[1]
		}
		// Nokia (ALU) SROS: A:hostname>, *A:hostname#, etc.
		re = regexp.MustCompile(`\n\S{1,2}:(\S+?)[>#]`)
		matches = re.FindStringSubmatch(inputData)
		if len(matches) > 1 {
			return matches[1]
		}
		// If no pattern matches, return False (Python TTP behavior)
		return false
	}
	// Not a getter function, return as-is
	return value
}
