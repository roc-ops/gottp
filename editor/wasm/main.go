package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"

	"github.com/roc-ops/gottp"
	"github.com/roc-ops/gottp/internal/formatters"
)

// Cache compiled templates to avoid JSON deserialization overhead
var (
	compiledTemplateCache = make(map[string]*gottp.CompiledTemplate)
	cacheMutex            sync.RWMutex
)

// functionRegistry holds pre-registered function sets that can be referenced by name.
// Go closures cannot be created from JavaScript via WASM, so this registry pattern
// allows Go-side function sets to be used in CompileTemplateWithOptions calls.
var (
	functionRegistry     = make(map[string]*gottp.Functions)
	functionRegistryMux  sync.RWMutex
)

// compileTemplate compiles a TTP template and returns a cache key or error
func compileTemplate(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"error": "missing template argument",
		}
	}

	templateText := args[0].String()

	// Compile template
	compiled, err := gottp.CompileTemplate(templateText)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Store in cache and return cache key (use template text hash as key)
	// For simplicity, we'll use the template text itself as the key
	// In production, you might want to use a hash
	cacheKey := templateText

	cacheMutex.Lock()
	compiledTemplateCache[cacheKey] = compiled
	cacheMutex.Unlock()

	// Also serialize to JSON for backward compatibility (if needed)
	compiledJSON, err := gottp.SaveCompiledTemplate(compiled, "json")
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize template: %v", err),
		}
	}

	// Get compilation warnings
	warnings := compiled.GetWarnings()
	warningsJSON, marshalErr := json.Marshal(warnings)
	if marshalErr != nil {
		warningsJSON = []byte("[]")
	}

	// Create result object - ensure all fields are explicitly set
	// Go WASM may drop nil values, so we use empty string for error if nil
	resultError := ""
	if err != nil {
		resultError = err.Error()
	}

	// Create result map and explicitly set all fields
	// Using js.ValueOf on the map should preserve all fields
	result := make(map[string]interface{})
	result["result"] = string(compiledJSON)
	result["cacheKey"] = cacheKey
	result["warnings"] = string(warningsJSON)
	result["error"] = resultError

	return result
}

