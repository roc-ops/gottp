package gottp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/yang"
)

// The deployment shape of #175: one parent module that includes one
// submodule per command, each submodule declaring belongs-to the parent.
const (
	devParent = `module t-dev {
  yang-version 1.1;
  namespace "urn:t:dev";
  prefix dev;
  include t-dev-cpu;
  include t-dev-ver;
  revision 2026-01-01;
}`
	devCPU = `submodule t-dev-cpu {
  yang-version 1.1;
  belongs-to t-dev { prefix "dev"; }
  revision 2026-01-01;
  container show-cpu {
    list cpu {
      key id;
      leaf id { type uint8; }
      leaf load { type uint8 { range "0..100"; } }
    }
  }
}`
	devVer = `submodule t-dev-ver {
  yang-version 1.1;
  belongs-to t-dev { prefix "dev"; }
  revision 2026-01-01;
  container show-version {
    leaf version { type string { pattern '[0-9]+\.[0-9]+'; } }
  }
}`
)

const devTemplate = `<group name="cpu*" yang="t-dev:show-cpu/cpu">
CPU {{ id | to_int }} load {{ load | to_int }}
</group>
<group name="version" yang="t-dev:show-version">
Version {{ version }}
</group>`

func writeYANGFiles(t *testing.T, files map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for name, content := range files {
		p := filepath.Join(dir, name+".yang")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

func devModules(t *testing.T) map[string]*YANGModules {
	files := map[string]string{"t-dev": devParent, "t-dev-cpu": devCPU, "t-dev-ver": devVer}
	return map[string]*YANGModules{
		"Files":   {Files: writeYANGFiles(t, files)},
		"Modules": {Modules: files},
	}
}

// #175: submodules load through Files and Modules, register under their
// belongs-to module, and paths into their content validate.
func TestYANGSubmodulesIncludedByParent(t *testing.T) {
	ct, err := CompileTemplate(devTemplate)
	if err != nil {
		t.Fatal(err)
	}
	for how, mods := range devModules(t) {
		t.Run(how, func(t *testing.T) {
			res, err := ct.ParseWithValidation(Inputs{"Default_Input": "CPU 1 load 20\nCPU 2 load 30\nVersion 17.3\n"},
				nil, &ParseOptions{YANGModules: mods})
			if err != nil {
				t.Fatalf("ParseWithValidation: %v", err)
			}
			for _, g := range []string{"cpu*", "version"} {
				vr := res.ValidationResults[g]
				if vr == nil || !vr.Valid || len(vr.Errors) > 0 {
					t.Errorf("group %s: want valid, got %+v", g, describeVR(vr))
				}
			}

			res, err = ct.ParseWithValidation(Inputs{"Default_Input": "CPU 1 load 200\nVersion 17.x\n"},
				nil, &ParseOptions{YANGModules: mods})
			if err != nil {
				t.Fatalf("ParseWithValidation: %v", err)
			}
			for _, g := range []string{"cpu*", "version"} {
				if vr := res.ValidationResults[g]; vr == nil || vr.Valid {
					t.Errorf("group %s: want invalid, got %+v", g, describeVR(vr))
				}
			}
		})
	}
}

// #175: a submodule whose parent is not loaded is ignored, as yangson
// ignores one no loaded module includes; a path into it reports that its
// parent is missing. A parent whose included submodule is missing fails to
// load.
func TestYANGSubmoduleWithoutParent(t *testing.T) {
	ct, err := CompileTemplate(devTemplate)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ct.ParseWithValidation(Inputs{"Default_Input": "CPU 1 load 20\n"}, nil,
		&ParseOptions{YANGModules: &YANGModules{Files: writeYANGFiles(t, map[string]string{"t-dev-cpu": devCPU})}})
	if err != nil {
		t.Fatalf("ParseWithValidation: %v", err)
	}
	vr := res.ValidationResults["cpu*"]
	if vr == nil || vr.Valid || len(vr.Errors) != 1 ||
		!strings.Contains(vr.Errors[0].Message, "submodule t-dev-cpu belongs to t-dev, which is not loaded") {
		t.Fatalf("want a path error naming the missing parent, got %s", describeVR(vr))
	}

	_, err = ct.ParseWithValidation(Inputs{"Default_Input": "CPU 1 load 20\n"}, nil,
		&ParseOptions{YANGModules: &YANGModules{Modules: map[string]string{"t-dev": devParent, "t-dev-cpu": devCPU}}})
	if err == nil || !strings.Contains(err.Error(), "t-dev-ver") {
		t.Fatalf("want a load error naming the missing submodule t-dev-ver, got %v", err)
	}
}

func describeVR(vr *yang.ValidationResult) string {
	if vr == nil {
		return "<nil>"
	}
	return sprintVR(vr)
}

// #58: Parse does not validate, so a broken module set or bad data cannot
// change its result or fail it.
func TestParseDoesNotValidate(t *testing.T) {
	ct, err := CompileTemplate(devTemplate)
	if err != nil {
		t.Fatal(err)
	}
	broken := &YANGModules{Modules: map[string]string{"x": "this is not yang"}}
	got, err := ct.Parse(Inputs{"Default_Input": "CPU 1 load 200\n"}, nil, &ParseOptions{YANGModules: broken})
	if err != nil {
		t.Fatalf("Parse with YANG modules: %v", err)
	}
	want, err := ct.Parse(Inputs{"Default_Input": "CPU 1 load 200\n"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sprintJSON(got) != sprintJSON(want) {
		t.Fatalf("Parse result changed with YANG modules: %s vs %s", sprintJSON(got), sprintJSON(want))
	}
	rt := ct.NewRuntime()
	if _, err := rt.Parse(Inputs{"Default_Input": "CPU 1 load 200\n"}, nil, &ParseOptions{YANGModules: broken}); err != nil {
		t.Fatalf("Runtime.Parse with YANG modules: %v", err)
	}
}

// #58: with several inputs, a group's validation results merge across them.
func TestYANGValidationMergesAcrossInputs(t *testing.T) {
	ct, err := CompileTemplate(devTemplate)
	if err != nil {
		t.Fatal(err)
	}
	mods := devModules(t)["Modules"]
	inputs := Inputs{"a": "CPU 1 load 200\n", "b": "CPU 2 load 20\n"}
	for _, order := range []Inputs{inputs, {"a": "CPU 2 load 20\n", "b": "CPU 1 load 200\n"}} {
		res, err := ct.ParseWithValidation(order, nil, &ParseOptions{YANGModules: mods})
		if err != nil {
			t.Fatal(err)
		}
		vr := res.ValidationResults["cpu*"]
		if vr == nil || vr.Valid || len(vr.Errors) != 1 {
			t.Fatalf("want one error merged from the bad input, got %s", describeVR(vr))
		}
	}
	// Two bad inputs: both errors are kept; a bad path is reported once.
	res, err := ct.ParseWithValidation(Inputs{"a": "CPU 1 load 200\n", "b": "CPU 2 load 201\n"}, nil, &ParseOptions{YANGModules: mods})
	if err != nil {
		t.Fatal(err)
	}
	if vr := res.ValidationResults["cpu*"]; vr == nil || vr.Valid || len(vr.Errors) != 2 {
		t.Fatalf("want two errors, got %s", describeVR(vr))
	}
	bad, err := CompileTemplate(`<group name="cpu*" yang="t-dev:show-cpu/nope">
CPU {{ id | to_int }} load {{ load | to_int }}
</group>`)
	if err != nil {
		t.Fatal(err)
	}
	res, err = bad.ParseWithValidation(Inputs{"a": "CPU 1 load 2\n", "b": "CPU 2 load 3\n"}, nil, &ParseOptions{YANGModules: mods})
	if err != nil {
		t.Fatal(err)
	}
	if vr := res.ValidationResults["cpu*"]; vr == nil || vr.Valid || len(vr.Errors) != 1 {
		t.Fatalf("want the path error once, got %s", describeVR(vr))
	}
}

// #58: every yang= group has an entry, even one that produced no records;
// its path is still checked.
func TestYANGEntryForGroupWithoutRecords(t *testing.T) {
	tmpl := devTemplate + `
<group name="bad" yang="t-dev:show-nothing">
Nothing {{ x }}
</group>
<group name="outer">
Outer {{ o }}
<group name="inner" yang="t-dev:show-version">
Inner {{ version }}
</group>
</group>`
	ct, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ct.ParseWithValidation(Inputs{"Default_Input": "CPU 1 load 2\n"}, nil,
		&ParseOptions{YANGModules: devModules(t)["Modules"]})
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{"version", "inner"} {
		if vr := res.ValidationResults[g]; vr == nil || !vr.Valid || len(vr.Errors) > 0 {
			t.Errorf("group %s: want a valid entry, got %s", g, describeVR(vr))
		}
	}
	vr := res.ValidationResults["bad"]
	if vr == nil || vr.Valid || len(vr.Errors) != 1 || !strings.Contains(vr.Errors[0].Message, "YANG path not found") {
		t.Errorf("group bad: want a path error, got %s", describeVR(vr))
	}
	// Without modules there is nothing to validate and no entries.
	res, err = ct.ParseWithValidation(Inputs{"Default_Input": "CPU 1 load 2\n"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.ValidationResults) != 0 {
		t.Errorf("want no entries without modules, got %v", res.ValidationResults)
	}
}

// #58: ParseStream does not validate, so yang= groups with YANG modules
// supplied are not streamable; without modules they stream.
func TestParseStreamYANGNotStreamable(t *testing.T) {
	ct, err := CompileTemplate(devTemplate)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	fn := func(map[string]interface{}, [2]int, string) error { called = true; return nil }
	err = ct.ParseStream(Inputs{"Default_Input": "CPU 1 load 2\n"}, nil,
		&ParseOptions{YANGModules: devModules(t)["Modules"]}, fn)
	var nse *TemplateNotStreamableError
	if !errors.As(err, &nse) || !errors.Is(err, ErrTemplateNotStreamable) {
		t.Fatalf("want TemplateNotStreamableError, got %v", err)
	}
	if called || len(nse.Reasons) != 2 || !strings.Contains(nse.Reasons[0], `"cpu*"`) || !strings.Contains(nse.Reasons[0], "yang=") {
		t.Fatalf("called=%v reasons=%q", called, nse.Reasons)
	}
	if err := ct.ParseStream(Inputs{"Default_Input": "CPU 1 load 2\n"}, nil, nil, fn); err != nil || !called {
		t.Fatalf("without modules: err=%v called=%v", err, called)
	}
}

func sprintJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func sprintVR(r *yang.ValidationResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "valid=%v", r.Valid)
	for _, e := range r.Errors {
		fmt.Fprintf(&b, " error=%q", e.Message)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, " warning=%q", w.Message)
	}
	return b.String()
}

// #175: a module may include only its own submodules (RFC 7950 7.1.6).
func TestYANGIncludeOfAnotherModulesSubmodule(t *testing.T) {
	other := `module t-other {
  yang-version 1.1;
  namespace "urn:t:other";
  prefix oth;
  include t-dev-cpu;
}`
	ct, err := CompileTemplate(devTemplate)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ct.ParseWithValidation(Inputs{"Default_Input": "CPU 1 load 20\n"}, nil,
		&ParseOptions{YANGModules: &YANGModules{Modules: map[string]string{"t-other": other, "t-dev-cpu": devCPU}}})
	if err == nil || !strings.Contains(err.Error(), "module t-other includes submodule t-dev-cpu, which belongs to t-dev") {
		t.Fatalf("want a belongs-to error, got %v", err)
	}
}

// A top-level yang= group with no records keeps its entry when a nested
// group of the same name has records ("inner" next to "inner[0]").
func TestYANGEntryPerGroupDespiteNameCollision(t *testing.T) {
	ct, err := CompileTemplate(`<group name="inner" yang="t-dev:no-such">
NOMATCH {{ z }}
</group>
<group name="outer">
OUT {{ o }}
<group name="inner" yang="t-dev:show-version">
Version {{ version }}
</group>
</group>`)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ct.ParseWithValidation(Inputs{"Default_Input": "OUT 1\nVersion 17.3\n"}, nil,
		&ParseOptions{YANGModules: devModules(t)["Modules"]})
	if err != nil {
		t.Fatal(err)
	}
	if vr := res.ValidationResults["inner[0]"]; vr == nil || !vr.Valid {
		t.Errorf("inner[0]: want valid, got %s", describeVR(vr))
	}
	vr := res.ValidationResults["inner"]
	if vr == nil || vr.Valid || len(vr.Errors) == 0 || !strings.Contains(vr.Errors[0].Message, "no-such") {
		t.Errorf("inner: want the top group's path error, got %s", describeVR(vr))
	}
}

// A parent module holding typedefs its submodules use: goyang 1.6.3 panics
// on it unaided. yangson 1.6.7: n 5 valid, n 10 out of range.
func TestYANGSubmoduleUsesParentTypedef(t *testing.T) {
	mods := &YANGModules{Files: writeYANGFiles(t, map[string]string{
		"p":  `module p { yang-version 1.1; namespace "urn:p"; prefix pp; include sb; typedef small { type uint8 { range "1..9"; } } }`,
		"sb": `submodule sb { yang-version 1.1; belongs-to p { prefix pp; } container cb { leaf n { type small; } } }`,
	})}
	ct, err := CompileTemplate(`<group name="cb" yang="p:cb">
N {{ n | to_int }}
</group>`)
	if err != nil {
		t.Fatal(err)
	}
	for in, valid := range map[string]bool{"N 5\n": true, "N 10\n": false} {
		res, err := ct.ParseWithValidation(Inputs{"Default_Input": in}, nil, &ParseOptions{YANGModules: mods})
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if vr := res.ValidationResults["cb"]; vr == nil || vr.Valid != valid {
			t.Errorf("%q: want valid=%v, got %s", in, valid, describeVR(vr))
		}
	}
}
