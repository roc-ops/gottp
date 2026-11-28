package gottp

import (
	"strings"
	"testing"
)

// parseWithYANG compiles tmpl, parses input with the YANG module loaded and
// returns the validation result recorded for group.
func parseWithYANG(t *testing.T, tmpl, input, module, group string) (valid bool, errs, warns []string) {
	t.Helper()
	ct, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("CompileTemplate: %v", err)
	}
	res, err := ct.ParseWithValidation(Inputs{"_root_template_": input}, nil,
		&ParseOptions{YANGModules: &YANGModules{Modules: map[string]string{"m": module}}})
	if err != nil {
		t.Fatalf("ParseWithValidation: %v", err)
	}
	vr, ok := res.ValidationResults[group]
	if !ok {
		t.Fatalf("no validation result for group %q (have %v)", group, res.ValidationResults)
	}
	for _, e := range vr.Errors {
		errs = append(errs, e.Message)
	}
	for _, w := range vr.Warnings {
		warns = append(warns, w.Message)
	}
	return vr.Valid, errs, warns
}

func requireYANGError(t *testing.T, valid bool, errs, warns []string, want string) {
	t.Helper()
	if valid {
		t.Fatalf("expected Valid=false, got true (errors %q, warnings %q)", errs, warns)
	}
	for _, e := range errs {
		if strings.Contains(e, want) {
			return
		}
	}
	t.Fatalf("expected an error containing %q, got %q", want, errs)
}

func requireYANGValid(t *testing.T, valid bool, errs, warns []string) {
	t.Helper()
	if !valid || len(errs) > 0 {
		t.Fatalf("expected Valid=true, got %v (errors %q, warnings %q)", valid, errs, warns)
	}
}

// Issue #50: a fractional number in an integer leaf must fail.
func TestYANGIntegerLeafRejectsFraction(t *testing.T) {
	module := `module t-cpu {
  namespace "urn:t:cpu"; prefix cpu;
  container show { list cpu { key id; leaf id { type uint8; } } }
}`
	tmpl := `<group name="cpu*" yang="t-cpu:show/cpu">
CPU {{ id | to_float }}
</group>`
	valid, errs, warns := parseWithYANG(t, tmpl, "CPU 12.5\n", module, "cpu*")
	requireYANGError(t, valid, errs, warns, "Field 'id' has invalid type")

	valid, errs, warns = parseWithYANG(t, tmpl, "CPU 12\n", module, "cpu*")
	requireYANGValid(t, valid, errs, warns)
}

// Issue #59: a leaf under choice/case is type-checked, not warned about.
func TestYANGChoiceCaseLeafIsValidated(t *testing.T) {
	module := `module t-if {
  namespace "urn:t:if"; prefix tif;
  container show { list intf { key name;
    leaf name { type string; }
    choice addr { case v4 { leaf mtu { type uint16 { range "68..9216"; } } } }
  } }
}`
	tmpl := `<group name="intf*" yang="t-if:show/intf">
{{ name }} mtu {{ mtu | to_int }}
</group>`
	valid, errs, warns := parseWithYANG(t, tmpl, "eth0 mtu 20\n", module, "intf*")
	requireYANGError(t, valid, errs, warns, "Field 'mtu' value is out of range")

	valid, errs, warns = parseWithYANG(t, tmpl, "eth0 mtu 1500\n", module, "intf*")
	requireYANGValid(t, valid, errs, warns)
	if len(warns) > 0 {
		t.Fatalf("expected no warnings, got %q", warns)
	}
}
