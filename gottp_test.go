package gottp

import (
	"strings"
	"testing"
)

func TestCompiledTemplate_GetWarnings(t *testing.T) {
	tests := []struct {
		name           string
		template       string
		wantWarnings   bool
		wantFString    bool
		wantConcatenation bool
	}{
		{
			name: "template with f-string warning",
			template: `<template>
<macro>
def test(data):
    return f"{data}"
</macro>
<group name="test">test</group>
</template>`,
			wantWarnings:   true,
			wantFString:    true,
			wantConcatenation: false,
		},
		{
			name: "template with implicit concatenation warning",
			template: `<template>
<macro>
def test():
    return "hello" "world"
</macro>
<group name="test">test</group>
</template>`,
			wantWarnings:   true,
			wantFString:    false,
			wantConcatenation: true,
		},
		{
			name: "template with no warnings",
			template: `<template>
<macro>
def test(data):
    return str(data) + ".txt"
</macro>
<group name="test">test</group>
</template>`,
			wantWarnings:   false,
			wantFString:    false,
			wantConcatenation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := CompileTemplate(tt.template)
			if err != nil {
				t.Fatalf("Failed to compile template: %v", err)
			}

			warnings := compiled.GetWarnings()
			hasWarnings := len(warnings) > 0

			if hasWarnings != tt.wantWarnings {
				t.Errorf("GetWarnings() has warnings = %v, want %v. Warnings: %v", hasWarnings, tt.wantWarnings, warnings)
			}

			if tt.wantWarnings {
				hasFString := false
				hasConcatenation := false
				for _, w := range warnings {
					if strings.Contains(w, "f-string") {
						hasFString = true
					}
					if strings.Contains(w, "implicit string concatenation") {
						hasConcatenation = true
					}
				}

				if hasFString != tt.wantFString {
					t.Errorf("GetWarnings() f-string warning = %v, want %v", hasFString, tt.wantFString)
				}
				if hasConcatenation != tt.wantConcatenation {
					t.Errorf("GetWarnings() concatenation warning = %v, want %v", hasConcatenation, tt.wantConcatenation)
				}
			}
		})
	}
}

func TestCompiledTemplate_GetWarnings_EmptyTemplate(t *testing.T) {
	compiled, err := CompileTemplate(`<template><group name="test">test</group></template>`)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	warnings := compiled.GetWarnings()
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for template without macros, got: %v", warnings)
	}
}