// parseTemplate parses input data using a compiled template
// First tries to use cached template, falls back to JSON deserialization
func parseTemplate(this js.Value, args []js.Value) interface{} {
	if len(args) < 3 {
		return map[string]interface{}{
			"error": "missing arguments: need compiledTemplateJSON, inputsJSON, varsJSON",
		}
	}

	compiledTemplateJSON := args[0].String()
	inputsJSON := args[1].String()
	varsJSON := args[2].String()
	
	// Optional 4th argument: yangModulesJSON
	var yangModules *gottp.YANGModules
	if len(args) >= 4 && args[3].String() != "" && args[3].String() != "null" {
		yangModulesJSON := args[3].String()
		if err := json.Unmarshal([]byte(yangModulesJSON), &yangModules); err != nil {
			return map[string]interface{}{
				"error": fmt.Sprintf("failed to parse YANG modules: %v", err),
			}
		}
	}

	// Optional 5th argument: cacheKey (for faster lookup)
	// Optional 6th argument: enableSourceMap (boolean string)
	var compiled *gottp.CompiledTemplate
	var err error
	enableSourceMap := false
	
	if len(args) >= 5 && args[4].String() != "" && args[4].String() != "null" {
		// Try to get from cache first
		cacheKey := args[4].String()
		cacheMutex.RLock()
		cached, found := compiledTemplateCache[cacheKey]
		cacheMutex.RUnlock()
		
		if found {
			compiled = cached
		} else {
			// Fall back to JSON deserialization
			compiled, err = gottp.LoadCompiledTemplate([]byte(compiledTemplateJSON), "json")
			if err != nil {
				return map[string]interface{}{
					"error": fmt.Sprintf("failed to load compiled template: %v", err),
				}
			}
		}
	} else {
		// Load compiled template from JSON (backward compatibility)
		compiled, err = gottp.LoadCompiledTemplate([]byte(compiledTemplateJSON), "json")
		if err != nil {
			return map[string]interface{}{
				"error": fmt.Sprintf("failed to load compiled template: %v", err),
			}
		}
	}
	
	// Check for enableSourceMap parameter (6th argument)
	if len(args) >= 6 && args[5].String() != "" && args[5].String() != "null" {
		enableSourceMapStr := args[5].String()
		enableSourceMap = enableSourceMapStr == "true" || enableSourceMapStr == "True" || enableSourceMapStr == "TRUE"
	}

	// Optional 7th argument: lookupsJSON
	var lookups map[string]map[string]interface{}
	if len(args) >= 7 && args[6].String() != "" && args[6].String() != "null" {
		lookupsJSON := args[6].String()
		if err := json.Unmarshal([]byte(lookupsJSON), &lookups); err != nil {
			return map[string]interface{}{
				"error": fmt.Sprintf("failed to parse lookups: %v", err),
			}
		}
	}

	// Parse inputs
	var inputs gottp.Inputs
	if err := json.Unmarshal([]byte(inputsJSON), &inputs); err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to parse inputs: %v", err),
		}
	}

	// Parse vars (can be empty/null)
	var vars gottp.Vars
	if varsJSON != "" && varsJSON != "null" {
		if err := json.Unmarshal([]byte(varsJSON), &vars); err != nil {
			return map[string]interface{}{
				"error": fmt.Sprintf("failed to parse vars: %v", err),
			}
		}
	}

	// Create parse options with YANG modules, source map, and lookups
	var options *gottp.ParseOptions
	if yangModules != nil || enableSourceMap || lookups != nil {
		options = &gottp.ParseOptions{
			YANGModules:     yangModules,
			EnableSourceMap: enableSourceMap,
			Lookups:         lookups,
		}
	}

	// Parse data with validation
	parseResult, err := compiled.ParseWithValidation(inputs, vars, options)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to parse: %v", err),
		}
	}

	// Serialize result data to JSON
	resultJSON, err := json.Marshal(parseResult.Data)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize result: %v", err),
		}
	}

	// Serialize validation results to JSON
	validationResultsJSON, err := json.Marshal(parseResult.ValidationResults)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize validation results: %v", err),
		}
	}

	// Serialize source map to JSON if present
	var sourceMapJSON string
	if parseResult.SourceMap != nil {
		sourceMapBytes, err := json.Marshal(parseResult.SourceMap)
		if err != nil {
			return map[string]interface{}{
				"error": fmt.Sprintf("failed to serialize source map: %v", err),
			}
		}
		sourceMapJSON = string(sourceMapBytes)
	}

	result := map[string]interface{}{
		"result":            string(resultJSON),
		"validationResults": string(validationResultsJSON),
		"error":             nil,
	}
	
	if sourceMapJSON != "" {
		result["sourceMap"] = sourceMapJSON
	}

	return result
}

// formatJSON formats data as JSON
func formatJSON(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"error": "missing dataJSON argument",
		}
	}

	dataJSON := args[0].String()

	// Parse data
	var data interface{}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to parse data: %v", err),
		}
	}

	// Format as JSON
	formatter := formatters.NewJSONFormatter()
	formatted, err := formatter.FormatString(data)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to format JSON: %v", err),
		}
	}

	return map[string]interface{}{
		"result": formatted,
		"error":  nil,
	}
}

// formatYAML formats data as YAML
func formatYAML(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"error": "missing dataJSON argument",
		}
	}

	dataJSON := args[0].String()

	// Parse data
	var data interface{}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to parse data: %v", err),
		}
	}

	// Format as YAML
	formatter := formatters.NewYAMLFormatter()
	formatted, err := formatter.FormatString(data)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to format YAML: %v", err),
		}
	}

	return map[string]interface{}{
		"result": formatted,
		"error":  nil,
	}
}

