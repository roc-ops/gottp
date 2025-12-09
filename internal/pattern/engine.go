package pattern

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Built-in pattern constants
const (
	PatternPHRASE   = `(?:\S+ {1})+?\S+`
	PatternROW      = `(?:\S+ +)+?\S+`
	PatternORPHRASE = `\S+|(?:\S+ {1})+?\S+`
	PatternDIGIT    = `\d+`
	PatternIP       = `(?:[0-9]{1,3}\.){3}[0-9]{1,3}`
	PatternPREFIX   = `(?:[0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}`
	PatternIPV6     = `(?:[a-fA-F0-9]{1,4}:|:){1,7}(?:[a-fA-F0-9]{1,4}|:?)`
	PatternPREFIXV6 = `(?:[a-fA-F0-9]{1,4}:|:){1,7}(?:[a-fA-F0-9]{1,4}|:?)/[0-9]{1,3}`
	PatternLINE     = `.+`
	PatternWORD     = `\S+`
	PatternMAC      = `(?:[0-9a-fA-F]{2}(?::|\.|\-)){5}(?:[0-9a-fA-F]{2})|(?:[0-9a-fA-F]{4}(?::|\.|\-)){2}(?:[0-9a-fA-F]{4})`
)

// MatchVariable represents a parsed match variable
type MatchVariable struct {
	Name                  string
	Functions             []string // function pipeline
	Pattern               string   // regex pattern for this variable
	IgnoreUsesTemplateVar bool     // true if ignore() uses a template variable (Python TTP quirk: returns empty result)
}

// CompiledPattern represents a compiled regex pattern with variable information
type CompiledPattern struct {
	Regex         *regexp.Regexp
	Variables     map[string]*MatchVariable
	VariableOrder []string // Order of variables as they appear in the pattern
	Original      string
}

// Engine handles pattern generation and compilation
type Engine struct {
	patterns map[string]string // built-in patterns
}

// NewEngine creates a new pattern engine
func NewEngine() *Engine {
	return &Engine{
		patterns: map[string]string{
			"PHRASE":   PatternPHRASE,
			"ROW":      PatternROW,
			"ORPHRASE": PatternORPHRASE,
			"DIGIT":    PatternDIGIT,
			"IP":       PatternIP,
			"PREFIX":   PatternPREFIX,
			"IPV6":     PatternIPV6,
			"PREFIXV6": PatternPREFIXV6,
			"_line_":   PatternLINE,
			"WORD":     PatternWORD,
			"MAC":      PatternMAC,
		},
	}
}

// ExtractVariables extracts match variables from a template line
func (e *Engine) ExtractVariables(line string) []*MatchVariable {
	return e.ExtractVariablesWithVars(line, nil)
}

// ExtractVariablesWithVars extracts match variables from a template line with access to template variables
// This allows chain() function to expand functions from variables
func (e *Engine) ExtractVariablesWithVars(line string, vars map[string]interface{}) []*MatchVariable {
	var variables []*MatchVariable

	// Special case: if line is just "_start_", "_end_", or "_line_" (possibly with whitespace), treat as variable
	trimmedLine := strings.TrimSpace(line)
	if trimmedLine == "_start_" || trimmedLine == "_end_" || trimmedLine == "_line_" {
		variable := &MatchVariable{
			Name:      trimmedLine,
			Functions: []string{},
			Pattern:   "", // Will be set by getPatternForVariable
		}
		// Set pattern using the same logic as other variables
		variable.Pattern = e.getPatternForVariable(variable, nil)
		variables = append(variables, variable)
		return variables
	}

	// Find all {{ variable }} patterns
	re := regexp.MustCompile(`\{\{([\S\s]+?)\}\}`)
	matches := re.FindAllStringSubmatch(line, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		variable := e.parseVariableWithVars(match[1], vars)
		variables = append(variables, variable)
	}

	return variables
}

