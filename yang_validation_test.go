package gottp

import (
	"fmt"
	"reflect"
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

// Issue #56: an enumeration leaf only takes its declared names, also as a
// union member.
func TestYANGEnumerationMembership(t *testing.T) {
	module := `module t-port {
  namespace "urn:t:port"; prefix tp;
  container show { list port { key name;
    leaf name { type string; }
    leaf state { type enumeration { enum up; enum down; } }
    leaf speed { type union { type uint32; type enumeration { enum auto; } } }
  } }
}`
	tmpl := `<group name="port*" yang="t-port:show/port">
{{ name }} is {{ state }} speed {{ speed }}
</group>`
	valid, errs, warns := parseWithYANG(t, tmpl, "ge1 is sideways speed auto\n", module, "port*")
	requireYANGError(t, valid, errs, warns, `Field 'state' has invalid value "sideways" in item 0, expected one of up, down`)

	valid, errs, warns = parseWithYANG(t, tmpl, "ge1 is up speed fast\n", module, "port*")
	requireYANGError(t, valid, errs, warns, `Field 'speed' has invalid value "fast"`)

	valid, errs, warns = parseWithYANG(t, tmpl, "ge1 is up speed auto\n", module, "port*")
	requireYANGValid(t, valid, errs, warns)
}

// Issue #55: a pattern restriction must match the whole value.
func TestYANGPatternIsEnforced(t *testing.T) {
	module := `module t-mac {
  namespace "urn:t:mac"; prefix tm;
  typedef mac { type string { pattern "[0-9a-f]{4}\\.[0-9a-f]{4}\\.[0-9a-f]{4}"; } }
  container show { list modem { key mac; leaf mac { type mac; } } }
}`
	tmpl := `<group name="modem*" yang="t-mac:show/modem">
modem {{ mac }}
</group>`
	valid, errs, warns := parseWithYANG(t, tmpl, "modem 0011.2233.44zz\n", module, "modem*")
	requireYANGError(t, valid, errs, warns, `Field 'mac' value "0011.2233.44zz" does not match pattern`)

	// A substring match is not enough.
	valid, errs, warns = parseWithYANG(t, tmpl, "modem 0011.2233.4455.6677\n", module, "modem*")
	requireYANGError(t, valid, errs, warns, "does not match pattern")

	valid, errs, warns = parseWithYANG(t, tmpl, "modem 0011.2233.4455\n", module, "modem*")
	requireYANGValid(t, valid, errs, warns)
}

// Issue #57: a yang= path one level too high (the container, not the list
// each match maps to) is an error, not Valid=true with warnings.
func TestYANGPathAtWrongLevelIsAnError(t *testing.T) {
	module := `module t-cpuinfo {
  namespace "urn:t:cpuinfo"; prefix ci;
  container show_cpuinfo { list cpu { key cpu-id;
    leaf cpu-id { type uint8; }
    leaf usage-5s { type decimal64 { fraction-digits 2; range "0..100"; } }
  } }
}`
	input := "CPU0 12.50%\nCPU1 140.00%\n"
	wrong := `<group name="cpu*" yang="t-cpuinfo:show_cpuinfo">
CPU{{ cpu-id | to_int }} {{ usage-5s | to_float }}%
</group>`
	valid, errs, warns := parseWithYANG(t, wrong, input, module, "cpu*")
	requireYANGError(t, valid, errs, warns,
		"YANG path 't-cpuinfo:show_cpuinfo' defines none of the fields cpu-id, usage-5s")
	requireYANGError(t, valid, errs, warns, "did you mean t-cpuinfo:show_cpuinfo/cpu?")

	right := strings.Replace(wrong, `show_cpuinfo"`, `show_cpuinfo/cpu"`, 1)
	valid, errs, warns = parseWithYANG(t, right, input, module, "cpu*")
	requireYANGError(t, valid, errs, warns, "Field 'usage-5s' value is out of range in item 1")
}

// Issue #52: with source maps on, top-level groups were validated a second
// time after parsing, and that result replaced the one parseGroup recorded.
// Validation results must be the same with source maps on and off.
func TestYANGValidationSameWithSourceMap(t *testing.T) {
	module := `module t-if {
  namespace "urn:t:if"; prefix tif;
  container show {
    list intf { key name;
      leaf name { type string; }
      leaf mtu { type uint16 { range "68..9216"; } }
      list addr { key ip; leaf ip { type string; } }
    }
  }
}`
	tmpl := `<group name="intf*" yang="t-if:show/intf">
interface {{ name }}
 mtu {{ mtu | to_int }}
 <group name="addr*" yang="t-if:show/intf/addr">
 ip {{ ip }}
 </group>
</group>`
	input := "interface a\n mtu 20\n ip 1.1.1.1\ninterface b\n mtu 1500\n ip 2.2.2.2\n"
	ct, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("CompileTemplate: %v", err)
	}
	summarize := func(sourceMap bool) map[string]string {
		res, err := ct.ParseWithValidation(Inputs{"Default_Input": input}, nil, &ParseOptions{
			YANGModules:     &YANGModules{Modules: map[string]string{"m": module}},
			EnableSourceMap: sourceMap,
		})
		if err != nil {
			t.Fatalf("ParseWithValidation(sourceMap=%v): %v", sourceMap, err)
		}
		out := map[string]string{}
		for group, vr := range res.ValidationResults {
			var msgs []string
			for _, e := range vr.Errors {
				msgs = append(msgs, "E:"+e.Message)
			}
			for _, w := range vr.Warnings {
				msgs = append(msgs, "W:"+w.Message)
			}
			out[group] = fmt.Sprintf("valid=%v %q", vr.Valid, msgs)
		}
		return out
	}
	off, on := summarize(false), summarize(true)
	if !reflect.DeepEqual(off, on) {
		t.Fatalf("validation results differ with source maps on\noff: %v\non:  %v", off, on)
	}
	if !strings.Contains(off["intf*"], "valid=false") {
		t.Fatalf("expected intf* to be invalid (mtu 20 is out of range), got %v", off)
	}

	// A group that matches nothing but returns its start-line defaults was
	// validated only by the second, source-map-only pass.
	defaultsTmpl := `<group name="intf*" yang="t-if:show/intf">
interface {{ name | default("none") }}
 mtu {{ mtu | to_int }}
</group>`
	ct, err = CompileTemplate(defaultsTmpl)
	if err != nil {
		t.Fatalf("CompileTemplate: %v", err)
	}
	input = "nothing here\n"
	off, on = summarize(false), summarize(true)
	if !reflect.DeepEqual(off, on) {
		t.Fatalf("validation results differ with source maps on (defaults-only group)\noff: %v\non:  %v", off, on)
	}
}
