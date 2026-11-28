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