// parseVariable parses a variable string into a MatchVariable
func (e *Engine) parseVariable(variableStr string) *MatchVariable {
	return e.parseVariableWithVars(variableStr, nil)
}

// parseVariableWithVars parses a variable string into a MatchVariable with access to template variables
func (e *Engine) parseVariableWithVars(variableStr string, vars map[string]interface{}) *MatchVariable {
	variable := &MatchVariable{
		Functions: []string{},
	}

	// Split by pipe to get name and functions
	parts := strings.Split(variableStr, "|")
	varName := strings.TrimSpace(parts[0])

	// Check if variable name has arguments (e.g., ignore("pattern"))
	// This is special handling for ignore, which can take a pattern argument
	if strings.HasPrefix(varName, "ignore(") && strings.HasSuffix(varName, ")") {
		// Extract the pattern argument
		argStr := varName[7 : len(varName)-1] // Remove "ignore(" and ")"
		argStr = strings.TrimSpace(argStr)
		// Remove quotes if present
		argStr = strings.Trim(argStr, `"'`)
		variable.Name = "ignore"
		// Store the pattern in Functions for now (we'll use it in getPatternForVariable)
		if argStr != "" {
			variable.Functions = append(variable.Functions, "pattern:"+argStr)
		}
	} else {
		variable.Name = varName
	}

	// Parse functions (after handling ignore arguments)
	if len(parts) > 1 {
		for i := 1; i < len(parts); i++ {
			funcStr := strings.TrimSpace(parts[i])
			if funcStr != "" {
				// Check if this is a chain function
				if strings.HasPrefix(funcStr, "chain(") && strings.HasSuffix(funcStr, ")") {
					// Extract chain variable name
					chainVarStr := funcStr[6 : len(funcStr)-1] // Remove "chain(" and ")"
					chainVarStr = strings.TrimSpace(chainVarStr)
					chainVarStr = strings.Trim(chainVarStr, `"'`)

					// Expand chain if vars are available
					if vars != nil {
						if chainValue, ok := vars[chainVarStr]; ok {
							// Extract functions from chain variable
							chainFuncs := e.extractFunctionsFromChainValue(chainValue)
							// Add chain functions to variable's function list
							variable.Functions = append(variable.Functions, chainFuncs...)
						}
					}
					// Don't add "chain(...)" itself to functions
				} else {
					variable.Functions = append(variable.Functions, funcStr)
				}
			}
		}
	}

	// Determine pattern based on variable name or functions
	variable.Pattern = e.getPatternForVariable(variable, vars)

	return variable
}

// extractFunctionsFromChainValue extracts function strings from a chain variable value
// The value can be a string (pipe-separated functions) or a list of strings
func (e *Engine) extractFunctionsFromChainValue(value interface{}) []string {
	var funcs []string

	switch v := value.(type) {
	case string:
		// String value - split by pipe to get functions
		parts := strings.Split(v, "|")
		for _, part := range parts {
			funcStr := strings.TrimSpace(part)
			if funcStr != "" {
				funcs = append(funcs, funcStr)
			}
		}
	case []interface{}:
		// List of strings - each item is a function string
		for _, item := range v {
			if str, ok := item.(string); ok {
				funcStr := strings.TrimSpace(str)
				if funcStr != "" {
					funcs = append(funcs, funcStr)
				}
			}
		}
	case []string:
		// List of strings directly
		for _, str := range v {
			funcStr := strings.TrimSpace(str)
			if funcStr != "" {
				funcs = append(funcs, funcStr)
			}
		}
	}

	return funcs
}

