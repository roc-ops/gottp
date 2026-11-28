package pattern

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/roc-ops/gottp/internal/call"
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

// Names of the indicator variables and functions. A template marks a line
// with these instead of capturing a value: {{ _start_ }}, {{ x | _end_ }}.
const (
	IndicatorStart      = "_start_"
	IndicatorEnd        = "_end_"
	IndicatorLine       = "_line_"
	IndicatorHeaders    = "_headers_"
	IndicatorExact      = "_exact_"
	IndicatorExactSpace = "_exact_space_"
	// VarIgnore is the variable name whose match is discarded.
	VarIgnore = "ignore"
)

// IsIndicatorVar reports whether a variable (or result key) named name is
// ignore, _start_, _end_ or _line_: a name whose match is never a result
// value of its own.
func IsIndicatorVar(name string) bool {
	return name == VarIgnore || IsMarkerVar(name)
}

// IsMarkerVar reports whether a variable (or result key) named name is
// _start_, _end_ or _line_. A match whose result has only such keys is a pure
// indicator match.
func IsMarkerVar(name string) bool {
	switch name {
	case IndicatorStart, IndicatorEnd, IndicatorLine:
		return true
	}
	return false
}

// IsNonCapturingVar reports whether a variable named name captures nothing:
// ignore, _start_, _end_, _exact_ or _exact_space_. Unlike IsIndicatorVar it
// excludes _line_, which captures the whole line.
func IsNonCapturingVar(name string) bool {
	switch name {
	case VarIgnore, IndicatorStart, IndicatorEnd, IndicatorExact, IndicatorExactSpace:
		return true
	}
	return false
}

// MatchVariable represents a parsed match variable
type MatchVariable struct {
	Name      string
	Functions []string // function pipeline
	Pattern   string   // regex pattern for this variable

	// Compile-time cached function flags. Populated by CompiledPattern.PopulateFlags
	// to avoid per-match scans of Functions in the runtime hot path.
	HasSet         bool // a function is set()
	HasJoinMatches bool // a function is joinmatches()

	// DefaultFallback is true when Pattern is the .*? the engine gives a
	// default() variable with no pattern of its own (not a user re(".*?")).
	DefaultFallback bool

	// calls is Functions read as calls (see Calls), built once.
	calls atomic.Pointer[VariableCalls]
}

// VariableCalls are a variable's functions read as calls (package call).
type VariableCalls struct {
	// All are the variable's functions in order.
	All []*call.Call
	// NoJoin are All without joinmatches, which the record assembler
	// applies instead.
	NoJoin []*call.Call
}

// Calls returns the variable's functions read as calls. They are read once,
// the first time Calls is called (the compiler calls it, so a compiled
// template's calls are read at compile time). A function that can't be
// read is left out: the compiler reports it as an error (CheckCalls).
func (v *MatchVariable) Calls() *VariableCalls {
	if c := v.calls.Load(); c != nil {
		return c
	}
	c := &VariableCalls{}
	for _, f := range v.Functions {
		if strings.HasPrefix(f, ignorePatternPrefix) {
			continue
		}
		parsed, err := call.Parse(f)
		if err != nil {
			continue
		}
		c.All = append(c.All, parsed)
		if parsed.Name != "joinmatches" {
			c.NoJoin = append(c.NoJoin, parsed)
		}
	}
	if v.calls.CompareAndSwap(nil, c) {
		return c
	}
	return v.calls.Load()
}

// ignorePatternPrefix marks the entry of Functions that holds the regex of
// ignore("regex").
const ignorePatternPrefix = "pattern:"

// FunctionName returns the name of one of Functions (call.Name).
func FunctionName(f string) string {
	return call.Name(f)
}

// firstArg returns the first positional argument of function f as a
// string (Python str()), read as Python TTP reads it.
func firstArg(f string) (string, bool) {
	c, err := call.Parse(f)
	if err != nil || len(c.Strs) == 0 {
		return "", false
	}
	return c.Strs[0], true
}

// CutIndex returns the index in variables of the variable a line's regex is
// cut at, or -1. Python TTP's regex_deleteVar ends the regex just before a
// variable named _start_ or _end_ ("interface {{ _start_ }}" is
// \ninterface[\t ]*(?=\n|\r\n)), so the line matches only its text up to
// there, and variables after it capture nothing. A _start_ or _end_
// variable with its own re() is not cut: matching the regex is a gottp
// extension (TTP ignores the re()).
func CutIndex(variables []*MatchVariable) int {
	for i, v := range variables {
		if v.Name != IndicatorStart && v.Name != IndicatorEnd {
			continue
		}
		if !hasReFunction(v) {
			return i
		}
	}
	return -1
}

