package ttpsuite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

const divergencesFile = "sourcemap_divergences.json"

// parityOutcome is what TestSourceMapParity compares: the parsed data (after
// a JSON round trip) and the key fields reported by ParseResult.KeyFields.
type parityOutcome struct {
	Data      interface{}         `json:"data"`
	KeyFields map[string][]string `json:"key_fields,omitempty"`
}

// parseCompiled parses one case through CompiledTemplate.ParseWithValidation,
// with source maps off or on, and returns the data (after a JSON round trip)
// and the key fields. With source maps off it runs Runtime.Parse; with them
// on it runs Runtime.ParseWithSourceMap, the path the editor uses.
func parseCompiled(c ttpCase, sourceMap bool) (interface{}, error) {
	res, err := parseWithSourceMapOption(c, sourceMap)
	if err != nil {
		return nil, err
	}
	out := parityOutcome{Data: res.Data}
	if len(res.KeyFields) > 0 {
		out.KeyFields = res.KeyFields
	}
	// The round trip turns the outcome into plain maps and lists, so
	// firstDiff reports paths like $.data[0].g or $.key_fields.g.
	return jsonRoundTrip(out)
}

// parseWithSourceMapOption compiles c and runs ParseWithValidation with
// EnableSourceMap set to sourceMap.
func parseWithSourceMapOption(c ttpCase, sourceMap bool) (*gottp.ParseResult, error) {
	ct, err := gottp.CompileTemplate(c.Template)
	if err != nil {
		return nil, fmt.Errorf("CompileTemplate: %w", err)
	}
	inputs := gottp.Inputs{}
	for _, in := range c.Inputs {
		inputs[in.Name] = in.Data
	}
	var opts *gottp.ParseOptions
	if sourceMap {
		opts = &gottp.ParseOptions{EnableSourceMap: true}
	}
	res, err := ct.ParseWithValidation(inputs, nil, opts)
	if err != nil {
		return nil, fmt.Errorf("ParseWithValidation: %w", err)
	}
	if sourceMap && res.SourceMap == nil {
		return nil, fmt.Errorf("EnableSourceMap: true returned no source map")
	}
	return res, nil
}

// sourceMapParityDir holds gottp-only cases for TestSourceMapParity (and the
// source-map snapshots): NAME.ttp (template) and NAME.txt (the Default_Input
// data). They have no Python TTP expectation because they cover gottp
// features (keys=, input macros) or paths the TTP suite doesn't reach. Their
// cases are named "sourcemap-parity/NAME".
var sourceMapParityDir = filepath.Join("testdata", "sourcemap-parity")

const sourceMapParityPrefix = "sourcemap-parity/"

func loadSourceMapParityCases(t *testing.T) []ttpCase {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(sourceMapParityDir, "*.ttp"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no cases in %s (err=%v)", sourceMapParityDir, err)
	}
	var all []ttpCase
	for _, f := range files {
		stem := strings.TrimSuffix(f, ".ttp")
		tmpl, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(stem + ".txt")
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		all = append(all, ttpCase{
			Name:     sourceMapParityPrefix + filepath.Base(stem),
			Source:   filepath.ToSlash(f),
			Template: string(tmpl),
			Inputs:   []input{{Name: "Default_Input", Data: string(data)}},
		})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all
}

// sourceMapCases is every case the source-map tests run: the TTP suite, the
// gottp repros and the gottp-only source-map parity cases.
func sourceMapCases(t *testing.T) []ttpCase {
	t.Helper()
	cases := append(loadCases(t), loadRepros(t)...)
	return append(cases, loadSourceMapParityCases(t)...)
}

// TestSourceMapParity checks that turning source maps on doesn't change the
// parsed data or the key fields, for every case in testdata/cases,
// testdata/gottp-repros and testdata/sourcemap-parity. The source map itself
// isn't compared here (TestSourceMapSnapshots pins it). This is independent of whether the
// data matches TTP: a case can fail TestTTPSuite and still be at parity.
//
// sourcemap_divergences.json lists the cases whose data differs, each with
// the issue that tracks the root cause. As with known_failures.json, a listed
// case that reaches parity fails the test until its entry is removed.
func TestSourceMapParity(t *testing.T) {
	cases := sourceMapCases(t)
	known := loadKnownFailures(t, divergencesFile)
	checkNoStaleEntries(t, divergencesFile, known, cases, func(string) bool { return true })
	abs := enterPytestDir(t)

	// TTPSUITE_SOURCEMAP_REPORT=file writes every diverging case with both
	// results, for triage.
	type reportRow struct {
		Name    string      `json:"name"`
		Problem string      `json:"problem"`
		Known   string      `json:"known,omitempty"`
		Off     interface{} `json:"off"`
		On      interface{} `json:"on"`
	}
	var report []reportRow

	var same, diverge int
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			for k, v := range c.Env {
				t.Setenv(k, filepath.Join(abs, v))
			}
			off, errOff := runWithTimeout(func() (interface{}, error) { return parseCompiled(c, false) })
			on, errOn := runWithTimeout(func() (interface{}, error) { return parseCompiled(c, true) })
			problem := parityDiff(off, errOff, on, errOn)
			k, isKnown := known[c.Name]
			if problem != "" {
				diverge++
				report = append(report, reportRow{Name: c.Name, Problem: problem, Known: k.describe(), Off: off, On: on})
			}
			switch {
			case problem == "" && isKnown:
				t.Errorf("%s is now at source-map parity: remove it from %s (was: %s)", c.Name, divergencesFile, k.describe())
			case problem == "":
				same++
			case isKnown:
				t.Skipf("%s; first difference (off vs on): %s", k.describe(), truncate(problem, 300))
			default:
				t.Errorf("%s: source maps on changes the result and the case is not in %s\nfirst difference (off vs on): %s\noff: %s\non:  %s",
					c.Name, divergencesFile, problem, truncate(jsonString(off), 1500), truncate(jsonString(on), 1500))
			}
		})
	}
	if path := os.Getenv("TTPSUITE_SOURCEMAP_REPORT"); path != "" {
		b, _ := json.MarshalIndent(report, "", " ")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Errorf("TTPSUITE_SOURCEMAP_REPORT: %v", err)
		}
	}
	t.Logf("source-map parity: same %d / diverge %d of %d", same, diverge, len(cases))
}

// parityDiff describes the first difference between the source-map-off and
// source-map-on outcomes, or returns "" if they match (equal data, or the
// same error).
func parityDiff(off interface{}, errOff error, on interface{}, errOn error) string {
	switch {
	case errOff != nil || errOn != nil:
		if errOff != nil && errOn != nil && errOff.Error() == errOn.Error() {
			return ""
		}
		return fmt.Sprintf("off error: %v; on error: %v", errOff, errOn)
	default:
		return firstDiff(off, on, "$")
	}
}
