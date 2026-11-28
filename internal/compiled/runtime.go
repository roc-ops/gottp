package compiled

import (
	"errors"
	"fmt"
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
	pathResolver      *PathResolver            // active dynamic-path cache; per input during a parse (see usePathResolverFor)
	pathResolvers     map[string]*PathResolver // per-parse dynamic-path caches, keyed by input name
	lastParse         *lastParseResults        // outputs of the most recent parse, shared by every per-parse copy
	matchCollector    *MatchCollector
	yangValidator     *yang.Validator
	validationResults map[string]*yang.ValidationResult // group name -> validation result
	keyFields         map[string][]string               // group name -> key field names (from keys= attribute)
	recordedVars      map[string]interface{}            // global recorded variables (from record() function)
	runtimeLookups    map[string]map[string]interface{} // per-parse runtime lookup tables from ParseOptions
	runtimeFunctions  *RuntimeFunctions                 // per-parse custom function overrides from ParseOptions
	compileFunctions  *RuntimeFunctions                 // compile-time custom function overrides from CompileOptions
	smObs             *smObserver                       // source-map observer for this parse; nil when source maps are off
	// treeResults is the results map of the input being parsed, where a
	// child group with an absolute path outside its top group's saves its
	// records (tree.go); set by the orchestrator before each parseGroup.
	treeResults map[string]interface{}
	// pipelines holds each group's per-record steps for this parse
	// (pipeline.go), built on first use.
	pipelines map[*compiler.CompiledGroup]*groupPipeline
	// shared holds the pipelines built once for the template, shared by
	// every per-parse copy (and, through NewRuntimeWithCache, by every
	// Runtime of the template); nil for a Runtime not made by NewRuntime*.
	shared *PipelineCache
	// parseVars and parseOptVars are the vars passed to this parse (Parse's
	// vars argument and ParseOptions.Vars), which decide whether it can use
	// the shared pipelines.
	parseVars    map[string]interface{}
	parseOptVars map[string]interface{}
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
	return NewRuntimeWithCache(compiled, compileFns, nil)
}

// NewRuntimeWithCache is NewRuntimeWithFunctions with the template's
// PipelineCache, which every Runtime made for the same compiled template and
// compile-time functions may share; nil gives the Runtime its own.
func NewRuntimeWithCache(compiled *compiler.CompiledTemplate, compileFns *RuntimeFunctions, cache *PipelineCache) *Runtime {
	if cache == nil {
		cache = NewPipelineCache()
	}
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
		lastParse:         &lastParseResults{},
		shared:            cache,
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
	validationResults, _ := r.lastParse.get()
	return validationResults
}

// GetKeyFields returns declared key fields for each group (from keys= attribute)
func (r *Runtime) GetKeyFields() map[string][]string {
	_, keyFields := r.lastParse.get()
	return keyFields
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
	YANGModuleSet   *yang.ModuleSet                   // YANG modules for validation
	EnableSourceMap bool                              // Enable source map collection
	Lookups         map[string]map[string]interface{} // Runtime lookup tables
	Vars            map[string]interface{}            // Runtime variables
	Functions       *RuntimeFunctions                 // Custom function overrides
}

// Parse executes the compiled template with given inputs and variables.
//
// This method can be called repeatedly without reset: per-parse state lives
// on a private copy of the Runtime (see beginParse), so nothing one call
// matches or caches is visible to the next.
//
// Thread-safety: concurrent calls on one Runtime do not share parse state.
// Two caveats remain: macros from ParseOptions.Functions.Macro are registered
// on the shared macro registry, and GetValidationResults / GetKeyFields
// report whichever call finished last. Use one Runtime per goroutine if
// either matters.
func (r *Runtime) Parse(inputs map[string]string, vars map[string]interface{}, options *ParseOptions) (interface{}, error) {
	return r.parse(inputs, vars, options, nil)
}

