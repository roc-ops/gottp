package yang

import (
	"strings"
	"testing"
)

// newTestValidator loads one YANG module and returns a Validator for it.
func newTestValidator(t *testing.T, name, src string) *Validator {
	t.Helper()
	ms := NewModuleSet()
	if err := ms.AddModule(name, src); err != nil {
		t.Fatalf("AddModule: %v", err)
	}
	if err := ms.ProcessAllModules(); err != nil {
		t.Fatalf("ProcessAllModules: %v", err)
	}
	return NewValidator(ms)
}

func assertValid(t *testing.T, r *ValidationResult) {
	t.Helper()
	if !r.Valid || len(r.Errors) > 0 {
		t.Errorf("expected valid, got Valid=%v errors: %s", r.Valid, formatErrors(r))
	}
}

// assertInvalid checks that r failed with an error on field whose message
// contains want.
func assertInvalid(t *testing.T, r *ValidationResult, field, want string) {
	t.Helper()
	if r.Valid {
		t.Errorf("expected invalid, got Valid=true (warnings: %s)", formatWarnings(r))
		return
	}
	for _, e := range r.Errors {
		if e.Field == field && strings.Contains(e.Message, want) {
			return
		}
	}
	t.Errorf("expected an error on %q containing %q, got: %s", field, want, formatErrors(r))
}

func formatWarnings(r *ValidationResult) string {
	var msgs []string
	for _, w := range r.Warnings {
		msgs = append(msgs, w.Message)
	}
	return strings.Join(msgs, "; ")
}

// Issue #50: YANG integer types admit whole numbers only (RFC 7950 9.2).
const integerYANG = `
module t-int {
  namespace "urn:t:int"; prefix ti;
  container c {
    leaf u8 { type uint8; }
    leaf i8 { type int8; }
    leaf-list u32s { type uint32; }
  }
}`

func TestIntegerTypesRejectFractionalValues(t *testing.T) {
	v := newTestValidator(t, "t-int", integerYANG)
	cases := []struct {
		name  string
		data  map[string]interface{}
		field string
	}{
		{"uint8 12.5", map[string]interface{}{"u8": 12.5}, "u8"},
		{"int8 -3.5", map[string]interface{}{"i8": -3.5}, "i8"},
		{"uint32 leaf-list element 1.5", map[string]interface{}{"u32s": []interface{}{1.0, 1.5}}, "u32s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertInvalid(t, v.Validate(tc.data, "t-int:c", "g"), tc.field, "invalid type")
		})
	}
}

func TestIntegerTypesAcceptWholeNumbers(t *testing.T) {
	v := newTestValidator(t, "t-int", integerYANG)
	// float64 with no fractional part is what a JSON round trip turns an
	// int into, so it must stay valid.
	data := map[string]interface{}{"u8": 12, "i8": -3.0, "u32s": []interface{}{1.0, 2}}
	assertValid(t, v.Validate(data, "t-int:c", "g"))
}

// Issue #59: leaves under choice/case are data-tree children of the choice's
// parent (RFC 7950 7.9), so they must be found and type-checked.
const choiceYANG = `
module t-choice {
  namespace "urn:t:choice"; prefix tc;
  container c {
    leaf name { type string; }
    choice transport {
      case tcp {
        leaf tcp-port { type uint16; }
        choice auth {
          case key { leaf key-id { type uint8; mandatory true; } }
          case none { leaf open { type empty; } }
        }
      }
      case udp { leaf udp-port { type uint16 { range "1..1024"; } } }
      leaf local { type string { length "1..4"; } }
    }
  }
}`

func TestChoiceCaseLeavesAreValidated(t *testing.T) {
	v := newTestValidator(t, "t-choice", choiceYANG)
	cases := []struct {
		name  string
		data  map[string]interface{}
		field string
		want  string
	}{
		{"case leaf type", map[string]interface{}{"tcp-port": "eighty"}, "tcp-port", "invalid type"},
		{"case leaf range", map[string]interface{}{"udp-port": 2000}, "udp-port", "out of range"},
		{"shorthand case length", map[string]interface{}{"local": "toolong"}, "local", "length is out of range"},
		{"nested choice leaf", map[string]interface{}{"tcp-port": 22, "key-id": 300}, "key-id", "out of range"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertInvalid(t, v.Validate(tc.data, "t-choice:c", "g"), tc.field, tc.want)
		})
	}
}

