package compiler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/parser"
)

// The expectations in testdata/indicatorprofile/expected.json come from Python
// TTP 0.10.1 itself: dump_ttp_profile.py loads every template (the curated
// cases/*.ttp and every test/ttpsuite case) and records, per group and in line
// order, which list TTP put each regex in (start_re / end_re / re), its ACTION
// and its IS_LINE flag. Regenerate with:
//
//	python -I internal/compiler/testdata/indicatorprofile/dump_ttp_profile.py

type ttpPatternProfile struct {
	Role   string `json:"role"`
	Action string `json:"action"`
	IsLine bool   `json:"is_line"`
}

type ttpGroupProfile struct {
	Name     string              `json:"name"`
	Method   string              `json:"method"`
	Patterns []ttpPatternProfile `json:"patterns"`
	Children []ttpGroupProfile   `json:"children"`
}

type ttpTemplateProfile struct {
	Groups []ttpGroupProfile `json:"groups"`
	Error  string            `json:"error"`
}

// profileSkips lists templates whose profile can't be compared because gottp
// builds a different set of groups or patterns from them than TTP does (a
// compile difference, not a partition one). The test fails when a listed
// template starts comparing, so the table can't go stale.
var profileSkips = map[string]string{
	"ttpsuite/test_answers_and_docs/test_issue_48_answer":      "#112 chain() naming a <vars> list is ignored, so joinmatches from the chain is lost",
	"ttpsuite/test_answers_and_docs/test_issue_48_answer_more": "#112 chain() naming a <vars> list is ignored, so joinmatches from the chain is lost",
	// gottp copies the extended file's lines into the anonymous group; TTP
	// nests the extended group under it. Results agree, the structure doesn't.
	"ttpsuite/test_extend_tag/test_extend_tag_from_file_anonymous":                     "extend into an anonymous template inlines the group",
	"ttpsuite/test_extend_tag/test_extend_tag_from_file_nested_group_filter":           "#74 <extend> inside a <group> is ignored",
	"ttpsuite/test_extend_tag/test_extend_tag_within_group":                            "#74 <extend> inside a <group> is ignored",
	"ttpsuite/test_extend_tag/test_extend_tag_within_group_with_anonymous_group":       "#74 <extend> inside a <group> is ignored",
	"ttpsuite/test_extend_tag/test_extend_tag_within_group_with_child_groups_above":    "#74 <extend> inside a <group> is ignored",
	"ttpsuite/test_extend_tag/test_extend_tag_within_group_with_multiple_groups":       "#74 <extend> inside a <group> is ignored",
	"ttpsuite/test_extend_tag/test_extend_tag_within_group_with_non_hierarch_template": "#74 <extend> inside a <group> is ignored",
}

func loadTTPProfiles(t *testing.T) map[string]ttpTemplateProfile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "indicatorprofile", "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Templates map[string]ttpTemplateProfile `json:"templates"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file.Templates
}

// profileTemplates returns every template the expectations were generated
// from, by the same names dump_ttp_profile.py uses.
func profileTemplates(t *testing.T) map[string]string {
	t.Helper()
	templates := map[string]string{}
	paths, _ := filepath.Glob(filepath.Join("testdata", "indicatorprofile", "cases", "*.ttp"))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		templates["cases/"+strings.TrimSuffix(filepath.Base(p), ".ttp")] = string(data)
	}
	suite, _ := filepath.Glob(filepath.Join("..", "..", "test", "ttpsuite", "testdata", "cases", "*.json"))
	for _, p := range suite {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct {
			Name     string `json:"name"`
			Template string `json:"template"`
		}
		if err := json.Unmarshal(data, &cases); err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			templates["ttpsuite/"+c.Name] = c.Template
		}
	}
	return templates
}