// getPatternForVariable determines the regex pattern for a variable
func (e *Engine) getPatternForVariable(variable *MatchVariable, vars map[string]interface{}) string {
	// Check if variable name matches a built-in pattern
	// BUT: Only use built-in patterns if the name is exactly uppercase (e.g., "IP", not "ip")
	// This matches Python TTP behavior where {{ IP }} uses built-in pattern but {{ ip }} uses default
	upperName := strings.ToUpper(variable.Name)
	if upperName == variable.Name {
		// Name is already uppercase - use built-in pattern if available
		if pattern, ok := e.patterns[upperName]; ok {
			return pattern
		}
	}
	// If name is lowercase, don't use built-in patterns even if they match when uppercased
	// This matches Python TTP behavior

	// Check for special variables
	switch variable.Name {
	case "_line_":
		return PatternLINE
	case "_end_", "_start_":
		// _start_ matches the entire line but doesn't capture
		// _end_ matches until end
		return `.*?` // match anything (non-greedy)
	case "_exact_", "_exact_space_":
		// _exact_ and _exact_space_ are indicators, not variables that capture
		// They should not appear in the pattern at all
		// Return empty pattern - these will be handled specially
		return ``
	case "ignore":
		// ignore matches but doesn't save
		// Check if a custom pattern was provided
		for _, funcStr := range variable.Functions {
			if strings.HasPrefix(funcStr, "pattern:") {
				// Extract the pattern from "pattern:..."
				patternName := strings.TrimPrefix(funcStr, "pattern:")
				var pattern string
				// First check if it's a template variable
				if vars != nil {
					if varValue, ok := vars[patternName]; ok {
						// It's a template variable - use its value as the pattern
						// Python TTP quirk: when ignore uses a template variable, it returns empty result
						variable.IgnoreUsesTemplateVar = true
						pattern = fmt.Sprintf("%v", varValue)
					} else {
						// Check if it's a built-in pattern name (like ORPHRASE)
						if builtInPattern, ok := e.patterns[patternName]; ok {
							pattern = builtInPattern
						} else {
							// Otherwise use as-is (it's a regex pattern string)
							pattern = patternName
						}
					}
				} else {
					// Check if it's a built-in pattern name (like ORPHRASE)
					if builtInPattern, ok := e.patterns[patternName]; ok {
						pattern = builtInPattern
					} else {
						// Otherwise use as-is (it's a regex pattern string)
						pattern = patternName
					}
				}
				// Convert Python-style regex to RE2 format
				return e.convertPythonRegexToRE2(pattern)
			}
		}
		// Default pattern for ignore is \S+ (non-space characters)
		return `\S+`
	}

	// Check Functions list for pattern formatters (like DIGIT, IP, etc.)
	// These are used as pattern formatters, not as functions
	// Pattern formatters should be checked before other functions
	for _, funcStr := range variable.Functions {
		// Remove any function arguments (e.g., "DIGIT" from "DIGIT" or "re('pattern')" from "re('pattern')")
		funcName := funcStr
		if strings.Contains(funcStr, "(") {
			funcName = funcStr[:strings.Index(funcStr, "(")]
		}
		funcName = strings.TrimSpace(funcName)

		// Check if this is a built-in pattern formatter
		if pattern, ok := e.patterns[funcName]; ok {
			return pattern
		}
		// Also check re() function which can specify a pattern
		if funcName == "re" {
			// Extract pattern from re("pattern") or re('pattern')
			if strings.Contains(funcStr, "(") && strings.Contains(funcStr, ")") {
				start := strings.Index(funcStr, "(")
				end := strings.LastIndex(funcStr, ")")
				if start >= 0 && end > start {
					arg := funcStr[start+1 : end]
					arg = strings.TrimSpace(arg)
					arg = strings.Trim(arg, `"'`)
					// First check if it's a template variable
					if vars != nil {
						if varValue, ok := vars[arg]; ok {
							// It's a template variable - use its value as the pattern
							return fmt.Sprintf("%v", varValue)
						}
					}
					// Check if it's a built-in pattern name
					if pattern, ok := e.patterns[arg]; ok {
						return pattern
					}
					// Otherwise use the argument as-is (it's a regex pattern)
					return arg
				}
			}
		}
	}

	// Check if variable has a default function - if so, allow empty matches
	// This allows default() to work when the value is empty or whitespace-only
	// Use .*? (any characters, non-greedy) to allow empty matches including whitespace
	for _, funcStr := range variable.Functions {
		if strings.HasPrefix(funcStr, "default(") {
			// Use .*? (any characters, non-greedy) to allow empty matches
			// This matches Python TTP behavior where default() can handle empty values
			// Note: We use .*? instead of \S* because trailing spaces are trimmed from lines,
			// so we need to match the end of the line (which may be empty after trimming)
			return `.*?`
		}
	}

	// Check if variable has set() function - if so, allow empty matches
	// Variables with set() can match empty strings since set() will set the value regardless
	for _, funcStr := range variable.Functions {
		if strings.HasPrefix(funcStr, "set(") {
			// Use .*? (any characters, non-greedy) to allow empty matches
			// This allows set() to work even when the variable doesn't match anything
			return `.*?`
		}
	}

	// Default pattern - match non-whitespace (at least one character)
	return PatternWORD
}