// formResults turns one input's results map (or, for per_template, the
// combined results map) into that input's result, the way Python TTP's
// template.form_results does (ttp/ttp.py:989-1003 in TTP 0.10.1). Results of
// top-level anonymous groups are stored under "_anonymous_"; when present,
// that value becomes the result, and anything else in the map is appended to
// it as one more item. Without anonymous results the map is the result.
//
// Python never has an empty "_anonymous_" list (it only creates the key on a
// match); gottp can, so an empty one counts as absent. When "_anonymous_" is a
// single dict (the implicit group of a template without tags, matched once)
// and other results exist, Python raises AttributeError; gottp merges the
// dict into the rest instead, as it did before.
func formResults(result map[string]interface{}) interface{} {
	anonymous, ok := result["_anonymous_"]
	if !ok {
		return result
	}
	delete(result, "_anonymous_")
	switch a := anonymous.(type) {
	case []interface{}:
		if len(a) == 0 {
			return result
		}
		if len(result) == 0 {
			return a
		}
		datum := make([]interface{}, 0, len(a)+1)
		datum = append(datum, a...)
		return append(datum, result)
	case map[string]interface{}:
		if len(result) == 0 {
			return a
		}
		for k, v := range a {
			result[k] = v
		}
		return result
	case nil:
		return result
	default:
		if len(result) == 0 {
			return anonymous
		}
		return result
	}
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

	// --- Setup: shared with Parse ---

	// Per-parse state lives on a private copy of the Runtime, so repeated and
	// concurrent calls on one Runtime never share it (#83).
	run := r.beginParse()
	defer r.finishParse(run)
	r = run

	vars = r.applyParseOptions(vars, options)

	// Custom group functions given as group attributes are known only now
	// (compile-time or per-parse functions), as are the parse's vars a chain
	// can name; streaming does not apply functions outside the allowlist.
	var reasons []string
	for _, g := range r.compiled.Groups {
		if g.IsNested {
			continue
		}
		for _, attr := range r.customAttrGroupFunctions(g) {
			reasons = append(reasons, fmt.Sprintf("group %q uses the custom group function attribute %s=, which streaming does not apply", g.Name, attr))
		}
		// A chain= naming a template variable was checked at compile time
		// against the template's <vars>; the parse's vars can override it.
		reasons = append(reasons, compiler.NonStreamableChainReasons(g, vars)...)
	}
	if len(reasons) > 0 {
		return wrapNotStreamableError(reasons)
	}

	inputOrder := r.prepareInputs(inputs)

	// --- Drive parseGroupStream for each top-level group ---

	for _, g := range r.compiled.Groups {
		// Skip nested groups — they're processed within their parent group.
		// (Streamable templates have none, but guard for safety.)
		if g.IsNested {
			continue
		}

		r.collectGroupKeys(g)

		for _, inputName := range groupInputNames(g, inputOrder) {
			inputData, ok := inputs[inputName]
			if !ok {
				continue
			}

			r.usePathResolverFor(inputName)
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

// ParseWithSourceMap executes the compiled template and returns both data and
// source map. With source maps off it is Parse. With them on it runs the same
// pipeline as Parse and also records the source map, so the data, errors and
// key fields are the same either way.
func (r *Runtime) ParseWithSourceMap(inputs map[string]string, vars map[string]interface{}, options *ParseOptions) (interface{}, *SourceMap, error) {
	if options == nil || !options.EnableSourceMap {
		data, err := r.Parse(inputs, vars, options)
		return data, nil, err
	}
	sourceMap := &SourceMap{Inputs: make(map[string]*InputSourceMap)}
	data, err := r.parse(inputs, vars, options, sourceMap)
	if err != nil {
		return nil, nil, err
	}
	return data, sourceMap, nil
}

// patternMatch is one regex match of a group's pattern, as the record
// assembler takes it.
type patternMatch struct {
	group      int // the group's id in its tree (tree.go); 0 for a group without children
	patternIdx int
	spanStart  int
	spanEnd    int
	result     map[string]interface{}
	sm         *smMatchInfo // submatch indices, for the source map; nil when it is off
}

// parseGroup parses a top group: one without children with parseFlat, one
// with children with parseTree (tree.go). Either way each record runs the
// group's pipeline (pipeline.go) when it is saved; YANG validation then runs
// once on the group's result.
func (r *Runtime) parseGroup(group *compiler.CompiledGroup, inputData string, vars map[string]interface{}) (interface{}, error) {
	var result interface{}
	var jobs []yangJob
	var err error
	if len(group.Groups) > 0 {
		result, jobs, err = r.parseTree(group, inputData, vars)
	} else {
		result, jobs, err = r.parseFlat(group, inputData, vars)
	}
	if err != nil {
		return nil, err
	}
	r.validateYANG(jobs)
	return result, nil
}

// yangJob is one YANG validation of a group's records: data validated
// against path and stored in the validation results under key.
type yangJob struct {
	key, path, name string
	data            interface{}
}

// validateYANG runs the YANG validations of one top group's result.
func (r *Runtime) validateYANG(jobs []yangJob) {
	if r.yangValidator == nil {
		return
	}
	for _, j := range jobs {
		r.validationResults[j.key] = r.yangValidator.Validate(j.data, j.path, j.name)
	}
}

// parseFlat parses a top group without children, assembling its records
// with the TTP port in assembler.go and running each through the group's
// pipeline as it is saved.
func (r *Runtime) parseFlat(group *compiler.CompiledGroup, inputData string, vars map[string]interface{}) (interface{}, []yangJob, error) {
	// obs records the source map; nil (and every hook skipped) when source
	// maps are off.
	obs := r.smObs
	if obs != nil {
		obs.lastNodes = nil
	}
	if len(group.Patterns) == 0 {
		return nil, nil, nil
	}

	in := newInputLines(inputData)
	var allMatches []patternMatch
	prof := group.Profile()
	for patternIdx := range group.Patterns {
		if !prof.Matches(patternIdx) {
			continue // a line without variables: Python TTP skips it
		}
		allMatches = r.matchPattern(group, patternIdx, 0, in, 0, len(inputData)+1, vars, obs != nil, allMatches)
	}

	// Sort matches by position; at one position, in pattern order.
	sort.Slice(allMatches, func(i, j int) bool {
		if allMatches[i].spanStart == allMatches[j].spanStart {
			return allMatches[i].patternIdx < allMatches[j].patternIdx
		}
		return allMatches[i].spanStart < allMatches[j].spanStart
	})

	pl := r.pipelineFor(group, vars)
	ic := &itemizeCtx{root: r.treeResults, dyn: r.pathResolver.dynPathCache}
	var records []map[string]interface{}
	var items []interface{}
	var used [][]int // source map: every assembled record's matches
	var kept []int   // source map: the indices of the records kept
	n := 0
	asm := newAssembler(newAsmGroup(group), obs != nil, func(rec asmRecord) error {
		r.pathResolver.UpdateCache(rec.result)
		r.matchCollector.Clear()
		idx := n
		n++
		if obs != nil {
			used = append(used, rec.used)
		}
		value, keep, itemized, err := r.runPipeline(pl, rec.result, vars, false, ic)
		if err != nil {
			return err
		}
		if !keep || !r.topPathFormable(pl, value, vars) {
			return nil
		}
		if obs != nil {
			kept = append(kept, idx)
		}
		if itemized {
			items = append(items, value)
			return nil
		}
		if m, ok := value.(map[string]interface{}); ok {
			records = append(records, m)
		}
		return nil
	})
	for i, m := range allMatches {
		if err := asm.add(asmMatch{idx: i, pattern: m.patternIdx, start: m.spanStart, end: m.spanEnd, result: m.result}); err != nil {
			return nil, nil, err
		}
	}
	if err := asm.finish(len(inputData)); err != nil {
		return nil, nil, err
	}

	var recs []*smRecord
	if obs != nil {
		recs = smKeep(obs.recordMatches(group, allMatches, used), kept)
	}
	if pl.itemizes {
		// The group's result is the list of itemized values (TTP's itemize
		// makes the record's path end in "*").
		if obs != nil {
			obs.lastNodes = []*smRecord{obs.smCollapse(group, recs)}
		}
		if len(items) == 0 {
			return nil, nil, nil
		}
		return items, nil, nil
	}
	return r.finishFlat(group, pl, records, recs)
}

// topPathFormable is step 4 of the pipeline for a top group with a dynamic
// path: a record whose path variables have no value in the record, the
// dynamic-path cache, the recorded vars or the vars can't be saved, so TTP
// drops it.
func (r *Runtime) topPathFormable(pl *groupPipeline, value interface{}, vars map[string]interface{}) bool {
	if len(pl.pathVars) == 0 {
		return true
	}
	m, _ := value.(map[string]interface{})
	for _, name := range pl.pathVars {
		if _, ok := m[name]; ok {
			continue
		}
		if _, ok := r.pathResolver.dynPathCache[name]; ok {
			continue
		}
		if _, ok := r.recordedVars[name]; ok {
			continue
		}
		if _, ok := vars[name]; ok {
			continue
		}
		return false
	}
	return true
}

// inputLines is one input's data split into lines, with each line's offset
// (and a sentinel).
type inputLines struct {
	data    string
	lines   []string
	offsets []int
}

func newInputLines(data string) *inputLines {
	lines := strings.Split(data, "\n")
	offsets := make([]int, len(lines)+1)
	offset := 0
	for i, line := range lines {
		offsets[i] = offset
		offset += len(line) + 1
	}
	offsets[len(lines)] = offset
	return &inputLines{data: data, lines: lines, offsets: offsets}
}

// matchPattern appends the matches of one pattern of group (whose tree id is
// groupID) in the region [lo, hi) of the input to dst: a line-anchored
// pattern matches the lines that start in the region, any other pattern the
// region's text. As in TTP's check_matches, a match that a condition
// rejects is dropped unless it is a start pattern's (its result is then
// nil), and a match that captures nothing is kept. With sm, each match keeps
// its submatch indices for the source map.
func (r *Runtime) matchPattern(group *compiler.CompiledGroup, patternIdx, groupID int, in *inputLines, lo, hi int, vars map[string]interface{}, sm bool, dst []patternMatch) []patternMatch {
	compiledPattern := group.Patterns[patternIdx]
	prof := group.Profile()
	startsRecord := prof.StartsRecord(patternIdx)
	if compiledPattern.HasAnchors {
		matchesBlank := prof.MatchesBlank(patternIdx)
		first := sort.SearchInts(in.offsets[:len(in.lines)], lo)
		for lineIdx := first; lineIdx < len(in.lines) && in.offsets[lineIdx] < hi; lineIdx++ {
			// Trim \r and trailing spaces, but keep leading spaces: the
			// regex comes from the template line, indentation included.
			line := strings.TrimRight(in.lines[lineIdx], "\r \t")
			trimmedLine := strings.TrimSpace(line)
			// Skip empty lines unless this pattern is a start or end
			// indicator line, which may match a blank line.
			if trimmedLine == "" && !matchesBlank {
				continue
			}
			matchLine := line
			if trimmedLine == "" {
				matchLine = ""
			}
			var match []string
			var info *smMatchInfo
			if sm {
				if loc := compiledPattern.Regex.FindStringSubmatchIndex(matchLine); loc != nil {
					match = submatchStrings(matchLine, loc)
					info = &smMatchInfo{idx: loc, base: in.offsets[lineIdx]}
				}
			} else {
				match = compiledPattern.Regex.FindStringSubmatch(matchLine)
			}
			if match == nil {
				continue
			}
			result := r.extractMatchTTP(match, compiledPattern, vars)
			if result == nil && !startsRecord {
				continue
			}
			dst = append(dst, patternMatch{
				group:      groupID,
				patternIdx: patternIdx,
				spanStart:  in.offsets[lineIdx],
				spanEnd:    in.offsets[lineIdx] + len(line),
				result:     result,
				sm:         info,
			})
		}
		return dst
	}

	if lo < 0 {
		lo = 0
	}
	if hi > len(in.data) {
		hi = len(in.data)
	}
	if lo >= hi {
		return dst
	}
	region := in.data[lo:hi]
	for _, indices := range compiledPattern.Regex.FindAllStringSubmatchIndex(region, -1) {
		if len(indices) < 2 {
			continue
		}
		matchGroups := make([]string, len(indices)/2)
		for i := 0; i < len(indices); i += 2 {
			if indices[i] >= 0 && indices[i+1] >= 0 {
				matchGroups[i/2] = region[indices[i]:indices[i+1]]
			}
		}
		result := r.extractMatchTTP(matchGroups, compiledPattern, vars)
		if result == nil && !startsRecord {
			continue
		}
		var info *smMatchInfo
		if sm {
			info = &smMatchInfo{idx: indices, base: lo}
		}
		dst = append(dst, patternMatch{
			group:      groupID,
			patternIdx: patternIdx,
			spanStart:  lo + indices[0],
			spanEnd:    lo + indices[1],
			result:     result,
			sm:         info,
		})
	}
	return dst
}

// finishFlat forms the result of a top group without children from its kept
// records: the record a group that kept nothing gets (from default= or its
// start line's default() values), the group's YANG validation, and the
// result's shape. recs (source maps only) are the records' smRecords;
// finishFlat leaves the ones behind its result in r.smObs.lastNodes.
func (r *Runtime) finishFlat(group *compiler.CompiledGroup, pl *groupPipeline, records []map[string]interface{}, recs []*smRecord) (interface{}, []yangJob, error) {
	obs := r.smObs

	// A group that kept no record, with a default= string, gives a record
	// of every variable set to it (Python TTP's default start).
	if len(records) == 0 && pl.hasDefaultStr && len(pl.defaultVars) > 0 {
		m := make(map[string]interface{}, len(pl.defaultVars))
		for _, name := range pl.defaultVars {
			m[name] = pl.defaultString
		}
		records = append(records, m)
		if obs != nil {
			recs = append(recs, obs.newRecord(group))
		}
	}

	var jobs []yangJob
	if r.yangValidator != nil && records != nil {
		if yangPath, ok := group.Attributes["yang"]; ok && yangPath != "" {
			jobs = append(jobs, yangJob{key: group.Name, path: yangPath, name: group.Name, data: records})
		}
	}

	// A group that kept no record, with default() on its start line, gives
	// a record of those defaults (TTP's has_start_re_default).
	if len(records) == 0 {
		if defaultMatch := startLineDefaultsRecord(group); defaultMatch != nil {
			records = append(records, defaultMatch)
			if obs != nil {
				recs = append(recs, obs.newRecord(group))
			}
		}
	}

	// Records go to the group's path one after another, as TTP's
	// save_curelements puts them there: a first empty record is filled by
	// the next one, an empty record after the first becomes a list item,
	// and after that empty records are dropped (topSaver).
	saver := newTopSaver(group.Name)
	filtered := make([]map[string]interface{}, 0, len(records))
	var kept []int
	for mi, match := range records {
		switch saver.add(len(match) == 0) {
		case topAppend:
			filtered = append(filtered, match)
			if obs != nil {
				kept = append(kept, mi)
			}
		case topMerge:
			for k, v := range match {
				filtered[0][k] = v
			}
			if obs != nil {
				if rec := smAt(recs, mi); rec != nil {
					rec.alias = smAt(recs, kept[0])
				}
			}
		}
	}
	if obs != nil {
		obs.lastNodes = smKeep(recs, kept)
	}

	if group.Method != "table" && len(filtered) == 1 {
		return filtered[0], jobs, nil
	}
	return filtered, jobs, nil
}

// startLineDefaultsRecord returns the record Parse gives for a group that
// matched nothing when its start line (first pattern) has a variable with
// default(): every variable of the group with default(), set to its default
// value (the first pattern's value wins for a name used on several lines).
// It returns nil when the start line has no default().
func startLineDefaultsRecord(group *compiler.CompiledGroup) map[string]interface{} {
	if len(group.Patterns) == 0 {
		return nil
	}
	hasStartDefault := false
	defaultValues := make(map[string]interface{})
	for pi, cp := range group.Patterns {
		for varName, variable := range cp.Variables {
			if pattern.IsIndicatorVar(varName) {
				continue
			}
			for _, funcStr := range variable.Functions {
				if !strings.HasPrefix(funcStr, "default(") {
					continue
				}
				if pi == 0 {
					hasStartDefault = true
				}
				// Extract default value from default("value")
				start := strings.Index(funcStr, "(")
				end := strings.LastIndex(funcStr, ")")
				if start >= 0 && end > start {
					defaultVal := strings.TrimSpace(funcStr[start+1 : end])
					defaultVal = strings.Trim(defaultVal, `"'`)
					// The first pattern takes precedence.
					if _, exists := defaultValues[varName]; !exists {
						defaultValues[varName] = defaultVal
					}
				}
				break
			}
		}
		if pi == 0 && !hasStartDefault {
			return nil
		}
	}
	if len(defaultValues) == 0 {
		return nil
	}
	return defaultValues
}

// parseGroupStream is the streaming variant of parseGroup. It matches the
// input a line at a time and feeds the matches to the same record assembler
// parseGroup uses for a group without children (assembler.go), invoking fn
// for each finished record instead of collecting them. The assembler holds
// only the current line's matches and the current record, so memory stays
// bounded however large the input is.
//
// Caller must verify group.Streamable == true. parseGroupStream does not
// re-check (entry gate is in Runtime.ParseStream).
//
// Streamability guarantees simplify the body:
//   - No nested groups → no recursion needed
//   - No aggregating group functions (itemize, expand, ...)
//   - Group chain functions are from the per-record allowlist
//   - Every pattern is line-anchored, so a line's matches share a position
func (r *Runtime) parseGroupStream(
	group *compiler.CompiledGroup,
	inputData string,
	vars map[string]interface{},
	fn func(record map[string]interface{}, srcRange [2]int, groupPath string) error,
) error {
	if len(group.Patterns) == 0 {
		return nil
	}
	prof := group.Profile()

	// kept is set once a record survives the chain and macro. If none does,
	// Parse gives a record of start-line defaults (see the end of this
	// function).
	kept := false
	pl := r.pipelineFor(group, vars)

	asm := newAssembler(newAsmGroup(group), false, func(rec asmRecord) error {
		r.pathResolver.UpdateCache(rec.result)
		r.matchCollector.Clear()
		// The same per-record steps Parse runs (pipeline.go). The
		// streamability gate keeps out itemize, whose result is not a
		// stream of records.
		value, keep, itemized, err := r.runPipeline(pl, rec.result, vars, false, nil)
		if err != nil {
			return err
		}
		if !keep {
			return nil // dropped by a group function
		}
		if itemized {
			return fmt.Errorf("parseGroupStream: group %q itemizes its records; streamability classifier bug", group.Name)
		}
		filtered, _ := value.(map[string]interface{})
		// The record survived the chain and macro, so Parse would not make
		// its record of start-line defaults, even if the record is empty.
		kept = true
		if len(filtered) == 0 {
			// Parse keeps an empty record only where TTP's
			// save_curelements does (topSaver); a stream has no use for one.
			return nil
		}
		return fn(filtered, rec.srcRange, group.NormalizedPath)
	})

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

	// Line-major, pattern-major iteration: the matches of a line share its
	// start offset, which is the position the assembler resolves them at, and
	// come in pattern order, as in parseGroup's sorted matches.
	//
	// We walk the input as a single string with manual newline scanning
	// (instead of strings.Split) to avoid materializing a slice of ~256K
	// string headers (~4 MB for the phy fixture).
	totalLen := len(inputData)
	matchIdx := 0

	pos := 0
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

		// Trim \r and trailing spaces, as parseGroup does.
		line := strings.TrimRight(rawLine, "\r \t")
		trimmedLine := strings.TrimSpace(line)

		for patternIdx, compiledPattern := range group.Patterns {
			if !prof.Matches(patternIdx) {
				continue // a line without variables: Python TTP skips it
			}

			// Skip empty lines unless this pattern is a start or end
			// indicator line, which may match a blank line.
			matchesBlank := prof.MatchesBlank(patternIdx)
			if trimmedLine == "" && !matchesBlank {
				continue
			}

			matchLine := line
			if trimmedLine == "" && matchesBlank {
				matchLine = ""
			}

			match := compiledPattern.Regex.FindStringSubmatch(matchLine)
			if match == nil {
				continue
			}

			result := r.extractMatchTTP(match, compiledPattern, vars)
			if result == nil && !prof.StartsRecord(patternIdx) {
				continue
			}
			if err := asm.add(asmMatch{
				idx:     matchIdx,
				pattern: patternIdx,
				start:   lineStartOffset,
				end:     lineStartOffset + len(line),
				result:  result,
			}); err != nil {
				return err
			}
			matchIdx++
		}
	}

	// The last record ends at the end of the input.
	if err := asm.finish(totalLen); err != nil {
		return err
	}

	// Parse gives a record of defaults when nothing survived and the start
	// line has default(). That record covers no input text, so its srcRange
	// is the empty range [0, 0] at the start of the input (the same range
	// whether or not the input is empty); groupPath is the group's path, as
	// for any record. Like parseGroup, it is not passed through the chain or
	// the macro.
	if !kept {
		if defaults := startLineDefaultsRecord(group); defaults != nil {
			if err := fn(defaults, [2]int{0, 0}, group.NormalizedPath); err != nil {
				return err
			}
		}
	}

	return nil
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

// extractMatchTTP extracts a match's values with Python TTP's
// check_matches semantics: a condition that rejects any variable rejects the
// whole match (nil), and a joinmatches variable keeps its value as is, so
// the assembler's join can tell a list value from a single one.
func (r *Runtime) extractMatchTTP(match []string, compiledPattern *pattern.CompiledPattern, vars map[string]interface{}) map[string]interface{} {
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
		if pattern.IsNonCapturingVar(varName) {
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
					return nil
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
		// A joinmatches value is stored as is; the assembler joins it.
		result[variable.Name] = processedValue
		varIndex++
	}

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
				arg = resolveArgument(arg, wasQuoted, vars)
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
		arg = resolveArgument(arg, wasQuoted, vars)
		args = append(args, arg)
	}

	return args
}

// resolveArgument resolves one parsed argument. A quoted argument has already
// lost its quotes, so quote characters left in it are content (strip('"')
// passes a double quote): only a vars lookup applies to it.
func resolveArgument(arg string, wasQuoted bool, vars map[string]interface{}) string {
	if !wasQuoted {
		return resolveVariable(arg, vars)
	}
	if val, ok := vars[arg]; ok {
		return fmt.Sprintf("%v", val)
	}
	return arg
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

// macroErrorIsNotFound reports whether err represents an unregistered macro
// name - matching Python TTP's behavior of silently continuing with the
// original data when a referenced macro doesn't exist - as opposed to a
// real failure while running a macro that WAS found (a Starlark runtime
// error, a compile error, etc). Only the former should be swallowed; the
// latter was previously swallowed too, which meant a macro could fail
// outright (e.g. a Starlark `del` statement erroring) with no error
// surfaced and no indication the macro didn't run.
// See https://github.com/roc-ops/gottp/issues/26.
func macroErrorIsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
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

// processOutputFunctions processes output functions for results.
//
// formatted reports whether the final value is a formatter's output (the
// last output that changed the results had a format). The caller uses it
// to keep the per_input list wrap for formatter output (#98) without
// wrapping a value an output macro or function returned.
func (r *Runtime) processOutputFunctions(results interface{}, vars map[string]interface{}) (result interface{}, formatted bool, err error) {
	result = results

	// Process each output configuration
	for _, outputConfig := range r.compiled.Outputs {
		// Python TTP runs an output's functions (the functions attribute
		// and the macro attribute, in attribute order) before its
		// formatter (ttp/ttp.py _outputter_class.run).
		if outputConfig.Macro != "" && outputConfig.MacroBeforeFunctions {
			if result, err = r.runOutputMacroAttr(outputConfig.Macro, result); err != nil {
				return nil, false, err
			}
			formatted = false
		}
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
					return nil, false, fmt.Errorf("failed to parse output function %s: %w", funcStr, err)
				}

				// Get function from registry (checks runtime overrides first)
				fn, ok := r.getOutputFunc(funcName)
				if !ok {
					if funcName == "macro" {
						// gottp extension: macro('a', 'b') runs each macro
						// in order (Python TTP 0.10.1 raises TypeError).
						for _, name := range args {
							if result, err = r.runOutputMacro(strings.TrimSpace(name), result); err != nil {
								return nil, false, err
							}
						}
						formatted = false
					}
					// Other unknown functions are skipped (Python TTP logs
					// an error and skips them).
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
					return nil, false, fmt.Errorf("output function %s failed: %w", funcName, err)
				}
				result = processed
				formatted = false
			}
		}
		if outputConfig.Macro != "" && !outputConfig.MacroBeforeFunctions {
			if result, err = r.runOutputMacroAttr(outputConfig.Macro, result); err != nil {
				return nil, false, err
			}
			formatted = false
		}

		// Apply format conversion if format is specified
		if outputConfig.Format != "" && outputConfig.Format != "raw" {
			// Get format function from registry (checks runtime overrides first)
			formatFn, ok := r.getOutputFunc(outputConfig.Format)
			if !ok {
				return nil, false, fmt.Errorf("unknown output format: %s", outputConfig.Format)
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
						return nil, false, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
					}
					result = formatted
				} else {
					// For other formatters, format each input's results separately
					// BUT: Python TTP's table formatter takes the whole results
					// list and flattens each input's result into one table. An
					// input's result is a dict, or a list of matches when the
					// template has a top-level anonymous group (#62).
					if outputConfig.Format == "table" {
						allTableable := true
						for _, item := range resultList {
							switch item.(type) {
							case map[string]interface{}, []interface{}:
							default:
								allTableable = false
							}
							if !allTableable {
								break
							}
						}
						if allTableable {
							// Anonymous group results - combine into single table
							formatted, err := formatFn(resultList, nil, kwargs)
							if err != nil {
								return nil, false, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
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
									return nil, false, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
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
								return nil, false, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
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
					return nil, false, fmt.Errorf("output format %s failed: %w", outputConfig.Format, err)
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
					fmt.Printf("%v", result)
				}
				// Return original parsed data structure (not formatted)
				result = originalResult
			} else {
				formatted = true
			}
		}
	}

	return result, formatted, nil
}

// runOutputMacroAttr runs an output's macro attribute. A comma-separated
// list runs each macro in order on the result of the one before. This is a
// gottp extension: Python TTP 0.10.1 turns macro="a, b" into the call
// macro(data, "a", "b"), which raises TypeError.
func (r *Runtime) runOutputMacroAttr(attr string, data interface{}) (interface{}, error) {
	for _, name := range strings.Split(attr, ",") {
		var err error
		if data, err = r.runOutputMacro(strings.TrimSpace(name), data); err != nil {
			return nil, err
		}
	}
	return data, nil
}

// runOutputMacro runs macro name on the whole results structure, like
// Python TTP's output macro() function (ttp/output/macro.py): the macro
// gets the results (per_input: the list of per-input results;
// per_template: the combined result), and its return value replaces them
// unless it is None. False and True replace the results too. A macro that
// is not defined leaves the results unchanged. A Starlark macro that
// mutates its argument in place and returns None keeps the mutation, as it
// would in Python.
//
// A native Go macro takes a map, so it can run on per_template results but
// not on the per_input list; use a custom output function for that.
func (r *Runtime) runOutputMacro(name string, data interface{}) (interface{}, error) {
	if name == "" {
		return data, nil
	}
	if r.macroRegistry.HasGoMacro(name) {
		res, err := r.macroRegistry.ExecuteMacro("go", name, data, nil)
		if err != nil {
			return nil, fmt.Errorf("output macro %s failed: %w", name, err)
		}
		if m, ok := res.(map[string]interface{}); ok && m == nil {
			return data, nil
		}
		return res, nil
	}

	macroLang := "starlark"
	for _, m := range r.compiled.Macros {
		if m.Language != "" {
			macroLang = m.Language
			break
		}
	}

	if macroLang == "starlark" {
		engine := r.macroRegistry.GetStarlarkEngine()
		dataVal := engine.GoToStarlark(data)
		res, err := engine.ExecuteMacroStarlark(name, dataVal, nil)
		if err != nil {
			if macroErrorIsNotFound(err) {
				return data, nil
			}
			return nil, fmt.Errorf("output macro %s failed: %w", name, err)
		}
		if res == starlark.None {
			// The macro may have changed its argument in place.
			return engine.StarlarkToGo(dataVal), nil
		}
		return engine.StarlarkToGo(res), nil
	}

	res, err := r.macroRegistry.ExecuteMacro(macroLang, name, data, nil)
	if err != nil {
		if macroErrorIsNotFound(err) {
			return data, nil
		}
		return nil, fmt.Errorf("output macro %s failed: %w", name, err)
	}
	if res == nil {
		return data, nil
	}
	return res, nil
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