// gotProfile renders a compiled group's profile in TTP's terms: one entry per
// pattern TTP compiles (RoleNone lines are not TTP regexes).
func gotProfile(g *CompiledGroup) []ttpPatternProfile {
	prof := g.Profile()
	var out []ttpPatternProfile
	for i := range g.Patterns {
		pp := prof.Pattern(i)
		if pp.Role == RoleNone {
			continue
		}
		out = append(out, ttpPatternProfile{Role: pp.Role.String(), Action: pp.Action.String(), IsLine: pp.IsLine})
	}
	return out
}

func compareGroups(path string, want []ttpGroupProfile, got []*CompiledGroup) []string {
	var diffs []string
	if len(want) != len(got) {
		return []string{fmt.Sprintf("%s: TTP has %d groups, gottp %d", path, len(want), len(got))}
	}
	for i, w := range want {
		g := got[i]
		gp := fmt.Sprintf("%s/%s", path, w.Name)
		patterns := gotProfile(g)
		if len(w.Patterns) != len(patterns) {
			diffs = append(diffs, fmt.Sprintf("%s: TTP has %d patterns, gottp %d", gp, len(w.Patterns), len(patterns)))
		} else {
			for j := range w.Patterns {
				if w.Patterns[j] != patterns[j] {
					diffs = append(diffs, fmt.Sprintf("%s pattern %d %q: TTP %+v, gottp %+v", gp, j, g.Patterns[j].Original, w.Patterns[j], patterns[j]))
				}
			}
		}
		diffs = append(diffs, compareGroups(gp, w.Children, g.Groups)...)
	}
	return diffs
}

func TestIndicatorProfileMatchesTTP(t *testing.T) {
	want := loadTTPProfiles(t)
	templates := profileTemplates(t)

	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)

	// Suite templates read ./assets/... relative to TTP's pytest directory,
	// as they do when the suite runs.
	t.Chdir(filepath.Join("..", "..", "test", "ttpsuite", "testdata", "pytest"))

	for _, name := range names {
		w := want[name]
		t.Run(name, func(t *testing.T) {
			if w.Error != "" {
				t.Skipf("TTP itself fails to load this template (%s)", w.Error)
			}
			text, ok := templates[name]
			if !ok {
				t.Fatalf("expected.json names a template that no longer exists; rerun dump_ttp_profile.py")
			}
			var diffs []string
			tmpl, err := parser.ParseTemplate(text)
			if err != nil {
				diffs = []string{"gottp parse: " + err.Error()}
			} else if compiled, err := NewCompiler().CompileTemplate(tmpl); err != nil {
				diffs = []string{"gottp compile: " + err.Error()}
			} else {
				diffs = compareGroups("", w.Groups, compiled.Groups)
			}
			reason, skipped := profileSkips[name]
			switch {
			case skipped && len(diffs) == 0:
				t.Fatalf("listed in profileSkips (%s) but matches TTP now; remove it", reason)
			case skipped:
				t.Skipf("%s: %s", reason, diffs[0])
			case len(diffs) > 0:
				t.Errorf("profile differs from Python TTP:\n  %s", strings.Join(diffs, "\n  "))
			}
		})
	}
	for name := range templates {
		if _, ok := want[name]; !ok {
			t.Errorf("%s has no expectation; rerun dump_ttp_profile.py", name)
		}
	}
	for name := range profileSkips {
		if _, ok := want[name]; !ok {
			t.Errorf("profileSkips names unknown template %s", name)
		}
	}
}

