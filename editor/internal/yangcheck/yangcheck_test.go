package yangcheck

import (
	"strings"
	"testing"
)

const baseModule = `module base {
  namespace "urn:base";
  prefix b;
  typedef port { type uint16; }
}`

const userModule = `module user {
  namespace "urn:user";
  prefix u;
  import base { prefix b; }
  container interfaces {
    leaf port { type b:port; }
  }
}`

func TestCheckAcceptsValidModule(t *testing.T) {
	if err := Check("base", baseModule, nil); err != nil {
		t.Fatalf("valid module rejected: %v", err)
	}
}

func TestCheckRejectsSyntaxError(t *testing.T) {
	err := Check("broken", "module broken { namespace \"urn:x\"; prefix x; leaf a { type string; ", nil)
	if err == nil {
		t.Fatal("module with unbalanced braces was accepted")
	}
	msg := err.Error()
	for _, want := range []string{"broken", "syntax error", "closing brace"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
}

func TestCheckRejectsGarbage(t *testing.T) {
	err := Check("garbage", "this is not yang at all", nil)
	if err == nil {
		t.Fatal("garbage was accepted")
	}
	if !strings.Contains(err.Error(), "syntax error") {
		t.Errorf("error %q should say syntax error", err)
	}
}

func TestCheckRejectsEmptyContent(t *testing.T) {
	if err := Check("empty", "   \n", nil); err == nil {
		t.Fatal("empty content was accepted")
	}
}

func TestCheckReportsMissingImport(t *testing.T) {
	err := Check("user", userModule, nil)
	if err == nil {
		t.Fatal("module with unresolved import was accepted")
	}
	msg := err.Error()
	for _, want := range []string{"user", "base", "not loaded"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q should mention %q", msg, want)
		}
	}
}

func TestCheckResolvesImportsAgainstLoadedModules(t *testing.T) {
	loaded := map[string]string{"base": baseModule}
	if err := Check("user", userModule, loaded); err != nil {
		t.Fatalf("import of an already-loaded module not resolved: %v", err)
	}
}

func TestCheckRejectsSemanticError(t *testing.T) {
	bad := `module bad { namespace "urn:bad"; prefix x; leaf a { type nosuchtype; } }`
	err := Check("bad", bad, nil)
	if err == nil {
		t.Fatal("module with unknown type was accepted")
	}
	if !strings.Contains(err.Error(), "nosuchtype") {
		t.Errorf("error %q should mention the unknown type", err)
	}
}

func TestCheckReplacingLoadedModuleIsNotADuplicate(t *testing.T) {
	loaded := map[string]string{"base": baseModule}
	if err := Check("base", baseModule, loaded); err != nil {
		t.Fatalf("re-loading a module under its own name rejected: %v", err)
	}
}

func TestCheckIgnoresErrorsOwnedByOtherLoadedModules(t *testing.T) {
	// A previously stored module that no longer parses or resolves must not
	// block loading an unrelated, valid module.
	loaded := map[string]string{
		"junk":   "not yang",
		"orphan": `module orphan { namespace "urn:o"; prefix o; import gone { prefix g; } }`,
	}
	if err := Check("base", baseModule, loaded); err != nil {
		t.Fatalf("unrelated loaded-module errors blamed on new module: %v", err)
	}
}
