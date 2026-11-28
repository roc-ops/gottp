package ttpsuite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/roc-ops/gottp"
)

const divergencesFile = "sourcemap_divergences.json"

// parseCompiled parses one case through CompiledTemplate.ParseWithValidation,
// with source maps off or on, and returns the data after a JSON round trip.
// With source maps off it runs Runtime.Parse; with them on it runs
// Runtime.ParseWithSourceMap, the path the editor uses.
func parseCompiled(c ttpCase, sourceMap bool) (interface{}, error) {
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
	return jsonRoundTrip(res.Data)
}

// TestSourceMapParity checks that turning source maps on doesn't change the
// parsed data, for every case in testdata/cases and testdata/gottp-repros.
// The source map itself isn't compared. This is independent of whether the
// data matches TTP: a case can fail TestTTPSuite and still be at parity.
//
// sourcemap_divergences.json lists the cases whose data differs, each with
// the issue that tracks the root cause. As with known_failures.json, a listed
// case that reaches parity fails the test until its entry is removed.
func TestSourceMapParity(t *testing.T) {
	cases := append(loadCases(t), loadRepros(t)...)
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