// TestLoadCompiledTemplateRebuildsProfile checks that the profile is derived,
// not saved: a loaded template gets a freshly built profile, so templates
// saved under older rules also get the current ones.
func TestLoadCompiledTemplateRebuildsProfile(t *testing.T) {
	tmpl, err := parser.ParseTemplate(`<template>
<group name="parent">
router bgp {{ asn }}
 <group name="nbr">
 neighbor {{ ip }} remote-as {{ ras }}
 </group>
 tailstart {{ ts | _start_ }}
!{{ _end_ }}
</group>
</template>`)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := NewCompiler().CompileTemplate(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	want := gotProfile(compiled.Groups[0])
	wantChild := gotProfile(compiled.Groups[0].Groups[0])

	for _, format := range []SerializationFormat{FormatJSON, FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			data, err := SaveCompiledTemplateToBytes(compiled, format)
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := LoadCompiledTemplateFromBytes(data, format)
			if err != nil {
				t.Fatal(err)
			}
			g := loaded.Groups[0]
			if g.profile.Load() == nil || g.Groups[0].profile.Load() == nil {
				t.Fatal("LoadCompiledTemplate did not rebuild the profiles")
			}
			if got := gotProfile(g); fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("parent profile after load = %v, want %v", got, want)
			}
			if got := gotProfile(g.Groups[0]); fmt.Sprint(got) != fmt.Sprint(wantChild) {
				t.Errorf("child profile after load = %v, want %v", got, wantChild)
			}
			// The tail line keeps its role (ordinary, not start) after the round trip.
			found := false
			for i, cp := range g.Patterns {
				if strings.Contains(cp.Original, "tailstart") {
					found = true
					if pp := g.Profile().Pattern(i); pp.Role != RoleOrdinary || !pp.Tail {
						t.Errorf("tail line after load = %+v, want an ordinary tail pattern", pp)
					}
				}
			}
			if !found {
				t.Fatal("tailstart pattern not found")
			}
		})
	}
}

// TestIndicatorProfileLineShapes covers the gottp-side facts the profile adds
// to TTP's partition: which lines match at all, which may match a blank line,
// bare {{ _line_ }} / {{ _start_ }} lines, and which matches start a record.
func TestIndicatorProfileLineShapes(t *testing.T) {
	tmpl, err := parser.ParseTemplate(`<template>
<group name="g">
interface {{ name }}
 shutdown
{{ _line_ }}
{{ _start_ }}
 vlan {{ v | _start_ }}
 role {{ r | set("leaf") }}
 exit {{ _end_ }}
 <group name="c">
 c {{ x }}
 </group>
 tail {{ t | _start_ }}
</group>
</template>`)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := NewCompiler().CompileTemplate(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	g := compiled.Groups[0]
	prof := g.Profile()
	type shape struct {
		matches, blank, bareLine, blankStart, startsRecord, isStart, isEnd, tail bool
	}
	want := map[string]shape{
		"interface {{ name }}":        {matches: true, startsRecord: true, isStart: true},
		" shutdown":                   {},
		"{{ _line_ }}":                {matches: true, blank: true, bareLine: true},
		"{{ _start_ }}":               {matches: true, blank: true, blankStart: true, startsRecord: true, isStart: true},
		" vlan {{ v | _start_ }}":     {matches: true, startsRecord: true, isStart: true},
		` role {{ r | set("leaf") }}`: {matches: true},
		" exit {{ _end_ }}":           {matches: true, isEnd: true},
		" tail {{ t | _start_ }}":     {matches: true, startsRecord: true, tail: true},
	}
	seen := 0
	for i, cp := range g.Patterns {
		w, ok := want[cp.Original]
		if !ok {
			continue
		}
		seen++
		pp := prof.Pattern(i)
		got := shape{
			matches: prof.Matches(i), blank: prof.MatchesBlank(i), bareLine: prof.IsBareLine(i),
			blankStart: pp.BlankStart, startsRecord: prof.StartsRecord(i), isStart: prof.IsStart(i),
			isEnd: prof.IsEnd(i), tail: pp.Tail,
		}
		if got != w {
			t.Errorf("%q: got %+v, want %+v", cp.Original, got, w)
		}
	}
	if seen != len(want) {
		t.Errorf("found %d of %d lines", seen, len(want))
	}
	if !prof.HasBareLine() || !prof.HasBlankStart() || !prof.HasIndicatedStart() || !prof.HasEnd() {
		t.Errorf("group flags: bareLine=%v blankStart=%v indicated=%v end=%v",
			prof.HasBareLine(), prof.HasBlankStart(), prof.HasIndicatedStart(), prof.HasEnd())
	}
}
