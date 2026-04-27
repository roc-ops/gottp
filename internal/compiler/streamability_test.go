package compiler

import (
	"regexp"
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
	// The pattern engine always adds ^ / $ anchors, so the only way to produce a
	// CompiledPattern with HasAnchors=false is to construct it directly. This
	// exercises the rule: no _start_ AND not line-anchored → non-streamable.
	cp := &pattern.CompiledPattern{
		Regex: regexp.MustCompile(`(\S+)\s+(\S+)`), // no ^ or $ anchors
		Variables: map[string]*pattern.MatchVariable{
			"a": {Name: "a"},
			"b": {Name: "b"},
		},
		VariableOrder: []string{"a", "b"},
		HasAnchors:    false,
	}
	g := &CompiledGroup{
		Name:     "entry*",
		IsNested: false,
		Patterns: []*pattern.CompiledPattern{cp},
	}
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