// GenerateRegex generates a regex pattern from a template line
func (e *Engine) GenerateRegex(line string, exactSpace, exact bool) (string, []*MatchVariable, error) {
	// Extract variables
	variables := e.ExtractVariables(line)
	regexStr, err := e.GenerateRegexFromVariables(line, variables, exactSpace, exact)
	return regexStr, variables, err
}

// GenerateRegexFromVariables generates a regex pattern from a template line using pre-extracted variables
func (e *Engine) GenerateRegexFromVariables(line string, variables []*MatchVariable, exactSpace, exact bool) (string, error) {
	if len(variables) == 0 {
		// Lines without variables are treated as literal text that must match exactly
		// Escape the line and return it as a regex pattern
		escaped := e.escapeText(line, exactSpace, exact)
		regexStr := "^" + escaped + "$"
		return regexStr, nil
	}

	// Special handling for _start_, _end_, and _line_ patterns on their own lines
	// This must be checked before looking for {{ }} patterns, since they can be without {{ }}
	trimmedLine := strings.TrimSpace(line)
	if len(variables) == 1 {
		if variables[0].Name == "_start_" {
			// Check if line only has _start_ variable (possibly with whitespace)
			// Can be either "{{ _start_ }}" or just "_start_"
			if trimmedLine == "{{ _start_ }}" || trimmedLine == "{{_start_}}" || trimmedLine == "_start_" {
				// _start_ on its own line should match any line (including empty lines)
				// It serves as a marker to start a new group, matching the first line of the block
				// Use .* to match any line (including empty lines)
				return "^.*$", nil
			}
		}
		if variables[0].Name == "_end_" {
			// Check if line only has _end_ variable (possibly with whitespace)
			// Can be either "{{ _end_ }}" or just "_end_"
			if trimmedLine == "{{ _end_ }}" || trimmedLine == "{{_end_}}" || trimmedLine == "_end_" {
				// _end_ on its own line should match EMPTY lines only
				// It serves as a marker to end the group, typically matching the blank line
				// between data blocks. Using ^.*$ was wrong as it matched every line.
				return "^$", nil
			}
		}
		if variables[0].Name == "_line_" {
			// Check if line only has _line_ variable (possibly with whitespace)
			// Can be either "{{ _line_ }}" or just "_line_"
			if trimmedLine == "{{ _line_ }}" || trimmedLine == "{{_line_}}" || trimmedLine == "_line_" {
				// _line_ on its own line should match any line
				return "^.+$", nil
			}
		}
	}

	// Find variable positions
	re := regexp.MustCompile(`\{\{([\S\s]+?)\}\}`)
	matches := re.FindAllStringSubmatchIndex(line, -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("no variable matches found")
	}

	// Build regex by replacing variables and escaping text
	var regexParts []string
	lastEnd := 0

	for i, match := range matches {
		// Text before variable
		if match[0] > lastEnd {
			text := line[lastEnd:match[0]]
			escaped := e.escapeText(text, exactSpace, exact)
			// If this variable is _start_ or _end_, make trailing space optional
			// Also make trailing space optional if variable has default() or set() function
			if i < len(variables) {
				hasDefault := false
				hasSet := false
				for _, funcStr := range variables[i].Functions {
					if strings.HasPrefix(funcStr, "default(") {
						hasDefault = true
					}
					if strings.HasPrefix(funcStr, "set(") {
						hasSet = true
					}
				}
				if variables[i].Name == "_start_" || variables[i].Name == "_end_" || hasDefault || hasSet {
					// Make trailing space optional for _start_, _end_, and variables with default() or set()
					escaped = strings.TrimSuffix(escaped, `[ \t]+`)
					if strings.HasSuffix(escaped, `\ `) {
						escaped = strings.TrimSuffix(escaped, `\ `) + `[ \t]*`
					} else if strings.HasSuffix(escaped, ` `) {
						escaped = strings.TrimSuffix(escaped, ` `) + `[ \t]*`
					}
				}
			}
			regexParts = append(regexParts, escaped)
		}

		// Variable pattern
		if i < len(variables) {
			// Skip indicator variables - they are markers, not capture groups
			// _exact_, _exact_space_: exact matching indicators
			// _start_, _end_: match block boundary indicators (when not on their own line)
			varName := variables[i].Name
			if varName == "_exact_" || varName == "_exact_space_" {
				// These are pure indicators, skip entirely
			} else if varName == "_start_" || varName == "_end_" {
				// _start_ and _end_ are indicators when embedded in a pattern
				// They should NOT add any regex pattern (they just mark the line)
				// Only add pattern if this is the ONLY variable (handled earlier in GenerateRegex)
				// When embedded, just skip - the line content before them is what matters
			} else {
				regexParts = append(regexParts, "("+variables[i].Pattern+")")
			}
		}

		lastEnd = match[1]
	}

	// Text after last variable
	// Note: trailing whitespace has already been stripped by the compiler
	// We'll add optional trailing whitespace matching at the end (unless exactSpace is true)
	if lastEnd < len(line) {
		text := line[lastEnd:]
		escaped := e.escapeText(text, exactSpace, exact)
		regexParts = append(regexParts, escaped)
	}

	regexStr := strings.Join(regexParts, "")

	// Add start and end anchors if not present
	// If the original line has leading spaces, we need to account for them in the regex
	// Python TTP preserves leading spaces in the pattern
	if !strings.HasPrefix(regexStr, "^") {
		// Check if the original line has leading spaces
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " \t"))
		if leadingSpaces > 0 && len(regexParts) > 0 {
			// The first part should already have the leading spaces from escapeText
			// But we need to ensure the pattern starts with ^ to anchor to line start
			regexStr = "^" + regexStr
		} else {
			regexStr = "^" + regexStr
		}
	}

	// Handle trailing whitespace: Python TTP strips it and adds optional matching
	// Add optional trailing whitespace pattern before the end anchor (unless exactSpace is true)
	if !exactSpace {
		// Add pattern to match optional trailing spaces/tabs before end of line
		// This matches Python TTP's behavior: r"[\t ]*(?=\n|\r\n)"
		// We use a lookahead to match trailing whitespace without consuming the newline
		regexStr = regexStr + `[\t ]*`
	}

	if !strings.HasSuffix(regexStr, "$") {
		regexStr = regexStr + "$"
	}

	return regexStr, nil
}

