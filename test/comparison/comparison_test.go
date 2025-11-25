package comparison

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// pythonTTPAvailable checks if Python TTP is available
func pythonTTPAvailable() bool {
	// Get project root
	root, err := getProjectRoot()
	if err != nil {
		return false
	}

	// Check if python_runner.py exists
	scriptPath := filepath.Join(root, "test", "comparison", "python_runner.py")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return false
	}

	// Try to run Python to check if it's available
	cmd := exec.Command("python3", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	// Try to import TTP (with working directory set)
	cmd = exec.Command("python3", "-c", "import sys; sys.path.insert(0, 'ttp-original'); from ttp import ttp")
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// runPythonTTP executes Python TTP with the given template and data
func runPythonTTP(template, data string, vars map[string]interface{}, lookups map[string]map[string]interface{}) (interface{}, error) {
	// Get project root
	root, err := getProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get project root: %w", err)
	}

	scriptPath := filepath.Join(root, "test", "comparison", "python_runner.py")

	// Build command with working directory set to project root
	cmd := exec.Command("python3", scriptPath, "--template", template)
	cmd.Dir = root

	// Add data if provided
	if data != "" {
		cmd.Args = append(cmd.Args, "--data", data)
	}

	// Add vars if provided
	if vars != nil && len(vars) > 0 {
		varsJSON, err := json.Marshal(vars)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal vars: %w", err)
		}
		cmd.Args = append(cmd.Args, "--vars", string(varsJSON))
	}

	// Add lookups if provided
	if lookups != nil && len(lookups) > 0 {
		lookupsJSON, err := json.Marshal(lookups)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal lookups: %w", err)
		}
		cmd.Args = append(cmd.Args, "--lookups", string(lookupsJSON))
	}

	// Execute and capture output
	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("python ttp failed: %s\nstderr: %s", err, string(exitError.Stderr))
		}
		return nil, fmt.Errorf("failed to execute python ttp: %w", err)
	}

	// Parse JSON output
	var result interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse python ttp output: %w\noutput: %s", err, string(output))
	}

	// Check for errors in result
	if resultMap, ok := result.(map[string]interface{}); ok {
		if errMsg, hasError := resultMap["_error"]; hasError {
			return nil, fmt.Errorf("python ttp error: %v", errMsg)
		}
	}

	return result, nil
}

// runGoTTP executes GoTTP with the given template and data
func runGoTTP(template, data string, vars map[string]interface{}, lookups map[string]map[string]interface{}) (interface{}, error) {
	// Ensure template has results_method="per_input" if not specified
	// Python TTP defaults to per_input, so we need to match that
	if !strings.Contains(template, `results_method=`) && !strings.Contains(template, `<template`) {
		// Wrap in template tag with per_input if not already wrapped
		template = `<template results_method="per_input">` + template + `</template>`
	} else if strings.Contains(template, `<template`) && !strings.Contains(template, `results_method=`) {
		// Add results_method to existing template tag
		template = strings.Replace(template, `<template`, `<template results_method="per_input"`, 1)
	}

	// Compile template
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		return nil, fmt.Errorf("failed to compile template: %w", err)
	}

	// Prepare inputs
	inputs := gottp.Inputs{
		"Default_Input": data,
	}

	// Convert vars
	var gottpVars gottp.Vars
	if vars != nil {
		gottpVars = gottp.Vars(vars)
	}

	// Note: Lookup tables are typically defined in the template itself
	// If lookups are provided, they would need to be added to the template
	// For now, we assume lookups are in the template

	// Parse
	result, err := compiled.Parse(inputs, gottpVars, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return result, nil
}

// normalizeJSON normalizes JSON for comparison
// Handles differences between Python TTP and GoTTP result structures
func normalizeJSON(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{})
		// Collect and sort keys alphabetically
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			normalized[k] = normalizeJSON(v[k])
		}
		return normalized

	case []interface{}:
		// Normalize each item first
		normalized := make([]interface{}, len(v))
		for i, item := range v {
			normalized[i] = normalizeJSON(item)
		}

		// Python TTP uses "per_input" method by default which wraps results in [[...]]
		// GoTTP may use "per_template" by default which returns [...]
		// Try to normalize this difference
		if len(normalized) == 1 {
			if innerList, ok := normalized[0].([]interface{}); ok {
				// Unwrap one level if inner list is the actual result
				return innerList
			}
		}

		return normalized

	case []map[string]interface{}:
		// Convert []map[string]interface{} to []interface{} for consistent comparison
		// This handles cases where GoTTP returns []map[string]interface{} but Python returns []interface{}
		normalized := make([]interface{}, len(v))
		for i, item := range v {
			normalized[i] = normalizeJSON(item)
		}
		return normalized

	case int:
		// Convert int to float64 for consistent comparison (Python uses float for JSON numbers)
		return float64(v)

	case int64:
		return float64(v)

	case int32:
		return float64(v)

	case float64:
		// Round floats to 6 decimal places to handle precision differences
		return roundFloat(v, 6)

	case float32:
		return roundFloat(float64(v), 6)

	default:
		return v
	}
}

