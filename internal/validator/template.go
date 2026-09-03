package validator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/roc-ops/gottp/internal/errors"
	"github.com/roc-ops/gottp/internal/functions/group"
	"github.com/roc-ops/gottp/internal/functions/match"
	"github.com/roc-ops/gottp/internal/parser"
	"go.starlark.net/starlark"
)

// Validator validates TTP templates
type Validator struct {
	matchRegistry *match.Registry
	groupRegistry *group.Registry
}

// NewValidator creates a new template validator
func NewValidator() *Validator {
	return &Validator{
		matchRegistry: match.NewRegistry(),
		groupRegistry: group.NewRegistry(),
	}
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid   bool
	Errors  []error
	Warnings []string
}

// ValidateTemplate validates a parsed template
func (v *Validator) ValidateTemplate(tmpl *parser.Template) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []error{},
		Warnings: []string{},
	}

	// Validate groups
	for _, g := range tmpl.Groups {
		v.validateGroup(g, tmpl.Name, result)
	}

	// Validate inputs
	for _, input := range tmpl.Inputs {
		v.validateInput(input, tmpl.Name, result)
	}

	// Validate outputs
	for _, output := range tmpl.Outputs {
		v.validateOutput(output, tmpl.Name, result)
	}

	// Validate lookups
	for _, lookup := range tmpl.Lookups {
		v.validateLookup(lookup, tmpl.Name, result)
	}

	// Validate macros
	for _, macro := range tmpl.Macros {
		macroWarnings := ValidateMacroSource(macro.Content, macro.Language)
		for _, warning := range macroWarnings {
			result.Warnings = append(result.Warnings, fmt.Sprintf("macro (language=%s): %s", macro.Language, warning))
		}
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// validateGroup validates a group
func (v *Validator) validateGroup(g *parser.Group, templateName string, result *ValidationResult) {
	// Validate group name
	if g.Name == "" {
		result.Errors = append(result.Errors, errors.NewTemplateError(
			templateName, "group", "name", "", "group name is required", 0, nil,
		))
	}

	// Validate functions
	if g.Functions != "" {
		v.validateGroupFunctionString(g.Functions, "group", templateName, result)
	}

	// Validate chain
	if g.Chain != "" {
		// Chain should reference a variable - just check it's not empty
		if strings.TrimSpace(g.Chain) == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("group '%s' has empty chain attribute", g.Name))
		}
	}

	// Validate pattern syntax (basic check)
	if g.Pattern != "" {
		if _, err := regexp.Compile(g.Pattern); err != nil {
			result.Errors = append(result.Errors, errors.NewTemplateError(
				templateName, "group", "pattern", g.Pattern, fmt.Sprintf("invalid regex pattern: %v", err), 0, err,
			))
		}
	}

	// Validate nested groups
	for _, nested := range g.Groups {
		v.validateGroup(nested, templateName, result)
	}
}

// validateInput validates an input
func (v *Validator) validateInput(input *parser.Input, templateName string, result *ValidationResult) {
	// Validate load type
	validLoadTypes := map[string]bool{
		"text":     true,
		"yaml":     true,
		"json":     true,
		"csv":      true,
		"file":     true,
		"directory": true,
		"url":      true,
		"database": true,
	}

	if input.Load != "" && !validLoadTypes[input.Load] {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"input '%s' has unknown load type '%s'", input.Name, input.Load,
		))
	}

	// Validate functions
	if input.Functions != "" {
		// Input functions are handled differently - just check syntax
		if strings.TrimSpace(input.Functions) == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("input '%s' has empty functions attribute", input.Name))
		}
	}
}

// validateOutput validates an output
func (v *Validator) validateOutput(output *parser.Output, templateName string, result *ValidationResult) {
	// Validate format
	validFormats := map[string]bool{
		"json":    true,
		"yaml":    true,
		"raw":     true,
		"csv":     true,
		"table":   true,
		"pprint":  true,
		"tabulate": true,
		"excel":   true,
		"jinja2":  true,
		"n2g":     true,
	}

	if output.Format != "" && !validFormats[output.Format] {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"output '%s' has unknown format '%s'", output.Name, output.Format,
		))
	}

	// Validate returner
	validReturners := map[string]bool{
		"self":     true,
		"terminal": true,
		"file":     true,
		"syslog":   true,
	}

	if output.Returner != "" && !validReturners[output.Returner] {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"output '%s' has unknown returner '%s'", output.Name, output.Returner,
		))
	}
}

// validateLookup validates a lookup
func (v *Validator) validateLookup(lookup *parser.Lookup, templateName string, result *ValidationResult) {
	// Validate load type
	validLoadTypes := map[string]bool{
		"yaml":     true,
		"json":     true,
		"csv":      true,
		"file":     true,
		"directory": true,
		"database": true,
	}

	if lookup.Load != "" && !validLoadTypes[lookup.Load] {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"lookup '%s' has unknown load type '%s'", lookup.Name, lookup.Load,
		))
	}
}

