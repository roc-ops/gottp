package compiled

import (
	"sync"

	"github.com/roc-ops/gottp/internal/yang"
)

// Per-parse state (#83).
//
// A Runtime is built once per compiled template and may be reused for many
// Parse calls, including concurrent ones. Everything a parse writes therefore
// lives on a shallow copy made by beginParse, never on the shared Runtime:
// the registries, compiled template and compile-time functions are shared
// read-only, while the fields reset below start fresh for every call.
//
// The dynamic-path cache follows Python TTP 0.10.1, which keeps
// dyn_path_cache on a _results_class created for every input it parses: a
// value matched in one input never names the path of a record from another.
// gottp walks groups first and inputs second, so a parse holds one
// PathResolver per input and usePathResolverFor selects it.

// lastParseResults holds the outputs callers read back after Parse through
// GetValidationResults and GetKeyFields. It is shared by a Runtime and all of
// its per-parse copies. When parses on one Runtime overlap, it holds the
// outputs of whichever finished last.
type lastParseResults struct {
	mu                sync.Mutex
	validationResults map[string]*yang.ValidationResult
	keyFields         map[string][]string
}

func (l *lastParseResults) get() (map[string]*yang.ValidationResult, map[string][]string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	validationResults, keyFields := l.validationResults, l.keyFields
	if validationResults == nil {
		validationResults = make(map[string]*yang.ValidationResult)
	}
	if keyFields == nil {
		keyFields = make(map[string][]string)
	}
	return validationResults, keyFields
}

// beginParse returns a private copy of r with fresh per-parse state.
func (r *Runtime) beginParse() *Runtime {
	run := *r
	run.validationResults = make(map[string]*yang.ValidationResult)
	run.keyFields = make(map[string][]string)
	run.recordedVars = make(map[string]interface{})
	run.runtimeLookups = nil
	run.runtimeFunctions = nil
	run.pathResolvers = make(map[string]*PathResolver)
	run.pathResolver = NewPathResolver()
	run.matchCollector = NewMatchCollector()
	return &run
}

// finishParse publishes run's outputs on r for GetValidationResults and
// GetKeyFields.
func (r *Runtime) finishParse(run *Runtime) {
	r.lastParse.mu.Lock()
	defer r.lastParse.mu.Unlock()
	r.lastParse.validationResults = run.validationResults
	r.lastParse.keyFields = run.keyFields
}

// usePathResolverFor makes inputName's dynamic-path cache the active one,
// creating it on first use. Only valid on a copy returned by beginParse.
func (r *Runtime) usePathResolverFor(inputName string) {
	pr, ok := r.pathResolvers[inputName]
	if !ok {
		pr = NewPathResolver()
		r.pathResolvers[inputName] = pr
	}
	r.pathResolver = pr
}
