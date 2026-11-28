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

// hasIgnoreWithTemplateVar reports whether a pattern of group uses
// ignore("<template variable>").
func hasIgnoreWithTemplateVar(group *compiler.CompiledGroup) bool {
	for _, compiledPattern := range group.Patterns {
		for _, variable := range compiledPattern.Variables {
			if variable.Name == "ignore" && variable.IgnoreUsesTemplateVar {
				return true
			}
		}
	}
	return false
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
	if sourceMap != nil {
		initSourceMapInputs(sourceMap, inputs, inputOrder)
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

		ignoreQuirk := hasIgnoreWithTemplateVar(group)
		void := isVoidGroup(group)

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

			resultsMap := getResultsMap(inputName)
			var groupResults interface{}
			if sourceMap != nil {
				groupResults, err = r.parseGroupWithSourceMap(group, processedInputData, vars, inputName, sourceMap, resultsMap)
			} else {
				groupResults, err = r.parseGroup(group, processedInputData, vars)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to parse group %s: %w", group.Name, err)
			}

			// Python TTP quirk: when ignore() uses a template variable, return one empty result
			if ignoreQuirk {
				groupResults = []map[string]interface{}{{}}
			}

			var storedPaths []string
			if groupResults != nil && !void {
				storedPaths = r.storeGroupResults(resultsMap, group, groupResults, vars)
			}
			if sourceMap != nil {
				setGroupResultPaths(sourceMap.Inputs[inputName], group, storedPaths)
			}
		}
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
	if len(r.compiled.Outputs) > 0 {
		var err error
		finalResults, err = r.processOutputFunctions(resultsToFormat, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to process output functions: %w", err)
		}
	}

	if !perInput {
		return finalResults, nil
	}
	// per_input - return as list (already wrapped if formatted, or wrap if not)
	if _, isList := finalResults.([]interface{}); !isList {
		return []interface{}{finalResults}, nil
	}
	return finalResults, nil
}

// storeGroupResults stores a top-level group's results in resultsMap under
// the group's (possibly dynamic) path and returns the paths it stored at.
func (r *Runtime) storeGroupResults(resultsMap map[string]interface{}, group *compiler.CompiledGroup, groupResults interface{}, vars map[string]interface{}) []string {
	if group.Name == "" {
		// Anonymous group: Python TTP stores it as "_anonymous_*" (the *
		// keeps it a list).
		r.storeAtPath(resultsMap, "_anonymous_*", groupResults)
		return []string{"_anonymous_*"}
	}

	isDynamicPath := strings.Contains(group.Name, "{{")

	// Variables used in a dynamic path are removed from the stored matches.
	pathVars := r.pathResolver.ExtractVariablesFromPath(group.Name)
	pathVarSet := make(map[string]bool, len(pathVars))
	for _, v := range pathVars {
		pathVarSet[v] = true
	}
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

	switch v := groupResults.(type) {
	case []map[string]interface{}:
		// Each match may resolve to its own path; group them by path in
		// first-seen order (Go map order is random, #78).
		groupedResults := make(map[string][]map[string]interface{})
		var resolvedOrder []string
		for _, match := range v {
			key := group.Name
			if resolvedName, err := r.pathResolver.ResolvePath(group.Name, match, vars); err == nil && resolvedName != "" {
				key = resolvedName
			}
			if _, seen := groupedResults[key]; !seen {
				resolvedOrder = append(resolvedOrder, key)
			}
			groupedResults[key] = append(groupedResults[key], removePathVars(match))
		}
		paths := make([]string, 0, len(resolvedOrder))
		for _, resolvedName := range resolvedOrder {
			matches := groupedResults[resolvedName]
			// Python TTP: top-level groups - single match = map, multiple matches = list
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
			// Several matches under a path without a * or ** formatter
			// still form a list.
			pathToUse := resolvedName
			if len(matches) > 1 && !strings.HasSuffix(pathToUse, "*") && !strings.HasSuffix(pathToUse, "**") {
				pathToUse += "*"
			}
			r.storeAtPath(resultsMap, pathToUse, valueToStore)
			paths = append(paths, pathToUse)
		}
		return paths
	case map[string]interface{}:
		path := group.Name
		if resolvedName, err := r.pathResolver.ResolvePath(group.Name, v, vars); err == nil && resolvedName != "" {
			path = resolvedName
		}
		r.storeAtPath(resultsMap, path, removePathVars(v))
		return []string{path}
	default:
		resolvedName, err := r.pathResolver.ResolvePath(group.Name, nil, vars)
		if err != nil || resolvedName == "" || resolvedName == group.Name {
			r.storeAtPath(resultsMap, group.Name, groupResults)
			return []string{group.Name}
		}
		// A resolved dynamic path always stores a list.
		if isDynamicPath {
			if valueList, ok := groupResults.([]interface{}); ok {
				r.storeAtPath(resultsMap, resolvedName, valueList)
			} else {
				r.storeAtPath(resultsMap, resolvedName, []interface{}{groupResults})
			}
		} else {
			r.storeAtPath(resultsMap, resolvedName, groupResults)
		}
		return []string{resolvedName}
	}
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
		for _, v := range r.pathResolver.ExtractVariablesFromPath(varsWithName.Name) {
			pathVarSet[v] = true
		}

		resolvedPath, err := r.pathResolver.ResolvePath(varsWithName.Name, nil, resolutionVars)
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

// setGroupResultPaths sets the ResultPath of every match of a top-level group
// that doesn't have a per-instance path yet to the first path the group's
// results were stored at (or the group's name if nothing was stored).
func setGroupResultPaths(inputSourceMap *InputSourceMap, group *compiler.CompiledGroup, storedPaths []string) {
	if inputSourceMap == nil {
		return
	}
	path := group.Name
	if len(storedPaths) > 0 {
		path = storedPaths[0]
	} else if group.Name == "" {
		path = "_anonymous_"
	}
	for _, lineMapping := range inputSourceMap.Lines {
		for _, matchMapping := range lineMapping.Matches {
			// parseGroupWithSourceMap may already have resolved a match to
			// its instance path (e.g. "interfaces[0]"), which is more precise.
			if matchMapping.GroupName == group.Name && !strings.Contains(matchMapping.ResultPath, "[") {
				matchMapping.ResultPath = path
			}
		}
	}
}