// validateGroupFunctionString validates a group function string
func (v *Validator) validateGroupFunctionString(funcStr, context, templateName string, result *ValidationResult) {
	// Split by pipe
	funcs := strings.Split(funcStr, "|")
	for _, fn := range funcs {
		fn = strings.TrimSpace(fn)
		if fn == "" {
			continue
		}

		// Extract function name (before first '(')
		funcName := fn
		if idx := strings.Index(fn, "("); idx > 0 {
			funcName = fn[:idx]
		}

		// Check if function exists in registry
		if _, exists := v.groupRegistry.Get(funcName); !exists {
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"%s function '%s' not found in registry (may be a macro or custom function)", context, funcName,
			))
		}

		// Basic syntax validation - check parentheses are balanced
		openCount := strings.Count(fn, "(")
		closeCount := strings.Count(fn, ")")
		if openCount != closeCount {
			result.Errors = append(result.Errors, errors.NewTemplateError(
				templateName, context, "functions", funcStr,
				fmt.Sprintf("unbalanced parentheses in function '%s'", fn), 0, nil,
			))
		}
	}
}

// ValidateTemplateString validates a template string before parsing
func (v *Validator) ValidateTemplateString(templateStr, templateName string) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []error{},
		Warnings: []string{},
	}

	// Basic XML structure validation
	if !strings.Contains(templateStr, "<template") && !strings.Contains(templateStr, "<group") {
		result.Errors = append(result.Errors, errors.NewContextError(
			"template does not contain <template> or <group> tags", templateName, 0, nil,
		))
	}

	// Check for balanced XML tags (basic check)
	openTags := strings.Count(templateStr, "<")
	closeTags := strings.Count(templateStr, ">")
	if openTags != closeTags {
		result.Warnings = append(result.Warnings, "unbalanced XML tags detected")
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateMacroSource validates macro source code and returns warnings for
// Python-specific features that are not compatible with Starlark
func ValidateMacroSource(source, language string) []string {
	var warnings []string
	
	// Only validate Starlark macros (default) - Python macros are expected to use Python syntax
	if language != "" && language != "starlark" {
		return warnings
	}
	
	// Check for Python f-strings (f"..." or f'...')
	fStringPattern := regexp.MustCompile(`\bf["']`)
	if fStringPattern.MatchString(source) {
		warnings = append(warnings, "macro contains Python f-string syntax (f\"...\" or f'...'). Starlark does not support f-strings. Use string concatenation with + or .format() instead.")
	}
	
	// Check for implicit string concatenation (adjacent string literals)
	// Pattern: "string1" "string2" or 'string1' 'string2'
	// This is tricky because we need to avoid matching strings that are part of expressions
	// We'll look for patterns like: "..." "..." or '...' '...' on the same line
	// Go's regexp doesn't support backreferences, so we'll check for both patterns separately
	lines := strings.Split(source, "\n")
	for lineNum, line := range lines {
		// Look for adjacent double-quoted strings: "..." "..."
		doubleQuotePattern := regexp.MustCompile(`"[^"]*"\s+"[^"]*"`)
		// Look for adjacent single-quoted strings: '...' '...'
		singleQuotePattern := regexp.MustCompile(`'[^']*'\s+'[^']*'`)
		
		if doubleQuotePattern.MatchString(line) || singleQuotePattern.MatchString(line) {
			warnings = append(warnings, fmt.Sprintf("macro contains implicit string concatenation (adjacent string literals) on line %d. Starlark does not support implicit concatenation. Use + operator to concatenate strings explicitly.", lineNum+1))
		}
	}

	// The checks above only catch specific known Python-isms by pattern
	// matching. Actually attempt to compile the source as Starlark so any
	// other invalid construct (e.g. `del`, which this version of
	// go.starlark.net doesn't support at all) is caught too. Without this,
	// macro registration fails silently at parse time (it's intentionally
	// non-fatal there, matching Python TTP's "an unavailable macro is
	// skipped" behavior) and every function defined in this source block
	// becomes permanently unavailable with no visible error at all.
	// See https://github.com/roc-ops/gottp/issues/26.
	thread := &starlark.Thread{Name: "macro-validation"}
	predeclared := starlark.StringDict{"_ttp_": starlark.NewDict(0)}
	if _, err := starlark.ExecFile(thread, "<macro>", source, predeclared); err != nil {
		warnings = append(warnings, fmt.Sprintf("macro failed to compile as Starlark and will not be available at parse time: %v", err))
	}

	return warnings
}