// escapeText escapes special regex characters in text
func (e *Engine) escapeText(text string, exactSpace, exact bool) string {
	// Escape special regex characters
	escaped := regexp.QuoteMeta(text)

	// Handle spaces and tabs
	if !exactSpace {
		// Replace sequences of spaces/tabs with flexible whitespace pattern
		// regexp.QuoteMeta doesn't escape spaces, so we need to match literal spaces
		// Replace one or more consecutive spaces or tabs with [ \t]+
		spaceRe := regexp.MustCompile(`( +|\t+)`)
		escaped = spaceRe.ReplaceAllString(escaped, `[ \t]+`)
	}

	// Handle digits
	if !exact {
		// Replace digits with \d+ pattern
		digitRe := regexp.MustCompile(`\d+`)
		escaped = digitRe.ReplaceAllString(escaped, `\d+`)
	}

	return escaped
}

// CompilePattern compiles a regex pattern and returns a CompiledPattern
func (e *Engine) CompilePattern(line string, exactSpace, exact bool) (*CompiledPattern, error) {
	// Validate pattern for common errors before processing
	if err := e.validatePattern(line); err != nil {
		return nil, err
	}

	regexStr, variables, err := e.GenerateRegex(line, exactSpace, exact)
	if err != nil {
		return nil, err
	}

	compiled, err := regexp.Compile(regexStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %w", err)
	}

	// Create variable map and preserve order
	varMap := make(map[string]*MatchVariable)
	varOrder := make([]string, len(variables))
	for i, v := range variables {
		varMap[v.Name] = v
		varOrder[i] = v.Name
	}

	return &CompiledPattern{
		Regex:         compiled,
		Variables:     varMap,
		VariableOrder: varOrder,
		Original:      line,
	}, nil
}

