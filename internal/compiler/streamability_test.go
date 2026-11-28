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

	analyzeStreamability(g, nil)

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
	analyzeStreamability(g, nil)

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
	analyzeStreamability(parent, nil)

	if parent.Streamable {
		t.Fatal("expected Streamable=false (parent has nested child)")
	}
	if !containsString(parent.NonStreamableReasons, "nested child group") {
		t.Errorf("expected reason mentioning nested child, got: %v", parent.NonStreamableReasons)
	}
}

func TestAnalyzeStreamability_NestedGroupItself(t *testing.T) {
	g := makeGroup(t, "inner*", true, []string{"{{ y }}"}, "", nil)
	analyzeStreamability(g, nil)
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
	analyzeStreamability(g, nil)
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
	analyzeStreamability(g, nil)
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
	analyzeStreamability(g, nil)
	if !g.Streamable {
		t.Fatalf("expected Streamable=true (line-anchored), got false; reasons: %v", g.NonStreamableReasons)
	}
}

func TestAnalyzeStreamability_NormalizedPath(t *testing.T) {
	g := makeGroup(t, "casa-ios-cli.show_cable_modem_verbose.cm-entry*", false,
		[]string{"mac {{ mac | _start_ }}"}, "", nil)
	analyzeStreamability(g, nil)
	if g.NormalizedPath != "casa-ios-cli.show_cable_modem_verbose.cm-entry" {
		t.Errorf("NormalizedPath: got %q, want %q (suffix * should be stripped)",
			g.NormalizedPath, "casa-ios-cli.show_cable_modem_verbose.cm-entry")
	}

	// Group without trailing *.
	g2 := makeGroup(t, "plain", false, []string{"mac {{ mac | _start_ }}"}, "", nil)
	analyzeStreamability(g2, nil)
	if g2.NormalizedPath != "plain" {
		t.Errorf("NormalizedPath: got %q, want %q (no suffix to strip)",
			g2.NormalizedPath, "plain")
	}
}

func TestCompileTemplate_PropagatesStreamable(t *testing.T) {
	g := makeGroup(t, "entry*", false, []string{"mac {{ mac | _start_ }}"}, "", nil)
	analyzeStreamability(g, nil)
	if !g.Streamable {
		t.Fatalf("group should be streamable, reasons: %v", g.NonStreamableReasons)
	}

	tmpl := &CompiledTemplate{Groups: []*CompiledGroup{g}}
	computeTemplateStreamable(tmpl)
	if !tmpl.Streamable {
		t.Errorf("template Streamable should be true when all groups streamable")
	}

	// Add a non-streamable group; template flips to false. Use itemize to
	// guarantee non-streamability (engine auto-anchors so we can't rely on
	// "no record boundary" via plain {{ a }} {{ b }} — see A5 deviation).
	bad := makeGroup(t, "bad*", false, []string{"mac {{ mac | _start_ }}"}, "itemize", nil)
	analyzeStreamability(bad, nil)
	if bad.Streamable {
		t.Fatalf("expected bad to be non-streamable")
	}
	tmpl.Groups = append(tmpl.Groups, bad)
	computeTemplateStreamable(tmpl)
	if tmpl.Streamable {
		t.Errorf("template Streamable should be false when any group not streamable")
	}
}

func TestValidateGroupPathCollisions_FooVsFooStar(t *testing.T) {
	a := makeGroup(t, "foo", false, []string{"{{ x }}"}, "", nil)
	b := makeGroup(t, "foo*", false, []string{"{{ y }}"}, "", nil)
	analyzeStreamability(a, nil)
	analyzeStreamability(b, nil)
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
	analyzeStreamability(a, nil)
	analyzeStreamability(b, nil)
	tmpl := &CompiledTemplate{Groups: []*CompiledGroup{a, b}}

	if err := validateGroupPathCollisions(tmpl); err != nil {
		t.Fatalf("expected no error for identical-name groups, got: %v", err)
	}
}

