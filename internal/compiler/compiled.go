package compiler

import (
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"

	"github.com/roc-ops/gottp/internal/parser"
	"github.com/roc-ops/gottp/internal/pattern"
	"github.com/roc-ops/gottp/internal/validator"
	"github.com/roc-ops/gottp/internal/variable"
)

// CompiledTemplate represents a compiled, immutable template ready for execution
type CompiledTemplate struct {
	// Template metadata
	Name          string
	BasePath      string
	ResultsMethod string
	PathChar      string
	Doc           string

	// Compiled groups with their patterns
	Groups []*CompiledGroup

	// Input configurations
	Inputs []*CompiledInput

	// Output configurations
	Outputs []*CompiledOutput

	// Lookup tables
	Lookups []*CompiledLookup

	// Macros (stored as metadata, executed per-parse)
	Macros []*CompiledMacro

	// Child templates
	Templates []*CompiledTemplate

	// Variables (pre-parsed)
	Vars map[string]interface{}

	// Vars with name attribute (to be stored in result structure)
	VarsWithName []*CompiledVarsWithName

	// Version for compatibility checking
	Version string

	// Compilation warnings (non-fatal issues like Python-specific syntax in Starlark macros)
	Warnings []string

	// Streamable is true iff every top-level group is streamable.
	// Used by ParseStream to gate entry; false means ParseStream returns
	// *TemplateNotStreamableError without invoking the callback.
	Streamable bool
}

// CompiledVarsWithName represents compiled vars with name attribute
type CompiledVarsWithName struct {
	Name       string                 // name attribute (path in result structure)
	Vars       map[string]interface{} // parsed vars
	Attributes map[string]string      // all attributes
}

// CompiledGroup represents a compiled group with compiled regex patterns
type CompiledGroup struct {
	Name       string
	Input      string
	Output     string
	Method     string
	Functions  string
	Chain      string
	Macro      string
	Patterns   []*pattern.CompiledPattern // compiled regex patterns for each line
	Groups     []*CompiledGroup           // nested groups
	Attributes map[string]string
	IsNested   bool                   // true if this group is nested (should not be processed as top-level)
	Defaults   map[string]interface{} // default values for variables (e.g., from unconditional set())

	// Streamability — set during compile by analyzeStreamability.
	// Streamable is true iff this group passed the strict streamability check.
	// NonStreamableReasons lists one human-readable explanation per failed
	// rule when Streamable is false; empty otherwise.
	// NormalizedPath is Name with trailing "*" stripped, used as the
	// groupPath argument to the ParseStream callback.
	Streamable           bool
	NonStreamableReasons []string
	NormalizedPath       string
}

// CompiledInput represents a compiled input configuration
type CompiledInput struct {
	Name            string
	Groups          []string
	Load            string
	URL             string
	Extensions      []string
	Filters         []string
	Functions       string   // pipe-separated list of input functions
	Macro           string   // comma-separated list of macro function names
	ExtractCommands []string // comma-separated list of commands to extract
	Data            string   // input content (for load="text")
	Attributes      map[string]string
}

// CompiledOutput represents a compiled output configuration
type CompiledOutput struct {
	Name       string
	Format     string
	Functions  string
	Path       string
	Returner   string
	Headers    string
	Attributes map[string]string
}

// CompiledLookup represents a compiled lookup table
type CompiledLookup struct {
	Name       string
	Load       string
	Include    string
	Key        string
	Data       interface{} // parsed lookup data
	Attributes map[string]string
}

// CompiledMacro represents a compiled macro (metadata only, source stored)
type CompiledMacro struct {
	Language   string // "starlark", "javascript", "python"
	Source     string // macro source code
	Attributes map[string]string
}

// Compiler compiles templates into CompiledTemplate structures
type Compiler struct {
	patternEngine *pattern.Engine
}

// NewCompiler creates a new template compiler
func NewCompiler() *Compiler {
	return &Compiler{
		patternEngine: pattern.NewEngine(),
	}
}

