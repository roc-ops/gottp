package parser

import (
	"strings"
	"testing"
)

// A well-formed unknown tag counts as a tag for the implicit-group rule, as
// in Python TTP (#31 review); a bare '<' in template text does not.
func TestUnknownTagIgnoresTopText(t *testing.T) {
	cases := []struct {
		name, tmpl string
		implicit   bool
		warnTag    string
	}{
		{"paired", "<foo>bar</foo>\ninterface {{ name }}\n", false, "foo"},
		{"self-closing", "<foo/>\ninterface {{ name }}\n", false, "foo"},
		{"bare angle", "IPv4 for <{{ vr }}> x\ninterface {{ name }}\n", true, ""},
		{"mismatched", "<foo>bar</baz>\ninterface {{ name }}\n", true, ""},
		{"no tags", "interface {{ name }}\n", true, ""},
	}
	for _, c := range cases {
		tmpl, err := ParseTemplate(c.tmpl)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := len(tmpl.Groups) > 0; got != c.implicit {
			t.Errorf("%s: implicit group = %v, want %v", c.name, got, c.implicit)
		}
		warned := strings.Contains(strings.Join(tmpl.Warnings, "\n"), "unknown tag <"+c.warnTag+">")
		if c.warnTag != "" && !warned {
			t.Errorf("%s: warnings %q lack unknown tag <%s>", c.name, tmpl.Warnings, c.warnTag)
		}
	}
}
