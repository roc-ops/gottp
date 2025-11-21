package compiler

import (
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/parser"
)

func TestCompiler_CompileTemplate_MacroWarnings(t *testing.T) {
	tests := []struct {
		name           string
		template       string
		wantFString    bool
		wantConcatenation bool
	}{
		{
			name: "macro with f-string",
			template: `<template>
<macro>
def test(data):
    return f"{data}"
</macro>
<group name="test">test</group>
</template>`,
			wantFString:    true,
			wantConcatenation: false,
		},
		{
			name: "macro with implicit string concatenation",
			template: `<template>
<macro>
def test():
    return "hello" "world"
</macro>
<group name="test">test</group>
</template>`,
			wantFString:    false,
			wantConcatenation: true,
		},
		{
			name: "macro with both issues",
			template: `<template>
<macro>
def test(data):
    return f"{data}" "extra"
</macro>
<group name="test">test</group>
</template>`,
			wantFString:    true,
			wantConcatenation: true,
		},
		{
			name: "valid macro (no warnings)",
			template: `<template>
<macro>
def test(data):
    return str(data) + ".txt"
</macro>
<group name="test">test</group>
</template>`,
			wantFString:    false,
			wantConcatenation: false,
		},
		{
			name: "Python macro (should not warn)",
			template: `<template>
<macro language="python">
def test(data):
    return f"{data}"
</macro>
<group name="test">test</group>
</template>`,
			wantFString:    false,
			wantConcatenation: false,
		},
		{
			name: "multiple macros with warnings",
			template: `<template>
<macro>
def test1(data):
    return f"{data}"
</macro>
<macro>
def test2():
    return "a" "b"
</macro>
<group name="test">test</group>
</template>`,
			wantFString:    true,
			wantConcatenation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse template
			tmpl, err := parser.ParseTemplate(tt.template)
			if err != nil {
				t.Fatalf("Failed to parse template: %v", err)
			}

			// Compile template
			comp := NewCompiler()
			compiled, err := comp.CompileTemplate(tmpl)
			if err != nil {
				t.Fatalf("Failed to compile template: %v", err)
			}

			// Check warnings
			hasFString := false
			hasConcatenation := false
			for _, warning := range compiled.Warnings {
				if strings.Contains(warning, "f-string") {
					hasFString = true
				}
				if strings.Contains(warning, "implicit string concatenation") {
					hasConcatenation = true
				}
			}

			if hasFString != tt.wantFString {
				t.Errorf("CompileTemplate() f-string warning = %v, want %v. Warnings: %v", hasFString, tt.wantFString, compiled.Warnings)
			}
			if hasConcatenation != tt.wantConcatenation {
				t.Errorf("CompileTemplate() concatenation warning = %v, want %v. Warnings: %v", hasConcatenation, tt.wantConcatenation, compiled.Warnings)
			}
		})
	}
}

func TestCompiler_CompileTemplate_NoWarningsForValidMacros(t *testing.T) {
	// Test the user's example - valid Starlark string concatenation
	template := `<template>
<macro>
def versions(data):
    software_version = str(data["major"]) + "." + \
                       str(data["minor"]) + "." + \
                       str(data["patch"]) + ", Ver " + \
                       str(data["version"]) + "," + \
                       str(data["build"])
    data["software-version"] = software_version
    return data
</macro>
<group name="version-info" macro="versions">
Running Image: {{smm-type}} Rel {{major}}.{{minor}}.{{patch}}, Ver {{version | re('[^,]+')}},{{build}}, {{date | re('[^,]+')}},(relmgr)
</group>
</template>`

	tmpl, err := parser.ParseTemplate(template)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	comp := NewCompiler()
	compiled, err := comp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	// Should have no warnings for valid Starlark syntax
	if len(compiled.Warnings) > 0 {
		t.Errorf("Expected no warnings for valid macro, got: %v", compiled.Warnings)
	}
}

