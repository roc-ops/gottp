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

	return map[string]interface{}{
		"result":   string(compiledJSON),
		"cacheKey": cacheKey,
		"error":    nil,
	}
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
	var compiled *gottp.CompiledTemplate
	var err error
	
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

	// Create parse options with YANG modules
	var options *gottp.ParseOptions
	if yangModules != nil {
		options = &gottp.ParseOptions{
			YANGModules: yangModules,
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

	return map[string]interface{}{
		"result":            string(resultJSON),
		"validationResults": string(validationResultsJSON),
		"error":             nil,
	}
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

func main() {
	// Export functions to JavaScript global scope
	js.Global().Set("gottp", js.ValueOf(map[string]interface{}{
		"compileTemplate": js.FuncOf(compileTemplate),
		"parseTemplate":   js.FuncOf(parseTemplate),
		"loadYANGModule":  js.FuncOf(loadYANGModule),
		"formatJSON":      js.FuncOf(formatJSON),
		"formatYAML":      js.FuncOf(formatYAML),
		"formatTable":     js.FuncOf(formatTable),
		"formatCSV":       js.FuncOf(formatCSV),
	}))

	// Keep the program running
	select {}
}

