package yang

import (
	"strings"
	"testing"
)

func TestExtractModuleHeader(t *testing.T) {
	for _, tc := range []struct{ content, kind, name string }{
		{"module foo {\n}", "module", "foo"},
		{"// c\nsubmodule foo-bar {\n belongs-to foo { prefix f; }\n}", "submodule", "foo-bar"},
		{"submodule \"q\" {", "submodule", "q"},
		{"module m{", "module", "m"},
	} {
		kind, name, err := ExtractModuleHeader(tc.content)
		if err != nil || kind != tc.kind || name != tc.name {
			t.Errorf("%q: got %q %q %v, want %q %q", tc.content, kind, name, err, tc.kind, tc.name)
		}
	}
	if _, _, err := ExtractModuleHeader("container x {}"); err == nil {
		t.Error("want an error for content with no module or submodule")
	}
}

func TestMergeResults(t *testing.T) {
	pathErr := &ValidationError{Message: "YANG path not found: x", once: true}
	a := &ValidationResult{Valid: false, Errors: []*ValidationError{pathErr, {Message: "e1"}}}
	b := &ValidationResult{Valid: true, Warnings: []*ValidationError{{Message: "w"}}}
	c := &ValidationResult{Valid: false, Errors: []*ValidationError{{Message: "YANG path not found: x", once: true}, {Message: "e1"}}}

	if MergeResults(nil, b) != b || MergeResults(a, nil) != a {
		t.Fatal("merging with nil must return the other result")
	}
	m := MergeResults(MergeResults(a, b), c)
	var msgs []string
	for _, e := range m.Errors {
		msgs = append(msgs, e.Message)
	}
	if m.Valid || strings.Join(msgs, "|") != "YANG path not found: x|e1|e1" || len(m.Warnings) != 1 {
		t.Fatalf("got valid=%v errors=%q warnings=%d", m.Valid, msgs, len(m.Warnings))
	}
	if len(a.Errors) != 2 {
		t.Fatal("MergeResults modified its argument")
	}
}