// validatePattern validates a pattern string for common regex errors
func (e *Engine) validatePattern(pattern string) error {
	// For patterns without variables, they will be escaped, so we need to check
	// if they would be valid regex after escaping. However, unclosed brackets/parens
	// and trailing backslashes would still be invalid even after escaping, so we check those for all patterns.

	// Check for trailing backslash (applies to all patterns)
	// A trailing backslash is invalid because it escapes nothing
	// However, for literal patterns (no variables), escapeText will escape it,
	// so we need to check if it would be valid after escaping
	// For patterns with variables, a trailing backslash is always invalid
	hasVariables := strings.Contains(pattern, "{{")
	if hasVariables {
		// For patterns with variables, check for trailing backslash
		escaped := false
		for i, r := range pattern {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				// Check for trailing backslash (invalid escape sequence)
				if i == len(pattern)-1 {
					return fmt.Errorf("invalid escape sequence: trailing backslash")
				}
				continue
			}
		}
	} else {
		// For literal patterns, check for trailing backslash
		// However, if the pattern contains regex special characters that will be escaped,
		// a trailing backslash might be intentional (e.g., "test.*+?^$[]{}()|\\")
		// In this case, escapeText will escape it, making it valid
		// So we only check for trailing backslash if it's a simple pattern
		// For patterns with special regex chars, we allow trailing backslash
		if len(pattern) > 0 && pattern[len(pattern)-1] == '\\' {
			// Check if it's actually a trailing backslash (not escaped)
			// Count backslashes from the end
			backslashCount := 0
			for i := len(pattern) - 1; i >= 0 && pattern[i] == '\\'; i-- {
				backslashCount++
			}
			// If odd number of backslashes, the last one is unescaped
			// But for literal patterns with special chars, this is OK (will be escaped)
			// Only fail if it's a simple pattern (no special regex chars)
			hasSpecialChars := strings.ContainsAny(pattern[:len(pattern)-backslashCount], ".*+?^$[]{}()|")
			if backslashCount%2 == 1 && !hasSpecialChars {
				return fmt.Errorf("invalid escape sequence: trailing backslash")
			}
		}
	}

	// Check for unclosed character class (applies to all patterns)
	bracketCount := 0
	escaped := false
	for i, r := range pattern {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		// Count brackets (they'll be escaped if literal, but unclosed is still invalid)
		if r == '[' && (i == 0 || pattern[i-1] != '\\') {
			bracketCount++
		}
		if r == ']' && (i == 0 || pattern[i-1] != '\\') {
			bracketCount--
		}
	}
	if bracketCount > 0 {
		return fmt.Errorf("unclosed character class")
	}
	if bracketCount < 0 {
		return fmt.Errorf("unexpected closing bracket")
	}

	// Check for unclosed group (applies to all patterns)
	parenCount := 0
	escaped = false
	for i, r := range pattern {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		// Count parens (they'll be escaped if literal, but unclosed is still invalid)
		if r == '(' && (i == 0 || pattern[i-1] != '\\') {
			parenCount++
		}
		if r == ')' && (i == 0 || pattern[i-1] != '\\') {
			parenCount--
		}
	}
	if parenCount > 0 {
		return fmt.Errorf("unclosed group")
	}
	if parenCount < 0 {
		return fmt.Errorf("unexpected closing parenthesis")
	}

	// Check for invalid quantifier syntax like {a,b} or {5,3} (min > max)
	// This applies to all patterns, as quantifiers are regex syntax
	quantifierRe := regexp.MustCompile(`\{([^}]+)\}`)
	matches := quantifierRe.FindAllStringSubmatch(pattern, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		quantifier := match[1]
		if strings.Contains(quantifier, ",") {
			parts := strings.Split(quantifier, ",")
			if len(parts) == 2 {
				minStr := strings.TrimSpace(parts[0])
				maxStr := strings.TrimSpace(parts[1])
				// Check if both are numeric
				min, minErr := strconv.Atoi(minStr)
				max, maxErr := strconv.Atoi(maxStr)
				if minErr == nil && maxErr == nil {
					// Both are numeric - check if min > max
					if min > max {
						return fmt.Errorf("invalid quantifier: min (%d) > max (%d)", min, max)
					}
				} else {
					// At least one is not numeric - invalid syntax
					return fmt.Errorf("invalid quantifier syntax: non-numeric values")
				}
			}
		}
	}

	return nil
}

