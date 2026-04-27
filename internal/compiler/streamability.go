package compiler

import (
	"strings"

	"github.com/roc-ops/gottp/internal/pattern"
)

// streamableGroupFunctions lists the group-level functions (the ones in
// CompiledGroup.Functions) that operate per-record and are therefore safe
// in streaming mode. The allowlist approach is conservative — any group
// function not in this set makes the group non-streamable, so unknown
// functions fail closed rather than open.
//
// Aggregating functions deliberately excluded:
//   - itemize: builds a dict of records keyed by a field, needs the full set
//   - expand:  reshapes the result list structure
var streamableGroupFunctions = map[string]bool{
	"contains":     true,
	"containsall":  true,
	"contains_val": true,
	"exclude":      true,
	"excludeall":   true,
	"exclude_val":  true,
	"equal":        true,
	"set":          true,
	"record":       true,
	"to_int":       true,
	"to_ip":        true,
	"sformat":      true,
	"cerberus":     true,
	"validate":     true,
	"delete":       true,
	"del":          true,
}

// parseGroupFunctionNames extracts function names from a group's
// Functions= attribute string. The format is comma-separated function
// calls, e.g. "containsall('mac-address'), to_int". Returns the bare
// names ("containsall", "to_int") with arguments stripped.
func parseGroupFunctionNames(functions string) []string {
	if functions == "" {
		return nil
	}
	var names []string
	for _, part := range strings.Split(functions, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Strip arguments: "containsall('x')" -> "containsall"
		if idx := strings.IndexByte(part, '('); idx >= 0 {
			part = part[:idx]
		}
		part = strings.TrimSpace(part)
		if part != "" {
			names = append(names, part)
		}
	}
	return names
}

// hasStartIndicator returns true if the group has at least one variable
// (or variable function) that produces a _start_ record boundary.
func hasStartIndicator(g *CompiledGroup) bool {
	for _, p := range g.Patterns {
		for _, v := range p.Variables {
			if v.Name == "_start_" {
				return true
			}
			for _, f := range v.Functions {
				if f == "_start_" {
					return true
				}
			}
		}
	}
	return false
}

// allPatternsAnchored returns true if every pattern in the group has
// regex anchors (^ or $), meaning each pattern matches at most one line.
func allPatternsAnchored(g *CompiledGroup) bool {
	if len(g.Patterns) == 0 {
		return false
	}
	for _, p := range g.Patterns {
		if !p.HasAnchors {
			return false
		}
	}
	return true
}

// hasAnyJoinMatches returns true if any variable in any pattern uses
// the joinmatches function (which aggregates across records).
func hasAnyJoinMatches(g *CompiledGroup) bool {
	for _, p := range g.Patterns {
		for _, v := range p.Variables {
			if v.HasJoinMatches {
				return true
			}
		}
	}
	return false
}

// patternUnused — silence unused import warning until we add the analyzer.
var _ = pattern.CompiledPattern{}
