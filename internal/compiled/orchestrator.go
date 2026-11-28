package compiled

import (
	"fmt"
	"sort"
	"strconv"
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
		obs = &smObserver{}
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

			if obs != nil {
				obs.beginInput(inputName, processedInputData)
			}
			r.treeResults = getResultsMap(inputName)
			groupResults, err := r.parseGroup(group, processedInputData, vars)
			r.treeResults = nil
			if err != nil {
				return nil, fmt.Errorf("failed to parse group %s: %w", group.Name, err)
			}
			var recs []*smRecord // source map: the records behind groupResults
			if obs != nil {
				recs = obs.takeNodes()
			}

			if groupResults != nil && !void {
				r.storeGroupResults(getResultsMap(inputName), group, groupResults, vars, recs)
			} else {
				smDrop(recs)
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

// storeGroupResults stores a top-level group's results in resultsMap under
// the group's (possibly dynamic) path. recs (source maps only, else nil) are
// the records behind groupResults; each learns the map it is stored as.
func (r *Runtime) storeGroupResults(resultsMap map[string]interface{}, group *compiler.CompiledGroup, groupResults interface{}, vars map[string]interface{}, recs []*smRecord) {
	if group.Name == "" {
		// Anonymous group: Python TTP stores it as "_anonymous_*" (the *
		// keeps it a list).
		switch v := groupResults.(type) {
		case []map[string]interface{}:
			for k, m := range v {
				if rec := smAt(recs, k); rec != nil {
					rec.final = m
					rec.rel = "_anonymous_[" + strconv.Itoa(k) + "]"
				}
			}
		case map[string]interface{}:
			if rec := smAt(recs, 0); rec != nil {
				rec.final = v
				rec.rel = "_anonymous_"
			}
		default:
			for _, rec := range recs {
				if rec != nil {
					rec.rel = "_anonymous_"
				}
			}
		}
		r.storeAtPath(resultsMap, "_anonymous_*", groupResults)
		return
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
		var groupedRecs map[string][]*smRecord
		if recs != nil {
			groupedRecs = make(map[string][]*smRecord)
		}
		var resolvedOrder []string
		for k, match := range v {
			key := group.Name
			if resolvedName, err := r.pathResolver.ResolvePath(group.Name, match, vars); err == nil && resolvedName != "" {
				key = resolvedName
			}
			if _, seen := groupedResults[key]; !seen {
				resolvedOrder = append(resolvedOrder, key)
			}
			cleaned := removePathVars(match)
			groupedResults[key] = append(groupedResults[key], cleaned)
			if rec := smAt(recs, k); rec != nil {
				rec.final = cleaned
				groupedRecs[key] = append(groupedRecs[key], rec)
			}
		}
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
			if recs != nil {
				setStoredRel(resultsMap, pathKeys(pathToUse), groupedRecs[resolvedName])
			}
		}
	case map[string]interface{}:
		path := group.Name
		if resolvedName, err := r.pathResolver.ResolvePath(group.Name, v, vars); err == nil && resolvedName != "" {
			path = resolvedName
		}
		cleaned := removePathVars(v)
		r.storeAtPath(resultsMap, path, cleaned)
		if rec := smAt(recs, 0); rec != nil {
			rec.final = cleaned
			setStoredRel(resultsMap, pathKeys(path), []*smRecord{rec})
		}
	default:
		resolvedName, err := r.pathResolver.ResolvePath(group.Name, nil, vars)
		for _, rec := range recs {
			if rec == nil {
				continue
			}
			if err != nil || resolvedName == "" {
				rec.rel = trimFormatter(group.Name)
			} else {
				rec.rel = trimFormatter(resolvedName)
			}
		}
		if err != nil || resolvedName == "" || resolvedName == group.Name {
			name := group.Name
			if _, itemized := groupResults.([]interface{}); itemized && strings.HasSuffix(name, "**") {
				// itemize makes the path end in a single "*" (a list of
				// the values), even after "**".
				name = strings.TrimSuffix(name, "*")
			}
			r.storeAtPath(resultsMap, name, groupResults)
			return
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

// builtinAttrGroupFunctions are the group functions parseGroup applies when
// given as group attributes. A group whose attribute order is unknown runs
// them in this order.
var builtinAttrGroupFunctions = []string{"record", "containsall", "contains", "del", "delete", "exclude", "excludeall", "equal", "to_int", "contains_val", "exclude_val", "sformat", "itemize", "set", "expand", "items2dict", "to_ip"}

// nonFunctionGroupAttrs are group attributes parseGroup never treats as a
// custom group function.
var nonFunctionGroupAttrs = map[string]bool{
	"name": true, "input": true, "method": true, "output": true, "default": true,
	"chain": true, "functions": true, "macro": true,
}

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
