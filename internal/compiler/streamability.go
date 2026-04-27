package compiler

import (
	"fmt"
	"strings"
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

// analyzeStreamability runs the strict streamability check defined in
// docs/superpowers/specs/2026-04-27-streaming-parsegroup-design.md and
// populates g.Streamable, g.NonStreamableReasons, and g.NormalizedPath.
//
// Each failed rule contributes one reason. Order matches the spec for
// reviewability.
func analyzeStreamability(g *CompiledGroup) {
	g.NormalizedPath = strings.TrimSuffix(g.Name, "*")

	var reasons []string

	// Rule 1: top-level only in v1.
	if g.IsNested {
		reasons = append(reasons, fmt.Sprintf("group %q is nested (nested groups deferred to v2)", g.Name))
	}

	// Rule 2: no nested children in v1.
	if len(g.Groups) > 0 {
		reasons = append(reasons, fmt.Sprintf("group %q has %d nested child group(s) (deferred to v2)", g.Name, len(g.Groups)))
	}

	// Rule 3: no joinmatches anywhere in the group's variables.
	if hasAnyJoinMatches(g) {
		// Find which variable for a clearer message.
		for _, p := range g.Patterns {
			for _, v := range p.Variables {
				if v.HasJoinMatches {
					reasons = append(reasons, fmt.Sprintf("group %q variable %q uses joinmatches (aggregates across records)", g.Name, v.Name))
					goto joinmatchesDone
				}
			}
		}
	joinmatchesDone:
	}

	// Rule 4: must have a record boundary (either _start_ or fully line-anchored).
	if !hasStartIndicator(g) && !allPatternsAnchored(g) {
		reasons = append(reasons, fmt.Sprintf("group %q has no record boundary: no _start_ indicator and not all patterns are line-anchored", g.Name))
	}

	// Rule 5: group functions must all be in the streamable allowlist.
	for _, fn := range parseGroupFunctionNames(g.Functions) {
		if !streamableGroupFunctions[fn] {
			reasons = append(reasons, fmt.Sprintf("group %q uses group function %q which is not in the streamable allowlist", g.Name, fn))
		}
	}

	g.NonStreamableReasons = reasons
	g.Streamable = len(reasons) == 0
}