// A submodule's content is part of its module's schema, under the module's
// name and prefix, and paths into it resolve.
func TestSubmoduleContentResolvesUnderParent(t *testing.T) {
	ms := NewModuleSet()
	for name, content := range map[string]string{
		"p": `module p { namespace "urn:p"; prefix pp; include s; }`,
		"s": `submodule s { belongs-to p { prefix sp; } container c { leaf l { type string; } } }`,
	} {
		if err := ms.AddModule(name, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := ms.ProcessAllModules(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"p:c/l", "pp:c/l", "sp:c/l"} {
		if _, err := ms.FindSchemaByPath(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
	if _, ok := ms.GetSchema("s"); ok {
		t.Error("a submodule must not be a schema of its own")
	}
}

func loadSet(t *testing.T, mods map[string]string) (*ModuleSet, error) {
	t.Helper()
	ms := NewModuleSet()
	for name, content := range mods {
		if err := ms.AddModule(name, content); err != nil {
			t.Fatal(err)
		}
	}
	return ms, ms.ProcessAllModules()
}

// YANG 1.1 (RFC 7950 5.1): a submodule sees every top-level typedef and
// grouping of its module and of the module's other submodules, without
// including them. goyang resolves neither; ProcessAllModules makes them
// visible. Verdicts match yangson 1.6.7.
func TestSubmoduleSeesParentAndSiblingDefinitions(t *testing.T) {
	ms, err := loadSet(t, map[string]string{
		"p":  `module p { yang-version 1.1; namespace "urn:p"; prefix pp; include sb; include sc; include sd; typedef small { type uint8 { range "1..9"; } } }`,
		"sb": `submodule sb { yang-version 1.1; belongs-to p { prefix pp; } container cb { leaf n { type small; } } }`,
		"sc": `submodule sc { yang-version 1.1; belongs-to p { prefix pq; } typedef tiny { type pq:small { range "1..3"; } } grouping gc { leaf m { type tiny; } } }`,
		"sd": `submodule sd { yang-version 1.1; belongs-to p { prefix pp; } container cd { uses gc; leaf k { type tiny; } } }`,
	})
	if err != nil {
		t.Fatal(err)
	}
	v := NewValidator(ms)
	for _, tc := range []struct {
		path  string
		data  map[string]interface{}
		valid bool
	}{
		{"p:cb", map[string]interface{}{"n": 5}, true},
		{"p:cb", map[string]interface{}{"n": 10}, false},
		{"p:cd", map[string]interface{}{"m": 2, "k": 3}, true},
		{"p:cd", map[string]interface{}{"m": 4}, false},
		{"p:cd", map[string]interface{}{"k": 4}, false},
	} {
		if r := v.Validate(tc.data, tc.path, "g"); r.Valid != tc.valid {
			t.Errorf("%s %v: valid=%v, want %v (errors %v)", tc.path, tc.data, r.Valid, tc.valid, r.Errors)
		}
	}
}

// A goyang panic on user modules is a load error naming the modules, not a
// crash. goyang 1.6.3 panics building the error for an unknown type in a
// submodule.
func TestGoyangPanicIsLoadError(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	_, err := loadSet(t, map[string]string{
		"p":  `module p { yang-version 1.1; namespace "urn:p"; prefix pp; include sb; }`,
		"sb": `submodule sb { yang-version 1.1; belongs-to p { prefix pp; } container cb { leaf n { type nosuch; } } }`,
	})
	if err == nil || !strings.Contains(err.Error(), "p") || !strings.Contains(err.Error(), "sb") {
		t.Fatalf("want a load error naming p and sb, got %v", err)
	}
	t.Log(err)
}

// A belongs-to prefix two loaded modules' submodules both use names neither
// module: a path using it is an error, whichever order the maps iterate.
func TestAmbiguousBelongsToPrefix(t *testing.T) {
	for i := 0; i < 20; i++ {
		ms, err := loadSet(t, map[string]string{
			"a":   `module a { namespace "urn:a"; prefix a; include sa; }`,
			"sa":  `submodule sa { belongs-to a { prefix x; } container ca { leaf l { type string; } } }`,
			"b":   `module b { namespace "urn:b"; prefix b; include sb2; }`,
			"sb2": `submodule sb2 { belongs-to b { prefix x; } container ca { leaf l { type uint8; } } }`,
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = ms.FindSchemaByPath("x:ca/l")
		if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "a, b") {
			t.Fatalf("want an ambiguous-prefix error naming a and b, got %v", err)
		}
		for _, p := range []string{"a:ca/l", "b:ca/l"} {
			if _, err := ms.FindSchemaByPath(p); err != nil {
				t.Errorf("%s: %v", p, err)
			}
		}
	}
}

// The "module or prefix not found" error lists modules and prefixes in sorted
// order, so the same error from two inputs or two same-named groups is equal
// and MergeResults keeps one copy.
func TestModuleNotFoundErrorIsStable(t *testing.T) {
	ms := NewModuleSet()
	for _, n := range []string{"m1", "m2", "m3", "m4", "m5", "m6"} {
		if err := ms.AddModule(n, "module "+n+" { namespace \"urn:"+n+"\"; prefix "+n+"p; container c { leaf l { type string; } } }"); err != nil {
			t.Fatal(err)
		}
	}
	if err := ms.ProcessAllModules(); err != nil {
		t.Fatal(err)
	}
	_, first := ms.FindSchemaByPath("nope:c/l")
	if first == nil {
		t.Fatal("want an error")
	}
	for i := 0; i < 20; i++ {
		if _, err := ms.FindSchemaByPath("nope:c/l"); err == nil || err.Error() != first.Error() {
			t.Fatalf("error text changed between calls:\n%v\n%v", first, err)
		}
	}
}
