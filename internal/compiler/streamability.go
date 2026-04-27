package compiler

import (
	"fmt"
	"sort"
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

// analyzeStreamabilityRecursive runs analyzeStreamability on a group and
// all its descendants. Called by the compiler after group assembly.
func analyzeStreamabilityRecursive(g *CompiledGroup) {
	analyzeStreamability(g)
	for _, child := range g.Groups {
		analyzeStreamabilityRecursive(child)
	}
}

// computeTemplateStreamable sets t.Streamable = true iff every top-level
// group is streamable.
func computeTemplateStreamable(t *CompiledTemplate) {
	t.Streamable = true
	for _, g := range t.Groups {
		if !g.Streamable {
			t.Streamable = false
			return
		}
	}
}

// GroupPathCollisionError is returned by CompileTemplate when two or
// more groups in the template have distinct literal Name values that
// normalize to the same path. The collision is a template-authoring
// bug: if the author wanted them to merge, they should use the same
// literal Name (the deliberate alternative-pattern synthesis pattern).
type GroupPathCollisionError struct {
	NormalizedPath string
	GroupNames     []string // the literal Name values that collide
}

func (e *GroupPathCollisionError) Error() string {
	return fmt.Sprintf("group path collision: %d groups normalize to %q: %s",
		len(e.GroupNames), e.NormalizedPath, strings.Join(e.GroupNames, ", "))
}

// validateGroupPathCollisions walks every group (including nested) and
// returns a *GroupPathCollisionError if any normalized path is shared
// by groups with distinct literal names. Identical literal names are
// allowed (deliberate alternative-pattern synthesis).
func validateGroupPathCollisions(t *CompiledTemplate) error {
	// Map: normalizedPath -> set of distinct literal names that produced it.
	pathToNames := make(map[string]map[string]bool)
	collectGroupPaths(t.Groups, pathToNames)

	for path, names := range pathToNames {
		if len(names) > 1 {
			distinct := make([]string, 0, len(names))
			for n := range names {
				distinct = append(distinct, n)
			}
			// Sort for stable error messages.
			sort.Strings(distinct)
			return &GroupPathCollisionError{
				NormalizedPath: path,
				GroupNames:     distinct,
			}
		}
	}
	return nil
}

// collectGroupPaths populates pathToNames with every group's
// (NormalizedPath, set of literal Names) including nested groups.
func collectGroupPaths(groups []*CompiledGroup, pathToNames map[string]map[string]bool) {
	for _, g := range groups {
		if pathToNames[g.NormalizedPath] == nil {
			pathToNames[g.NormalizedPath] = make(map[string]bool)
		}
		pathToNames[g.NormalizedPath][g.Name] = true
		if len(g.Groups) > 0 {
			collectGroupPaths(g.Groups, pathToNames)
		}
	}
}
