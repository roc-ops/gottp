# Streaming `ParseStream` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `compiled.ParseStream(...)` API that emits one record at a time via callback for streamable templates, bounding peak heap from 778 MB to <20 MB on `show_cable_modem_phy.txt` (24 MB / 256K records).

**Architecture:** Compile-time strict streamability analysis on `CompiledGroup` gates a new `parseGroupStream` runtime variant that drives the existing merge state machine record-by-record, emitting completed records via callback instead of accumulating. Existing `Parse` is untouched; non-streamable templates error from `ParseStream` rather than silently fall back.

**Tech Stack:** Go 1.21+, gottp's existing internal packages (`internal/pattern`, `internal/compiler`, `internal/compiled`, `internal/functions/group`).

**Reference spec:** `docs/superpowers/specs/2026-04-27-streaming-parsegroup-design.md`

---

## Task 0: Set up feature branch

**Files:**
- None (git operation)

- [ ] **Step 1: Verify clean main and pull latest**

```bash
git checkout main
git status
git pull --ff-only origin main
```

Expected: clean working tree, up to date with origin/main.

- [ ] **Step 2: Create feature branch**

```bash
git checkout -b feat/parsestream-v1
```

- [ ] **Step 3: Verify baseline tests pass**

```bash
go test ./... -count=1
```

Expected: all packages OK, no failures.

---

# Phase A — Compile-time streamability analysis

Phase A is purely additive: new fields on existing types, new analysis pass, new error types. No runtime behavior change. Phase A exits when classification tests + collision tests are green and all 4 Class 1 prod templates classify as streamable.

## Task A1: Add streamability fields to `CompiledGroup`

**Files:**
- Modify: `internal/compiler/compiled.go`

- [ ] **Step 1: Add fields to CompiledGroup struct**

Find the `type CompiledGroup struct { ... }` block (around line 63) and add after the existing `Defaults` field:

```go
type CompiledGroup struct {
	Name       string
	Input      string
	Output     string
	Method     string
	Functions  string
	Chain      string
	Macro      string
	Patterns   []*pattern.CompiledPattern
	Groups     []*CompiledGroup
	Attributes map[string]string
	IsNested   bool
	Defaults   map[string]interface{}

	// Streamability — set during compile by analyzeStreamability.
	// Streamable is true iff this group passed the strict streamability check.
	// NonStreamableReasons lists one human-readable explanation per failed
	// rule when Streamable is false; empty otherwise.
	// NormalizedPath is Name with trailing "*" stripped, used as the
	// groupPath argument to the ParseStream callback.
	Streamable           bool
	NonStreamableReasons []string
	NormalizedPath       string
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Run existing tests**

```bash
go test ./internal/compiler/ -count=1
```

Expected: all pass. Adding zero-valued fields shouldn't break anything.

- [ ] **Step 4: Commit**

```bash
git add internal/compiler/compiled.go
git commit -m "feat(compiler): add streamability fields to CompiledGroup

Streamable, NonStreamableReasons, NormalizedPath. Populated during
compile by analyzeStreamability (next commit). Refs #20."
```

## Task A2: Add `Streamable` field to `CompiledTemplate`

**Files:**
- Modify: `internal/compiler/compiled.go`

- [ ] **Step 1: Find CompiledTemplate struct and add field**

Find `type CompiledTemplate struct { ... }` and add a `Streamable bool` field at the end:

```go
type CompiledTemplate struct {
	// ... existing fields ...

	// Streamable is true iff every top-level group is streamable.
	// Used by ParseStream to gate entry; false means ParseStream returns
	// *TemplateNotStreamableError without invoking the callback.
	Streamable bool
}
```

(Replace `// ... existing fields ...` comment with the actual existing fields — read the struct first to see them.)

- [ ] **Step 2: Build + test**

```bash
go build ./... && go test ./internal/compiler/ -count=1
```

Expected: pass.

- [ ] **Step 3: Commit**

```bash
git add internal/compiler/compiled.go
git commit -m "feat(compiler): add Streamable field to CompiledTemplate"
```

## Task A3: Define streamable group function allowlist

**Files:**
- Create: `internal/compiler/streamability.go`

Background: `internal/functions/group/functions.go` registers these group functions:
`contains`, `set`, `record`, `delete`, `del`, `expand`, `itemize`, `containsall`, `exclude`, `excludeall`, `equal`, `to_int`, `contains_val`, `exclude_val`, `sformat`, `to_ip`, `cerberus`, `validate`.

Per-record (streamable): `contains`, `containsall`, `contains_val`, `exclude`, `excludeall`, `exclude_val`, `equal`, `set`, `record`, `to_int`, `to_ip`, `sformat`, `cerberus`, `validate`, `delete`, `del`.

Aggregating (NOT streamable): `itemize` (groups records by key into dict), `expand` (transforms list structure).

- [ ] **Step 1: Create streamability.go with the allowlist**

```go
package compiler

import (
	"fmt"
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
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: pass.

- [ ] **Step 3: Commit**

```bash
git add internal/compiler/streamability.go
git commit -m "feat(compiler): add streamability helpers and group function allowlist"
```

## Task A4: Implement `analyzeStreamability` (TDD)

**Files:**
- Modify: `internal/compiler/streamability.go`
- Create: `internal/compiler/streamability_test.go`

- [ ] **Step 1: Write failing test for plain streamable template**

Create `internal/compiler/streamability_test.go`:

```go
package compiler

import (
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/pattern"
)

// makeGroup builds a minimal CompiledGroup for testing. patterns are
// pre-compiled via the pattern engine.
func makeGroup(t *testing.T, name string, isNested bool, lines []string, groupFns string, children []*CompiledGroup) *CompiledGroup {
	t.Helper()
	eng := pattern.NewEngine()
	var patterns []*pattern.CompiledPattern
	for _, line := range lines {
		cp, err := eng.CompilePattern(line, false, false)
		if err != nil {
			t.Fatalf("compile pattern %q: %v", line, err)
		}
		patterns = append(patterns, cp)
	}
	return &CompiledGroup{
		Name:      name,
		IsNested:  isNested,
		Functions: groupFns,
		Patterns:  patterns,
		Groups:    children,
	}
}

func TestAnalyzeStreamability_PlainStreamable(t *testing.T) {
	g := makeGroup(t, "entry*", false, []string{
		"mac {{ mac | _start_ }}",
		"ip {{ ip }}",
	}, "", nil)

	analyzeStreamability(g)

	if !g.Streamable {
		t.Fatalf("expected Streamable=true, got false; reasons: %v", g.NonStreamableReasons)
	}
	if len(g.NonStreamableReasons) != 0 {
		t.Errorf("expected no reasons, got: %v", g.NonStreamableReasons)
	}
	if g.NormalizedPath != "entry" {
		t.Errorf("NormalizedPath: got %q, want %q", g.NormalizedPath, "entry")
	}
}
```

- [ ] **Step 2: Run test, expect fail**

```bash
go test ./internal/compiler/ -run TestAnalyzeStreamability -v
```

Expected: `analyzeStreamability undefined`.

- [ ] **Step 3: Implement analyzeStreamability**

Append to `internal/compiler/streamability.go`:

```go
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
```

Also remove the `var _ = pattern.CompiledPattern{}` placeholder line — `analyzeStreamability` references `g.Patterns[].Variables` so the `pattern` import is now used implicitly. (If `goimports` removes the import, that's fine — the helpers above don't reference `pattern.` directly either; they walk g.Patterns which is `[]*pattern.CompiledPattern`. Re-check after edit.)

- [ ] **Step 4: Run test, expect pass**

```bash
go test ./internal/compiler/ -run TestAnalyzeStreamability_PlainStreamable -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/compiler/streamability.go internal/compiler/streamability_test.go
git commit -m "feat(compiler): implement analyzeStreamability for plain streamable case"
```

## Task A5: Add classification tests for each negative rule

**Files:**
- Modify: `internal/compiler/streamability_test.go`

- [ ] **Step 1: Add joinmatches test**

Append to `streamability_test.go`:

```go
func TestAnalyzeStreamability_JoinMatches(t *testing.T) {
	g := makeGroup(t, "entry*", false, []string{
		"desc {{ desc | joinmatches }}",
	}, "", nil)
	analyzeStreamability(g)

	if g.Streamable {
		t.Fatal("expected Streamable=false (joinmatches present)")
	}
	if !containsString(g.NonStreamableReasons, "joinmatches") {
		t.Errorf("expected reason mentioning joinmatches, got: %v", g.NonStreamableReasons)
	}
}