// formatTable formats data as a table (returns JSON array of arrays)
func formatTable(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"error": "missing dataJSON argument",
		}
	}

	dataJSON := args[0].String()

	// Parse data
	var data interface{}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to parse data: %v", err),
		}
	}

	// Format as table
	formatter := formatters.NewTableFormatter()
	table, err := formatter.Format(data, nil)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to format table: %v", err),
		}
	}

	// Serialize table to JSON
	tableJSON, err := json.Marshal(table)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize table: %v", err),
		}
	}

	return map[string]interface{}{
		"result": string(tableJSON),
		"error":  nil,
	}
}

// formatCSV formats data as CSV
func formatCSV(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"error": "missing dataJSON argument",
		}
	}

	dataJSON := args[0].String()

	// Parse data
	var data interface{}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to parse data: %v", err),
		}
	}

	// Format as CSV
	formatter := formatters.NewCSVFormatter()
	formatted, err := formatter.FormatString(data, nil)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to format CSV: %v", err),
		}
	}

	return map[string]interface{}{
		"result": formatted,
		"error":  nil,
	}
}

// loadYANGModule loads a YANG module from string content
func loadYANGModule(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return map[string]interface{}{
			"error": "missing arguments: need moduleName, moduleContent",
		}
	}

	moduleName := args[0].String()
	moduleContent := args[1].String()

	// Create YANGModules structure
	yangModules := &gottp.YANGModules{
		Modules: map[string]string{
			moduleName: moduleContent,
		},
	}

	// Serialize to JSON
	yangModulesJSON, err := json.Marshal(yangModules)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize YANG modules: %v", err),
		}
	}

	return map[string]interface{}{
		"result": string(yangModulesJSON),
		"error":  nil,
	}
}

// compileTemplateWithOptions compiles a TTP template with compile-time options
func compileTemplateWithOptions(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"error": "missing template argument",
		}
	}

	templateText := args[0].String()

	// Optional 2nd argument: optionsJSON
	var compileOpts *gottp.CompileOptions
	if len(args) >= 2 && args[1].String() != "" && args[1].String() != "null" {
		optionsJSON := args[1].String()

		// Parse the options JSON to extract functionSet name
		var optionsMap map[string]interface{}
		if err := json.Unmarshal([]byte(optionsJSON), &optionsMap); err != nil {
			return map[string]interface{}{
				"error": fmt.Sprintf("failed to parse options: %v", err),
			}
		}

		// Look up function set by name if specified
		if functionSetName, ok := optionsMap["functionSet"].(string); ok && functionSetName != "" {
			functionRegistryMux.RLock()
			funcs, found := functionRegistry[functionSetName]
			functionRegistryMux.RUnlock()

			if !found {
				return map[string]interface{}{
					"error": fmt.Sprintf("function set %q not found in registry", functionSetName),
				}
			}

			compileOpts = &gottp.CompileOptions{
				Functions: funcs,
			}
		}
	}

	// Compile template with options
	compiled, err := gottp.CompileTemplateWithOptions(templateText, compileOpts)
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Store in cache
	cacheKey := templateText

	cacheMutex.Lock()
	compiledTemplateCache[cacheKey] = compiled
	cacheMutex.Unlock()

	// Serialize to JSON for backward compatibility
	compiledJSON, err := gottp.SaveCompiledTemplate(compiled, "json")
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize template: %v", err),
		}
	}

	// Get compilation warnings
	warnings := compiled.GetWarnings()
	warningsJSON, marshalErr := json.Marshal(warnings)
	if marshalErr != nil {
		warningsJSON = []byte("[]")
	}

	resultError := ""
	if err != nil {
		resultError = err.Error()
	}

	result := make(map[string]interface{})
	result["result"] = string(compiledJSON)
	result["cacheKey"] = cacheKey
	result["warnings"] = string(warningsJSON)
	result["error"] = resultError

	return result
}

