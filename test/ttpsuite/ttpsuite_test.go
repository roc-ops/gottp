// Package ttpsuite replays Python TTP 0.10.1's own test suite through gottp.
//
// testdata/cases/*.json holds the 200 auto-portable cases from TTP's
// test/pytest, each with the result TTP 0.10.1 produced (see README.md and
// record.py). Every case must either match that result exactly or be listed in
// known_failures.json with the issue that tracks it.
package ttpsuite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

type input struct {
	Name string `json:"name"`
	Data string `json:"data"`
	// Constructor marks data passed as ttp(data=...) rather than add_input().
	// gottp's Python-style API has only AddInput, so both use it here.
	Constructor bool `json:"constructor,omitempty"`
}

type ttpCase struct {
	Name     string            `json:"name"`
	Source   string            `json:"source"`
	Env      map[string]string `json:"env,omitempty"`
	Template string            `json:"template"`
	Inputs   []input           `json:"inputs"`
	Expected interface{}       `json:"expected"`
}

// knownFailure is one entry in known_failures.json: either a tracking issue
// (the one the first strict difference belongs to, plus any issues the case
// hits after it), or a reason starting with "design:" for a deliberate
// difference.
type knownFailure struct {
	Issue  int    `json:"issue,omitempty"`
	Also   []int  `json:"also,omitempty"`
	Reason string `json:"reason"`
}

func (k knownFailure) design() bool { return strings.HasPrefix(k.Reason, "design:") }

const caseTimeout = 20 * time.Second

func loadCases(t *testing.T) []ttpCase {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "cases", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no case files in testdata/cases (err=%v)", err)
	}
	var all []ttpCase
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var cs []ttpCase
		if err := json.Unmarshal(b, &cs); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		all = append(all, cs...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all
}

func loadKnownFailures(t *testing.T) map[string]knownFailure {
	t.Helper()
	b, err := os.ReadFile("known_failures.json")
	if err != nil {
		t.Fatal(err)
	}
	var kf struct {
		Cases map[string]knownFailure `json:"cases"`
	}
	if err := json.Unmarshal(b, &kf); err != nil {
		t.Fatalf("known_failures.json: %v", err)
	}
	for name, k := range kf.Cases {
		if k.Reason == "" || (k.Issue <= 0 && !k.design()) {
			t.Errorf("known_failures.json: %s needs an issue number and a reason, or a reason starting with \"design:\"", name)
		}
	}
	return kf.Cases
}

// runCase parses one case through gottp's Python-style API and returns the
// result after a JSON round trip, so it compares like the recorded one.
func runCase(c ttpCase) (interface{}, error) {
	type outcome struct {
		result interface{}
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		var o outcome
		defer func() {
			if r := recover(); r != nil {
				o = outcome{err: fmt.Errorf("panic: %v", r)}
			}
			done <- o
		}()
		o.result, o.err = parseCase(c)
	}()
	select {
	case o := <-done:
		return o.result, o.err
	case <-time.After(caseTimeout):
		return nil, fmt.Errorf("timed out after %s", caseTimeout)
	}
}

func parseCase(c ttpCase) (interface{}, error) {
	p := gottp.NewParser()
	if err := p.AddTemplate(c.Template); err != nil {
		return nil, fmt.Errorf("AddTemplate: %w", err)
	}
	for _, in := range c.Inputs {
		p.AddInput(in.Data, in.Name)
	}
	if err := p.Parse(); err != nil {
		return nil, fmt.Errorf("Parse: %w", err)
	}
	b, err := json.Marshal(p.Result())
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	var result interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	return result, nil
}

