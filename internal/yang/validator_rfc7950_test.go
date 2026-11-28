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