func hasReFunction(v *MatchVariable) bool {
	for _, f := range v.Functions {
		if FunctionName(f) == "re" {
			return true
		}
	}
	return false
}

// CutVariables returns the indicator (_start_ or _end_) cp's regex is cut at
// (see CutIndex) and the names of the variables after it, in template order
// and each once, that would capture a value if they weren't cut: not
// indicators, ignore, _exact_ or set() variables (set() sets its value
// without a capture).
func CutVariables(cp *CompiledPattern) (indicator string, names []string) {
	vars := cp.orderedVariables()
	cut := CutIndex(vars)
	if cut < 0 {
		return "", nil
	}
	for _, v := range vars[cut+1:] {
		if IsNonCapturingVar(v.Name) || slices.Contains(names, v.Name) {
			continue
		}
		hasSet := false
		for _, f := range v.Functions {
			if FunctionName(f) == "set" {
				hasSet = true
				break
			}
		}
		if !hasSet {
			names = append(names, v.Name)
		}
	}
	return vars[cut].Name, names
}

// CompiledPattern represents a compiled regex pattern with variable information
type CompiledPattern struct {
	Regex         *regexp.Regexp
	Variables     map[string]*MatchVariable
	VariableOrder []string // Order of variables as they appear in the pattern
	Original      string
	// Tail is true when the template line follows a child group (it is in
	// the XML tail of a child, not the group's own text).
	Tail bool
	// Headers is true for a {{ _headers_ }} line, whose Variables are its
	// column headers (HeadersLine). The line starts a record.
	Headers bool

	// Compile-time cached flags consumed by the runtime hot path. Populated by
	// PopulateFlags after Variables and Regex are assigned.
	HasAnchors               bool // regex contains ^ or $ -> match line-by-line
	HasOnlySpecialIndicators bool // every variable is ignore/_start_/_end_/_line_/_exact_/_exact_space_
	HasJoinMatches           bool // any variable has a joinmatches function
	// WholeMatch is true when the regex is cut at _start_ or _end_ (see
	// CutIndex) and no variable before the cut captures: TTP's regex then
	// has no named group, and its check_matches gives every variable of
	// the line the whole match text, "\n" first ("exit {{ _end_ }} {{ w }}"
	// saves "\n exit" in _end_ and w). It is false for a line with no
	// variable after the cut, whose values TTP drops.
	WholeMatch bool
}

// PopulateFlags computes the compile-time cached booleans from the pattern's
// regex and variable functions. Must be called after Regex and Variables are
// assigned. Idempotent.
func (cp *CompiledPattern) PopulateFlags() {
	regexStr := ""
	if cp.Regex != nil {
		regexStr = cp.Regex.String()
	}
	cp.HasAnchors = strings.Contains(regexStr, "^") || strings.Contains(regexStr, "$")

	hasOnlySpecial := true
	patternHasJoinMatches := false
	for _, v := range cp.Variables {
		if v.Name != "ignore" && v.Name != "_start_" && v.Name != "_end_" &&
			v.Name != "_line_" && v.Name != "_exact_" && v.Name != "_exact_space_" {
			hasOnlySpecial = false
		}

		varHasSet := false
		varHasJoinMatches := false
		for _, f := range v.Functions {
			name := FunctionName(f)
			if !varHasSet && name == "set" {
				varHasSet = true
			}
			if !varHasJoinMatches && name == "joinmatches" {
				varHasJoinMatches = true
			}
			if varHasSet && varHasJoinMatches {
				break
			}
		}
		v.HasSet = varHasSet
		v.HasJoinMatches = varHasJoinMatches
		if varHasJoinMatches {
			patternHasJoinMatches = true
		}
	}
	cp.HasOnlySpecialIndicators = hasOnlySpecial
	cp.HasJoinMatches = patternHasJoinMatches

	vars := cp.orderedVariables()
	cp.WholeMatch = false
	// Only a variable after the cut can make the line keep its values (an
	// end or startempty line drops them), so a line that ends at the
	// indicator skips the work.
	if cut := CutIndex(vars); cut >= 0 && cut < len(vars)-1 {
		cp.WholeMatch = true
		for _, v := range vars[:cut] {
			if !IsNonCapturingVar(v.Name) && !v.HasSet {
				cp.WholeMatch = false
				break
			}
		}
	}
}