// listFunctionSets returns the names of all registered function sets
func listFunctionSets(this js.Value, args []js.Value) interface{} {
	functionRegistryMux.RLock()
	defer functionRegistryMux.RUnlock()

	names := make([]string, 0, len(functionRegistry))
	for name := range functionRegistry {
		names = append(names, name)
	}

	namesJSON, err := json.Marshal(names)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize function set names: %v", err),
		}
	}

	return map[string]interface{}{
		"result": string(namesJSON),
		"error":  nil,
	}
}

// loadLookupFromJSON loads a named lookup table from JSON data
func loadLookupFromJSON(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return map[string]interface{}{
			"error": "missing arguments: need name, jsonData",
		}
	}

	name := args[0].String()
	jsonData := args[1].String()

	lookup, err := gottp.LoadLookupFromJSON(name, []byte(jsonData))
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to load lookup from JSON: %v", err),
		}
	}

	resultJSON, err := json.Marshal(lookup)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize lookup: %v", err),
		}
	}

	return map[string]interface{}{
		"result": string(resultJSON),
		"error":  nil,
	}
}

// loadLookupFromYAML loads a named lookup table from YAML data
func loadLookupFromYAML(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return map[string]interface{}{
			"error": "missing arguments: need name, yamlData",
		}
	}

	name := args[0].String()
	yamlData := args[1].String()

	lookup, err := gottp.LoadLookupFromYAML(name, []byte(yamlData))
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to load lookup from YAML: %v", err),
		}
	}

	resultJSON, err := json.Marshal(lookup)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize lookup: %v", err),
		}
	}

	return map[string]interface{}{
		"result": string(resultJSON),
		"error":  nil,
	}
}

// loadLookupFromCSV loads a named lookup table from CSV data
func loadLookupFromCSV(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return map[string]interface{}{
			"error": "missing arguments: need name, csvData",
		}
	}

	name := args[0].String()
	csvData := args[1].String()

	// Optional 3rd argument: keyColumn
	keyColumn := ""
	if len(args) >= 3 && args[2].String() != "" && args[2].String() != "null" {
		keyColumn = args[2].String()
	}

	lookup, err := gottp.LoadLookupFromCSV(name, []byte(csvData), keyColumn)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to load lookup from CSV: %v", err),
		}
	}

	resultJSON, err := json.Marshal(lookup)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize lookup: %v", err),
		}
	}

	return map[string]interface{}{
		"result": string(resultJSON),
		"error":  nil,
	}
}

// loadLookupsFromJSON loads multiple named lookup tables from JSON data
func loadLookupsFromJSON(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"error": "missing argument: need jsonData",
		}
	}

	jsonData := args[0].String()

	lookups, err := gottp.LoadLookupsFromJSON([]byte(jsonData))
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to load lookups from JSON: %v", err),
		}
	}

	resultJSON, err := json.Marshal(lookups)
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("failed to serialize lookups: %v", err),
		}
	}

	return map[string]interface{}{
		"result": string(resultJSON),
		"error":  nil,
	}
}

func main() {
	// Export functions to JavaScript global scope
	js.Global().Set("gottp", js.ValueOf(map[string]interface{}{
		"compileTemplate":            js.FuncOf(compileTemplate),
		"compileTemplateWithOptions": js.FuncOf(compileTemplateWithOptions),
		"parseTemplate":              js.FuncOf(parseTemplate),
		"loadYANGModule":             js.FuncOf(loadYANGModule),
		"loadLookupFromJSON":         js.FuncOf(loadLookupFromJSON),
		"loadLookupFromYAML":         js.FuncOf(loadLookupFromYAML),
		"loadLookupFromCSV":          js.FuncOf(loadLookupFromCSV),
		"loadLookupsFromJSON":        js.FuncOf(loadLookupsFromJSON),
		"listFunctionSets":           js.FuncOf(listFunctionSets),
		"formatJSON":                 js.FuncOf(formatJSON),
		"formatYAML":                 js.FuncOf(formatYAML),
		"formatTable":                js.FuncOf(formatTable),
		"formatCSV":                  js.FuncOf(formatCSV),
	}))

	// Keep the program running
	select {}
}

