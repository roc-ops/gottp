package parser

import (
	"strings"
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

func TestParseTemplateMacroWithBareAngleBrackets(t *testing.T) {
	templateText := `
<template>
<group name="test">
{{ value }}
</group>
<macro name="check_values" language="starlark">
def check_values(data):
    x = data.get("count", 0)
    if x < 0:
        data["negative"] = True
    if x > 100:
        data["large"] = True
    if x <= 10:
        data["small"] = True
    if x >= 50:
        data["medium"] = True
    return data
</macro>
</template>
`

	tmpl, err := ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template with bare angle brackets in macro: %v", err)
	}

	if len(tmpl.Macros) != 1 {
		t.Fatalf("Expected 1 macro, got %d", len(tmpl.Macros))
	}

	macro := tmpl.Macros[0]
	if macro.Language != "starlark" {
		t.Errorf("Expected macro language 'starlark', got '%s'", macro.Language)
	}

	// Verify the content preserves the original < and > characters
	if !strings.Contains(macro.Content, "< 0") {
		t.Errorf("Expected macro content to contain '< 0', got: %s", macro.Content)
	}
	if !strings.Contains(macro.Content, "> 100") {
		t.Errorf("Expected macro content to contain '> 100', got: %s", macro.Content)
	}
	if !strings.Contains(macro.Content, "<= 10") {
		t.Errorf("Expected macro content to contain '<= 10', got: %s", macro.Content)
	}
	if !strings.Contains(macro.Content, ">= 50") {
		t.Errorf("Expected macro content to contain '>= 50', got: %s", macro.Content)
	}
}

func TestParseTemplateShowLogTTP(t *testing.T) {
	// This is the template that caused the original crash (stack overflow / OOM)
	// due to bare < and > in Starlark code inside <macro> blocks.
	templateText := `
<template>
<group name="logs">
{{ timestamp }} {{ hostname }} {{ facility }}-{{ severity }}-{{ mnemonic }}: {{ message }}
</group>
<macro name="process_log" language="starlark">
def process_log(data):
    raw = data.get("_raw_line", "")
    bracket_end = raw.find("]")
    if bracket_end < 0:
        return data
    parts = rest.split("-", 2)
    if len(parts) >= 2:
        colon_idx = remainder.find(": ")
        if colon_idx >= 0:
            dash_idx = fac_part.rfind("-")
            if dash_idx >= 0:
                data["facility"] = fac_part[:dash_idx]
    return data
</macro>
</template>
`

	tmpl, err := ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse show_log template: %v", err)
	}

	if len(tmpl.Macros) != 1 {
		t.Fatalf("Expected 1 macro, got %d", len(tmpl.Macros))
	}

	macro := tmpl.Macros[0]
	if macro.Attributes["name"] != "process_log" {
		t.Errorf("Expected macro name 'process_log', got '%s'", macro.Attributes["name"])
	}

	// Verify the content preserves comparison operators
	if !strings.Contains(macro.Content, "< 0") {
		t.Errorf("Expected macro content to contain '< 0', got: %s", macro.Content)
	}
	if !strings.Contains(macro.Content, ">= 2") {
		t.Errorf("Expected macro content to contain '>= 2', got: %s", macro.Content)
	}
	if !strings.Contains(macro.Content, ">= 0") {
		t.Errorf("Expected macro content to contain '>= 0', got: %s", macro.Content)
	}
}

func TestParseTemplateDocWithAngleBrackets(t *testing.T) {
	templateText := `
<doc>
This template parses output where values may be < 100 or > 200.
Use comparison operators like <= and >= in your filters.
</doc>
<group name="test">
{{ value }}
</group>
`

	tmpl, err := ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template with angle brackets in doc: %v", err)
	}

	if !strings.Contains(tmpl.Doc, "< 100") {
		t.Errorf("Expected doc to contain '< 100', got: %s", tmpl.Doc)
	}
	if !strings.Contains(tmpl.Doc, "> 200") {
		t.Errorf("Expected doc to contain '> 200', got: %s", tmpl.Doc)
	}
}

func TestParseTemplateMultipleMacrosWithAngleBrackets(t *testing.T) {
	templateText := `
<template>
<group name="test">
{{ value }}
</group>
<macro name="first" language="starlark">
def first(data):
    if data.get("x", 0) < 5:
        return data
</macro>
<macro name="second" language="starlark">
def second(data):
    if data.get("y", 0) > 10:
        return data
</macro>
</template>
`

	tmpl, err := ParseTemplate(templateText)
	if err != nil {
		t.Fatalf("Failed to parse template with multiple macros: %v", err)
	}

	if len(tmpl.Macros) != 2 {
		t.Fatalf("Expected 2 macros, got %d", len(tmpl.Macros))
	}

	if !strings.Contains(tmpl.Macros[0].Content, "< 5") {
		t.Errorf("Expected first macro to contain '< 5', got: %s", tmpl.Macros[0].Content)
	}
	if !strings.Contains(tmpl.Macros[1].Content, "> 10") {
		t.Errorf("Expected second macro to contain '> 10', got: %s", tmpl.Macros[1].Content)
	}
}

func TestEscapeContentBlocks(t *testing.T) {
	// Test that escapeContentBlocks escapes bare < everywhere except known XML tags
	input := `<group name="test">x < 0</group><macro name="m">if x < 0:</macro><doc>a < b</doc>`
	result := escapeContentBlocks(input)

	// All bare < should be escaped (group, macro, doc content)
	expected := `<group name="test">x &lt; 0</group><macro name="m">if x &lt; 0:</macro><doc>a &lt; b</doc>`
	if result != expected {
		t.Errorf("escapeContentBlocks:\n  got:  %s\n  want: %s", result, expected)
	}

	// Known XML tags should NOT be escaped
	if strings.Contains(result, "&lt;group") || strings.Contains(result, "&lt;macro") || strings.Contains(result, "&lt;doc") {
		t.Errorf("Known XML tags should not be escaped, got: %s", result)
	}
}