// convertPythonRegexToRE2 converts Python-style regex patterns to Go RE2 patterns
// RE2 doesn't support \d, \w, \s, \D, \W, \S - need to convert to character classes
func (e *Engine) convertPythonRegexToRE2(pattern string) string {
	// First, handle escaped backslashes - if we have \\d, convert to \d
	// This handles cases where the pattern string has double backslashes
	// But also handle cases where the pattern already has single backslashes
	// We need to normalize both cases to single backslashes first
	pattern = strings.ReplaceAll(pattern, `\\d`, `\d`)
	pattern = strings.ReplaceAll(pattern, `\\D`, `\D`)
	pattern = strings.ReplaceAll(pattern, `\\w`, `\w`)
	pattern = strings.ReplaceAll(pattern, `\\W`, `\W`)
	pattern = strings.ReplaceAll(pattern, `\\s`, `\s`)
	pattern = strings.ReplaceAll(pattern, `\\S`, `\S`)

	// Now replace common Python regex shortcuts with RE2 equivalents
	// \d -> [0-9]
	pattern = strings.ReplaceAll(pattern, `\d`, `[0-9]`)
	// \D -> [^0-9]
	pattern = strings.ReplaceAll(pattern, `\D`, `[^0-9]`)
	// \w -> [0-9A-Za-z_]
	pattern = strings.ReplaceAll(pattern, `\w`, `[0-9A-Za-z_]`)
	// \W -> [^0-9A-Za-z_]
	pattern = strings.ReplaceAll(pattern, `\W`, `[^0-9A-Za-z_]`)
	// \s -> [\t\n\f\r ]
	pattern = strings.ReplaceAll(pattern, `\s`, `[\t\n\f\r ]`)
	// \S -> [^\t\n\f\r ]
	pattern = strings.ReplaceAll(pattern, `\S`, `[^\t\n\f\r ]`)

	return pattern
}

// GetBuiltinPattern returns a built-in pattern by name
func (e *Engine) GetBuiltinPattern(name string) (string, bool) {
	pattern, ok := e.patterns[strings.ToUpper(name)]
	return pattern, ok
}

// RegisterPattern registers a custom pattern
func (e *Engine) RegisterPattern(name, pattern string) {
	e.patterns[strings.ToUpper(name)] = pattern
}