// CompileTemplate compiles a parsed template into a CompiledTemplate
// It resolves extend tags before compilation
func (c *Compiler) CompileTemplate(tmpl *parser.Template) (*CompiledTemplate, error) {
	// Resolve extend tags first
	resolver := NewExtendResolver(tmpl.BasePath, nil)
	resolved, err := resolver.ResolveExtends(tmpl)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve extends: %w", err)
	}
	tmpl = resolved

	compiled := &CompiledTemplate{
		Name:          tmpl.Name,
		BasePath:      tmpl.BasePath,
		ResultsMethod: tmpl.ResultsMethod,
		PathChar:      tmpl.PathChar,
		Doc:           tmpl.Doc,
		Version:       "1.0.0", // TODO: use actual version
		Vars:          make(map[string]interface{}),
		VarsWithName:  []*CompiledVarsWithName{},
		Warnings:      []string{},
	}

	// Parse and copy vars
	if tmpl.Vars != nil {
		// Check if vars need to be parsed from raw content
		if rawContent, hasRaw := tmpl.Vars["_raw_content_"]; hasRaw {
			load := "python" // default
			if loadVal, hasLoad := tmpl.Vars["_load_"]; hasLoad {
				if loadStr, ok := loadVal.(string); ok {
					load = loadStr
				}
			}

			// Parse vars using variable loader
			if contentStr, ok := rawContent.(string); ok && contentStr != "" {
				varLoader := variable.NewLoader()
				parsedVars, err := varLoader.Load(contentStr, load)
				if err == nil {
					// Merge parsed vars into compiled.Vars
					for k, v := range parsedVars {
						compiled.Vars[k] = v
					}
				}
				// If parsing fails, we'll just skip it (vars might be empty or invalid)
			}
		} else {
			// Vars are already parsed, just copy them
			for k, v := range tmpl.Vars {
				compiled.Vars[k] = v
			}
		}
	}

	// Parse and compile vars with name attribute
	if tmpl.VarsWithName != nil {
		for _, varsWithName := range tmpl.VarsWithName {
			compiledVarsWithName := &CompiledVarsWithName{
				Name:       varsWithName.Name,
				Attributes: varsWithName.Attributes,
				Vars:       make(map[string]interface{}),
			}

			// Check if vars need to be parsed from raw content
			if rawContent, hasRaw := varsWithName.Vars["_raw_content_"]; hasRaw {
				load := varsWithName.Load
				if load == "" {
					load = "python" // default
				}

				// Parse vars using variable loader
				if contentStr, ok := rawContent.(string); ok && contentStr != "" {
					varLoader := variable.NewLoader()
					parsedVars, err := varLoader.Load(contentStr, load)
					if err == nil {
						// Store parsed vars
						for k, v := range parsedVars {
							compiledVarsWithName.Vars[k] = v
						}
					}
					// If parsing fails, we'll just skip it (vars might be empty or invalid)
				}
			} else {
				// Vars are already parsed, just copy them
				for k, v := range varsWithName.Vars {
					compiledVarsWithName.Vars[k] = v
				}
			}

			compiled.VarsWithName = append(compiled.VarsWithName, compiledVarsWithName)
		}
	}

	// Compile groups (pass vars for chain() support)
	// Only add top-level groups to compiled.Groups - nested groups are stored in parent's Groups field
	for _, group := range tmpl.Groups {
		compiledGroup, err := c.compileGroup(group, compiled.Vars, false) // false = not nested (top-level)
		if err != nil {
			return nil, err
		}
		// Top-level groups should not be marked as nested
		compiledGroup.IsNested = false
		compiled.Groups = append(compiled.Groups, compiledGroup)
	}

	// Compile inputs
	for _, input := range tmpl.Inputs {
		compiledInput := &CompiledInput{
			Name:            input.Name,
			Groups:          input.Groups,
			Load:            input.Load,
			URL:             input.URL,
			Extensions:      input.Extensions,
			Filters:         input.Filters,
			Functions:       input.Functions,
			Macro:           input.Macro,
			ExtractCommands: input.ExtractCommands,
			Data:            input.Content, // Store input content for load="text"
			Attributes:      input.Attributes,
		}
		compiled.Inputs = append(compiled.Inputs, compiledInput)
	}

	// Compile outputs
	for _, output := range tmpl.Outputs {
		compiledOutput := &CompiledOutput{
			Name:       output.Name,
			Format:     output.Format,
			Functions:  output.Functions,
			Path:       output.Path,
			Returner:   output.Returner,
			Headers:    output.Headers,
			Attributes: output.Attributes,
		}
		compiled.Outputs = append(compiled.Outputs, compiledOutput)
	}

	// Compile lookups
	loader := variable.NewLoader()
	for _, lookup := range tmpl.Lookups {
		var lookupData interface{}
		var err error

		// Load lookup data based on load type
		if lookup.Include != "" {
			// Load from file
			lookupData, err = loader.LoadFromFile(lookup.Include, lookup.Load)
			if err != nil {
				return nil, fmt.Errorf("failed to load lookup table %s from file %s: %w", lookup.Name, lookup.Include, err)
			}
		} else if lookup.Content != "" {
			// Load from content
			loadType := lookup.Load
			if loadType == "" {
				loadType = "python" // Default loader
			}
			if loadType == "csv" {
				// For CSV, use key column if specified, otherwise use first column
				keyColumn := lookup.Key
				if keyColumn == "" {
					// Parse first line to get first column name (Python TTP behavior: first column is default key)
					// Use CSV reader to properly parse the header line
					reader := csv.NewReader(strings.NewReader(lookup.Content))
					headers, err := reader.Read()
					if err == nil && len(headers) > 0 {
						keyColumn = strings.TrimSpace(headers[0])
					} else {
						// Fallback: simple string split
						lines := strings.Split(lookup.Content, "\n")
						if len(lines) > 0 {
							firstLine := strings.TrimSpace(lines[0])
							if firstLine != "" {
								parts := strings.Split(firstLine, ",")
								if len(parts) > 0 {
									keyColumn = strings.TrimSpace(parts[0])
								}
							}
						}
					}
				}
				lookupData, err = loader.LoadCSV(lookup.Content, keyColumn)
			} else {
				lookupData, err = loader.Load(lookup.Content, loadType)
			}
			if err != nil {
				// Unknown load types (e.g., "gnmi") compile with nil data.
				// The lookup can be populated at parse time via ParseOptions.Lookups.
				if strings.Contains(err.Error(), "unsupported variable format") {
					lookupData = nil
					err = nil
				} else {
					return nil, fmt.Errorf("failed to load lookup table %s: %w", lookup.Name, err)
				}
			}
		}
		// Unknown load types with no content (e.g., load="gnmi" with yang_path attrs)
		// compile with nil data — populated at parse time via ParseOptions.Lookups.

		compiledLookup := &CompiledLookup{
			Name:       lookup.Name,
			Load:       lookup.Load,
			Include:    lookup.Include,
			Key:        lookup.Key,
			Data:       lookupData,
			Attributes: lookup.Attributes,
		}
		compiled.Lookups = append(compiled.Lookups, compiledLookup)
	}

	// Compile macros and validate for Starlark compatibility
	for _, macro := range tmpl.Macros {
		compiledMacro := &CompiledMacro{
			Language:   macro.Language,
			Source:     macro.Content,
			Attributes: macro.Attributes,
		}
		compiled.Macros = append(compiled.Macros, compiledMacro)
		
		// Validate macro source for Python-specific syntax that's not compatible with Starlark
		macroWarnings := validator.ValidateMacroSource(macro.Content, macro.Language)
		for _, warning := range macroWarnings {
			compiled.Warnings = append(compiled.Warnings, fmt.Sprintf("macro (language=%s): %s", macro.Language, warning))
		}
	}

	// Compile child templates
	// Python TTP behavior: when a document has multiple <template> sections,
	// each is processed independently against the same inputs, and their results
	// are merged. We flatten child template groups into the root template so
	// the runtime processes them all together. This matches the behavior for
	// the common case where all templates share the same input data.
	for _, childTmpl := range tmpl.Templates {
		compiledChild, err := c.CompileTemplate(childTmpl)
		if err != nil {
			return nil, err
		}
		compiled.Templates = append(compiled.Templates, compiledChild)

		// Hoist child template groups into root template for unified processing
		for _, childGroup := range compiledChild.Groups {
			// Mark as not nested (these are top-level groups from child templates)
			childGroup.IsNested = false
			compiled.Groups = append(compiled.Groups, childGroup)
		}

		// Hoist child template inputs, outputs, lookups, macros, and vars
		compiled.Inputs = append(compiled.Inputs, compiledChild.Inputs...)
		compiled.Outputs = append(compiled.Outputs, compiledChild.Outputs...)
		compiled.Lookups = append(compiled.Lookups, compiledChild.Lookups...)
		compiled.Macros = append(compiled.Macros, compiledChild.Macros...)

		// Merge child template vars (child vars don't override root vars)
		for k, v := range compiledChild.Vars {
			if _, exists := compiled.Vars[k]; !exists {
				compiled.Vars[k] = v
			}
		}

		// Merge child template VarsWithName
		compiled.VarsWithName = append(compiled.VarsWithName, compiledChild.VarsWithName...)
	}

	// Analyze streamability for all top-level groups (and their descendants),
	// then compute the template-level Streamable flag.
	for _, g := range compiled.Groups {
		analyzeStreamabilityRecursive(g)
	}
	computeTemplateStreamable(compiled)

	if err := validateGroupPathCollisions(compiled); err != nil {
		return nil, err
	}

	return compiled, nil
}