func TestChoiceCaseLeavesAreKnownFields(t *testing.T) {
	v := newTestValidator(t, "t-choice", choiceYANG)
	r := v.Validate(map[string]interface{}{"name": "a", "tcp-port": 22, "key-id": 7}, "t-choice:c", "g")
	assertValid(t, r)
	if len(r.Warnings) > 0 {
		t.Errorf("expected no warnings, got: %s", formatWarnings(r))
	}
}

func TestMandatoryLeafInInactiveCaseIsNotRequired(t *testing.T) {
	v := newTestValidator(t, "t-choice", choiceYANG)
	// key-id is mandatory only when its case is the one present
	// (RFC 7950 7.6.5); here no node of case "key" exists.
	assertValid(t, v.Validate(map[string]interface{}{"udp-port": 53}, "t-choice:c", "g"))
}

// Issue #56: enumeration values must be declared names (RFC 7950 9.6) and
// identityref values must name an identity derived from the base
// (RFC 7950 9.10, JSON encoding per RFC 7951 6.8).
const enumIdentityYANG = `
module t-enum {
  namespace "urn:t:enum"; prefix te;
  identity crypto;
  identity aes { base crypto; }
  identity aes-256 { base aes; }
  typedef state { type enumeration { enum up; enum down; } }
  container c {
    leaf oper { type state; }
    leaf-list states { type state; }
    leaf speed { type union { type uint8; type enumeration { enum auto; } } }
    leaf alg { type identityref { base crypto; } }
  }
}`

const enumIdentityExtYANG = `
module t-enum-ext {
  namespace "urn:t:enum-ext"; prefix tex;
  import t-enum { prefix te; }
  identity blowfish { base te:crypto; }
}`

func newEnumValidator(t *testing.T) *Validator {
	t.Helper()
	ms := NewModuleSet()
	for name, src := range map[string]string{"t-enum": enumIdentityYANG, "t-enum-ext": enumIdentityExtYANG} {
		if err := ms.AddModule(name, src); err != nil {
			t.Fatalf("AddModule %s: %v", name, err)
		}
	}
	if err := ms.ProcessAllModules(); err != nil {
		t.Fatalf("ProcessAllModules: %v", err)
	}
	return NewValidator(ms)
}

func TestEnumerationMembership(t *testing.T) {
	v := newEnumValidator(t)
	for _, tc := range []struct {
		name  string
		data  map[string]interface{}
		field string
	}{
		{"undeclared enum", map[string]interface{}{"oper": "sideways"}, "oper"},
		{"undeclared leaf-list enum", map[string]interface{}{"states": []interface{}{"up", "left"}}, "states"},
		{"union enum member does not accept any string", map[string]interface{}{"speed": "zzz"}, "speed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertInvalid(t, v.Validate(tc.data, "t-enum:c", "g"), tc.field, "invalid")
		})
	}
	assertValid(t, v.Validate(map[string]interface{}{
		"oper": "up", "states": []interface{}{"up", "down"}, "speed": "auto",
	}, "t-enum:c", "g"))
	assertValid(t, v.Validate(map[string]interface{}{"speed": 100}, "t-enum:c", "g"))
}

func TestIdentityrefMembership(t *testing.T) {
	v := newEnumValidator(t)
	for _, ok := range []string{"aes", "aes-256", "t-enum:aes", "t-enum-ext:blowfish"} {
		t.Run("valid "+ok, func(t *testing.T) {
			assertValid(t, v.Validate(map[string]interface{}{"alg": ok}, "t-enum:c", "g"))
		})
	}
	for _, bad := range []string{
		"rot13",          // not an identity
		"crypto",         // the base itself is not derived from the base
		"t-enum-ext:aes", // wrong module
		"blowfish",       // defined in another module, so it must be qualified
		"te:aes",         // a prefix is not a module name
	} {
		t.Run("invalid "+bad, func(t *testing.T) {
			assertInvalid(t, v.Validate(map[string]interface{}{"alg": bad}, "t-enum:c", "g"), "alg", "invalid")
		})
	}
}
