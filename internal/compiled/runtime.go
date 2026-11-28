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
	pathResolver      *PathResolver                     // active dynamic-path cache; per input during a parse (see usePathResolverFor)
	pathResolvers     map[string]*PathResolver          // per-parse dynamic-path caches, keyed by input name
	lastParse         *lastParseResults                 // outputs of the most recent parse, shared by every per-parse copy
	matchCollector    *MatchCollector
	yangValidator     *yang.Validator
	validationResults map[string]*yang.ValidationResult // group name -> validation result
	keyFields         map[string][]string               // group name -> key field names (from keys= attribute)
	recordedVars      map[string]interface{}            // global recorded variables (from record() function)
	runtimeLookups    map[string]map[string]interface{} // per-parse runtime lookup tables from ParseOptions
	runtimeFunctions  *RuntimeFunctions                 // per-parse custom function overrides from ParseOptions
	compileFunctions  *RuntimeFunctions                 // compile-time custom function overrides from CompileOptions
	smObs             *smObserver                       // source-map observer for this parse; nil when source maps are off
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
		lastParse:         &lastParseResults{},
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

// matchIndexRange is the half-open range [lo, hi) of allMatches indices that
// belong to one parent match. Each parent's indices are always contiguous and
// written exactly once (parentIdx is state.recordCount, incremented per flush),
// so storing the bounds is equivalent to storing the expanded index list -- and
// avoids allocating one []int per parent match, which on large inputs was
// O(len(allMatches)) ints for a list whose only readers want its ends.
type matchIndexRange struct {
	lo int
	hi int
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
	parentMatchToAllMatches    map[int]matchIndexRange
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

	// trackUsed records which matches each record consumes, for the source
	// map. curUsed holds the allMatches indices merged into currentMatch.
	// Only parseGroup with a source-map observer sets it.
	trackUsed    bool
	curUsed      []int
	usedByRecord [][]int // per flushed record, the allMatches indices it consumed
}

// emit flushes record and, when tracking, notes the matches it consumed.
func (s *mergeState) emit(record map[string]interface{}, srcRange [2]int) error {
	if s.trackUsed {
		s.usedByRecord = append(s.usedByRecord, s.takeUsed())
	}
	return s.flush(record, srcRange)
}

// useMatch notes that allMatches[idx] went into the current record. With
// newRecord, the current record has just been started by this match.
func (s *mergeState) useMatch(idx int, newRecord bool) {
	if !s.trackUsed {
		return
	}
	if newRecord {
		s.curUsed = s.curUsed[:0]
	} else if n := len(s.curUsed); n > 0 && s.curUsed[n-1] == idx {
		return
	}
	s.curUsed = append(s.curUsed, idx)
}

