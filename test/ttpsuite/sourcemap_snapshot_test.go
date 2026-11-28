package ttpsuite

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// updateSnapshots rewrites the source-map snapshots instead of comparing:
//
//	go test ./test/ttpsuite -run TestSourceMapSnapshots -update-sourcemap-snapshots
var updateSnapshots = flag.Bool("update-sourcemap-snapshots", false, "rewrite testdata/sourcemap-snapshots instead of comparing against them")

var snapshotsDir = filepath.Join("testdata", "sourcemap-snapshots")

// TestSourceMapSnapshots pins the source map that ParseWithValidation returns
// with EnableSourceMap for every source-map parity case, against
// testdata/sourcemap-snapshots/<case>.json. Any difference fails; a snapshot
// that names no case fails too. After a deliberate change, rerun with
// -update-sourcemap-snapshots and review the diff.
func TestSourceMapSnapshots(t *testing.T) {
	cases := sourceMapCases(t)
	absSnapshots, err := filepath.Abs(snapshotsDir)
	if err != nil {
		t.Fatal(err)
	}
	checkNoStaleSnapshots(t, absSnapshots, cases)
	abs := enterPytestDir(t)

	var changed int
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			for k, v := range c.Env {
				t.Setenv(k, filepath.Join(abs, v))
			}
			got, err := runWithTimeout(func() (interface{}, error) {
				res, err := parseWithSourceMapOption(c, true)
				if err != nil {
					return nil, err
				}
				return res.SourceMap, nil
			})
			var sm *gottp.SourceMap
			if got != nil {
				sm = got.(*gottp.SourceMap)
			}
			snap := renderSourceMapSnapshot(sm, err)
			path := filepath.Join(absSnapshots, filepath.FromSlash(c.Name)+".json")
			if *updateSnapshots {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, snap) {
					return
				}
				changed++
				if err := os.WriteFile(path, snap, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("no snapshot for %s (%v): run with -update-sourcemap-snapshots", c.Name, err)
			}
			if !bytes.Equal(want, snap) {
				t.Errorf("source map differs from %s (rerun with -update-sourcemap-snapshots after reviewing):\n%s",
					filepath.ToSlash(filepath.Join(snapshotsDir, c.Name+".json")), firstLineDiff(string(want), string(snap)))
			}
		})
	}
	if *updateSnapshots {
		t.Logf("source-map snapshots: %d of %d rewritten", changed, len(cases))
	}
}

// checkNoStaleSnapshots fails for every snapshot file that names no case.
func checkNoStaleSnapshots(t *testing.T, dir string, cases []ttpCase) {
	t.Helper()
	seen := map[string]bool{}
	for _, c := range cases {
		seen[filepath.FromSlash(c.Name)+".json"] = true
	}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if !seen[rel] {
			if *updateSnapshots {
				_ = os.Remove(path)
			} else {
				t.Errorf("%s is not a case: delete it (or run with -update-sourcemap-snapshots)", filepath.ToSlash(filepath.Join(snapshotsDir, rel)))
			}
		}
		return nil
	})
}

// renderSourceMapSnapshot writes a source map (or the parse error) as JSON
// with one input line per row, so a diff points at the line that changed.
// Map keys are sorted, so the output is deterministic for a deterministic
// source map.
func renderSourceMapSnapshot(sm *gottp.SourceMap, parseErr error) []byte {
	var b bytes.Buffer
	b.WriteString("{\n")
	if parseErr != nil {
		e, _ := json.Marshal(parseErr.Error())
		fmt.Fprintf(&b, " \"error\": %s\n}\n", e)
		return b.Bytes()
	}
	if sm == nil {
		b.WriteString(" \"source_map\": null\n}\n")
		return b.Bytes()
	}
	names := make([]string, 0, len(sm.Inputs))
	for name := range sm.Inputs {
		names = append(names, name)
	}
	sort.Strings(names)
	b.WriteString(" \"inputs\": {")
	for i, name := range names {
		if i > 0 {
			b.WriteString(",")
		}
		n, _ := json.Marshal(name)
		fmt.Fprintf(&b, "\n  %s: ", n)
		in := sm.Inputs[name]
		if in == nil {
			b.WriteString("null")
			continue
		}
		b.WriteString("[")
		for j, line := range in.Lines {
			if j > 0 {
				b.WriteString(",")
			}
			l, err := json.Marshal(line)
			if err != nil {
				l, _ = json.Marshal(err.Error())
			}
			fmt.Fprintf(&b, "\n   %s", l)
		}
		b.WriteString("\n  ]")
	}
	b.WriteString("\n }\n}\n")
	return b.Bytes()
}

// firstLineDiff shows the first differing line of two snapshots.
func firstLineDiff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("line %d\nwant: %s\ngot:  %s", i+1, truncate(wl, 1500), truncate(gl, 1500))
		}
	}
	return "(no line difference)"
}