// orderedVariables returns cp's variables in template order.
func (cp *CompiledPattern) orderedVariables() []*MatchVariable {
	vars := make([]*MatchVariable, 0, len(cp.VariableOrder))
	for _, name := range cp.VariableOrder {
		if v := cp.Variables[name]; v != nil {
			vars = append(vars, v)
		}
	}
	return vars
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

// parseVariableWithVars parses a variable string into a MatchVariable with access to template variables
func (e *Engine) parseVariableWithVars(variableStr string, vars map[string]interface{}) *MatchVariable {
	variable := &MatchVariable{
		Functions: []string{},
	}

	// Split on "|" as Python TTP does, quotes or not (#23): the first part
	// is the variable name, the rest its functions.
	parts := strings.Split(variableStr, "|")
	varName := strings.TrimSpace(parts[0])

	// ignore("regex") takes the regex to match, read as a Python string,
	// so ignore("\\d+") is the regex \d+.
	if call.Name(varName) == VarIgnore && varName != VarIgnore {
		variable.Name = VarIgnore
		if arg, ok := firstArg(varName); ok && arg != "" {
			variable.Functions = append(variable.Functions, ignorePatternPrefix+arg)
		}
	} else {
		variable.Name = varName
	}

	for _, funcStr := range call.Split(strings.Join(parts[1:], "|")) {
		// chain(var) is replaced by the functions in the variable var, at
		// compile time as TTP's extract_chain does; an unknown variable
		// drops it.
		if c, err := call.Parse(funcStr); err == nil && c.Name == "chain" && len(c.Strs) > 0 {
			if chainValue, ok := vars[c.Strs[0]]; ok {
				variable.Functions = append(variable.Functions, e.extractFunctionsFromChainValue(chainValue)...)
			}
			continue
		}
		variable.Functions = append(variable.Functions, funcStr)
	}

	// Determine pattern based on variable name or functions
	variable.Pattern = e.getPatternForVariable(variable, vars)

	return variable
}

// extractFunctionsFromChainValue extracts function strings from a chain variable value
// The value can be a string (pipe-separated functions) or a list of strings
func (e *Engine) extractFunctionsFromChainValue(value interface{}) []string {
	var funcs []string

	// TTP reads a string, and each string of a list, with get_attributes,
	// which splits it on "|".
	switch v := value.(type) {
	case string:
		funcs = call.Split(v)
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				funcs = append(funcs, call.Split(str)...)
			}
		}
	case []string:
		for _, str := range v {
			funcs = append(funcs, call.Split(str)...)
		}
	}

	return funcs
}