// takeUsed returns a copy of the current record's match indices.
func (s *mergeState) takeUsed() []int {
	return append([]int(nil), s.curUsed...)
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
		parentMatchToAllMatches:    make(map[int]matchIndexRange),
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
	prof := group.Profile()

	isStartPattern := startPatterns[match.patternIdx]
	isEndPattern := endPatterns[match.patternIdx]

	// Declare shouldMerge here so it's available in both branches
	shouldMerge := false

	if isStartPattern {
		// Start pattern - save previous match and start new one
		shouldStartNewMatch := true
		if hasLineIndicator && match.patternIdx == 0 && state.currentMatch != nil {
			shouldStartNewMatch = false
		} else if hasAnyStartIndicator && state.currentMatch != nil && !state.currentMatchHasEnd {
			// (A record that an end pattern already closed never absorbs a
			// start match: the match starts a new record, as in Python TTP.)
			hasEndPatterns := len(endPatterns) > 0
			isStartPatternVar := prof.IsIndicatedStart(match.patternIdx)
			if isStartPatternVar && (hasEndPatterns || hasAnyStartIndicator) {
				hasPatternsToMergeOnSameLine := false
				for i := matchIdx + 1; i < len(allMatches); i++ {
					nextMatch := allMatches[i]
					if nextMatch.spanStart != match.spanStart {
						break
					}
					nextIsEnd := endPatterns[nextMatch.patternIdx]
					nextIsStartPatternVar := prof.IsStartEmpty(nextMatch.patternIdx)
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
			} else if match.patternIdx == state.currentStartPatternIdx {
				count := state.patternMatchCount[match.patternIdx]
				isStartPatternVar := prof.IsStartEmpty(match.patternIdx)
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
			} else if hasEndPatterns && !state.currentMatchHasEnd {
				shouldStartNewMatch = false
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
				state.useMatch(matchIdx, false)
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
				hasNonSpecialVars := hasNonIndicatorKey(state.currentMatch)

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
					if err := state.emit(state.currentMatch, srcRange); err != nil {
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
			state.useMatch(matchIdx, true)
			return nil // continue
		}

		if shouldStartNewMatch && !shouldMerge {
			shouldFinalizePrevious := true
			hasEndPatterns := len(endPatterns) > 0
			isStartPatternVar := prof.IsIndicatedStart(match.patternIdx)
			isRepeatStartMatch := isStartPatternVar && match.patternIdx == state.currentStartPatternIdx
			hasPatternsToMergeOnSameLine := false
			if isStartPatternVar && hasEndPatterns {
				for i := matchIdx + 1; i < len(allMatches); i++ {
					nextMatch := allMatches[i]
					if nextMatch.spanStart != match.spanStart {
						break
					}
					nextIsEnd := endPatterns[nextMatch.patternIdx]
					nextIsStartPatternVar := prof.IsStartEmpty(nextMatch.patternIdx)
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
					hasNonSpecialVars := hasNonIndicatorKey(state.currentMatch)
					shouldSave := hasNonSpecialVars || isRepeatStartMatch
					if shouldSave {
						matchCopy := make(map[string]interface{})
						for k, v := range state.currentMatch {
							matchCopy[k] = v
						}
						parentIdx := state.recordCount
						srcRange := [2]int{state.currentStartPos, match.spanStart}
						if err := state.emit(matchCopy, srcRange); err != nil {
							return err
						}
						state.recordCount++
						if state.currentParentMatchStartIdx >= 0 && !state.skipParentBookkeeping {
							if hi := matchIdx + 1; state.currentParentMatchStartIdx < hi {
								state.parentMatchToAllMatches[parentIdx] = matchIndexRange{lo: state.currentParentMatchStartIdx, hi: hi}
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
				state.useMatch(matchIdx, true)
			} else {
				shouldMerge = true
				state.currentStartPos = match.spanStart
				state.currentStartLineIdx = match.lineIdx
				state.patternMatchCount[match.patternIdx]++
			}

			if isEndPattern {
				state.currentMatchHasEnd = true
				srcRange := [2]int{state.currentStartPos, match.spanEnd}
				if err := state.emit(state.currentMatch, srcRange); err != nil {
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
		currentMatchStartedByStart := prof.IsIndicatedStart(state.currentStartPatternIdx)
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
			hasNonSpecialVars := hasNonIndicatorKey(state.currentMatch)
			if hasNonSpecialVars {
				matchCopy := make(map[string]interface{})
				for k, v := range state.currentMatch {
					matchCopy[k] = v
				}
				parentIdx := state.recordCount
				srcRange := [2]int{state.currentStartPos, match.spanStart}
				if err := state.emit(matchCopy, srcRange); err != nil {
					return err
				}
				state.recordCount++
				if state.currentParentMatchStartIdx >= 0 && !state.skipParentBookkeeping {
					if state.currentParentMatchStartIdx < matchIdx {
						state.parentMatchToAllMatches[parentIdx] = matchIndexRange{lo: state.currentParentMatchStartIdx, hi: matchIdx}
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
			state.useMatch(matchIdx, true)
			shouldMerge = true
		}

		if shouldMerge {
			state.useMatch(matchIdx, false)
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
					if pattern.IsIndicatorVar(k) {
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
				state.useMatch(matchIdx, true)
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
		state.useMatch(matchIdx, false)
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
				if pattern.IsIndicatorVar(k) && k != pattern.IndicatorLine {
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
	// obs records the source map; nil (and every hook skipped) when source
	// maps are off. recs follows mergedMatches: recs[i] is record i's node.
	obs := r.smObs
	var recs []*smRecord
	if obs != nil {
		obs.lastNodes = nil
		clear(obs.submatches)
	}

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

	ind := indicatorsFor(group)

	for patternIdx, compiledPattern := range group.Patterns {
		if !ind.profile.Matches(patternIdx) {
			continue // a line without variables: Python TTP skips it
		}

		if compiledPattern.HasAnchors {
			// Match line by line
			for lineIdx, line := range lines {
				// Trim \r and trailing spaces, but preserve leading spaces for regex matching
				// The regex was generated from the template which may have leading spaces
				line = strings.TrimRight(line, "\r \t")
				trimmedLine := strings.TrimSpace(line)

				// Skip empty lines unless this pattern is a start or end
				// indicator line, which may match a blank line.
				matchesBlank := ind.profile.MatchesBlank(patternIdx)
				if trimmedLine == "" && !matchesBlank {
					continue
				}

				// Use the line (with leading spaces but trailing spaces trimmed) for matching
				// For _start_ or _end_ patterns on empty lines, use empty string
				matchLine := line
				if trimmedLine == "" && matchesBlank {
					matchLine = ""
				}

				var match []string
				var smInfo *smMatchInfo
				if obs != nil {
					if loc := compiledPattern.Regex.FindStringSubmatchIndex(matchLine); loc != nil {
						match = submatchStrings(matchLine, loc)
						smInfo = &smMatchInfo{idx: loc, base: lineOffsets[lineIdx]}
					}
				} else {
					match = compiledPattern.Regex.FindStringSubmatch(matchLine)
				}

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
						if smInfo != nil {
							obs.noteSubmatches(lineOffsets[lineIdx], patternIdx, smInfo)
						}
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
					if obs != nil {
						obs.noteSubmatches(indices[0], patternIdx, &smMatchInfo{idx: indices})
					}
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
				if !isPureIndicatorResult(allMatches[j].result) {
					hasNormalMatch = true
				}
				j++
			}

			// Process matches at this position
			for k := i; k < j; k++ {
				isPureIndicator := isPureIndicatorResult(allMatches[k].result)

				if hasNormalMatch && isPureIndicator {
					// Skip pure indicator match if we have a normal match
					// EXCEPT _start_ and _line_ (on its own line), which we need to start matches even if redundant
					// Filter out _end_ and _line_ (with joinmatches) when redundant, but keep _start_ and standalone _line_
					if keepsRedundantIndicator(ind.profile, allMatches[k].patternIdx) {
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

	// joinmatches variables and their join characters (from the profile)
	joinMatchesVars := ind.joinMatchesVars
	joinMatchesChars := ind.joinMatchesChars
	joinMatchesHasToList := ind.joinMatchesHasToList

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
	// With a source map, the merge state also notes which matches each
	// record consumed (state.usedByRecord, aligned with mergedMatches).
	state.trackUsed = obs != nil
	state.flush = func(record map[string]interface{}, _ [2]int) error {
		mergedMatches = append(mergedMatches, record)
		return nil
	}

	// Start and end patterns are Python TTP's start_re and end_re, read
	// from the group's indicator profile.
	startPatterns := ind.startPatterns
	endPatterns := ind.endPatterns
	hasLineIndicator := ind.hasLineIndicator
	hasAnyStartIndicator := ind.hasAnyStartIndicator
	hasEmptyStartPattern := ind.hasEmptyStartPattern

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
		hasNonSpecialVars := hasNonIndicatorKey(currentMatch)
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
			if obs != nil {
				state.usedByRecord = append(state.usedByRecord, state.takeUsed())
			}
			// Track which matches from allMatches belong to this parent
			// All matches from currentParentMatchStartIdx to the last match index belong to this parent
			if currentParentMatchStartIdx >= 0 && len(allMatches) > 0 {
				if currentParentMatchStartIdx < len(allMatches) {
					parentMatchToAllMatches[parentIdx] = matchIndexRange{lo: currentParentMatchStartIdx, hi: len(allMatches)}
				}
			}
			// Update path resolver cache with final match values
			r.pathResolver.UpdateCache(matchCopy)
		}
	}

	if obs != nil {
		recs = obs.recordMatches(group, allMatches, state.usedByRecord)
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
		// Group matches by parent match index.
		//
		// Only the first and last spanStart of each parent are ever read below,
		// so we keep just those two instead of materializing a full []int per
		// parent (which allocated O(len(allMatches)) ints for no benefit).
		type parentSpan struct {
			first int
			last  int
			ok    bool
		}
		parentSpans := make([]parentSpan, len(mergedMatches))

		// Group matches by parent match using the tracked mapping
		// Use parentMatchToAllMatches to get the correct matches for each parent
		for parentIdx := 0; parentIdx < len(mergedMatches); parentIdx++ {
			if idxRange, ok := parentMatchToAllMatches[parentIdx]; ok {
				lo, hi := idxRange.lo, idxRange.hi
				if hi > len(allMatches) {
					hi = len(allMatches)
				}
				if lo < hi {
					parentSpans[parentIdx].first = allMatches[lo].spanStart
					parentSpans[parentIdx].last = allMatches[hi-1].spanStart
					parentSpans[parentIdx].ok = true
				}
			}
		}

		// Calculate ranges based on match positions
		// Check if we have _end_ patterns - if so, use _end_ position as range end
		endPatternIndices := endPatterns
		hasEndPatterns := len(endPatternIndices) > 0

		// allMatches is sorted ascending by (spanStart, patternIdx) above, which
		// lets both lookups below binary-search instead of rescanning the whole
		// slice per parent match. On a 5.6 MB input with ~100K matches those
		// linear rescans were the dominant cost of parseGroup (O(parents ×
		// matches)); the results are identical because "first element in slice
		// order satisfying a spanStart predicate" is exactly the lower bound of
		// a sorted slice.

		// firstSpanEnd returns the spanEnd of the first match whose spanStart
		// equals pos, or (0, false) if there is none.
		firstSpanEnd := func(pos int) (int, bool) {
			idx := sort.Search(len(allMatches), func(k int) bool {
				return allMatches[k].spanStart >= pos
			})
			if idx < len(allMatches) && allMatches[idx].spanStart == pos {
				return allMatches[idx].spanEnd, true
			}
			return 0, false
		}

		// End-pattern matches are a small subset; filtering once (order
		// preserved, so still sorted) keeps the per-parent lookup logarithmic
		// in that subset rather than linear in allMatches.
		var endMatches []patternMatch
		if hasEndPatterns {
			for _, match := range allMatches {
				if endPatternIndices[match.patternIdx] {
					endMatches = append(endMatches, match)
				}
			}
		}

		for i := range parentMatchRanges {
			if i < len(parentSpans) && parentSpans[i].ok {
				// Use first match position as start
				parentMatchRanges[i].start = parentSpans[i].first
				// Use last match position + some buffer as end
				lastPos := parentSpans[i].last
				// Find the end of the last match
				// If we have _end_ patterns, look for the _end_ pattern match for this parent
				if hasEndPatterns {
					// Find the _end_ pattern match that belongs to this parent
					// It should be after the start but before the next parent's start
					nextParentStart := len(inputData)
					if i < len(parentSpans)-1 && parentSpans[i+1].ok {
						nextParentStart = parentSpans[i+1].first
					}
					// Look for _end_ pattern matches in this parent's range
					idx := sort.Search(len(endMatches), func(k int) bool {
						return endMatches[k].spanStart >= parentMatchRanges[i].start
					})
					if idx < len(endMatches) && endMatches[idx].spanStart < nextParentStart {
						// Found _end_ pattern - use its end position
						parentMatchRanges[i].end = endMatches[idx].spanEnd
					}
				}
				// If no _end_ found or no _end_ patterns, check if we have nested groups
				// If we have nested groups, extend range to end of input (or next parent) so nested groups can match
				if parentMatchRanges[i].end == 0 {
					if len(group.Groups) > 0 {
						// We have nested groups - extend range to end of input or next parent
						if i < len(mergedMatches)-1 && i+1 < len(parentSpans) && parentSpans[i+1].ok {
							parentMatchRanges[i].end = parentSpans[i+1].first
						} else {
							parentMatchRanges[i].end = len(inputData)
						}
					} else {
						// No nested groups - use last match end
						if spanEnd, ok := firstSpanEnd(lastPos); ok {
							parentMatchRanges[i].end = spanEnd
						}
					}
				}
				// If still no end found, use next parent's start or end of input
				if parentMatchRanges[i].end == 0 {
					if i < len(mergedMatches)-1 && i+1 < len(parentSpans) && parentSpans[i+1].ok {
						parentMatchRanges[i].end = parentSpans[i+1].first
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
				var nestedResults interface{}
				var err error
				var nestedRecs []*smRecord
				if obs != nil {
					scope := obs.enter(rangeStart, smAt(recs, i))
					nestedResults, err = r.parseGroup(nestedGroup, parentInputData, nestedVars)
					nestedRecs = obs.leave(scope)
				} else {
					nestedResults, err = r.parseGroup(nestedGroup, parentInputData, nestedVars)
				}
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
				if obs != nil && (nestedResults == nil || hasVoid) {
					smDrop(nestedRecs)
				}
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
						smDrop(nestedRecs)
						continue
					}

					// Check if nested group has void attribute
					nestedHasVoid := false
					if voidVal, ok := nestedGroup.Attributes["void"]; ok {
						nestedHasVoid = voidVal == "" || voidVal == "true" || voidVal == "True" || voidVal == "TRUE"
					}

					if !nestedHasVoid {
						// If nested group name is "_", merge into parent match
						switch nestedResults.(type) {
						case []map[string]interface{}, map[string]interface{}:
						default:
							// Neither stored nor merged below (e.g. itemize on a
							// child group, #88), so its records have no path.
							smDrop(nestedRecs)
						}
						if nestedGroup.Name == "_" || nestedGroup.Name == "" {
							// Merged into the parent record: same path.
							for _, nr := range nestedRecs {
								if nr != nil {
									nr.alias = smAt(recs, i)
								}
							}
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
									rec   *smRecord
								}
								groupedNestedResults := make(map[string][]pathMatch)
								var nestedPathOrder []string // first-seen order; Go map order is random (#78)
								for k, nestedMatch := range v {
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
											if rec := smAt(nestedRecs, k); rec != nil {
												rec.dropped = true
											}
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
										nestedPathOrder = append(nestedPathOrder, pathKey)
									}
									rec := smAt(nestedRecs, k)
									if rec != nil {
										rec.final = cleanedNestedMatch
									}
									groupedNestedResults[pathKey] = append(groupedNestedResults[pathKey], pathMatch{
										parts: resolvedParts,
										match: cleanedNestedMatch,
										rec:   rec,
									})
								}

								// Store grouped results using storeAtPathSegments to handle formatters
								for _, pathKey := range nestedPathOrder {
									pathMatches := groupedNestedResults[pathKey]
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
									if obs != nil {
										stored := make([]*smRecord, len(pathMatches))
										for j := range pathMatches {
											stored[j] = pathMatches[j].rec
										}
										setStoredRel(parentMatch, segmentKeys(resolvedParts), stored)
									}

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
										smDrop(nestedRecs)
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
								if rec := smAt(nestedRecs, 0); rec != nil {
									rec.final = cleanedNestedMatch
									setStoredRel(parentMatch, segmentKeys(resolvedParts), []*smRecord{rec})
								}

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

		// recordOrder keeps record values in first-seen order so output order
		// follows input order (Python TTP) rather than Go map order (#78).
		groupedByRecord := make(map[string][]map[string]interface{})
		var recordOrder []string
		var recsByRecord map[string][]*smRecord
		if obs != nil {
			recsByRecord = make(map[string][]*smRecord)
		}

		for mi, match := range mergedMatches {
			// Get the value of the record variable from the match
			recordValue := ""
			if val, exists := match[recordVar]; exists {
				// Convert to string for grouping
				recordValue = fmt.Sprintf("%v", val)
			}
			if _, seen := groupedByRecord[recordValue]; !seen {
				recordOrder = append(recordOrder, recordValue)
			}
			groupedByRecord[recordValue] = append(groupedByRecord[recordValue], match)
			if obs != nil {
				recsByRecord[recordValue] = append(recsByRecord[recordValue], smAt(recs, mi))
			}
		}

		// Convert grouped matches to a list (Python TTP behavior: record groups matches as a list)
		// Each unique record value becomes a separate entry in the list
		var groupedMatches []map[string]interface{}
		var groupedRecs []*smRecord
		for _, recordValue := range recordOrder {
			matches := groupedByRecord[recordValue]
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
				if obs != nil {
					groupedRecs = append(groupedRecs, obs.smCollapse(group, recsByRecord[recordValue]))
				}
			}
		}

		// Replace mergedMatches with grouped matches
		mergedMatches = groupedMatches
		recs = groupedRecs
	}

	// Process group function attributes (containsall, contains, del, exclude, excludeall, etc.)
	// These are group attributes that act as functions to filter/transform matches
	// They should be processed before the functions/chain attribute
	// Note: record attribute is handled above, so we skip it here
	groupFunctionAttrs := builtinAttrGroupFunctions
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
				var kept []int
				for mi, match := range mergedMatches {
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
						if obs != nil {
					kept = append(kept, mi)
				}
					}
				}
				mergedMatches = expandedMatches
				if obs != nil {
					recs = smKeep(recs, kept)
				}
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
				if obs != nil {
					obs.lastNodes = []*smRecord{obs.smCollapse(group, recs)}
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
			var kept []int
			for mi, match := range mergedMatches {
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
					if obs != nil {
					kept = append(kept, mi)
				}
				}
			}
			mergedMatches = filteredMatches
			if obs != nil {
				recs = smKeep(recs, kept)
			}
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
			if nonFunctionGroupAttrs[attrName] {
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
			var kept []int
			for mi, matchData := range mergedMatches {
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
					if obs != nil {
					kept = append(kept, mi)
				}
				}
			}
			mergedMatches = filteredMatches
			if obs != nil {
				recs = smKeep(recs, kept)
			}
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
		var kept *[]int
		var onKeep func(int)
		if obs != nil {
			kept = new([]int)
			onKeep = func(i int) { *kept = append(*kept, i) }
		}
		processedMatches, err := r.processGroupFunctions(mergedMatches, chainStr, vars, onKeep)
		if err != nil {
			return nil, fmt.Errorf("failed to process group functions: %w", err)
		}
		mergedMatches = processedMatches
		if obs != nil {
			recs = smKeep(recs, *kept)
		}

		// Check if itemize was used - if so, return the value directly
		if len(mergedMatches) > 0 {
			if itemizeValue, ok := mergedMatches[0]["_itemize_value"]; ok {
				// itemize returns the value directly, not as a map
				// Return the value as-is (it's already wrapped in a list)
				if obs != nil {
					obs.lastNodes = []*smRecord{obs.smCollapse(group, recs)}
				}
				return itemizeValue, nil
			}
		}
	}

	// Process group macro if specified
	if group.Macro != "" {
		var kept *[]int
		var onKeep func(int)
		if obs != nil {
			kept = new([]int)
			onKeep = func(i int) { *kept = append(*kept, i) }
		}
		processedMatches, err := r.processGroupMacro(mergedMatches, group.Macro, vars, onKeep)
		if err != nil {
			return nil, fmt.Errorf("failed to process group macro: %w", err)
		}
		mergedMatches = processedMatches
		if obs != nil {
			recs = smKeep(recs, *kept)
		}
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
				for _, cp := range group.Patterns {
					for varName := range cp.Variables {
						// Skip special variables that don't get defaults
						if !pattern.IsIndicatorVar(varName) {
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
					if obs != nil {
						recs = append(recs, obs.newRecord(group))
					}
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
	if len(mergedMatches) == 0 {
		if defaultMatch := startLineDefaultsRecord(group); defaultMatch != nil {
			mergedMatches = append(mergedMatches, defaultMatch)
			if obs != nil {
				recs = append(recs, obs.newRecord(group))
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
			if obs != nil {
				obs.lastNodes = []*smRecord{obs.smCollapse(group, recs)}
			}
			return []map[string]interface{}{{}}, nil
		}
	}

	// Filter out empty matches from mergedMatches
	// This can happen when a match is started but not populated (e.g., _start_ pattern with no other variables)
	filteredMatches := make([]map[string]interface{}, 0, len(mergedMatches))
	var kept []int
	for mi, match := range mergedMatches {
		if len(match) > 0 {
			filteredMatches = append(filteredMatches, match)
			if obs != nil {
					kept = append(kept, mi)
				}
		}
	}
	mergedMatches = filteredMatches
	if obs != nil {
		recs = smKeep(recs, kept)
		obs.lastNodes = recs
	}

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
	out, err := r.processGroupFunctions([]map[string]interface{}{record}, chainStr, vars, nil)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	if out[0] == nil {
		// Kept but empty: distinct from dropped (nil).
		return map[string]interface{}{}, nil
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
	out, err := r.processGroupMacro([]map[string]interface{}{record}, group.Macro, vars, nil)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	if out[0] == nil {
		// Kept but empty: distinct from dropped (nil).
		return map[string]interface{}{}, nil
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

	// kept is set once a record survives the chain and macro. If none does,
	// Parse gives a record of start-line defaults (see the end of this
	// function).
	kept := false

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
		// The record survived the chain and macro, so Parse would not make
		// its record of start-line defaults, even if the record is empty.
		kept = true
		if len(filtered) == 0 {
			return nil // emptied (parseGroup drops empty records)
		}
		return fn(filtered, srcRange, group.NormalizedPath)
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
			if !indicator.profile.Matches(patternIdx) {
				continue // a line without variables: Python TTP skips it
			}

			// Skip empty lines unless this pattern is a start or end
			// indicator line, which may match a blank line.
			matchesBlank := indicator.profile.MatchesBlank(patternIdx)
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
		hasNonSpecialVars := hasNonIndicatorKey(state.currentMatch)
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

// streamIndicatorMeta is the indicator metadata stepMerge consumes in the
// streaming pipeline; it is the same groupIndicators parseGroup uses.
type streamIndicatorMeta = groupIndicators

// computeStreamIndicatorMeta reads the group's merge indicators from its
// compiled profile, exactly as parseGroup does, without scanning input.
func computeStreamIndicatorMeta(group *compiler.CompiledGroup) streamIndicatorMeta {
	return indicatorsFor(group)
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
		if !isPureIndicatorResult(m.result) {
			hasNormalMatch = true
			break
		}
	}

	if !hasNormalMatch {
		return lineMatches
	}

	prof := group.Profile()
	out := lineMatches[:0]
	for _, m := range lineMatches {
		if isPureIndicatorResult(m.result) {
			// hasNormalMatch is true here; keep _start_ and standalone _line_.
			if keepsRedundantIndicator(prof, m.patternIdx) {
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

// processGroupFunctions processes group functions from chain/functions attribute
// Functions are pipe-separated: "contains('ip') | set('key', 'value')"
// Returns filtered/processed matches
//
// onKeep, when not nil, is called with the index of every match that is kept.
func (r *Runtime) processGroupFunctions(matches []map[string]interface{}, chainStr string, vars map[string]interface{}, onKeep func(int)) ([]map[string]interface{}, error) {
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
			return r.applyGroupFunctions(matches, funcStrs, vars, onKeep)
		}
	}

	// Parse pipe-separated function calls
	funcStrs := strings.Split(chainStr, "|")
	for i := range funcStrs {
		funcStrs[i] = strings.TrimSpace(funcStrs[i])
	}

	return r.applyGroupFunctions(matches, funcStrs, vars, onKeep)
}

// applyGroupFunctions applies a list of group functions to matches
func (r *Runtime) applyGroupFunctions(matches []map[string]interface{}, funcStrs []string, vars map[string]interface{}, onKeep func(int)) ([]map[string]interface{}, error) {
	var result []map[string]interface{}

	for mi, match := range matches {
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
			if onKeep != nil {
				onKeep(mi)
			}
		}
	}

	return result, nil
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

// processGroupMacro processes group macro on each match
// Macro can be a single macro name or comma-separated list
// Optimized to chain Starlark macros without Go↔Starlark conversions
//
// onKeep, when not nil, is called with the index of every match that is kept.
func (r *Runtime) processGroupMacro(matches []map[string]interface{}, macroStr string, vars map[string]interface{}, onKeep func(int)) ([]map[string]interface{}, error) {
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

	for mi, match := range matches {
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
						if !macroErrorIsNotFound(err) {
							return nil, fmt.Errorf("group macro %v failed: %w", actualMacroNames, err)
						}
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
					if !macroErrorIsNotFound(err) {
						return nil, fmt.Errorf("macro %s failed: %w", actualMacroName, err)
					}
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
			if onKeep != nil {
				onKeep(mi)
			}
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
					fmt.Printf("%v", result)
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
