package compiled

import (
	"github.com/roc-ops/gottp/internal/pattern"
)

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
