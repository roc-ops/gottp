package parser

import (
	"testing"
)

func TestParseTemplate(t *testing.T) {
	templateText := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>
`

	tmpl, err := ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	if len(tmpl.Groups) != 1 {
		t.Fatalf("Expected 1 group, got %d", len(tmpl.Groups))
	}

	group := tmpl.Groups[0]
	if group.Name != "interfaces" {
		t.Errorf("Expected group name 'interfaces', got '%s'", group.Name)
	}

	if group.Pattern == "" {
		t.Error("Expected group pattern to be set")
	}
}

func TestParseTemplateWithMacro(t *testing.T) {
	templateText := `
<macro language="starlark">
def process(data):
    return data.upper()
</macro>
<group name="test">
{{ value | macro("process") }}
</group>
`

	tmpl, err := ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	if len(tmpl.Macros) != 1 {
		t.Fatalf("Expected 1 macro, got %d", len(tmpl.Macros))
	}

	macro := tmpl.Macros[0]
	if macro.Language != "starlark" {
		t.Errorf("Expected macro language 'starlark', got '%s'", macro.Language)
	}
}

