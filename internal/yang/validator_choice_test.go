package yang

import (
	"strings"
	"testing"
)

// choiceSemanticsYANG is the repro from issues #122 and #124: a mandatory
// choice with a case holding a mandatory leaf, a case holding a container,
// a shorthand case and a nested choice.
const choiceSemanticsYANG = `
module p {
  namespace "urn:p"; prefix p;
  container c {
    choice ch {
      mandatory true;
      case a {
        leaf a1 { type string; }
        leaf a2 { type string; mandatory true; }
      }
      case b {
        leaf b1 { type string; }
        container bc {
          leaf x { type string { length "1..3"; } }
          choice inner {
            case i1 { container deep { leaf d { type uint8; } } }
            case i2 { leaf j1 { type string; } leaf j2 { type string; mandatory true; } }
          }
        }
      }
      leaf sh { type string; }
    }
    choice opt {
      case o1 { leaf o1a { type string; } leaf o1b { type string; mandatory true; } }
      case o2 { leaf o2a { type string; } }
    }
    leaf s { type string { length "1..3"; } }
  }
}`

// Issue #122: the choice name never appears in the data tree (RFC 7950
// 7.9.4); a mandatory choice is satisfied by a node from any of its cases.
// Behaviour checked against yangson (Python TTP's validate_yangson).
func TestMandatoryChoiceSatisfiedByAnyCase(t *testing.T) {
	v := newTestValidator(t, "p", choiceSemanticsYANG)
	for _, data := range []map[string]interface{}{
		{"b1": "x"},
		{"a1": "x", "a2": "y"},
		{"sh": "q"},
		{"bc": map[string]interface{}{"x": "x"}},
	} {
		assertValid(t, v.Validate(data, "p:c", "g"))
	}
}

func TestMandatoryLeafInActiveCaseIsRequired(t *testing.T) {
	v := newTestValidator(t, "p", choiceSemanticsYANG)
	r := v.Validate(map[string]interface{}{"a1": "x"}, "p:c", "g")
	assertInvalid(t, r, "a2", "Required field 'a2' is missing")
	assertNoErrorOn(t, r, "ch")

	// The same rule for a choice that is not itself mandatory.
	r = v.Validate(map[string]interface{}{"b1": "x", "o1a": "x"}, "p:c", "g")
	assertInvalid(t, r, "o1b", "Required field 'o1b' is missing")

	// And for a choice nested in a container under a case.
	r = v.Validate(map[string]interface{}{"bc": map[string]interface{}{"j1": "x"}}, "p:c", "g")
	assertInvalid(t, r, "j2", "Required field 'j2' is missing")
}

func TestMandatoryChoiceWithNoCaseIsAnError(t *testing.T) {
	v := newTestValidator(t, "p", choiceSemanticsYANG)
	r := v.Validate(map[string]interface{}{"s": "abc"}, "p:c", "g")
	assertInvalid(t, r, "ch", "choice 'ch'")
	if len(r.Errors) != 1 {
		t.Errorf("expected one error, got: %s", formatErrors(r))
	}
	// A list of items reports the item.
	r = v.Validate([]map[string]interface{}{{"b1": "x"}, {"s": "abc"}}, "p:c", "g")
	assertInvalid(t, r, "ch", "in item 1")
}

// RFC 7950 7.9: data nodes from at most one case may exist; yangson
// reports member-not-allowed.
func TestNodesFromTwoCasesAreAnError(t *testing.T) {
	v := newTestValidator(t, "p", choiceSemanticsYANG)
	r := v.Validate(map[string]interface{}{"a1": "x", "a2": "y", "b1": "z"}, "p:c", "g")
	assertInvalid(t, r, "ch", "more than one case")
	r = v.Validate(map[string]interface{}{"b1": "z", "sh": "y"}, "p:c", "g")
	assertInvalid(t, r, "ch", "more than one case")
}

// Issue #122: length restrictions count characters (RFC 7950 9.4.4), not
// bytes; yangson accepts "ééé" for length 1..3 and rejects "éééé".
func TestLengthCountsCharacters(t *testing.T) {
	v := newTestValidator(t, "p", choiceSemanticsYANG)
	assertValid(t, v.Validate(map[string]interface{}{"b1": "x", "s": "ééé"}, "p:c", "g"))
	assertInvalid(t, v.Validate(map[string]interface{}{"b1": "x", "s": "éééé"}, "p:c", "g"), "s", "length is out of range")
}

// Issue #124: a yang= path names data nodes; choice and case are skipped
// (RFC 7950 7.9) the way field lookups skip them.
func TestPathResolvesDataNodeUnderChoice(t *testing.T) {
	v := newTestValidator(t, "p", choiceSemanticsYANG)
	for _, path := range []string{"p:c/bc", "p:c/ch/b/bc", "p:c/ch/bc", "p:c.bc"} {
		t.Run(path, func(t *testing.T) {
			assertValid(t, v.Validate(map[string]interface{}{"x": "ok"}, path, "g"))
			assertInvalid(t, v.Validate(map[string]interface{}{"x": "toolong"}, path, "g"), "x", "length is out of range")
		})
	}
	// Nested choice, both forms.
	for _, path := range []string{"p:c/bc/deep", "p:c/ch/b/bc/inner/i1/deep"} {
		t.Run(path, func(t *testing.T) {
			assertInvalid(t, v.Validate(map[string]interface{}{"d": 300}, path, "g"), "d", "out of range")
		})
	}
}

// A shorthand case has the name of its node; a data-node-first lookup
// finds the leaf, not the case.
func TestPathPrefersDataNodeOverSameNamedCase(t *testing.T) {
	ms := newTestValidator(t, "p", choiceSemanticsYANG).moduleSet
	e, err := ms.FindSchemaByPath("p:c/ch/sh")
	if err != nil {
		t.Fatalf("FindSchemaByPath: %v", err)
	}
	if !e.IsLeaf() {
		t.Errorf("expected leaf sh, got kind %v", e.Kind)
	}
}

func TestPathEndingOnChoiceOrCaseIsAnError(t *testing.T) {
	v := newTestValidator(t, "p", choiceSemanticsYANG)
	for path, want := range map[string]string{
		"p:c/ch":         "choice",
		"p:c/ch/b":       "case",
		"p:c/bc/inner":   "choice",
		"p:c/opt/o1":     "case",
		"p:c/ch/b/bc/i2": "",
	} {
		t.Run(path, func(t *testing.T) {
			r := v.Validate(map[string]interface{}{"b1": "x"}, path, "g")
			if r.Valid {
				t.Fatalf("expected invalid")
			}
			msg := formatErrors(r)
			if want == "" {
				// i2 is a case of choice inner, not a child of bc itself.
				if !strings.Contains(msg, "not found") {
					t.Errorf("expected not found, got: %s", msg)
				}
				return
			}
			if !strings.Contains(msg, "names a "+want) {
				t.Errorf("expected 'names a %s', got: %s", want, msg)
			}
		})
	}
}

func TestPathNotFoundListsDataNodeChildren(t *testing.T) {
	v := newTestValidator(t, "p", choiceSemanticsYANG)
	r := v.Validate(map[string]interface{}{"x": "x"}, "p:c/nope", "g")
	msg := formatErrors(r)
	for _, want := range []string{"node 'nope' not found at path 'c'", "bc", "b1", "s"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected %q in: %s", want, msg)
		}
	}
}

func assertNoErrorOn(t *testing.T, r *ValidationResult, field string) {
	t.Helper()
	for _, e := range r.Errors {
		if e.Field == field {
			t.Errorf("unexpected error on %q: %s", field, e.Message)
		}
	}
}