// getPatternForVariable determines the regex pattern for a variable
func (e *Engine) getPatternForVariable(variable *MatchVariable, vars map[string]interface{}) string {
	variable.DefaultFallback = false
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
		// Check if re() function provides a custom regex before using default
		for _, funcStr := range variable.Functions {
			if FunctionName(funcStr) == "re" {
				if arg, ok := firstArg(funcStr); ok && arg != "" {
					return arg
				}
			}
		}
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
			if strings.HasPrefix(funcStr, ignorePatternPrefix) {
				// Extract the pattern from "pattern:..."
				patternName := strings.TrimPrefix(funcStr, ignorePatternPrefix)
				var pattern string
				// First check if it's a template variable
				if vars != nil {
					if varValue, ok := vars[patternName]; ok {
						// It's a template variable - use its value as the pattern
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
				// The regex is used as written, as re() does: RE2 knows
				// \d, \s, \w and their negations, inside classes too.
				return pattern
			}
		}
		// Default pattern for ignore is \S+ (non-space characters)
		return `\S+`
	}

	// Check Functions list for pattern formatters (like DIGIT, IP, etc.)
	// These are used as pattern formatters, not as functions
	// Pattern formatters should be checked before other functions
	// Every pattern formatter and re() adds a regex; several are
	// alternatives, as TTP joins its var_res: (?:a)|(?:b).
	var res []string
	for _, funcStr := range variable.Functions {
		funcName := FunctionName(funcStr)

		// Check if this is a built-in pattern formatter
		if pattern, ok := e.patterns[funcName]; ok {
			res = append(res, pattern)
			continue
		}
		// re("regex"): the argument is read as a Python string (#151), so
		// re("\\d+") and re(r"\d+") are both the regex \d+. It may name a
		// template variable or a built-in pattern instead. Without a
		// positional argument TTP's extract_re uses the function's name,
		// so re() is the regex "re".
		if funcName == "re" {
			arg, ok := firstArg(funcStr)
			if !ok {
				arg = funcName
			}
			if varValue, ok := vars[arg]; ok {
				res = append(res, fmt.Sprintf("%v", varValue))
			} else if pattern, ok := e.patterns[arg]; ok {
				res = append(res, pattern)
			} else {
				res = append(res, arg)
			}
		}
	}
	switch len(res) {
	case 0:
	case 1:
		return res[0]
	default:
		return "(?:" + strings.Join(res, ")|(?:") + ")"
	}

	// Check if variable has a default function - if so, allow empty matches
	// This allows default() to work when the value is empty or whitespace-only
	// Use .*? (any characters, non-greedy) to allow empty matches including whitespace
	for _, funcStr := range variable.Functions {
		if FunctionName(funcStr) == "default" {
			// Use .*? (any characters, non-greedy) to allow empty matches
			// This matches Python TTP behavior where default() can handle empty values
			// Note: We use .*? instead of \S* because trailing spaces are trimmed from lines,
			// so we need to match the end of the line (which may be empty after trimming)
			variable.DefaultFallback = true
			return `.*?`
		}
	}

	// Check if variable has set() function - if so, allow empty matches
	// Variables with set() can match empty strings since set() will set the value regardless
	for _, funcStr := range variable.Functions {
		if FunctionName(funcStr) == "set" {
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
	// TTP keeps a line's indent in its regex (#30), also for a line that is
	// only {{ _start_ }} or {{ _end_ }}.
	lineIndent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	if len(variables) == 1 {
		if variables[0].Name == "_start_" {
			// Check if line only has _start_ variable (possibly with whitespace)
			// Can be either "{{ _start_ }}" or just "_start_"
			if trimmedLine == "{{ _start_ }}" || trimmedLine == "{{_start_}}" {
				// {{ _start_ }} on its own line should match empty lines (or whitespace-only lines).
				// This matches Python TTP behavior where standalone {{ _start_ }} generates
				// regex \n[\t ]*(?=\n|\r\n), which matches blank lines.
				return "^" + lineIndent + `[\t ]*$`, nil
			}
			if trimmedLine == "_start_" {
				// Bare _start_ (without {{ }}) on its own line - match any line.
				// In Python TTP, bare _start_ is actually skipped (not compiled as a pattern),
				// but gottp treats it as a pattern. Using ^.*$ preserves backward compatibility.
				return "^.*$", nil
			}
		}
		if variables[0].Name == "_end_" {
			// Check if line only has _end_ variable (possibly with whitespace)
			// Can be either "{{ _end_ }}" or just "_end_"
			if trimmedLine == "{{ _end_ }}" || trimmedLine == "{{_end_}}" {
				// {{ _end_ }} on its own line should match empty lines (or whitespace-only lines).
				// This matches Python TTP behavior where standalone {{ _end_ }} generates
				// regex \n[\t ]*(?=\n|\r\n), which matches blank lines.
				return "^" + lineIndent + `[\t ]*$`, nil
			}
			if trimmedLine == "_end_" {
				// Bare _end_ (without {{ }}) on its own line - match any line.
				// In Python TTP, bare _end_ is actually skipped (not compiled as a pattern),
				// but gottp treats it as a pattern. Using ^.*$ preserves backward compatibility.
				return "^.*$", nil
			}
		}
		if variables[0].Name == "_line_" {
			// Check if line only has _line_ variable (possibly with whitespace)
			// Can be either "{{ _line_ }}" or just "_line_"
			if trimmedLine == "{{ _line_ }}" || trimmedLine == "{{_line_}}" || trimmedLine == "_line_" {
				// _line_ on its own line should match any line
				return "^(.+)$", nil
			}
		}
	}

	// Python TTP rebuilds the line's leading whitespace verbatim
	// ("\n" + indent + ...), so an indented line matches only data with
	// exactly that indent (#30). Emit it literally (spaces and tabs need no
	// escaping) and build the rest of the regex from the unindented line.
	indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
	indent := line[:indentLen]
	line = line[indentLen:]

	// Find variable positions
	re := regexp.MustCompile(`\{\{([\S\s]+?)\}\}`)
	matches := re.FindAllStringSubmatchIndex(line, -1)

	if len(matches) == 0 {
		return "", fmt.Errorf("no variable matches found")
	}

	// Build regex by replacing variables and escaping text
	regexParts := []string{indent}
	lastEnd := 0
	cut := CutIndex(variables)

	for i, match := range matches {
		if i == cut {
			// TTP's regex_deleteVar: keep the regex before the variable,
			// less one trailing [ \t]+ (a space kept by _exact_space_
			// stays), and end the line there.
			if match[0] > lastEnd {
				escaped := e.escapeText(line[lastEnd:match[0]], exactSpace, exact)
				regexParts = append(regexParts, strings.TrimSuffix(escaped, `[ \t]+`))
			}
			lastEnd = len(line)
			break
		}
		// Text before variable
		if match[0] > lastEnd {
			text := line[lastEnd:match[0]]
			escaped := e.escapeText(text, exactSpace, exact)
			// Make the space before the variable optional for a _start_ or
			// _end_ variable with its own re() (any other is cut above) and
			// for a variable with default() or set().
			if i < len(variables) {
				hasDefault := false
				hasSet := false
				for _, funcStr := range variables[i].Functions {
					switch FunctionName(funcStr) {
					case "default":
						hasDefault = true
					case "set":
						hasSet = true
					}
				}
				if variables[i].Name == "_start_" || variables[i].Name == "_end_" || hasDefault || hasSet {
					// Make trailing space optional for _start_, _end_, and variables with default() or set()
					if t := strings.TrimSuffix(escaped, `[ \t]+`); t != escaped {
						escaped = t + `[ \t]*`
					} else if strings.HasSuffix(escaped, `\ `) {
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
			// Skip _exact_ and _exact_space_ variables - they are indicators, not capture groups
			if variables[i].Name != "_exact_" && variables[i].Name != "_exact_space_" {
				regexParts = append(regexParts, "("+variables[i].Pattern+")")
			}
		}

		lastEnd = match[1]
	}

	// Text after last variable
	// Note: trailing whitespace has already been stripped by the compiler;
	// optional trailing whitespace is matched at the end.
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
		regexStr = "^" + regexStr
	}

	// Optional trailing spaces/tabs before the end anchor, as TTP's
	// form_regex appends r"[\t ]*(?=\n|\r\n)". TTP appends it whether or
	// not _exact_space_ is set, and data lines keep their trailing
	// whitespace (#33), so this must be unconditional.
	regexStr = regexStr + `[\t ]*`

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
		// Any run of spaces and tabs, mixed or not, becomes one [ \t]+, as
		// TTP's re.sub(r"(\\ |\\\t)+", r"[ \\t]+", ...) does. QuoteMeta
		// leaves spaces and tabs as they are.
		escaped = blankRunRe.ReplaceAllString(escaped, `[ \t]+`)
	}

	// Handle digits
	if !exact {
		// Replace digits with \d+ pattern
		escaped = digitRunRe.ReplaceAllString(escaped, `\d+`)
	}

	return escaped
}

var (
	blankRunRe = regexp.MustCompile(`[ \t]+`)
	digitRunRe = regexp.MustCompile(`\d+`)
)

// CompilePattern compiles a regex pattern and returns a CompiledPattern
func (e *Engine) CompilePattern(line string, exactSpace, exact bool) (*CompiledPattern, error) {
	// Validate pattern for common errors before processing
	if err := e.validatePattern(line); err != nil {
		return nil, err
	}

	variables := e.ExtractVariables(line)
	regexStr, headerVars, isHeaders, err := e.HeadersLine(line, variables)
	if err != nil {
		return nil, err
	}
	if isHeaders {
		variables = headerVars
	} else if regexStr, err = e.GenerateRegexFromVariables(line, variables, exactSpace, exact); err != nil {
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

	cp := &CompiledPattern{
		Regex:         compiled,
		Variables:     varMap,
		VariableOrder: varOrder,
		Original:      line,
		Headers:       isHeaders,
	}
	cp.PopulateFlags()
	return cp, nil
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

// PatternNames returns the names of the built-in patterns, such as PHRASE.
func (e *Engine) PatternNames() []string {
	names := make([]string, 0, len(e.patterns))
	for name := range e.patterns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
