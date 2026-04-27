package pattern

import (
	"strings"
	"testing"
)

// TestCachedFlagsParity verifies that the compile-time cached booleans on
// CompiledPattern and MatchVariable match the on-the-fly computation that
// the runtime previously did per-match. Each case exercises a different
// indicator/function combination so a regression in the population logic
// shows up here rather than as a behavior change in parsing.
func TestCachedFlagsParity(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"plain", "interface {{ interface }} ip {{ ip }}"},
		{"with_set", "interface {{ interface }} {{ source | set('cli') }}"},
		{"with_joinmatches", "description {{ desc | joinmatches }}"},
		{"start_indicator", "_start_"},
		{"end_indicator", "_end_"},
		{"line_indicator", "_line_"},
		{"with_ignore", "{{ ignore }} {{ name }}"},
		{"only_specials", "_start_"},
	}

	engine := NewEngine()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cp, err := engine.CompilePattern(tc.line, false, false)
			if err != nil {
				t.Fatalf("CompilePattern(%q): %v", tc.line, err)
			}

			// Pattern-level flags
			expectedHasAnchors := strings.Contains(cp.Regex.String(), "^") ||
				strings.Contains(cp.Regex.String(), "$")
			if cp.HasAnchors != expectedHasAnchors {
				t.Errorf("HasAnchors: got %v, want %v", cp.HasAnchors, expectedHasAnchors)
			}

			expectedHasOnlySpecial := true
			for varName := range cp.Variables {
				if varName != "ignore" && varName != "_start_" && varName != "_end_" &&
					varName != "_line_" && varName != "_exact_" && varName != "_exact_space_" {
					expectedHasOnlySpecial = false
					break
				}
			}
			if cp.HasOnlySpecialIndicators != expectedHasOnlySpecial {
				t.Errorf("HasOnlySpecialIndicators: got %v, want %v",
					cp.HasOnlySpecialIndicators, expectedHasOnlySpecial)
			}

			expectedIgnoreUsesTemplateVar := false
			for _, v := range cp.Variables {
				if v.Name == "ignore" && v.IgnoreUsesTemplateVar {
					expectedIgnoreUsesTemplateVar = true
					break
				}
			}
			if cp.IgnoreUsesTemplateVar != expectedIgnoreUsesTemplateVar {
				t.Errorf("IgnoreUsesTemplateVar: got %v, want %v",
					cp.IgnoreUsesTemplateVar, expectedIgnoreUsesTemplateVar)
			}

			expectedPatternHasJoinMatches := false
			for _, v := range cp.Variables {
				for _, f := range v.Functions {
					if strings.HasPrefix(f, "joinmatches") {
						expectedPatternHasJoinMatches = true
						break
					}
				}
				if expectedPatternHasJoinMatches {
					break
				}
			}
			if cp.HasJoinMatches != expectedPatternHasJoinMatches {
				t.Errorf("CompiledPattern.HasJoinMatches: got %v, want %v",
					cp.HasJoinMatches, expectedPatternHasJoinMatches)
			}

			// Variable-level flags
			for _, v := range cp.Variables {
				expectedHasSet := false
				for _, f := range v.Functions {
					if strings.HasPrefix(f, "set(") {
						expectedHasSet = true
						break
					}
				}
				if v.HasSet != expectedHasSet {
					t.Errorf("var %q HasSet: got %v, want %v", v.Name, v.HasSet, expectedHasSet)
				}

				expectedVarHasJoinMatches := false
				for _, f := range v.Functions {
					if strings.HasPrefix(f, "joinmatches") {
						expectedVarHasJoinMatches = true
						break
					}
				}
				if v.HasJoinMatches != expectedVarHasJoinMatches {
					t.Errorf("var %q HasJoinMatches: got %v, want %v",
						v.Name, v.HasJoinMatches, expectedVarHasJoinMatches)
				}
			}
		})
	}
}
