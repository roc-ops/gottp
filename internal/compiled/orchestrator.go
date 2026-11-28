package compiled

import (
	"fmt"
	"sort"
	"strings"

	"github.com/roc-ops/gottp/internal/compiler"
)

// This file holds the one parse pipeline behind Parse and ParseWithSourceMap,
// and the setup and input-order helpers that ParseStream shares with it
// (refactor A, #60). Before, ParseWithSourceMap carried its own copy of the
// orchestration, and it drifted from Parse (#105, #106, #107, #108, #52).

// applyParseOptions installs the per-parse options on r (which must be the
// private copy returned by beginParse), re-registers macro overrides and
// returns the merged variables: template <vars>, then the vars argument,
// then ParseOptions.Vars, each overriding the one before.
func (r *Runtime) applyParseOptions(vars map[string]interface{}, options *ParseOptions) map[string]interface{} {
	if options != nil && options.YANGModuleSet != nil {
		r.SetYANGModuleSet(options.YANGModuleSet)
	}
	if options != nil && options.Lookups != nil {
		r.runtimeLookups = options.Lookups
	}
	if options != nil && options.Functions != nil {
		r.runtimeFunctions = options.Functions
	}
	r.parseVars = vars
	if options != nil {
		r.parseOptVars = options.Vars
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

	// Template vars (from the <vars> tag) are the base; passed vars override them.
	if r.compiled.Vars != nil || vars != nil {
		mergedVars := make(map[string]interface{})
		for k, v := range r.compiled.Vars {
			mergedVars[k] = v
		}
		for k, v := range vars {
			mergedVars[k] = v
		}
		vars = mergedVars
	}
	// ParseOptions.Vars has the highest precedence.
	if options != nil && options.Vars != nil {
		if vars == nil {
			vars = make(map[string]interface{})
		}
		for k, v := range options.Vars {
			vars[k] = v
		}
	}
	return vars
}

// prepareInputs adds the data of the template's <input load="text"> tags to
// inputs (passed data wins over embedded data) and returns the input order:
// the template's inputs in template order, then the other passed inputs
// sorted by name. Every input gets a result entry, even one no group reads.
func (r *Runtime) prepareInputs(inputs map[string]string) []string {
	for _, in := range r.compiled.Inputs {
		if in.Load == "text" && in.Data != "" {
			if _, exists := inputs[in.Name]; !exists {
				inputs[in.Name] = in.Data
			}
		}
	}

	seen := make(map[string]bool, len(inputs))
	inputOrder := make([]string, 0, len(inputs))
	for _, in := range r.compiled.Inputs {
		if _, exists := inputs[in.Name]; exists && !seen[in.Name] {
			seen[in.Name] = true
			inputOrder = append(inputOrder, in.Name)
		}
	}
	// Go maps have no order, so passed inputs that aren't template inputs
	// are added sorted by name, which keeps the result deterministic.
	passed := make([]string, 0, len(inputs))
	for name := range inputs {
		if !seen[name] {
			passed = append(passed, name)
		}
	}
	sort.Strings(passed)
	return append(inputOrder, passed...)
}

// groupInputNames returns the inputs a top-level group parses: the
// comma-separated names in its input= attribute, or every input in order.
// Names that aren't inputs are kept; the caller skips them (Python TTP does
// the same).
func groupInputNames(group *compiler.CompiledGroup, inputOrder []string) []string {
	if group.Input == "" {
		return inputOrder
	}
	var names []string
	for _, name := range strings.Split(group.Input, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// isVoidGroup reports whether group has the void attribute (present and
// empty, or "true" in any of its usual spellings).
func isVoidGroup(group *compiler.CompiledGroup) bool {
	voidVal, ok := group.Attributes["void"]
	return ok && (voidVal == "" || voidVal == "true" || voidVal == "True" || voidVal == "TRUE")
}

// parse is the parse pipeline behind Parse and ParseWithSourceMap. With a
// non-nil sourceMap it also fills sourceMap; nothing else about the parse
// changes.
func (r *Runtime) parse(inputs map[string]string, vars map[string]interface{}, options *ParseOptions, sourceMap *SourceMap) (interface{}, error) {
	// Per-parse state lives on a private copy of the Runtime, so repeated and
	// concurrent calls on one Runtime never share it (#83).
	run := r.beginParse()
	defer r.finishParse(run)
	r = run

	vars = r.applyParseOptions(vars, options)
	inputOrder := r.prepareInputs(inputs)
	var obs *smObserver
	if sourceMap != nil {
		initSourceMapInputs(sourceMap, inputs, inputOrder)
		obs = newSMObserver()
		r.smObs = obs
	}

	// per_input keeps one results map per input; per_template one for all.
	perInput := r.compiled.ResultsMethod == "per_input"
	var results map[string]interface{}
	var resultsPerInput map[string]map[string]interface{}
	if perInput {
		resultsPerInput = make(map[string]map[string]interface{}, len(inputOrder))
		for _, inputName := range inputOrder {
			resultsPerInput[inputName] = make(map[string]interface{})
		}
	} else {
		results = make(map[string]interface{})
	}
	getResultsMap := func(inputName string) map[string]interface{} {
		if !perInput {
			return results
		}
		resultsMap, exists := resultsPerInput[inputName]
		if !exists {
			resultsMap = make(map[string]interface{})
			resultsPerInput[inputName] = resultsMap
		}
		return resultsMap
	}

	for _, group := range r.compiled.Groups {
		// Nested groups are parsed within their parent group.
		if group.IsNested {
			continue
		}

		// Collect key fields from keys= attribute (recursively includes nested groups)
		r.collectGroupKeys(group)

		for _, inputName := range groupInputNames(group, inputOrder) {
			inputData, ok := inputs[inputName]
			if !ok {
				// Input not found - skip (Python TTP behavior)
				continue
			}
			r.usePathResolverFor(inputName)

			processedInputData, err := r.processInputFunctions(inputName, inputData, vars)
			if err != nil {
				return nil, fmt.Errorf("failed to process input functions for %s: %w", inputName, err)
			}

			if obs != nil {
				obs.beginInput(inputName, processedInputData)
			}
			// Each record is saved at its own path in the input's results
			// as it is processed, as TTP's save_curelements saves it.
			r.treeResults = getResultsMap(inputName)
			_, err = r.parseGroup(group, processedInputData, vars)
			r.treeResults = nil
			if err != nil {
				return nil, fmt.Errorf("failed to parse group %s: %w", group.Name, err)
			}
			if err := r.raised.get(); err != nil {
				return nil, err
			}
		}
	}

	if obs != nil {
		obs.build(sourceMap, getResultsMap)
		r.smObs = nil
	}

	// Store vars with name attribute in each input's results
	for inputName, inputResults := range resultsPerInput {
		r.usePathResolverFor(inputName)
		r.storeVarsWithName(inputResults, inputs[inputName], vars)
	}

	// Form the result: one per input (per_input) or one for the template.
	// An anonymous group's results become the result (Python TTP form_results).
	var resultsToFormat interface{}
	if perInput {
		resultsList := make([]interface{}, 0, len(inputOrder))
		for _, inputName := range inputOrder {
			resultsList = append(resultsList, formResults(resultsPerInput[inputName]))
		}
		resultsToFormat = resultsList
	} else {
		resultsToFormat = formResults(results)
	}

	finalResults := resultsToFormat
	formatted := false
	if len(r.compiled.Outputs) > 0 {
		var err error
		finalResults, formatted, err = r.processOutputFunctions(resultsToFormat, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process output functions: %w", err)
		}
	}

	if !perInput || !formatted {
		// A value an output macro or function returned is the result as
		// is, like in Python TTP.
		return finalResults, nil
	}
	// per_input formatter output - return as list (already wrapped if
	// formatted, or wrap if not; the wrap is #98)
	if _, isList := finalResults.([]interface{}); !isList {
		return []interface{}{finalResults}, nil
	}
	return finalResults, nil
}

// storeVarsWithName stores every <vars name="..."> tag's variables in one
// input's results, at the tag's (possibly dynamic) path. inputData is that
// input's data, used by getters such as gethostname.
func (r *Runtime) storeVarsWithName(inputResults map[string]interface{}, inputData string, vars map[string]interface{}) {
	for _, varsWithName := range r.compiled.VarsWithName {
		// The path may contain {{ }}; it resolves against the template vars
		// overridden by this tag's own vars.
		resolutionVars := make(map[string]interface{}, len(vars)+len(varsWithName.Vars))
		for k, v := range vars {
			resolutionVars[k] = v
		}
		for k, v := range varsWithName.Vars {
			resolutionVars[k] = v
		}

		// Variables used in the path are not stored.
		pathVarSet := make(map[string]bool)
		for _, v := range NewPathResolver().ExtractVariablesFromPath(varsWithName.Name) {
			pathVarSet[v] = true
		}

		// The dynamic-path cache is not used: TTP saves these vars before
		// any group record, when the cache is still empty.
		resolvedPath, err := NewPathResolver().ResolvePath(varsWithName.Name, nil, resolutionVars)
		if err != nil || resolvedPath == "" {
			resolvedPath = varsWithName.Name
		}

		varsToStore := make(map[string]interface{})
		for k, v := range varsWithName.Vars {
			if pathVarSet[k] {
				continue
			}
			if strVal, ok := v.(string); ok {
				varsToStore[k] = r.resolveGetterFunction(strVal, inputData)
			} else {
				varsToStore[k] = v
			}
		}
		r.storeAtPath(inputResults, resolvedPath, varsToStore)
	}
}

// initSourceMapInputs gives every input an InputSourceMap with one
// unmatched LineMapping per line of its data.
func initSourceMapInputs(sourceMap *SourceMap, inputs map[string]string, inputOrder []string) {
	for _, inputName := range inputOrder {
		lines := strings.Split(inputs[inputName], "\n")
		inputSourceMap := &InputSourceMap{Lines: make([]*LineMapping, len(lines))}
		for i := range lines {
			inputSourceMap.Lines[i] = &LineMapping{
				LineNumber: i,
				Matches:    make([]*MatchMapping, 0),
			}
		}
		sourceMap.Inputs[inputName] = inputSourceMap
	}
}

// builtinAttrGroupFunctions are the group functions parseGroup applies when
// given as group attributes (compiler.BuiltinAttrGroupFunctions).
var builtinAttrGroupFunctions = compiler.BuiltinAttrGroupFunctions

// nonFunctionGroupAttrs are group attributes parseGroup never treats as a
// custom group function.
var nonFunctionGroupAttrs = compiler.NonFunctionGroupAttrs

func isBuiltinAttrGroupFunc(name string) bool {
	for _, b := range builtinAttrGroupFunctions {
		if name == b {
			return true
		}
	}
	return false
}

// customAttrGroupFunctions returns, sorted, the attributes of group that
// parseGroup applies as custom group functions: with compile-time or
// per-parse group functions present, any other attribute that names a group
// function. The compiler's streamability check can't see these, so
// ParseStream rejects them itself (#130).
func (r *Runtime) customAttrGroupFunctions(group *compiler.CompiledGroup) []string {
	if group.Attributes == nil || !(r.runtimeFunctions != nil && r.runtimeFunctions.Group != nil || r.compileFunctions != nil && r.compileFunctions.Group != nil) {
		return nil
	}
	var names []string
	for attrName := range group.Attributes {
		if nonFunctionGroupAttrs[attrName] {
			continue
		}
		if isBuiltinAttrGroupFunc(attrName) {
			continue
		}
		if _, ok := r.getGroupFunc(attrName); ok {
			names = append(names, attrName)
		}
	}
	sort.Strings(names)
	return names
}
