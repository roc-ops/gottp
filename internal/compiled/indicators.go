package compiled

import (
	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/pattern"
)

// groupIndicators is what the merge state machine (stepMerge) needs to know
// about a group's start, end and line patterns. Every field is read from the
// group's compiled indicator profile (compiler.IndicatorProfile), which is
// gottp's only definition of those patterns; the runtime never inspects
// variable names or functions for _start_, _end_ or _line_ itself.
//
// parseGroup and parseGroupStream both build it with indicatorsFor, so the
// batch and streaming pipelines classify patterns identically.
type groupIndicators struct {
	profile *compiler.IndicatorProfile

	joinMatchesVars      map[string]bool
	joinMatchesHasToList map[string]bool
	joinMatchesChars     map[string]string

	// startPatterns are the patterns whose match starts a record (TTP's
	// ACTION start/startempty); endPatterns are TTP's end_re.
	startPatterns map[int]bool
	endPatterns   map[int]bool

	// hasLineIndicator: some line is only {{ _line_ }}.
	hasLineIndicator bool
	// hasAnyStartIndicator: some line asks to start a record itself.
	hasAnyStartIndicator bool
	// hasEmptyStartPattern: some {{ _start_ }}-only line matches blank lines.
	hasEmptyStartPattern bool
}

// indicatorsFor reads a group's merge indicators from its profile.
func indicatorsFor(group *compiler.CompiledGroup) groupIndicators {
	prof := group.Profile()
	ind := groupIndicators{
		profile:              prof,
		joinMatchesVars:      make(map[string]bool),
		joinMatchesHasToList: make(map[string]bool),
		joinMatchesChars:     make(map[string]string),
		startPatterns:        make(map[int]bool),
		endPatterns:          make(map[int]bool),
		hasLineIndicator:     prof.HasBareLine(),
		hasAnyStartIndicator: prof.HasIndicatedStart(),
		hasEmptyStartPattern: prof.HasBlankStart(),
	}
	if group.Method == "table" {
		// Every line of a table is a start pattern and each match is its
		// own record; the start-indicator and _line_ merge rules don't apply.
		ind.hasLineIndicator = false
		ind.hasAnyStartIndicator = false
		ind.hasEmptyStartPattern = false
	}
	for name, spec := range prof.JoinMatches() {
		ind.joinMatchesVars[name] = true
		ind.joinMatchesHasToList[name] = spec.HasToList
		ind.joinMatchesChars[name] = spec.Char
	}
	for i := range group.Patterns {
		if prof.StartsRecord(i) {
			ind.startPatterns[i] = true
		}
		if prof.IsEnd(i) {
			ind.endPatterns[i] = true
		}
	}
	return ind
}

// isPureIndicatorResult reports whether a match result holds only _start_,
// _end_ or _line_ keys, i.e. the match captured no value of its own.
func isPureIndicatorResult(result map[string]interface{}) bool {
	for k := range result {
		if !pattern.IsMarkerVar(k) {
			return false
		}
	}
	return true
}

// hasNonIndicatorKey reports whether a record has a key other than ignore,
// _start_, _end_ or _line_.
func hasNonIndicatorKey(record map[string]interface{}) bool {
	for k := range record {
		if !pattern.IsIndicatorVar(k) {
			return true
		}
	}
	return false
}

// keepsRedundantIndicator reports whether a pure indicator match must be kept
// on a line that an ordinary pattern also matched: a {{ _start_ }} line or a
// line that is only {{ _line_ }} still has to start a record. Other pure
// indicator matches (_end_, _line_ with other variables) are redundant there.
func keepsRedundantIndicator(prof *compiler.IndicatorProfile, patternIdx int) bool {
	return prof.IsStartEmpty(patternIdx) || prof.IsBareLine(patternIdx)
}
