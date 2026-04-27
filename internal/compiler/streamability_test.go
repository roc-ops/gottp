package compiler

import (
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