func TestTTPSuite(t *testing.T) {
	cases := loadCases(t)
	known := loadKnownFailures(t)
	seen := map[string]bool{}
	for _, c := range cases {
		seen[c.Name] = true
	}
	for name := range known {
		if !seen[name] {
			t.Errorf("known_failures.json lists %q, which is not a case", name)
		}
	}

	abs, err := filepath.Abs(filepath.Join("testdata", "pytest"))
	if err != nil {
		t.Fatal(err)
	}
	// Templates reference assets as ./assets/..., relative to TTP's test/pytest.
	t.Chdir(abs)

	// TTPSUITE_REPORT=file writes every case's outcome and first difference
	// as JSON, for triaging failures into known_failures.json.
	type reportRow struct {
		Name    string      `json:"name"`
		Source  string      `json:"source"`
		Problem string      `json:"problem,omitempty"`
		Known   string      `json:"known,omitempty"`
		Got     interface{} `json:"got,omitempty"`
	}
	var report []reportRow

	var pass, byIssue, design, ran int
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			ran++
			for k, v := range c.Env {
				t.Setenv(k, filepath.Join(abs, v))
			}
			got, err := runCase(c)
			var problem string
			if err != nil {
				problem = "gottp error: " + err.Error()
			} else if d := firstDiff(c.Expected, got, "$"); d != "" {
				problem = d
			}
			k, isKnown := known[c.Name]
			report = append(report, reportRow{Name: c.Name, Source: c.Source, Problem: problem, Known: k.describe(), Got: got})
			switch {
			case problem == "" && isKnown:
				t.Errorf("%s now passes: remove it from known_failures.json (was: %s)", c.Name, k.describe())
			case problem == "":
				pass++
			case isKnown:
				if k.design() {
					design++
				} else {
					byIssue++
				}
				t.Skipf("%s; first difference: %s", k.describe(), truncate(problem, 300))
			default:
				t.Errorf("%s (%s) differs from TTP 0.10.1 and is not in known_failures.json\nfirst difference: %s\nexpected: %s\ngot:      %s",
					c.Name, c.Source, problem, truncate(jsonString(c.Expected), 1500), truncate(jsonString(got), 1500))
			}
		})
	}
	if path := os.Getenv("TTPSUITE_REPORT"); path != "" {
		b, _ := json.MarshalIndent(report, "", " ")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Errorf("TTPSUITE_REPORT: %v", err)
		}
	}
	summary := fmt.Sprintf("TTP 0.10.1 suite: pass %d / skipped-by-issue %d / design %d of %d", pass, byIssue, design, len(cases))
	if ran != len(cases) {
		summary += fmt.Sprintf(" (%d run)", ran)
	}
	t.Log(summary)
}

func (k knownFailure) describe() string {
	if k.Issue > 0 {
		s := fmt.Sprintf("#%d", k.Issue)
		for _, n := range k.Also {
			s += fmt.Sprintf(" (also #%d)", n)
		}
		return s + ": " + k.Reason
	}
	return k.Reason
}

// firstDiff walks expected and got together and describes the first place
// they differ, or returns "" if they are equal. No normalization: a missing
// list level is a difference.
func firstDiff(want, got interface{}, path string) string {
	switch w := want.(type) {
	case map[string]interface{}:
		g, ok := got.(map[string]interface{})
		if !ok {
			return fmt.Sprintf("%s: want object, got %s", path, truncate(jsonString(got), 200))
		}
		keys := make([]string, 0, len(w)+len(g))
		for k := range w {
			keys = append(keys, k)
		}
		for k := range g {
			if _, ok := w[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			wv, wok := w[k]
			gv, gok := g[k]
			p := path + "." + k
			switch {
			case !wok:
				return fmt.Sprintf("%s: unexpected key, got %s", p, truncate(jsonString(gv), 200))
			case !gok:
				return fmt.Sprintf("%s: missing, want %s", p, truncate(jsonString(wv), 200))
			}
			if d := firstDiff(wv, gv, p); d != "" {
				return d
			}
		}
		return ""
	case []interface{}:
		g, ok := got.([]interface{})
		if !ok {
			return fmt.Sprintf("%s: want list, got %s", path, truncate(jsonString(got), 200))
		}
		for i := 0; i < len(w) && i < len(g); i++ {
			if d := firstDiff(w[i], g[i], fmt.Sprintf("%s[%d]", path, i)); d != "" {
				return d
			}
		}
		if len(w) != len(g) {
			return fmt.Sprintf("%s: want %d items, got %d", path, len(w), len(g))
		}
		return ""
	default:
		if !reflect.DeepEqual(want, got) {
			return fmt.Sprintf("%s: want %s, got %s", path, truncate(jsonString(want), 200), truncate(jsonString(got), 200))
		}
		return ""
	}
}

func jsonString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%#v", v)
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