func TestAnalyzeStreamability_NestedGroup(t *testing.T) {
	child := makeGroup(t, "inner*", true, []string{"{{ y }}"}, "", nil)
	parent := makeGroup(t, "outer*", false, []string{"header {{ x }}"}, "", []*CompiledGroup{child})
	analyzeStreamability(parent)

	if parent.Streamable {
		t.Fatal("expected Streamable=false (parent has nested child)")
	}
	if !containsString(parent.NonStreamableReasons, "nested child group") {
		t.Errorf("expected reason mentioning nested child, got: %v", parent.NonStreamableReasons)
	}
}

func TestAnalyzeStreamability_NestedGroupItself(t *testing.T) {
	g := makeGroup(t, "inner*", true, []string{"{{ y }}"}, "", nil)
	analyzeStreamability(g)
	if g.Streamable {
		t.Fatal("expected Streamable=false (group is nested)")
	}
}

func TestAnalyzeStreamability_NoRecordBoundary(t *testing.T) {
	// Variables without _start_ in a pattern with no anchors.
	g := makeGroup(t, "entry*", false, []string{"{{ a }} {{ b }}"}, "", nil)
	analyzeStreamability(g)
	if g.Streamable {
		t.Fatal("expected Streamable=false (no record boundary)")
	}
	if !containsString(g.NonStreamableReasons, "no record boundary") {
		t.Errorf("expected reason mentioning no record boundary, got: %v", g.NonStreamableReasons)
	}
}

func TestAnalyzeStreamability_AggregatingGroupFunction(t *testing.T) {
	g := makeGroup(t, "entry*", false, []string{
		"mac {{ mac | _start_ }}",
	}, "itemize", nil)
	analyzeStreamability(g)
	if g.Streamable {
		t.Fatal("expected Streamable=false (itemize is aggregating)")
	}
	if !containsString(g.NonStreamableReasons, "itemize") {
		t.Errorf("expected reason mentioning itemize, got: %v", g.NonStreamableReasons)
	}
}

func TestAnalyzeStreamability_LineAnchoredNoStart(t *testing.T) {
	// Single-line pattern with anchors should be streamable even without _start_.
	// (e.g., show_cable_modem_phy: every line is its own record)
	g := makeGroup(t, "row*", false, []string{
		"^{{ mac }} {{ ip }} {{ status }}$",
	}, "", nil)
	analyzeStreamability(g)
	if !g.Streamable {
		t.Fatalf("expected Streamable=true (line-anchored), got false; reasons: %v", g.NonStreamableReasons)
	}
}

func TestAnalyzeStreamability_NormalizedPath(t *testing.T) {
	g := makeGroup(t, "casa-ios-cli.show_cable_modem_verbose.cm-entry*", false,
		[]string{"mac {{ mac | _start_ }}"}, "", nil)
	analyzeStreamability(g)
	if g.NormalizedPath != "casa-ios-cli.show_cable_modem_verbose.cm-entry" {
		t.Errorf("NormalizedPath: got %q, want %q (suffix * should be stripped)",
			g.NormalizedPath, "casa-ios-cli.show_cable_modem_verbose.cm-entry")
	}

	// Group without trailing *.
	g2 := makeGroup(t, "plain", false, []string{"mac {{ mac | _start_ }}"}, "", nil)
	analyzeStreamability(g2)
	if g2.NormalizedPath != "plain" {
		t.Errorf("NormalizedPath: got %q, want %q (no suffix to strip)",
			g2.NormalizedPath, "plain")
	}
}

// containsString reports whether any element of haystack contains needle.
func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run all classification tests**

```bash
go test ./internal/compiler/ -run TestAnalyzeStreamability -v
```