// compileGroup compiles a group with its patterns
// isNested indicates if this group is nested (not a top-level group)
func (c *Compiler) compileGroup(group *parser.Group, templateVars map[string]interface{}, isNested bool) (*CompiledGroup, error) {
	compiled := &CompiledGroup{
		Name:       group.Name,
		Input:      group.Input,
		Output:     group.Output,
		Method:     group.Method,
		Functions:  group.Functions,
		Chain:      group.Chain,
		Macro:      group.Macro,
		Attributes: group.Attributes,
		IsNested:   isNested,                     // Mark as nested if it is
		Defaults:   make(map[string]interface{}), // Initialize defaults map
	}

	// Compile patterns from group pattern text
	if group.Pattern != "" {
		lines := splitLines(group.Pattern)
		for _, line := range lines {
			if line == "" {
				continue
			}
			// Skip comments
			if strings.HasPrefix(strings.TrimSpace(line), "##") {
				continue
			}

			// Check for exact space/exact flags (before stripping trailing whitespace)
			exactSpace := strings.Contains(line, "_exact_space_")
			exact := strings.Contains(line, "_exact_")

			// Python TTP strips trailing whitespace from pattern lines
			// The pattern engine will handle matching optional trailing whitespace
			line = strings.TrimRight(line, " \t")

			// Check for unconditional set() - variables with set() on a line with no other text
			// Remove all {{ variable }} patterns to see if there's any text left
			re := regexp.MustCompile(`\{\{([\S\s]+?)\}\}`)
			lineWithoutVars := re.ReplaceAllString(line, "")
			lineWithoutVars = strings.TrimSpace(lineWithoutVars)

			// Extract variables to check for unconditional set()
			varMatches := re.FindAllStringSubmatch(line, -1)
			for _, varMatch := range varMatches {
				if len(varMatch) < 2 {
					continue
				}
				varStr := varMatch[1]
				// Check if this variable has set() function
				if strings.Contains(varStr, "set(") {
					// Parse variable to get name and set() argument
					parts := strings.Split(varStr, "|")
					if len(parts) >= 2 {
						varName := strings.TrimSpace(parts[0])
						// Find set() function call
						for _, part := range parts[1:] {
							part = strings.TrimSpace(part)
							if strings.HasPrefix(part, "set(") {
								// Extract argument from set("value") or set('value')
								start := strings.Index(part, "(")
								end := strings.LastIndex(part, ")")
								if start >= 0 && end > start {
									setValue := part[start+1 : end]
									setValue = strings.TrimSpace(setValue)
									setValue = strings.Trim(setValue, `"'`)
									// If line has no text (unconditional set), add to defaults
									// Python TTP: when set() is used without text to match, it sets skip_regex_dict = True
									// and adds the value to group.defaults
									if lineWithoutVars == "" {
										compiled.Defaults[varName] = setValue
										// Debug: fmt.Printf("[DEBUG] Unconditional set() detected: %s = %s\n", varName, setValue)
									}
								}
								break
							}
						}
					}
				}
			}

			// Compile pattern with access to template variables for chain() support
			pattern, err := c.compilePatternWithVars(line, exactSpace, exact, templateVars)
			if err != nil {
				return nil, err
			}
			compiled.Patterns = append(compiled.Patterns, pattern)
		}
	}

	// Compile nested groups (pass vars for chain() support)
	// Nested groups are stored in the parent group's Groups field (compiled.Groups), not in the top-level c.compiled.Groups
	// They are processed within their parent group's context, not as top-level groups
	for _, nestedGroup := range group.Groups {
		compiledNested, err := c.compileGroup(nestedGroup, templateVars, true) // true = nested
		if err != nil {
			return nil, err
		}
		// Ensure nested groups are marked (should already be set by compileGroup, but double-check)
		compiledNested.IsNested = true
		// Add nested groups to the parent group's Groups field (not to top-level c.compiled.Groups)
		compiled.Groups = append(compiled.Groups, compiledNested)
	}

	return compiled, nil
}

// compilePatternWithVars compiles a pattern with access to template variables (for chain() support)
func (c *Compiler) compilePatternWithVars(line string, exactSpace, exact bool, vars map[string]interface{}) (*pattern.CompiledPattern, error) {
	// Extract variables with vars access
	variables := c.patternEngine.ExtractVariablesWithVars(line, vars)

	// Generate regex pattern
	regexStr, err := c.patternEngine.GenerateRegexFromVariables(line, variables, exactSpace, exact)
	if err != nil {
		return nil, err
	}

	// Compile regex
	compiled, err := regexp.Compile(regexStr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regex: %w", err)
	}

	// Create variable map and preserve order
	varMap := make(map[string]*pattern.MatchVariable)
	varOrder := make([]string, len(variables))
	for i, v := range variables {
		varMap[v.Name] = v
		varOrder[i] = v.Name
	}

	cp := &pattern.CompiledPattern{
		Regex:         compiled,
		Variables:     varMap,
		VariableOrder: varOrder,
		Original:      line,
	}
	cp.PopulateFlags()
	return cp, nil
}

// splitLines splits text into lines, preserving empty lines
func splitLines(text string) []string {
	// Use strings.Split to preserve empty lines
	return strings.Split(text, "\n")
}