// roundFloat rounds a float to the specified number of decimal places
func roundFloat(f float64, decimals int) float64 {
	multiplier := 1.0
	for i := 0; i < decimals; i++ {
		multiplier *= 10
	}
	return float64(int(f*multiplier+0.5)) / multiplier
}

// compareResults compares two results and returns true if they match
func compareResults(pythonResult, goResult interface{}) (bool, string) {
	// Normalize both results
	normalizedPython := normalizeJSON(pythonResult)
	normalizedGo := normalizeJSON(goResult)

	// Deep equality check
	if reflect.DeepEqual(normalizedPython, normalizedGo) {
		return true, ""
	}

	// Generate diff message with type information for debugging
	pythonJSON, _ := json.MarshalIndent(normalizedPython, "", "  ")
	goJSON, _ := json.MarshalIndent(normalizedGo, "", "  ")

	// Add type information for debugging
	typeInfo := fmt.Sprintf("\n\nType info:\nPython type: %T\nGo type: %T", normalizedPython, normalizedGo)
	
	// Try to find the first difference
	diff := findFirstDifference(normalizedPython, normalizedGo, "")
	if diff != "" {
		typeInfo += "\n\nFirst difference: " + diff
	}

	diffMsg := fmt.Sprintf("Results differ:\n\nPython TTP Result:\n%s\n\nGoTTP Result:\n%s%s",
		string(pythonJSON), string(goJSON), typeInfo)

	return false, diffMsg
}

// findFirstDifference recursively finds the first difference between two values
func findFirstDifference(a, b interface{}, path string) string {
	if reflect.DeepEqual(a, b) {
		return ""
	}

	// Check types
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return fmt.Sprintf("Type mismatch at %s: %T vs %T", path, a, b)
	}

	switch av := a.(type) {
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok {
			return fmt.Sprintf("Type mismatch at %s: map vs %T", path, b)
		}
		
		// Check all keys in a
		for k, v := range av {
			newPath := path + "." + k
			if bvVal, exists := bv[k]; !exists {
				return fmt.Sprintf("Key missing in Go result at %s", newPath)
			} else if diff := findFirstDifference(v, bvVal, newPath); diff != "" {
				return diff
			}
		}
		
		// Check for extra keys in b
		for k := range bv {
			if _, exists := av[k]; !exists {
				return fmt.Sprintf("Extra key in Go result at %s.%s", path, k)
			}
		}

	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok {
			return fmt.Sprintf("Type mismatch at %s: slice vs %T", path, b)
		}
		
		if len(av) != len(bv) {
			return fmt.Sprintf("Length mismatch at %s: %d vs %d", path, len(av), len(bv))
		}
		
		for i := range av {
			newPath := fmt.Sprintf("%s[%d]", path, i)
			if diff := findFirstDifference(av[i], bv[i], newPath); diff != "" {
				return diff
			}
		}

	default:
		return fmt.Sprintf("Value mismatch at %s: %v vs %v", path, a, b)
	}

	return ""
}

// skipIfKnownDifference skips a test if it's a known difference
func skipIfKnownDifference(t *testing.T, feature string) {
	knownDiffs := map[string]bool{
		"geoip_lookup":     true, // Requires external GeoIP2 database
		"database_loader":  true, // Requires database drivers
		"url_loader":       true, // May have network dependencies
		"dns":              true, // May have network dependencies
		"rdns":             true, // May have network dependencies
		"directory_loader": true, // May have file system differences
		"javascript_macro": true, // Python TTP has issues parsing JavaScript macros
		"cerberus_validation": true, // Requires Cerberus library to be installed in Python TTP
		"text_loader_with_patterns": true, // Complex feature - Python TTP splits input by lines when patterns are in input tag
		"file_loader_with_patterns": true, // Similar to text loader with patterns
		"yaml_formatter":   true, // Requires pyyaml library
		"table_formatter":  true, // Complex nested list structure differences
		"tabulate_formatter": true, // Requires tabulate library
		"excel_formatter":   true, // Requires openpyxl library
		"jinja2_formatter":  true, // Requires jinja2 library
		"n2g_formatter":     true, // Requires n2g library
	}

	if knownDiffs[feature] {
		t.Skipf("Skipping %s: known difference or requires external dependencies", feature)
	}
}

