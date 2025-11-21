package validator

import (
	"strings"
	"testing"
)

func TestValidateMacroSource_FStringDetection(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		language string
		wantWarn bool
	}{
		{
			name:     "f-string with double quotes",
			source:   `def test(data): return f"{data}"`,
			language: "starlark",
			wantWarn: true,
		},
		{
			name:     "f-string with single quotes",
			source:   `def test(data): return f'{data}'`,
			language: "starlark",
			wantWarn: true,
		},
		{
			name:     "f-string in multiline macro",
			source:   "def test(data):\n    result = f\"{data.get('key')}\"\n    return result",
			language: "starlark",
			wantWarn: true,
		},
		{
			name:     "no f-string",
			source:   `def test(data): return str(data) + ".txt"`,
			language: "starlark",
			wantWarn: false,
		},
		{
			name:     "f-string in Python macro (should not warn)",
			source:   `def test(data): return f"{data}"`,
			language: "python",
			wantWarn: false,
		},
		{
			name:     "f-string in JavaScript macro (should not warn)",
			source:   "function test(data) { return `${data}`; }",
			language: "javascript",
			wantWarn: false,
		},
		{
			name:     "empty language defaults to starlark",
			source:   `def test(data): return f"{data}"`,
			language: "",
			wantWarn: true,
		},
		{
			name:     "word boundary - 'if' should not match",
			source:   `def test(data): if data: return "value"`,
			language: "starlark",
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateMacroSource(tt.source, tt.language)
			hasFStringWarning := false
			for _, w := range warnings {
				if strings.Contains(w, "f-string") {
					hasFStringWarning = true
					break
				}
			}

			if hasFStringWarning != tt.wantWarn {
				t.Errorf("ValidateMacroSource() f-string warning = %v, want %v. Warnings: %v", hasFStringWarning, tt.wantWarn, warnings)
			}
		})
	}
}

func TestValidateMacroSource_ImplicitStringConcatenation(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		language string
		wantWarn bool
		wantLine int // expected line number in warning (1-indexed)
	}{
		{
			name:     "adjacent double-quoted strings",
			source:   `def test(): return "hello" "world"`,
			language: "starlark",
			wantWarn: true,
			wantLine: 1,
		},
		{
			name:     "adjacent single-quoted strings",
			source:   `def test(): return 'hello' 'world'`,
			language: "starlark",
			wantWarn: true,
			wantLine: 1,
		},
		{
			name:     "adjacent strings on multiline",
			source:   "def test():\n    result = \"hello\" \"world\"\n    return result",
			language: "starlark",
			wantWarn: true,
			wantLine: 2,
		},
		{
			name:     "no adjacent strings",
			source:   `def test(): return "hello" + "world"`,
			language: "starlark",
			wantWarn: false,
		},
		{
			name:     "strings separated by operator",
			source:   `def test(): return "hello" + "world"`,
			language: "starlark",
			wantWarn: false,
		},
		{
			name:     "strings on different lines",
			source:   "def test():\n    a = \"hello\"\n    b = \"world\"",
			language: "starlark",
			wantWarn: false,
		},
		{
			name:     "adjacent strings in Python macro (should not warn)",
			source:   `def test(): return "hello" "world"`,
			language: "python",
			wantWarn: false,
		},
		{
			name:     "user's example - string concatenation with +",
			source:   "def versions(data):\n    software_version = str(data[\"major\"]) + \".\" + \\\n                       str(data[\"minor\"]) + \".\" + \\\n                       str(data[\"patch\"]) + \", Ver \" + \\\n                       str(data[\"version\"]) + \",\" + \\\n                       str(data[\"build\"])\n    data[\"software-version\"] = software_version\n    return data",
			language: "starlark",
			wantWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateMacroSource(tt.source, tt.language)
			hasConcatenationWarning := false
			for _, w := range warnings {
				if strings.Contains(w, "implicit string concatenation") {
					hasConcatenationWarning = true
					break
				}
			}

			if hasConcatenationWarning != tt.wantWarn {
				t.Errorf("ValidateMacroSource() implicit concatenation warning = %v, want %v. Warnings: %v", hasConcatenationWarning, tt.wantWarn, warnings)
			}
		})
	}
}

func TestValidateMacroSource_CombinedWarnings(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		language       string
		wantFString    bool
		wantConcatenation bool
	}{
		{
			name:           "both f-string and implicit concatenation",
			source:         `def test(): return f"{data}" "extra"`,
			language:       "starlark",
			wantFString:    true,
			wantConcatenation: true,
		},
		{
			name:           "only f-string",
			source:         `def test(): return f"{data}"`,
			language:       "starlark",
			wantFString:    true,
			wantConcatenation: false,
		},
		{
			name:           "only implicit concatenation",
			source:         `def test(): return "hello" "world"`,
			language:       "starlark",
			wantFString:    false,
			wantConcatenation: true,
		},
		{
			name:           "neither issue",
			source:         `def test(): return "hello" + "world"`,
			language:       "starlark",
			wantFString:    false,
			wantConcatenation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateMacroSource(tt.source, tt.language)
			
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
				t.Errorf("ValidateMacroSource() f-string warning = %v, want %v", hasFString, tt.wantFString)
			}
			if hasConcatenation != tt.wantConcatenation {
				t.Errorf("ValidateMacroSource() concatenation warning = %v, want %v", hasConcatenation, tt.wantConcatenation)
			}
		})
	}
}

func TestValidateMacroSource_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		language string
		wantWarn bool
	}{
		{
			name:     "empty source",
			source:   "",
			language: "starlark",
			wantWarn: false,
		},
		{
			name:     "source with 'f' but not f-string",
			source:   `def test(): return "file.txt"`,
			language: "starlark",
			wantWarn: false,
		},
		{
			name:     "f in variable name",
			source:   `def test(): f = "value"; return f`,
			language: "starlark",
			wantWarn: false,
		},
		{
			name:     "f-string in comment",
			source:   `def test(): # f"comment"\n    return "value"`,
			language: "starlark",
			wantWarn: true, // Our simple regex will match this, which is acceptable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := ValidateMacroSource(tt.source, tt.language)
			hasWarning := len(warnings) > 0

			if hasWarning != tt.wantWarn {
				t.Errorf("ValidateMacroSource() warning = %v, want %v. Warnings: %v", hasWarning, tt.wantWarn, warnings)
			}
		})
	}
}

