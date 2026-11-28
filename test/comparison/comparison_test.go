package comparison

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/roc-ops/gottp"
)

// pythonBin is the interpreter used to run Python TTP. Override with
// GOTTP_PYTHON (e.g. a venv's python) when python3 on PATH is not the one
// that has the pinned TTP installed.
func pythonBin() string {
	if p := os.Getenv("GOTTP_PYTHON"); p != "" {
		return p
	}
	return "python3"
}

var pythonTTPCheck struct {
	once sync.Once
	err  error
}

// pinnedTTPVersion returns the version pinned as "ttp==X" in requirements.txt.
func pinnedTTPVersion(root string) (string, error) {
	path := filepath.Join(root, "test", "comparison", "requirements.txt")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	for _, line := range strings.Split(string(content), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "ttp=="); ok {
			return strings.TrimSpace(v), nil
		}
	}
	return "", fmt.Errorf("%s has no ttp==<version> line", path)
}

// checkPythonTTP reports why the pinned Python TTP is unusable, or nil.
func checkPythonTTP() error {
	root, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	want, err := pinnedTTPVersion(root)
	if err != nil {
		return err
	}
	install := fmt.Sprintf("install it with: %s -m pip install -r test/comparison/requirements.txt", pythonBin())
	out, err := exec.Command(pythonBin(), "-c",
		"import importlib.metadata as m, ttp; print(m.version('ttp'))").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Python TTP %s is not importable with %s (%v: %s); %s",
			want, pythonBin(), err, strings.TrimSpace(string(out)), install)
	}
	if got := strings.TrimSpace(string(out)); got != want {
		return fmt.Errorf("Python TTP %s found with %s, but the comparison suite is pinned to %s; %s",
			got, pythonBin(), want, install)
	}
	return nil
}