Expected: all 7 tests PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/compiler/streamability_test.go
git commit -m "test(compiler): cover each streamability negative rule"
```

## Task A6: Wire `analyzeStreamability` into `CompileTemplate`

**Files:**
- Modify: `internal/compiler/compiled.go`

- [ ] **Step 1: Find where the Compiler builds CompiledTemplate / CompiledGroup**

```bash
grep -n "func .*CompileTemplate\|func .*compileGroup\|return &CompiledTemplate\|return.*&CompiledGroup" internal/compiler/compiled.go | head -20
```

Note the line numbers — you'll insert calls there.

- [ ] **Step 2: Call analyzeStreamability when groups are compiled**

After every site where a `*CompiledGroup` is fully assembled (Patterns, IsNested, Functions, Groups all set), invoke `analyzeStreamability(g)`. Walk nested groups recursively as well — even though nested groups can't stream in v1, computing `NormalizedPath` on all groups is needed for the collision check.

Add a helper at the bottom of `streamability.go`:

```go
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
```

(Replace `t.Groups` with whatever the actual top-level field is on CompiledTemplate — check by reading the struct first.)

In `CompileTemplate` (or whichever method assembles the final `CompiledTemplate`), after all groups are constructed and before returning, call:

```go
for _, g := range tmpl.Groups {  // adjust field name as needed
	analyzeStreamabilityRecursive(g)
}
computeTemplateStreamable(tmpl)
```

- [ ] **Step 3: Add a test verifying Streamable propagates to CompiledTemplate**

In `streamability_test.go`:

```go
func TestCompileTemplate_PropagatesStreamable(t *testing.T) {
	// This will use gottp.CompileTemplate at the package boundary, so the
	// test goes in a separate file/package later. For now, smoke test directly:
	// build a CompiledTemplate manually, run computeTemplateStreamable.
	g := makeGroup(t, "entry*", false, []string{"mac {{ mac | _start_ }}"}, "", nil)
	analyzeStreamability(g)
	if !g.Streamable {
		t.Fatalf("group should be streamable, reasons: %v", g.NonStreamableReasons)
	}

	tmpl := &CompiledTemplate{Groups: []*CompiledGroup{g}}  // adjust field
	computeTemplateStreamable(tmpl)
	if !tmpl.Streamable {
		t.Errorf("template Streamable should be true when all groups streamable")
	}

	// Add a non-streamable group; template flips to false.
	bad := makeGroup(t, "bad*", false, []string{"{{ a }} {{ b }}"}, "", nil)
	analyzeStreamability(bad)
	tmpl.Groups = append(tmpl.Groups, bad)
	computeTemplateStreamable(tmpl)
	if tmpl.Streamable {
		t.Errorf("template Streamable should be false when any group not streamable")
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/compiler/ -count=1
```

Expected: all pass.

- [ ] **Step 5: Run full suite for parity**

```bash
go test ./... -count=1
```

Expected: all packages pass — the new analysis is additive and changes no existing behavior.

- [ ] **Step 6: Commit**

```bash
git add internal/compiler/compiled.go internal/compiler/streamability.go internal/compiler/streamability_test.go
git commit -m "feat(compiler): wire analyzeStreamability into CompileTemplate"
```

## Task A7: Implement groupPath collision check

**Files:**
- Modify: `internal/compiler/streamability.go`
- Modify: `internal/compiler/streamability_test.go`

- [ ] **Step 1: Write failing test**

Append to `streamability_test.go`:

```go
func TestValidateGroupPathCollisions_FooVsFooStar(t *testing.T) {
	a := makeGroup(t, "foo", false, []string{"{{ x }}"}, "", nil)
	b := makeGroup(t, "foo*", false, []string{"{{ y }}"}, "", nil)
	analyzeStreamability(a)
	analyzeStreamability(b)
	tmpl := &CompiledTemplate{Groups: []*CompiledGroup{a, b}}

	err := validateGroupPathCollisions(tmpl)
	if err == nil {
		t.Fatal("expected error for foo / foo* collision, got nil")
	}
	gpErr, ok := err.(*GroupPathCollisionError)
	if !ok {
		t.Fatalf("expected *GroupPathCollisionError, got %T", err)
	}
	if gpErr.NormalizedPath != "foo" {
		t.Errorf("NormalizedPath: got %q, want %q", gpErr.NormalizedPath, "foo")
	}
	if len(gpErr.GroupNames) != 2 {
		t.Errorf("GroupNames: got %v, want 2 entries", gpErr.GroupNames)
	}
}

func TestValidateGroupPathCollisions_DuplicateAlternatives(t *testing.T) {
	// Two groups with identical Name (deliberate alternative-pattern
	// synthesis, like show_iftable_detail's ups-port-virtual-entry*) — no error.
	a := makeGroup(t, "alt*", false, []string{"a {{ x }}"}, "", nil)
	b := makeGroup(t, "alt*", false, []string{"b {{ x }}"}, "", nil)
	analyzeStreamability(a)
	analyzeStreamability(b)
	tmpl := &CompiledTemplate{Groups: []*CompiledGroup{a, b}}

	if err := validateGroupPathCollisions(tmpl); err != nil {
		t.Fatalf("expected no error for identical-name groups, got: %v", err)
	}
}

func TestValidateGroupPathCollisions_DistinctPaths(t *testing.T) {
	a := makeGroup(t, "foo*", false, []string{"{{ x }}"}, "", nil)
	b := makeGroup(t, "bar*", false, []string{"{{ x }}"}, "", nil)
	analyzeStreamability(a)
	analyzeStreamability(b)
	tmpl := &CompiledTemplate{Groups: []*CompiledGroup{a, b}}

	if err := validateGroupPathCollisions(tmpl); err != nil {
		t.Fatalf("expected no error for distinct paths, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test, expect compile error (function undefined)**

```bash
go test ./internal/compiler/ -run TestValidateGroupPathCollisions -v
```

Expected: `validateGroupPathCollisions undefined`, `GroupPathCollisionError undefined`.

- [ ] **Step 3: Implement collision check + error type**

Append to `internal/compiler/streamability.go`:

```go
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
			sortStrings(distinct)
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

func sortStrings(s []string) {
	// Use sort.Strings; importing sort here keeps the API obvious.
	// Inline to avoid an extra import line if sort is already imported elsewhere.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
```

- [ ] **Step 4: Wire into CompileTemplate**

In the same place you added `analyzeStreamabilityRecursive` calls (Task A6, Step 2), after `computeTemplateStreamable(tmpl)`, add:

```go
if err := validateGroupPathCollisions(tmpl); err != nil {
	return nil, err
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/compiler/ -run TestValidateGroupPathCollisions -v
```

Expected: all 3 tests PASS.

- [ ] **Step 6: Run full suite — compile failure on any existing template using foo / foo* would be flagged**

```bash
go test ./... -count=1
```

Expected: all pass. If any existing test template trips the collision check, that's a real bug in the test fixture and should be fixed in the fixture.

- [ ] **Step 7: Commit**

```bash
git add internal/compiler/streamability.go internal/compiler/streamability_test.go internal/compiler/compiled.go
git commit -m "feat(compiler): error on groupPath collision (foo vs foo*)"
```

## Task A8: Define error types in `gottp` top-level package

**Files:**
- Create: `errors.go` (or `streaming_errors.go` if `errors.go` is taken — check first)

- [ ] **Step 1: Check existing errors.go**

```bash
ls /Users/jasonpatterson/Data\ Harbor/gottp/*.go | head
```

If `errors.go` exists, append to it; otherwise create `streaming_errors.go`.

- [ ] **Step 2: Add error types**

```go
package gottp

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTemplateNotStreamable is returned (wrapped) by ParseStream when the
// template's top-level groups don't all pass the streamability check.
// Use errors.Is(err, ErrTemplateNotStreamable) to match.
var ErrTemplateNotStreamable = errors.New("template is not streamable")

// TemplateNotStreamableError carries the per-group reasons explaining why
// a template failed the streamability check. errors.Is matches against
// ErrTemplateNotStreamable.
type TemplateNotStreamableError struct {
	Reasons []string
}

func (e *TemplateNotStreamableError) Error() string {
	if len(e.Reasons) == 0 {
		return "template is not streamable"
	}
	return fmt.Sprintf("template is not streamable: %s", strings.Join(e.Reasons, "; "))
}

func (e *TemplateNotStreamableError) Is(target error) bool {
	return target == ErrTemplateNotStreamable
}

func (e *TemplateNotStreamableError) Unwrap() error {
	return ErrTemplateNotStreamable
}
```

Note: `GroupPathCollisionError` is already in `internal/compiler` (Task A7). The collision error returned by `CompileTemplate` already surfaces at the public API because `CompileTemplate` returns `error`.

- [ ] **Step 3: Add unit tests for the error type**

Create `streaming_errors_test.go`:

```go
package gottp

import (
	"errors"
	"strings"
	"testing"
)

func TestTemplateNotStreamableError_Is(t *testing.T) {
	err := &TemplateNotStreamableError{Reasons: []string{"because reasons"}}
	if !errors.Is(err, ErrTemplateNotStreamable) {
		t.Errorf("errors.Is should match ErrTemplateNotStreamable")
	}
}

func TestTemplateNotStreamableError_Message(t *testing.T) {
	err := &TemplateNotStreamableError{Reasons: []string{"a", "b"}}
	got := err.Error()
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Errorf("error message %q should contain both reasons", got)
	}
	if !strings.Contains(got, ";") {
		t.Errorf("error message %q should join reasons with semicolon", got)
	}
}

func TestTemplateNotStreamableError_EmptyReasons(t *testing.T) {
	err := &TemplateNotStreamableError{}
	got := err.Error()
	if got == "" {
		t.Errorf("error message should not be empty even with no reasons")
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./... -run "TemplateNotStreamable" -v
```

Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add streaming_errors.go streaming_errors_test.go
git commit -m "feat: add TemplateNotStreamableError + ErrTemplateNotStreamable"
```

## Task A9: Add `WhyNotStreamable` helper

**Files:**
- Modify: `streaming_errors.go` (or wherever Task A8 landed)
- Modify: `streaming_errors_test.go`

- [ ] **Step 1: Write failing test**

Append to `streaming_errors_test.go`:

```go
func TestWhyNotStreamable_Streamable(t *testing.T) {
	// Use a template that we know is streamable.
	tmpl := `<group name="entry*">
mac {{ mac | _start_ }}
ip {{ ip }}
</group>`
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	streamable, reasons := WhyNotStreamable(c)
	if !streamable {
		t.Errorf("expected streamable=true, got false; reasons: %v", reasons)
	}
	if len(reasons) != 0 {
		t.Errorf("expected no reasons when streamable, got: %v", reasons)
	}
}

func TestWhyNotStreamable_NotStreamable(t *testing.T) {
	// joinmatches makes it non-streamable.
	tmpl := `<group name="entry*">
desc {{ desc | joinmatches }}
</group>`
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	streamable, reasons := WhyNotStreamable(c)
	if streamable {
		t.Errorf("expected streamable=false")
	}
	if len(reasons) == 0 {
		t.Errorf("expected at least one reason")
	}
	joined := strings.Join(reasons, " ")
	if !strings.Contains(joined, "joinmatches") {
		t.Errorf("expected reason to mention joinmatches; got: %v", reasons)
	}
}
```

- [ ] **Step 2: Run, expect fail**

```bash
go test -run "WhyNotStreamable" ./...
```

Expected: `WhyNotStreamable undefined`.

- [ ] **Step 3: Implement WhyNotStreamable**

Append to `streaming_errors.go`:

```go
// WhyNotStreamable reports whether the template is streamable; if not,
// returns one human-readable reason per non-streamable top-level group.
// Useful for template-readiness audits without round-tripping through
// ParseStream + error inspection.
func WhyNotStreamable(c *CompiledTemplate) (streamable bool, reasons []string) {
	if c == nil || c.compiled == nil {
		return false, []string{"compiled template is nil"}
	}
	if c.compiled.Streamable {
		return true, nil
	}
	for _, g := range c.compiled.Groups {  // adjust field name to match
		if !g.Streamable {
			reasons = append(reasons, g.NonStreamableReasons...)
		}
	}
	return false, reasons
}
```

(Note: `c.compiled` is the internal `*compiler.CompiledTemplate`. Verify the field name on the public `CompiledTemplate` wrapper. If the wrapper uses a different field name, adjust accordingly. If `CompiledTemplate` *is* the internal type re-exported, just use `c.Streamable` and `c.Groups` directly.)

- [ ] **Step 4: Run tests**

```bash
go test -run "WhyNotStreamable" -v ./...
```

Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add streaming_errors.go streaming_errors_test.go
git commit -m "feat: add WhyNotStreamable diagnostic helper"
```

## Task A10: Verify Class 1 prod templates classify as streamable

**Files:**
- Create: `test/streaming/classification_test.go` (new directory)

- [ ] **Step 1: Create the classification fixture test**

```go
package streaming_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/roc-ops/gottp"
)

const class1TemplateRoot = "/Users/jasonpatterson/DH360_Device_Discovery/data/hardware_platforms/casa-systems/casa-chassis/8.8.3.5_build_b851/field-mappings/templates"

var class1Templates = []string{
	"show_cable_modem_verbose.ttp",
	"show_cable_modem_phy.ttp",
	"show_iftable_detail.ttp",
	"show_cable_modem_fec.ttp",
}

func TestClass1TemplatesAreStreamable(t *testing.T) {
	for _, name := range class1Templates {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(class1TemplateRoot, name)
			tmplBytes, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("template not available at %s: %v", path, err)
			}
			c, err := gottp.CompileTemplate(string(tmplBytes))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			streamable, reasons := gottp.WhyNotStreamable(c)
			if !streamable {
				t.Errorf("expected %s to be streamable; reasons: %v", name, reasons)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test**

```bash
go test ./test/streaming/ -v
```

Expected: 4 sub-tests PASS (one per template). If any FAIL, that's a real classification bug — investigate the reason and either fix the rule or document a deliberate non-streamable case.

- [ ] **Step 3: Commit**

```bash
git add test/streaming/classification_test.go
git commit -m "test(streaming): verify Class 1 prod templates classify as streamable"
```

## Phase A Exit Gate

Before moving to Phase B, verify:

- [ ] `go test ./... -count=1` is fully green
- [ ] All 4 Class 1 prod templates classify as streamable
- [ ] groupPath collision check fires on `foo` / `foo*`, doesn't fire on duplicate exact names
- [ ] `errors.Is(err, gottp.ErrTemplateNotStreamable)` works against `*TemplateNotStreamableError`

If any of those fail, fix before continuing.

---

# Phase B — Streaming runtime

Phase B implements the actual `parseGroupStream` runtime variant, the public `ParseStream` API, and the parity test infrastructure. Phase B exits when parity tests pass against synthetic + small prod fixtures.

## Task B1: Audit macro engine reuse

**Files:**
- (Read-only audit — no edits)
- Create: `internal/compiled/macro_reuse_audit.md` (notes file, deleted before Phase B end)

- [ ] **Step 1: Read macro execution code**

Find macro invocation sites:

```bash
grep -rn "starlark.\|RunMacro\|ExecuteMacro" internal/macro/ internal/compiled/ | head -30
```

- [ ] **Step 2: Determine whether engine is per-runtime or per-call**

Key question: does each call to a Starlark macro create a fresh `*starlark.Thread` and re-evaluate the macro source, or does the runtime cache a compiled `*starlark.Function` and reuse it?

Document findings inline in `internal/compiled/macro_reuse_audit.md`:

```markdown
# Macro engine reuse audit

## Starlark
- Macro source compiled at: <file>:<line>
- Per-invocation thread created at: <file>:<line>
- Reused across records: YES / NO
- If NO, what would change in Phase B: <description>

## Native Go
- Registered at: <file>:<line>
- Per-invocation overhead: <description>
- Reused across records: YES / NO
```

- [ ] **Step 3: Verify or document fix needed**

If macros are NOT reused, the Phase B implementation must address it. If they ARE reused, the audit confirms no change needed.

- [ ] **Step 4: Commit the audit**

```bash
git add internal/compiled/macro_reuse_audit.md
git commit -m "docs(compiled): audit macro engine reuse for streaming"
```

## Task B2: Extract merge state machine into a shared helper

**Files:**
- Modify: `internal/compiled/runtime.go`

The merge logic at runtime.go ~2200-2300 (the `currentMatch` / `_start_` / `containsall` filter machinery) currently lives inline in the post-collect phase of `parseGroup`. Extract it so both `parseGroup` and `parseGroupStream` (next task) can drive it.

- [ ] **Step 1: Identify the merge-state-machine block**

```bash
grep -n "currentMatch\|currentStartPatternIdx\|patternMatchCount" internal/compiled/runtime.go | head -30
```

- [ ] **Step 2: Define a `mergeState` struct and helper functions**

Add to `runtime.go` (near the existing `parseGroup`):

```go
// mergeState carries the cross-match state that the parseGroup merge
// phase tracks while walking sorted matches. Streaming and non-streaming
// variants share the same state machine.
type mergeState struct {
	currentMatch           map[string]interface{}
	currentStartPatternIdx int
	currentStartPos        int
	currentStartLineIdx    int
	currentMatchHasEnd     bool
	patternMatchCount      map[int]int
}

func newMergeState() *mergeState {
	return &mergeState{
		patternMatchCount: make(map[int]int),
	}
}

// stepMerge advances the merge state machine one match at a time.
// Returns (completed *map[string]interface{}, ok bool) where the
// completed pointer (when non-nil) is a record ready to emit/append.
// The caller decides whether to buffer or emit.
//
// TODO(streaming): factor out the existing inline merge logic into this
// function during Task B2 step 3 by lifting it directly from parseGroup.
// Until then, this is a no-op stub for the streaming code path; parseGroup
// continues to use its inline logic to preserve behavior.
func (r *Runtime) stepMerge(state *mergeState, m patternMatch, group *compiler.CompiledGroup, joinMatchesVars map[string]bool) (completed map[string]interface{}, ok bool) {
	// To be filled in during Task B2 step 3.
	return nil, false
}
```

- [ ] **Step 3: Move the inline merge logic into stepMerge**

This is the largest single change in the plan. Strategy:

1. Read the entire merge block (likely lines ~2150-2350 in current runtime.go) into your head.
2. Identify the per-match invariants: each iteration of the outer "for each match in sorted matches" loop reads one match, possibly emits a record, updates state.
3. Lift the loop body into `stepMerge`. Mechanical translation:
    - Variables that survive across iterations → fields on `mergeState`.
    - Variables initialized once before the loop → still initialized in `parseGroup` and passed to `stepMerge` via the `state` argument.
    - The "save current match to mergedMatches" path → returns `(currentMatch, true)` from stepMerge instead.
4. `parseGroup` becomes (sketched):
    ```go
    state := newMergeState()
    var mergedMatches []map[string]interface{}
    for _, m := range allMatches {
        if completed, ok := r.stepMerge(state, m, group, joinMatchesVars); ok {
            mergedMatches = append(mergedMatches, completed)
        }
    }
    // Final flush — the last in-flight currentMatch:
    if state.currentMatch != nil {
        // run final filter / append
        mergedMatches = append(mergedMatches, state.currentMatch)
    }
    ```
5. Verify behavior unchanged via the existing test suite.

- [ ] **Step 4: Run full test suite to verify parity**

```bash
go test ./... -count=1
```

Expected: all pass. Any regression here is a refactor bug — fix before proceeding.

- [ ] **Step 5: Run benchmark to verify no perf regression**

```bash
go test ./test/comparison/ -run=^$ -bench=BenchmarkParseGroupCableModem -benchmem -benchtime=20x
```

Compare against pre-refactor numbers (capture from `git stash` + run on prev commit if needed). Expected: within 5% of prior throughput.

- [ ] **Step 6: Commit**

```bash
git add internal/compiled/runtime.go
git commit -m "refactor(compiled): extract merge state machine into stepMerge

Lifts the cross-match merge logic out of parseGroup's inline loop into
a method on Runtime so the upcoming streaming variant can drive the
same state machine without duplicating it."
```

## Task B3: Implement `parseGroupStream`

**Files:**
- Modify: `internal/compiled/runtime.go`

- [ ] **Step 1: Add parseGroupStream**

```go
// parseGroupStream is the streaming variant of parseGroup. Instead of
// collecting every match into allMatches and then merging into a record
// list, it drives the merge state machine match-by-match and invokes
// fn for each completed record, dropping intermediates between records.
//
// Caller must have verified group.Streamable == true; this function
// does not re-check (entry gate is in (*Runtime).ParseStream).
func (r *Runtime) parseGroupStream(
	group *compiler.CompiledGroup,
	inputData string,
	vars map[string]interface{},
	fn func(match map[string]interface{}, srcRange [2]int, groupPath string) error,
) error {
	// Compute lineOffsets once (already hoisted in parseGroup).
	lines := strings.Split(inputData, "\n")
	lineOffsets := make([]int, len(lines)+1)
	off := 0
	for i, line := range lines {
		lineOffsets[i] = off
		off += len(line) + 1
	}
	lineOffsets[len(lines)] = off

	// joinMatchesVars: empty for streamable groups (rule 3 forbids joinmatches).
	joinMatchesVars := map[string]bool{}

	state := newMergeState()
	emit := func(record map[string]interface{}, m patternMatch) error {
		// Group filter (containsall, etc.) — must match parseGroup behavior.
		// For streamable groups the allowlist guarantees per-record filters.
		filtered, err := r.applyGroupFilter(record, group, vars)
		if err != nil {
			return err
		}
		if filtered == nil {
			return nil // dropped by filter
		}
		// Group macro.
		filtered, err = r.applyGroupMacro(filtered, group, vars)
		if err != nil {
			return err
		}
		if filtered == nil {
			return nil // dropped by macro
		}
		srcRange := [2]int{m.spanStart, m.spanEnd}
		return fn(filtered, srcRange, group.NormalizedPath)
	}

	for patternIdx, compiledPattern := range group.Patterns {
		if compiledPattern.HasAnchors {
			for lineIdx, line := range lines {
				line = strings.TrimRight(line, "\r \t")
				match := compiledPattern.Regex.FindStringSubmatch(line)
				if match == nil {
					continue
				}
				result := r.extractMatchResult(match, compiledPattern, vars)
				if result == nil || (len(result) == 0 && !compiledPattern.HasOnlySpecialIndicators && !compiledPattern.IgnoreUsesTemplateVar) {
					continue
				}
				pm := patternMatch{
					patternIdx: patternIdx,
					spanStart:  lineOffsets[lineIdx],
					spanEnd:    lineOffsets[lineIdx] + len(line),
					lineIdx:    lineIdx,
					result:     result,
				}
				if completed, ok := r.stepMerge(state, pm, group, joinMatchesVars); ok {
					if err := emit(completed, pm); err != nil {
						return err
					}
				}
			}
		} else {
			allIndices := compiledPattern.Regex.FindAllStringSubmatchIndex(inputData, -1)
			for _, indices := range allIndices {
				if len(indices) < 2 {
					continue
				}
				matchGroups := make([]string, len(indices)/2)
				for i := 0; i < len(indices); i += 2 {
					if indices[i] >= 0 && indices[i+1] >= 0 {
						matchGroups[i/2] = inputData[indices[i]:indices[i+1]]
					}
				}
				result := r.extractMatchResult(matchGroups, compiledPattern, vars)
				if result == nil || (len(result) == 0 && !compiledPattern.IgnoreUsesTemplateVar) {
					continue
				}
				pm := patternMatch{
					patternIdx: patternIdx,
					spanStart:  indices[0],
					spanEnd:    indices[1],
					lineIdx:    sort.SearchInts(lineOffsets, indices[0]+1) - 1,
					result:     result,
				}
				if completed, ok := r.stepMerge(state, pm, group, joinMatchesVars); ok {
					if err := emit(completed, pm); err != nil {
						return err
					}
				}
			}
		}
	}

	// Flush the final in-flight record at end-of-input.
	if state.currentMatch != nil && len(state.currentMatch) > 0 {
		final := patternMatch{
			patternIdx: state.currentStartPatternIdx,
			spanStart:  state.currentStartPos,
			spanEnd:    lineOffsets[len(lines)],
			lineIdx:    state.currentStartLineIdx,
			result:     state.currentMatch,
		}
		if err := emit(state.currentMatch, final); err != nil {
			return err
		}
	}

	return nil
}
```

(Note: `applyGroupFilter` and `applyGroupMacro` may need to be extracted from existing inline code in `parseGroup` similarly to `stepMerge`. If they don't already exist as discrete methods, add them as part of this task.)

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: pass. If `applyGroupFilter` / `applyGroupMacro` don't exist yet, this fails — add them as helpers extracted from the existing post-merge loop in `parseGroup`.

- [ ] **Step 3: Commit**

```bash
git add internal/compiled/runtime.go
git commit -m "feat(compiled): add parseGroupStream variant for streaming mode"
```

## Task B4: Add `Runtime.ParseStream` method

**Files:**
- Modify: `internal/compiled/runtime.go`

- [ ] **Step 1: Add the method**

```go
// ParseStream is the streaming counterpart to Parse. For each top-level
// group in the compiled template, it drives the streaming runtime and
// invokes fn once per completed record, in (group, scan) order.
//
// Returns *TemplateNotStreamableError (matches gottp.ErrTemplateNotStreamable
// via errors.Is) without invoking fn if any top-level group is not
// streamable. Returning a non-nil error from fn aborts the parse and
// returns that error wrapped.
func (r *Runtime) ParseStream(
	inputs map[string]string,
	vars map[string]interface{},
	options *ParseOptions,
	fn func(match map[string]interface{}, srcRange [2]int, groupPath string) error,
) error {
	if r.compiled == nil {
		return fmt.Errorf("ParseStream: compiled template is nil")
	}
	if !r.compiled.Streamable {
		// Build per-group reasons for the error.
		var reasons []string
		for _, g := range r.compiled.Groups {  // adjust field name
			if !g.Streamable {
				reasons = append(reasons, g.NonStreamableReasons...)
			}
		}
		return wrapNotStreamableError(reasons)  // helper in gottp package; bridge below
	}

	// processInputFunctions, processInputs etc. mirror Parse setup —
	// extract the existing setup from Parse into a shared helper if not
	// already factored.
	if err := r.setupForParse(inputs, vars, options); err != nil {
		return err
	}

	for _, group := range r.compiled.Groups {  // top-level groups
		// Determine which inputs this group is bound to (mirror Parse).
		for _, inputName := range r.inputsForGroup(group) {
			inputData := r.processedInputs[inputName]  // adjust to match
			if err := r.parseGroupStream(group, inputData, vars, fn); err != nil {
				if errors.Is(err, errCallbackAbort) {
					// Already wrapped; just return.
					return err
				}
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 2: Bridge `wrapNotStreamableError`**

The internal package can't directly construct `*gottp.TemplateNotStreamableError` (that's in the public package). Two options:

(a) Define an internal-only equivalent error type in `internal/compiled/`, and have the public `gottp.CompiledTemplate.ParseStream` wrap it.

(b) Have the internal Runtime return reasons + a sentinel error like `errStreamGate`, and the public wrapper builds the public error.

Pick (b). Add to `internal/compiled/runtime.go`:

```go
var errStreamGate = errors.New("template not streamable (internal sentinel)")

// streamGateError carries the per-group reasons through the package
// boundary. The public gottp wrapper translates this into
// *gottp.TemplateNotStreamableError.
type streamGateError struct {
	Reasons []string
}

func (e *streamGateError) Error() string { return errStreamGate.Error() }
func (e *streamGateError) Is(target error) bool { return target == errStreamGate }
func (e *streamGateError) Unwrap() error  { return errStreamGate }

func wrapNotStreamableError(reasons []string) error {
	return &streamGateError{Reasons: reasons}
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add internal/compiled/runtime.go
git commit -m "feat(compiled): add Runtime.ParseStream and stream gate error"
```

## Task B5: Add `CompiledTemplate.ParseStream` public method

**Files:**
- Find the file containing the existing `Parse` method (likely top-level `parse.go` or similar)
- Modify: that file

- [ ] **Step 1: Locate Parse**

```bash
grep -rn "func .*CompiledTemplate.*Parse\b" /Users/jasonpatterson/Data\ Harbor/gottp/*.go
```

- [ ] **Step 2: Add ParseStream alongside Parse**

```go
// ParseStream invokes fn for each record produced by the streamable
// groups in this template, dropping intermediate state between records
// to bound peak heap usage.
//
// Returns *TemplateNotStreamableError (matches ErrTemplateNotStreamable via
// errors.Is) if any top-level group fails the streamability check; in
// that case fn is never invoked.
//
// Calling order: groups in template definition order; within a group,
// matches in input scan order. No cross-group ordering guarantee.
//
// If fn returns non-nil, parsing aborts immediately: that error is
// wrapped and returned; no further fn invocations occur. Already-emitted
// records remain with the caller.
func (c *CompiledTemplate) ParseStream(
	inputs Inputs,
	vars map[string]interface{},
	options *ParseOptions,
	fn func(match map[string]interface{}, srcRange [2]int, groupPath string) error,
) error {
	if fn == nil {
		return fmt.Errorf("ParseStream: callback is nil")
	}

	r := compiled.NewRuntime(c.compiled)  // adjust to match how runtimes are created
	wrappedFn := func(m map[string]interface{}, sr [2]int, gp string) error {
		if err := fn(m, sr, gp); err != nil {
			return fmt.Errorf("ParseStream callback aborted: %w", err)
		}
		return nil
	}

	err := r.ParseStream(map[string]string(inputs), vars, options, wrappedFn)

	// Translate internal sentinel into public error type.
	var sgErr *compiled.StreamGateError  // exported as needed
	if errors.As(err, &sgErr) {
		return &TemplateNotStreamableError{Reasons: sgErr.Reasons}
	}
	return err
}
```

(Adjust types and method calls to match existing conventions in the gottp package. If `compiled.StreamGateError` isn't exported from `internal/compiled`, export it minimally for this purpose, or use an interface assertion.)

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: pass.

- [ ] **Step 4: Commit**

```bash
git add <files>
git commit -m "feat: add CompiledTemplate.ParseStream public API"
```

## Task B6: API contract tests

**Files:**
- Create: `parsestream_test.go` (top-level package)

- [ ] **Step 1: Write tests**

```go
package gottp

import (
	"errors"
	"strings"
	"testing"
)

func TestParseStream_NonStreamable_ReturnsError(t *testing.T) {
	tmpl := `<group name="entry*">
desc {{ desc | joinmatches }}
</group>`
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	called := false
	cb := func(m map[string]interface{}, sr [2]int, gp string) error {
		called = true
		return nil
	}
	err = c.ParseStream(Inputs{"Default_Input": "desc foo\ndesc bar\n"}, nil, nil, cb)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrTemplateNotStreamable) {
		t.Errorf("expected errors.Is(err, ErrTemplateNotStreamable); got %v", err)
	}
	if called {
		t.Error("callback should not have been invoked")
	}
}

func TestParseStream_CallbackError_Aborts(t *testing.T) {
	tmpl := `<group name="entry*">
mac {{ mac | _start_ }}
ip {{ ip }}
</group>`
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	input := "mac aa\nip 1.1.1.1\nmac bb\nip 2.2.2.2\nmac cc\nip 3.3.3.3\n"
	count := 0
	sentinel := errors.New("stop now")
	cb := func(m map[string]interface{}, sr [2]int, gp string) error {
		count++
		if count == 2 {
			return sentinel
		}
		return nil
	}
	err = c.ParseStream(Inputs{"Default_Input": input}, nil, nil, cb)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel; got %v", err)
	}
	if count != 2 {
		t.Errorf("expected callback called exactly 2 times; got %d", count)
	}
}

func TestParseStream_EmptyInput_ReturnsNil(t *testing.T) {
	tmpl := `<group name="entry*">
mac {{ mac | _start_ }}
</group>`
	c, _ := CompileTemplate(tmpl)
	called := false
	err := c.ParseStream(Inputs{"Default_Input": ""}, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			called = true
			return nil
		})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if called {
		t.Error("callback should not have been invoked on empty input")
	}
}

func TestParseStream_NilCallback_ReturnsError(t *testing.T) {
	tmpl := `<group name="entry*">
mac {{ mac | _start_ }}
</group>`
	c, _ := CompileTemplate(tmpl)
	err := c.ParseStream(Inputs{"Default_Input": "mac aa\n"}, nil, nil, nil)
	if err == nil {
		t.Error("expected error for nil callback")
	}
}

func TestParseStream_GroupPathStripsTrailingStar(t *testing.T) {
	tmpl := `<group name="my.path.entry*">
mac {{ mac | _start_ }}
ip {{ ip }}
</group>`
	c, _ := CompileTemplate(tmpl)
	var got string
	c.ParseStream(Inputs{"Default_Input": "mac aa\nip 1.1.1.1\n"}, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			got = gp
			return nil
		})
	if !strings.HasSuffix(got, "entry") || strings.HasSuffix(got, "*") {
		t.Errorf("groupPath %q should strip trailing star, got with star or wrong path", got)
	}
}
```

- [ ] **Step 2: Run**

```bash
go test ./... -run "ParseStream" -v
```

Expected: 5 tests PASS. If callback-abort fails because flush emits a 3rd record before abort propagates, audit `parseGroupStream` to ensure error returns are propagated immediately without continuing the loop.

- [ ] **Step 3: Commit**

```bash
git add parsestream_test.go
git commit -m "test: API contract for ParseStream (gating, abort, empty, nil cb, groupPath)"
```

## Task B7: Parity tests against synthetic + small prod fixtures

**Files:**
- Create: `test/streaming/parity_test.go`

- [ ] **Step 1: Write the parity harness**

```go
package streaming_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/roc-ops/gottp"
)

type parityCase struct {
	name     string
	template string
	input    string
	keyField string // primary key for stable sort
}

var parityCases = []parityCase{
	{
		name:     "simple_start",
		template: `<group name="entry*">
mac {{ mac | _start_ }}
ip {{ ip }}
</group>`,
		input:    "mac aa\nip 1.1.1.1\nmac bb\nip 2.2.2.2\n",
		keyField: "mac",
	},
	// Add more synthetic cases or paths to small prod samples here.
}

func TestParseStream_Parity(t *testing.T) {
	for _, tc := range parityCases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := gottp.CompileTemplate(tc.template)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			inputs := gottp.Inputs{"Default_Input": tc.input}

			// Get records from Parse.
			parseResult, err := c.Parse(inputs, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			parseRecords := flattenRecords(parseResult)

			// Get records from ParseStream.
			var streamRecords []map[string]interface{}
			err = c.ParseStream(inputs, nil, nil,
				func(m map[string]interface{}, sr [2]int, gp string) error {
					streamRecords = append(streamRecords, m)
					return nil
				})
			if err != nil {
				t.Fatalf("ParseStream: %v", err)
			}

			sortByKey(parseRecords, tc.keyField)
			sortByKey(streamRecords, tc.keyField)

			if !reflect.DeepEqual(parseRecords, streamRecords) {
				t.Errorf("parity mismatch\nParse:       %v\nParseStream: %v",
					parseRecords, streamRecords)
			}
		})
	}
}

func flattenRecords(result interface{}) []map[string]interface{} {
	var out []map[string]interface{}
	walk(result, &out)
	return out
}

func walk(v interface{}, out *[]map[string]interface{}) {
	switch t := v.(type) {
	case []interface{}:
		for _, item := range t {
			walk(item, out)
		}
	case map[string]interface{}:
		// If this map looks like a record (has scalar leaves), emit it;
		// otherwise descend.
		isRecord := true
		for _, val := range t {
			switch val.(type) {
			case map[string]interface{}, []interface{}:
				isRecord = false
			}
			if !isRecord {
				break
			}
		}
		if isRecord && len(t) > 0 {
			*out = append(*out, t)
			return
		}
		for _, val := range t {
			walk(val, out)
		}
	}
}

func sortByKey(records []map[string]interface{}, key string) {
	sort.SliceStable(records, func(i, j int) bool {
		ai, _ := records[i][key].(string)
		bi, _ := records[j][key].(string)
		return ai < bi
	})
}

// Add small-prod-fixture cases (build-tag the heavy ones if needed):
func TestParseStream_Parity_SmallProd(t *testing.T) {
	root := "/Users/jasonpatterson/DH360_Device_Discovery/data/hardware_platforms/casa-systems/casa-chassis/8.8.3.5_build_b851"
	prodCases := []struct {
		name     string
		template string
		input    string
		keyField string
	}{
		// Use the smaller dev samples in the discovery repo, NOT the
		// 46 MB prod captures (those go in the build-tagged Phase C tests).
		{"verbose_dev_sample", "field-mappings/templates/show_cable_modem_verbose.ttp", "raw/show_cable_modem_verbose.txt", "mac-address"},
		{"phy_dev_sample", "field-mappings/templates/show_cable_modem_phy.ttp", "raw/show_cable_modem_phy.txt", "mac-address"},
		{"fec_dev_sample", "field-mappings/templates/show_cable_modem_fec.ttp", "raw/show_cable_modem_fec.txt", "mac-address"},
	}

	for _, pc := range prodCases {
		t.Run(pc.name, func(t *testing.T) {
			tmplBytes, err := os.ReadFile(filepath.Join(root, pc.template))
			if err != nil {
				t.Skipf("template not available: %v", err)
			}
			inputBytes, err := os.ReadFile(filepath.Join(root, pc.input))
			if err != nil {
				t.Skipf("input not available: %v", err)
			}
			c, err := gottp.CompileTemplate(string(tmplBytes))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			inputs := gottp.Inputs{"Default_Input": string(inputBytes)}

			parseResult, err := c.Parse(inputs, nil, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			parseRecords := flattenRecords(parseResult)

			var streamRecords []map[string]interface{}
			err = c.ParseStream(inputs, nil, nil,
				func(m map[string]interface{}, sr [2]int, gp string) error {
					streamRecords = append(streamRecords, m)
					return nil
				})
			if err != nil {
				t.Fatalf("ParseStream: %v", err)
			}

			sortByKey(parseRecords, pc.keyField)
			sortByKey(streamRecords, pc.keyField)

			if len(parseRecords) != len(streamRecords) {
				t.Fatalf("record count mismatch: Parse=%d ParseStream=%d", len(parseRecords), len(streamRecords))
			}
			for i := range parseRecords {
				if !reflect.DeepEqual(parseRecords[i], streamRecords[i]) {
					t.Errorf("record %d mismatch:\nParse:       %v\nParseStream: %v",
						i, parseRecords[i], streamRecords[i])
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run**

```bash
go test ./test/streaming/ -v
```

Expected: parity tests PASS. Any divergence is a behavior bug in the streaming variant — fix in `parseGroupStream` or `stepMerge` before continuing.

- [ ] **Step 3: Commit**

```bash
git add test/streaming/parity_test.go
git commit -m "test(streaming): parity between Parse and ParseStream on synthetic + small prod"
```

## Phase B Exit Gate

Before moving to Phase C, verify:

- [ ] `go test ./... -count=1` is fully green
- [ ] Parity tests pass on synthetic + all 4 Class 1 dev fixtures
- [ ] Macro audit results documented (and fix applied if reuse was missing)

---

# Phase C — Production-scale validation

Phase C verifies that ParseStream meets the explicit success criteria from the spec on real production-scale inputs.

## Task C1: Memory-bound test (build-tagged)

**Files:**
- Create: `test/comparison/streaming_memory_test.go`

- [ ] **Step 1: Write the test**

```go
//go:build prodbaseline

package comparison

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

func TestParseStream_MemoryBound_Phy(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join(casaChassisB851, "show_cable_modem_phy.ttp"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile(filepath.Join(prodCaptureRoot, "show_cable_modem_phy.txt"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := gottp.CompileTemplate(string(tmpl))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	inputs := gottp.Inputs{"Default_Input": string(input)}

	runtime.GC(); runtime.GC()
	time.Sleep(150 * time.Millisecond)
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)

	var peakHeap uint64 = msBefore.HeapInuse
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		var ms runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				runtime.ReadMemStats(&ms)
				for {
					cur := atomic.LoadUint64(&peakHeap)
					if ms.HeapInuse <= cur {
						break
					}
					if atomic.CompareAndSwapUint64(&peakHeap, cur, ms.HeapInuse) {
						break
					}
				}
			}
		}
	}()

	count := 0
	err = c.ParseStream(inputs, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			count++
			return nil
		})
	close(stop)
	<-done
	if err != nil {
		t.Fatalf("ParseStream: %v", err)
	}

	peak := atomic.LoadUint64(&peakHeap)
	delta := int64(peak) - int64(msBefore.HeapInuse)
	deltaMB := float64(delta) / 1024.0 / 1024.0
	fmt.Printf("ParseStream show_cable_modem_phy: %d records, peak_delta=%.2f MB\n", count, deltaMB)

	const limitMB = 20.0
	if deltaMB > limitMB {
		t.Errorf("peak heap delta %.2f MB exceeds limit %.2f MB", deltaMB, limitMB)
	}
}
```

- [ ] **Step 2: Run**

```bash
go test -tags prodbaseline -run TestParseStream_MemoryBound_Phy -v -timeout 30m ./test/comparison/
```

Expected: PASS with `peak_delta` < 20 MB. If it fails — investigate. The most likely culprit is a buffer that should have been freed but isn't, or a macro engine that's leaking state.

- [ ] **Step 3: Commit**

```bash
git add test/comparison/streaming_memory_test.go
git commit -m "test(streaming): memory bound (peak delta < 20 MB on phy prod fixture)"
```

## Task C2: Performance regression guard

**Files:**
- Create: `test/comparison/streaming_perf_test.go`

- [ ] **Step 1: Write the test**

```go
//go:build prodbaseline

package comparison

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

func TestParseStream_NoRegression_Verbose(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join(casaChassisB851, "show_cable_modem_verbose.ttp"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile(filepath.Join(prodCaptureRoot, "show_cable_modem_verbose.txt"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := gottp.CompileTemplate(string(tmpl))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	inputs := gottp.Inputs{"Default_Input": string(input)}

	// Warm up.
	c.Parse(inputs, nil, nil)

	t0 := time.Now()
	_, err = c.Parse(inputs, nil, nil)
	parseTime := time.Since(t0)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	t0 = time.Now()
	count := 0
	err = c.ParseStream(inputs, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			count++
			return nil
		})
	streamTime := time.Since(t0)
	if err != nil {
		t.Fatalf("ParseStream: %v", err)
	}

	ratio := float64(streamTime) / float64(parseTime)
	fmt.Printf("Parse=%v ParseStream=%v ratio=%.2f\n", parseTime, streamTime, ratio)

	const maxRatio = 1.10
	if ratio > maxRatio {
		t.Errorf("ParseStream is %.2fx slower than Parse (limit %.2fx)", ratio, maxRatio)
	}
}
```

- [ ] **Step 2: Run**

```bash
go test -tags prodbaseline -run TestParseStream_NoRegression_Verbose -v -timeout 30m ./test/comparison/
```

Expected: PASS with `ratio` < 1.10. If it fails, look for places where streaming does extra work the buffered path doesn't (allocations per record that buffered amortizes, etc.).

- [ ] **Step 3: Commit**

```bash
git add test/comparison/streaming_perf_test.go
git commit -m "test(streaming): perf regression guard (ParseStream within 10% of Parse)"
```

## Task C3: Final integration sweep across all 4 prod fixtures

**Files:**
- Modify: `test/comparison/streaming_memory_test.go`

- [ ] **Step 1: Add subtests for the other 3 fixtures**

Replace the single test in `streaming_memory_test.go` with a table-driven sweep covering all 4 streaming candidates. Set per-template peak-delta limits based on observed record sizes:

```go
var streamingMemoryLimits = []struct {
	name      string
	template  string
	input     string
	limitMB   float64
}{
	{"show_cable_modem_verbose", "show_cable_modem_verbose.ttp", "show_cable_modem_verbose.txt", 25.0},
	{"show_cable_modem_phy", "show_cable_modem_phy.ttp", "show_cable_modem_phy.txt", 20.0},
	{"show_iftable_detail", "show_iftable_detail.ttp", "show_iftable_detail.txt", 15.0},
	{"show_cable_modem_fec", "show_cable_modem_fec.ttp", "show_cable_modem_fec.txt", 15.0},
}

func TestParseStream_MemoryBound_All(t *testing.T) {
	for _, lc := range streamingMemoryLimits {
		t.Run(lc.name, func(t *testing.T) {
			// (move the body of TestParseStream_MemoryBound_Phy here,
			// parameterized by lc — same heap sampler, same emit-counting cb)
		})
	}
}
```

- [ ] **Step 2: Run the full sweep**

```bash
go test -tags prodbaseline -run TestParseStream_MemoryBound_All -v -timeout 60m ./test/comparison/
```

Expected: all 4 sub-tests PASS. Adjust per-template limits up or down based on observed values; the per-record size varies (verbose has ~80-field records, fec has ~7 fields).

- [ ] **Step 3: Commit**

```bash
git add test/comparison/streaming_memory_test.go
git commit -m "test(streaming): memory bound across all 4 Class 1 prod fixtures"
```

## Phase C Exit Gate

- [ ] All 4 prod fixtures: peak_delta under per-template limit
- [ ] Verbose fixture: ParseStream within 10% of Parse runtime
- [ ] Full `go test ./... -count=1` still green

---

# Phase D — Out of scope for this plan

Phase D is the deploy + DH sync + observe-next-prod-profile cycle. That happens after this plan ships and is tracked outside the gottp repo as the validation step.

---

# Final wrap

## Task F1: Update README + godoc

**Files:**
- Modify: `README.md`
- (godoc is implicit via doc comments already added in Tasks B5, A8, A9)

- [ ] **Step 1: Add a "Streaming mode" subsection to README**

Brief description, point at the spec doc, sample call site:

```markdown
## Streaming mode (ParseStream)

For templates with high-cardinality output (thousands of records) and a
clear repeating record boundary, `ParseStream` emits records one at a
time via callback, bounding peak heap usage:

```go
err := compiled.ParseStream(inputs, nil, nil,
    func(record map[string]interface{}, srcRange [2]int, groupPath string) error {
        return handle(record)
    })
```

Returns `*TemplateNotStreamableError` (matching `errors.Is(err, gottp.ErrTemplateNotStreamable)`)
on templates that don't meet the streamability criteria. Use
`gottp.WhyNotStreamable(compiled)` to audit a template before calling.

See `docs/superpowers/specs/2026-04-27-streaming-parsegroup-design.md`
for full design and rationale.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: document ParseStream in README"
```

## Task F2: Open PR for review

- [ ] **Step 1: Push branch**

```bash
git push -u origin feat/parsestream-v1
```

- [ ] **Step 2: Open PR with summary referencing #20 and the design spec**

```bash
gh pr create --base main --title "feat: streaming ParseStream API (refs #20)" --body "..."
```

PR body should reference: design spec path, success criteria from the spec, baseline numbers, post-streaming numbers, what's deferred to v2.

- [ ] **Step 3: Wait for review, address feedback, merge**

---

## Plan self-review notes

- Spec coverage: each of the 5 success criteria from the spec maps to at least one task (functional → A8, B4, B5; parity → B7; memory → C1, C3; throughput → C2; diagnostic → A9, A10).
- The macro engine reuse open-item (spec → "Open items / risks") is task B1 with a fix path inside Phase B if needed.
- Group-function allowlist finalization (spec → "Open items / risks") is task A3.
- DH split-timing data (spec → "Open items / risks") is explicitly outside this plan; tracked separately on issue #20.
- Tasks A1–A10 are Phase A; B1–B7 are Phase B; C1–C3 are Phase C; F1–F2 are wrap. Each phase has an explicit exit gate.