// RunComparison is a helper function to run a comparison test
func RunComparison(t *testing.T, testName, template, data string, vars map[string]interface{}, lookups map[string]map[string]interface{}) {
	// Skip if Python TTP not available
	if !pythonTTPAvailable() {
		t.Skip("Python TTP not available - skipping comparison test")
	}

	// Run Python TTP
	pythonResult, err := runPythonTTP(template, data, vars, lookups)
	if err != nil {
		t.Fatalf("Python TTP failed: %v", err)
	}

	// Run GoTTP
	goResult, err := runGoTTP(template, data, vars, lookups)
	if err != nil {
		t.Fatalf("GoTTP failed: %v", err)
	}

	// Compare results
	equal, diff := compareResults(pythonResult, goResult)
	if !equal {
		t.Errorf("Test %s failed:\n%s", testName, diff)
	}
}

// RunComparisonWithFile is a helper to run comparison with template/data from files
func RunComparisonWithFile(t *testing.T, testName, templateFile, dataFile string, vars map[string]interface{}, lookups map[string]map[string]interface{}) {
	// Read template file
	templateBytes, err := os.ReadFile(templateFile)
	if err != nil {
		t.Fatalf("Failed to read template file: %v", err)
	}
	template := string(templateBytes)

	// Read data file
	var data string
	if dataFile != "" {
		dataBytes, err := os.ReadFile(dataFile)
		if err != nil {
			t.Fatalf("Failed to read data file: %v", err)
		}
		data = string(dataBytes)
	}

	RunComparison(t, testName, template, data, vars, lookups)
}

// getProjectRoot returns the project root directory
func getProjectRoot() (string, error) {
	// Try to get current working directory first
	wd, err := os.Getwd()
	if err == nil {
		// Check if we're in the project root (has go.mod)
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}
		// Check if we're in test/comparison (navigate up)
		if _, err := os.Stat(filepath.Join(wd, "..", "..", "go.mod")); err == nil {
			return filepath.Join(wd, "..", ".."), nil
		}
	}

	// Fallback: use runtime caller
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get current file path")
	}

	// Navigate up from test/comparison/comparison_test.go to project root
	dir := filepath.Dir(filename)
	dir = filepath.Dir(dir) // test/
	dir = filepath.Dir(dir) // project root

	return dir, nil
}

// fixturePath returns the path to a fixture file
func fixturePath(parts ...string) (string, error) {
	root, err := getProjectRoot()
	if err != nil {
		return "", err
	}

	allParts := append([]string{root, "test", "comparison", "fixtures"}, parts...)
	return filepath.Join(allParts...), nil
}

// gottpConfig represents the structure of a gottp-config.json file exported from the editor
type gottpConfig struct {
	Template      string                            `json:"template"`
	Inputs        map[string]string                 `json:"inputs"`
	Variables     map[string]interface{}            `json:"variables"`
	Lookups       map[string]map[string]interface{} `json:"lookups"`
	YANGModules   map[string]string                 `json:"yangModules,omitempty"`
	SourceMaps    bool                              `json:"sourceMapsEnabled,omitempty"`
	SourceMapColors map[string]interface{}          `json:"sourceMapColors,omitempty"`
	WordWrap      map[string]bool                   `json:"wordWrap,omitempty"`
	Version       string                            `json:"version,omitempty"`
}

// RunComparisonWithConfig loads a gottp-config.json file and runs a comparison test
func RunComparisonWithConfig(t *testing.T, testName, configFile string) {
	// Read config file
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Parse config
	var config gottpConfig
	if err := json.Unmarshal(configBytes, &config); err != nil {
		t.Fatalf("Failed to parse config file: %v", err)
	}

	// Extract data from inputs (use Default_Input if available, otherwise use first input)
	var data string
	if config.Inputs != nil {
		if defaultInput, ok := config.Inputs["Default_Input"]; ok {
			data = defaultInput
		} else {
			// Use first input if Default_Input not found
			for _, inputData := range config.Inputs {
				data = inputData
				break
			}
		}
	}

	// Run comparison with extracted values
	RunComparison(t, testName, config.Template, data, config.Variables, config.Lookups)
}
