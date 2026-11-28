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
// macro is per-record: Python TTP runs a chain macro() on each result, and
// gottp's processGroupFunctions does not run it at all yet (#113), so Parse
// and the stream path agree either way.
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
	"macro":        true,
}

// attributeGroupFunctions are the group functions parseGroup applies when
// they are given as group attributes (contains="ip", record="vrf", ...).
// parseGroupStream applies only the chain/functions attribute and the group
// macro, so a streamable group may use none of these.
var attributeGroupFunctions = []string{
	"containsall", "contains", "del", "delete", "exclude", "excludeall", "equal",
	"to_int", "contains_val", "exclude_val", "sformat", "itemize", "set",
	"expand", "items2dict", "to_ip", "record",
}

// parseGroupFunctionNames extracts function names from a group's
// functions= or chain= attribute. Functions are separated by "|" (what
// processGroupFunctions splits on) or ",", e.g.
// "containsall('mac-address', 'us-intf') | to_int". Returns the bare names
// ("containsall", "to_int") with arguments stripped.
//
// Argument lists and quoted text are removed before splitting, so commas
// and pipes inside them don't make names. A token that is not a function
// name afterwards (for example the "[" of a list literal kept as a string)
// is skipped: processGroupFunctions finds no function by that name and
// skips it too.
func parseGroupFunctionNames(functions string) []string {
	if functions == "" {
		return nil
	}
	var b strings.Builder
	depth := 0
	var quote rune
	for _, r := range functions {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '(':
			depth++
		case r == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			b.WriteRune(r)
		}
	}
	var names []string
	for _, part := range strings.FieldsFunc(b.String(), func(r rune) bool { return r == ',' || r == '|' }) {
		part = strings.TrimSpace(part)
		if isFunctionName(part) {
			names = append(names, part)
		}
	}
	return names
}

// isFunctionName reports whether s can name a group function: a letter or
// '_' followed by letters, digits, '_', '-' or '.'.
func isFunctionName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
		case i > 0 && (r == '-' || r == '.' || (r >= '0' && r <= '9')):
		default:
			return false
		}
	}
	return true
}

// ResolveGroupChain returns the function strings a group's chain= (or
// functions=) attribute names, resolved the way processGroupFunctions
// resolves them at parse time: if the attribute is the name of a variable in
// vars, a string value is the chain and a list value is one function per
// string item; otherwise the attribute text is the chain. Each returned
// string may still hold several "|"-separated functions.
func ResolveGroupChain(g *CompiledGroup, vars map[string]interface{}) []string {
	chain := g.Chain
	if chain == "" {
		chain = g.Functions
	}
	if chain == "" {
		return nil
	}
	if v, ok := vars[chain]; ok {
		switch val := v.(type) {
		case string:
			return []string{val}
		case []interface{}:
			var items []string
			for _, item := range val {
				if s, ok := item.(string); ok {
					items = append(items, s)
				}
			}
			return items
		}
	}
	return []string{chain}
}

// NonStreamableChainReasons returns one reason for each function in the
// group's resolved chain (see ResolveGroupChain) that is not in the
// streamable allowlist.
func NonStreamableChainReasons(g *CompiledGroup, vars map[string]interface{}) []string {
	var reasons []string
	for _, item := range ResolveGroupChain(g, vars) {
		for _, fn := range parseGroupFunctionNames(item) {
			if !streamableGroupFunctions[fn] {
				reasons = append(reasons, fmt.Sprintf("group %q uses group function %q which is not in the streamable allowlist", g.Name, fn))
			}
		}
	}
	return reasons
}

// hasStartIndicator returns true if some line of the group asks to start a
// record itself (_start_ or _headers_), per the group's indicator profile.
func hasStartIndicator(g *CompiledGroup) bool {
	return g.Profile().HasIndicatedStart()
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
//
// vars are the template's <vars>, used to resolve a chain= or functions=
// attribute that names a template variable.
func analyzeStreamability(g *CompiledGroup, vars map[string]interface{}) {
	g.NormalizedPath = strings.TrimSuffix(g.Name, "*")
	if g.NormalizedPath == "_anonymous_" {
		// The implicit group of a template without group tags is named
		// "_anonymous_", as in Python TTP (#62). It streams like an unnamed
		// <group>, with an empty path.
		g.NormalizedPath = ""
	}

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

	// Rule 4: must have a record boundary, and the stream path matches line
	// by line, so every pattern must be line-anchored.
	if !allPatternsAnchored(g) {
		if !hasStartIndicator(g) {
			reasons = append(reasons, fmt.Sprintf("group %q has no record boundary: no _start_ indicator and not all patterns are line-anchored", g.Name))
		} else {
			reasons = append(reasons, fmt.Sprintf("group %q has a pattern that is not line-anchored (the stream path matches line by line)", g.Name))
		}
	}

	// Rule 5: group functions (functions= or chain=; parseGroup uses chain
	// when both are set) must all be in the streamable allowlist. A chain
	// naming a template variable is resolved against the template's <vars>
	// first, as processGroupFunctions does; ParseStream repeats the check
	// with the parse's vars, which can override them.
	reasons = append(reasons, NonStreamableChainReasons(g, vars)...)

	// Rules 6-13 reject the group features parseGroup applies and
	// parseGroupStream does not (#130), so a template either streams with
	// the records Parse would give or fails up front.

	// Rule 6: group functions given as attributes (contains="ip", record=...).
	for _, attr := range attributeGroupFunctions {
		if v, ok := g.Attributes[attr]; ok && (v != "" || attr == "expand") {
			reasons = append(reasons, fmt.Sprintf("group %q uses the %s= attribute, which streaming does not apply", g.Name, attr))
		}
	}

	// Rule 7: default= fills in missing variables (and makes a record when
	// nothing matched).
	if v := g.Attributes["default"]; v != "" {
		reasons = append(reasons, fmt.Sprintf("group %q uses the default= attribute, which streaming does not apply", g.Name))
	}

	// Rule 8: a void group's records are not part of the result.
	if v, ok := g.Attributes["void"]; ok && (v == "" || v == "true" || v == "True" || v == "TRUE") {
		reasons = append(reasons, fmt.Sprintf("group %q is void (its records are not returned by Parse)", g.Name))
	}

	// Rule 9: defaults from set() on a line without other text.
	if len(g.Defaults) > 0 {
		reasons = append(reasons, fmt.Sprintf("group %q has set() defaults on a line without text, which streaming does not apply", g.Name))
	}

	// Rule 10: method="table" keeps only the first pattern per line.
	if g.Method == "table" {
		reasons = append(reasons, fmt.Sprintf("group %q uses method=\"table\", which streaming does not implement", g.Name))
	}

	// Rule 11: a dynamic path is resolved per record, and its variables are
	// removed from the records.
	if strings.Contains(g.Name, "{{") {
		reasons = append(reasons, fmt.Sprintf("group %q has a dynamic path, which streaming does not resolve", g.Name))
	}

	// A start line with default() makes a record of defaults when the group
	// matches nothing; parseGroupStream gives that record too.

	g.NonStreamableReasons = reasons
	g.Streamable = len(reasons) == 0
}

// analyzeStreamabilityRecursive runs analyzeStreamability on a group and
// all its descendants. Called by the compiler after group assembly.
func analyzeStreamabilityRecursive(g *CompiledGroup, vars map[string]interface{}) {
	analyzeStreamability(g, vars)
	for _, child := range g.Groups {
		analyzeStreamabilityRecursive(child, vars)
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