func TestValidateGroupPathCollisions_DistinctPaths(t *testing.T) {
	a := makeGroup(t, "foo*", false, []string{"{{ x }}"}, "", nil)
	b := makeGroup(t, "bar*", false, []string{"{{ x }}"}, "", nil)
	analyzeStreamability(a, nil)
	analyzeStreamability(b, nil)
	tmpl := &CompiledTemplate{Groups: []*CompiledGroup{a, b}}

	if err := validateGroupPathCollisions(tmpl); err != nil {
		t.Fatalf("expected no error for distinct paths, got: %v", err)
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

// #130: functions= and chain= separate functions with "|", so an aggregating
// function after a pipe must not slip past the allowlist.
func TestParseGroupFunctionNames_PipesAndCommas(t *testing.T) {
	got := parseGroupFunctionNames("contains('ip') | itemize(key='name') | to_int")
	want := []string{"contains", "itemize", "to_int"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("parseGroupFunctionNames: got %v, want %v", got, want)
	}
	got = parseGroupFunctionNames("containsall('mac-address', 'us-intf')")
	if strings.Join(got, ",") != "containsall" {
		t.Errorf("argument commas: got %v, want [containsall]", got)
	}
}

// #130: chain= is checked against the allowlist (it wins over functions=,
// as in parseGroup), and attribute-style group functions, method="table",
// group defaults and dynamic paths make a group non-streamable.
func TestAnalyzeStreamability_FeaturesStreamingDoesNotImplement(t *testing.T) {
	base := func() *CompiledGroup {
		return makeGroup(t, "entry*", false, []string{"mac {{ mac | _start_ }}", "ip {{ ip }}"}, "", nil)
	}
	cases := []struct {
		name   string
		modify func(g *CompiledGroup)
		reason string
	}{
		{"chain", func(g *CompiledGroup) { g.Chain = "contains('ip') | expand" }, `group function "expand"`},
		{"chain wins over functions", func(g *CompiledGroup) { g.Functions = "contains('ip')"; g.Chain = "itemize" }, `group function "itemize"`},
		{"contains attribute", func(g *CompiledGroup) { g.Attributes = map[string]string{"contains": "ip"} }, "contains= attribute"},
		{"expand attribute without value", func(g *CompiledGroup) { g.Attributes = map[string]string{"expand": ""} }, "expand= attribute"},
		{"record attribute", func(g *CompiledGroup) { g.Attributes = map[string]string{"record": "mac"} }, "record= attribute"},
		{"default attribute", func(g *CompiledGroup) { g.Attributes = map[string]string{"default": "x"} }, "default= attribute"},
		{"void", func(g *CompiledGroup) { g.Attributes = map[string]string{"void": ""} }, "is void"},
		{"group defaults", func(g *CompiledGroup) { g.Defaults = map[string]interface{}{"x": "1"} }, "set() defaults"},
		{"table", func(g *CompiledGroup) { g.Method = "table" }, `method="table"`},
		{"dynamic path", func(g *CompiledGroup) { g.Name = "e.{{ mac }}" }, "dynamic path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := base()
			tc.modify(g)
			analyzeStreamability(g, nil)
			if g.Streamable || !containsString(g.NonStreamableReasons, tc.reason) {
				t.Errorf("Streamable=%v reasons=%v, want a reason containing %q", g.Streamable, g.NonStreamableReasons, tc.reason)
			}
		})
	}

	// Allowed: an empty void-like attribute value that isn't void, and
	// allowlisted chain functions.
	g := base()
	g.Chain = "contains('ip') | del('ip')"
	g.Attributes = map[string]string{"void": "false", "contains": ""}
	analyzeStreamability(g, nil)
	if !g.Streamable {
		t.Errorf("expected streamable, reasons: %v", g.NonStreamableReasons)
	}
}

// A chain= naming a template variable is checked against the variable's
// value (a string chain, or a list of functions), as processGroupFunctions
// resolves it, not against the variable name.
func TestAnalyzeStreamability_ChainNamesTemplateVariable(t *testing.T) {
	base := func(chain string) *CompiledGroup {
		g := makeGroup(t, "entry*", false, []string{"mac {{ mac | _start_ }}", "ip {{ ip }}"}, "", nil)
		g.Chain = chain
		return g
	}
	cases := []struct {
		name       string
		vars       map[string]interface{}
		streamable bool
		reason     string
	}{
		{"string var, allowlisted", map[string]interface{}{"c1": "contains('ip')"}, true, ""},
		{"string var with pipes, allowlisted", map[string]interface{}{"c1": "contains('ip') | del('ip')"}, true, ""},
		{"list var, allowlisted", map[string]interface{}{"c1": []interface{}{"contains('ip')", "set('x', 'y')"}}, true, ""},
		{"string var, not allowlisted", map[string]interface{}{"c1": "contains('ip') | itemize(key='mac')"}, false, `group function "itemize"`},
		{"list var, not allowlisted", map[string]interface{}{"c1": []interface{}{"contains('ip')", "expand"}}, false, `group function "expand"`},
		{"no such var: the name is the function", nil, false, `group function "c1"`},
		{"non-string var: the name is the function", map[string]interface{}{"c1": 5}, false, `group function "c1"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := base("c1")
			analyzeStreamability(g, tc.vars)
			if g.Streamable != tc.streamable {
				t.Fatalf("Streamable=%v reasons=%v, want %v", g.Streamable, g.NonStreamableReasons, tc.streamable)
			}
			if !tc.streamable && !containsString(g.NonStreamableReasons, tc.reason) {
				t.Errorf("reasons=%v, want one containing %q", g.NonStreamableReasons, tc.reason)
			}
		})
	}
}

// A start line with default() no longer makes a group non-streamable:
// parseGroupStream gives the record of defaults Parse gives.
func TestAnalyzeStreamability_StartLineDefaultIsStreamable(t *testing.T) {
	g := makeGroup(t, "entry*", false, []string{"mac {{ mac | _start_ | default(\"none\") }}", "ip {{ ip }}"}, "", nil)
	analyzeStreamability(g, nil)
	if !g.Streamable {
		t.Errorf("expected streamable, reasons: %v", g.NonStreamableReasons)
	}
}

// Commas inside arguments don't make function names; a "|" inside quotes
// splits the chain, as in Python TTP; and text that isn't a call (a list
// literal kept as a string) is skipped, as the group pipeline skips it.
func TestParseGroupFunctionNames_ArgumentsAndNonNames(t *testing.T) {
	cases := map[string]string{
		"del(a, b, c) | to_int":                          "del,to_int",
		"contains_val(interface, 'Loop')":                "contains_val",
		"sformat('a|b, c') | itemize(key='x')":           "itemize",
		"[\n\"del(vlan)\",\n\"contains_val(i, 'L')\"\n]": "",
		"  macro('m1, m2') ":                             "macro",
	}
	for in, want := range cases {
		if got := strings.Join(parseGroupFunctionNames(in), ","); got != want {
			t.Errorf("parseGroupFunctionNames(%q) = %q, want %q", in, got, want)
		}
	}
}
