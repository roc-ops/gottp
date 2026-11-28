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

// Issue #55: pattern restrictions (RFC 7950 9.4.5) use XSD regular
// expressions, which match the whole value. Patterns are ANDed, a pattern
// with "modifier invert-match" must not match, and a pattern Go's RE2
// can't express is reported instead of silently passing.
const patternYANG = `
module t-pat {
  yang-version 1.1;
  namespace "urn:t:pat"; prefix tp;
  typedef lower { type string { pattern "[a-z]+"; } }
  typedef ident { type lower { pattern "x.*"; } }
  container c {
    leaf word { type lower; }
    leaf xword { type ident; }
    leaf no-admin {
      type string {
        pattern "admin" { modifier invert-match; }
      }
    }
    leaf mac { type string { pattern "[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}"; } }
    leaf digits { type string { pattern "\\d+"; } }
    leaf caret { type string { pattern "^a$"; } }
    leaf dot { type string { pattern "a.b"; } }
    leaf wordish { type string { pattern "[\\w\\-]+"; } }
    leaf-list tags { type lower; }
    leaf id-or-num { type union { type lower; type uint8; } }
    leaf xml-name { type string { pattern "\\i\\c*"; } }
    leaf block { type string { pattern "\\p{IsBasicLatin}+"; } }
  }
}`

func TestPatternIsEnforced(t *testing.T) {
	v := newTestValidator(t, "t-pat", patternYANG)
	for _, tc := range []struct {
		field string
		value interface{}
	}{
		{"word", "ABC"},
		{"word", "abc1"}, // a match must cover the whole value, not a substring
		{"xword", "abc"}, // matches the typedef's pattern but not the derived one
		{"no-admin", "admin"},
		{"mac", "00:11:22:33:44"},
		{"digits", "12a"},
		{"caret", "a"}, // ^ and $ are ordinary characters in XSD regexes
		{"dot", "a\nb"},
		{"wordish", "a_b"}, // XSD \w excludes punctuation, so also '_'
		{"tags", []interface{}{"ok", "NOT"}},
	} {
		t.Run(tc.field, func(t *testing.T) {
			r := v.Validate(map[string]interface{}{tc.field: tc.value}, "t-pat:c", "g")
			assertInvalid(t, r, tc.field, "pattern")
		})
	}
	// A union member's patterns decide whether the value is of that member.
	r := v.Validate(map[string]interface{}{"id-or-num": "ABC"}, "t-pat:c", "g")
	assertInvalid(t, r, "id-or-num", `invalid value "ABC"`)
}

func TestPatternAcceptsMatchingValues(t *testing.T) {
	v := newTestValidator(t, "t-pat", patternYANG)
	r := v.Validate(map[string]interface{}{
		"word": "abc", "xword": "xyz", "no-admin": "administrator",
		"mac": "00:11:22:aa:BB:cc", "digits": "0123", "caret": "^a$",
		"dot": "a-b", "wordish": "héllo-wörld", "tags": []interface{}{"a", "b"},
		"id-or-num": 7,
	}, "t-pat:c", "g")
	assertValid(t, r)
	if len(r.Warnings) > 0 {
		t.Errorf("expected no warnings, got: %s", formatWarnings(r))
	}
}

func TestPatternRE2CannotExpressIsReported(t *testing.T) {
	v := newTestValidator(t, "t-pat", patternYANG)
	items := []map[string]interface{}{
		{"xml-name": "a", "block": "a"},
		{"xml-name": "b", "block": "b"},
	}
	r := v.Validate(items, "t-pat:c", "g")
	assertValid(t, r) // unknown, not wrong
	for _, field := range []string{"xml-name", "block"} {
		n := 0
		for _, w := range r.Warnings {
			if w.Field == field && strings.Contains(w.Message, "cannot be checked") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("expected one 'cannot be checked' warning for %s, got %d: %s", field, n, formatWarnings(r))
		}
	}
}

// Issue #57: a path that names none of the data's fields is at the wrong
// level (typically the container instead of its list), so nothing would be
// checked. That is an error, not Valid=true with warnings.
const levelYANG = `
module t-level {
  namespace "urn:t:level"; prefix tl;
  container show {
    leaf version { type string; }
    list cpu { key id; leaf id { type uint8; } leaf usage { type uint8; } }
  }
}`

func TestPathMatchingNoFieldIsAnError(t *testing.T) {
	v := newTestValidator(t, "t-level", levelYANG)
	items := []map[string]interface{}{{"id": 1, "usage": 50}, {"id": 2, "usage": 60}}
	r := v.Validate(items, "t-level:show", "cpu*")
	assertInvalid(t, r, "", "none of the fields id, usage")
	assertInvalid(t, r, "", "t-level:show/cpu")
	if n := len(r.Errors); n != 1 {
		t.Errorf("expected one error for the whole group, got %d: %s", n, formatErrors(r))
	}

	// A path naming a leaf has no fields at all.
	assertInvalid(t, v.Validate(items, "t-level:show/version", "cpu*"), "", "none of the fields")
}

func TestPathMatchingSomeFieldsIsNotAnError(t *testing.T) {
	v := newTestValidator(t, "t-level", levelYANG)
	r := v.Validate([]map[string]interface{}{{"id": 1, "extra": "x"}}, "t-level:show/cpu", "cpu*")
	assertValid(t, r)
	if len(r.Warnings) != 1 {
		t.Errorf("expected one 'not defined' warning, got: %s", formatWarnings(r))
	}
	// Nothing to check is not an error either.
	assertValid(t, v.Validate([]map[string]interface{}{}, "t-level:show", "cpu*"))
	assertValid(t, v.Validate(map[string]interface{}{}, "t-level:show", "cpu*"))
}