// requirePythonTTP fails the test unless the pinned Python TTP version is
// installed. Comparison tests never skip silently: set GOTTP_SKIP_PYTHON_TTP=1
// to skip them on purpose.
func requirePythonTTP(tb testing.TB) {
	tb.Helper()
	pythonTTPCheck.once.Do(func() { pythonTTPCheck.err = checkPythonTTP() })
	if pythonTTPCheck.err == nil {
		return
	}
	if os.Getenv("GOTTP_SKIP_PYTHON_TTP") == "1" {
		tb.Skipf("GOTTP_SKIP_PYTHON_TTP=1: %v", pythonTTPCheck.err)
	}
	tb.Fatalf("%v (set GOTTP_SKIP_PYTHON_TTP=1 to skip comparison tests on purpose)", pythonTTPCheck.err)
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
	cmd := exec.Command(pythonBin(), scriptPath, "--template", template)
	cmd.Dir = root

	// Add data if provided
	if data != "" {
		cmd.Args = append(cmd.Args, "--data", data)
	}

	// Add vars if provided
	if len(vars) > 0 {
		varsJSON, err := json.Marshal(vars)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal vars: %w", err)
		}
		cmd.Args = append(cmd.Args, "--vars", string(varsJSON))
	}

	// Add lookups if provided
	if len(lookups) > 0 {
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

// normalizeJSON normalizes JSON for comparison: map keys sorted, numbers as
// float64. With unwrapSingleLists it also replaces every one-item list that
// holds a list with that inner list. That loose mode hides list-level bugs
// (it hid #62), so only the tests in looseListLevels use it.
func normalizeJSON(data interface{}, unwrapSingleLists bool) interface{} {
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
			normalized[k] = normalizeJSON(v[k], unwrapSingleLists)
		}
		return normalized

	case []interface{}:
		// Normalize each item first
		normalized := make([]interface{}, len(v))
		for i, item := range v {
			normalized[i] = normalizeJSON(item, unwrapSingleLists)
		}

		if unwrapSingleLists && len(normalized) == 1 {
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
			normalized[i] = normalizeJSON(item, unwrapSingleLists)
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
// (half away from zero, so -1 stays -1).
func roundFloat(f float64, decimals int) float64 {
	multiplier := 1.0
	for i := 0; i < decimals; i++ {
		multiplier *= 10
	}
	return math.Round(f*multiplier) / multiplier
}

// compareResults compares two results and returns true if they match
func compareResults(pythonResult, goResult interface{}, unwrapSingleLists bool) (bool, string) {
	// Normalize both results
	normalizedPython := normalizeJSON(pythonResult, unwrapSingleLists)
	normalizedGo := normalizeJSON(goResult, unwrapSingleLists)

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
		"geoip_lookup":              true, // Requires external GeoIP2 database
		"database_loader":           true, // Requires database drivers
		"url_loader":                true, // May have network dependencies
		"dns":                       true, // May have network dependencies
		"rdns":                      true, // May have network dependencies
		"directory_loader":          true, // May have file system differences
		"javascript_macro":          true, // Python TTP has issues parsing JavaScript macros
		"cerberus_validation":       true, // Requires Cerberus library to be installed in Python TTP
		"text_loader_with_patterns": true, // Complex feature - Python TTP splits input by lines when patterns are in input tag
		"file_loader_with_patterns": true, // Similar to text loader with patterns
		"yaml_formatter":            true, // Requires pyyaml library
		"table_formatter":           true, // Complex nested list structure differences
		"tabulate_formatter":        true, // Requires tabulate library
		"excel_formatter":           true, // Requires openpyxl library
		"jinja2_formatter":          true, // Requires jinja2 library
		"n2g_formatter":             true, // Requires n2g library
	}

	if knownDiffs[feature] {
		t.Skipf("Skipping %s: known difference or requires external dependencies", feature)
	}
}

// RunComparison is a helper function to run a comparison test
func RunComparison(t *testing.T, testName, template, data string, vars map[string]interface{}, lookups map[string]map[string]interface{}) {
	// Skip if Python TTP not available
	requirePythonTTP(t)

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

	// python_runner.py prints parser.result(), which has one item per
	// template; CompiledTemplate.Parse returns that one template's result.
	goResult = []interface{}{goResult}

	// Compare strictly, list levels included.
	equal, diff := compareResults(pythonResult, goResult, false)
	issue, loose := looseListLevels[testName]
	if !loose {
		if !equal {
			t.Errorf("Test %s failed:\n%s", testName, diff)
		}
		return
	}
	if equal {
		t.Errorf("Test %s now matches Python TTP strictly: remove it from looseListLevels (was: %s)", testName, issue)
		return
	}
	// Known list-level difference: compare the rest with lists unwrapped.
	equal, diff = compareResults(pythonResult, goResult, true)
	if !equal {
		t.Errorf("Test %s failed (single-item lists unwrapped for %s):\n%s", testName, issue, diff)
	}
}

// looseListLevels lists the comparison tests whose result matches Python TTP
// only after normalizeJSON unwraps one-item lists, with the issue that causes
// the list-level difference. Every other test is compared strictly. A listed
// test that matches strictly fails, so fixing the issue forces its entries out.
var looseListLevels = map[string]string{
	"csv_formatter":                   "#98: formatter output gets an extra list level",
	"json_formatter":                  "#98: formatter output gets an extra list level",
	"pprint_formatter":                "#98: formatter output gets an extra list level",
	"output_formatter_table_example1": "#98: formatter output gets an extra list level",
	"output_formatter_table_example2": "#98: formatter output gets an extra list level",
	"formatter_with_functions":        "#98: formatter output gets an extra list level",
	"formatter_with_path":             "#98: formatter output gets an extra list level",
	"template_input_output":           "#98: formatter output gets an extra list level",
	"output_macro_then_json":          "#98: formatter output gets an extra list level",
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
	Template        string                            `json:"template"`
	Inputs          map[string]string                 `json:"inputs"`
	Variables       map[string]interface{}            `json:"variables"`
	Lookups         map[string]map[string]interface{} `json:"lookups"`
	YANGModules     map[string]string                 `json:"yangModules,omitempty"`
	SourceMaps      bool                              `json:"sourceMapsEnabled,omitempty"`
	SourceMapColors map[string]interface{}            `json:"sourceMapColors,omitempty"`
	WordWrap        map[string]bool                   `json:"wordWrap,omitempty"`
	Version         string                            `json:"version,omitempty"`
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
