package compiled

import (
	"fmt"
)

// MatchCollector collects multiple matches for variables with joinmatches function
type MatchCollector struct {
	collections map[string][]interface{} // variable name -> collected values
}

// NewMatchCollector creates a new match collector
func NewMatchCollector() *MatchCollector {
	return &MatchCollector{
		collections: make(map[string][]interface{}),
	}
}

// Collect adds a value to the collection for a variable
func (mc *MatchCollector) Collect(varName string, value interface{}) {
	mc.collections[varName] = append(mc.collections[varName], value)
}

// GetCollected returns all collected values for a variable
func (mc *MatchCollector) GetCollected(varName string) []interface{} {
	return mc.collections[varName]
}

// HasCollected checks if a variable has collected values
func (mc *MatchCollector) HasCollected(varName string) bool {
	return len(mc.collections[varName]) > 0
}

// Clear clears all collections
func (mc *MatchCollector) Clear() {
	mc.collections = make(map[string][]interface{})
}

// ApplyJoinMatches applies joinmatches to variables that have it
// This should be called after all matches for a group are collected
func (mc *MatchCollector) ApplyJoinMatches(match map[string]interface{}, varName string, joinChar string) {
	if mc.HasCollected(varName) {
		collected := mc.GetCollected(varName)
		if len(collected) > 0 {
			// Join collected values
			if joinChar == "" {
				joinChar = ", "
			}
			// Convert to strings and join
			strs := make([]string, len(collected))
			for i, v := range collected {
				strs[i] = fmt.Sprintf("%v", v)
			}
			// Join with separator
			joined := ""
			for i, s := range strs {
				if i > 0 {
					joined += joinChar
				}
				joined += s
			}
			match[varName] = joined
		}
	}
}

