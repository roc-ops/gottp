package ttpsuite

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/roc-ops/gottp"
)

// updateStreamSnapshots rewrites the stream snapshots instead of comparing:
//
//	go test ./test/ttpsuite -run TestStreamSnapshots -update-stream-snapshots
var updateStreamSnapshots = flag.Bool("update-stream-snapshots", false, "rewrite testdata/stream-snapshots instead of comparing against them")

var streamSnapshotsDir = filepath.Join("testdata", "stream-snapshots")

// streamRow is one ParseStream callback: the record, its source range and
// its group path.
type streamRow struct {
	GroupPath string                 `json:"group_path"`
	SrcRange  [2]int                 `json:"src_range"`
	Record    map[string]interface{} `json:"record"`
}

// TestStreamSnapshots pins what ParseStream emits for every source-map
// parity case that the streaming gate accepts (suite, repros and
// sourcemap-parity cases): each record with its srcRange and groupPath, in
// callback order, against testdata/stream-snapshots/<case>.json. It also
// checks that the streamed records are the leaf records Parse returns (as a
// multiset), so the two pipelines can't drift apart. A case the gate rejects
// has no snapshot; a snapshot for such a case, or for no case, fails. After a
// deliberate change, rerun with -update-stream-snapshots and review the diff.
func TestStreamSnapshots(t *testing.T) {
	cases := sourceMapCases(t)
	absSnapshots, err := filepath.Abs(streamSnapshotsDir)
	if err != nil {
		t.Fatal(err)
	}
	streamable := map[string]bool{}
	abs := enterPytestDir(t)

	var changed, streamed int
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			for k, v := range c.Env {
				t.Setenv(k, filepath.Join(abs, v))
			}
			rows, parsed, err := streamCase(c)
			if errors.Is(err, errNotStreamable) {
				return
			}
			streamable[filepath.FromSlash(c.Name)+".json"] = true
			streamed++
			if _, skip := streamParityExceptions[c.Name]; err == nil && !skip {
				checkStreamMatchesParse(t, rows, parsed)
			}
			snap := renderStreamSnapshot(rows, err)
			path := filepath.Join(absSnapshots, filepath.FromSlash(c.Name)+".json")
			if *updateStreamSnapshots {
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
				t.Fatalf("no stream snapshot for %s (%v): run with -update-stream-snapshots", c.Name, err)
			}
			if !bytes.Equal(want, snap) {
				t.Errorf("ParseStream output differs from %s (rerun with -update-stream-snapshots after reviewing):\n%s",
					filepath.ToSlash(filepath.Join(streamSnapshotsDir, c.Name+".json")), firstLineDiff(string(want), string(snap)))
			}
		})
	}
	_ = filepath.WalkDir(absSnapshots, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(absSnapshots, path)
		if !streamable[rel] {
			if *updateStreamSnapshots {
				_ = os.Remove(path)
			} else {
				t.Errorf("%s names no streamable case: delete it (or run with -update-stream-snapshots)", filepath.ToSlash(filepath.Join(streamSnapshotsDir, rel)))
			}
		}
		return nil
	})
	t.Logf("stream snapshots: %d streamable of %d cases", streamed, len(cases))
	if *updateStreamSnapshots {
		t.Logf("stream snapshots: %d rewritten", changed)
	}
}

// streamParityExceptions are streamable cases whose Parse result is not just
// the group records, so checkStreamMatchesParse doesn't apply. ParseStream
// emits group records only; it never runs outputs, <vars name> or lookups
// that add data.
var streamParityExceptions = map[string]string{
	"test_anonymous_group/test_anonymous_group_with_vars":        "<vars name> adds a record to Parse's result",
	"test_extend_tag/test_extend_tag_from_file_output_filter":    "<output> filters Parse's result",
	"test_lookups/test_lookup_include_yaml":                      "lookup add_field nests the record in Parse's result",
	"test_output_formatters/test_csv_formatter_simple":           "<output format=csv> turns Parse's result into text",
	"test_output_formatters/test_csv_formatter_simple_empty_tag": "<output format=csv> turns Parse's result into text",
	"test_output_formatters/test_table_formatter_multiple_input": "<output format=table> reshapes Parse's result",
}

var errNotStreamable = errors.New("not streamable")

// streamCase runs ParseStream on c, and Parse for comparison. It returns
// errNotStreamable when the gate rejects the template.
func streamCase(c ttpCase) ([]streamRow, interface{}, error) {
	type out struct {
		rows   []streamRow
		parsed interface{}
	}
	res, err := runWithTimeout(func() (interface{}, error) {
		ct, err := gottp.CompileTemplate(c.Template)
		if err != nil {
			return nil, fmt.Errorf("CompileTemplate: %w", err)
		}
		inputs := gottp.Inputs{}
		for _, in := range c.Inputs {
			inputs[in.Name] = in.Data
		}
		var o out
		err = ct.ParseStream(inputs, nil, nil, func(m map[string]interface{}, sr [2]int, gp string) error {
			rec, err := jsonRoundTrip(m)
			if err != nil {
				return err
			}
			o.rows = append(o.rows, streamRow{GroupPath: gp, SrcRange: sr, Record: rec.(map[string]interface{})})
			return nil
		})
		var gateErr *gottp.TemplateNotStreamableError
		if errors.As(err, &gateErr) {
			return nil, errNotStreamable
		}
		if err != nil {
			return nil, fmt.Errorf("ParseStream: %w", err)
		}
		parsed, err := ct.Parse(inputs, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("Parse: %w", err)
		}
		if o.parsed, err = jsonRoundTrip(parsed); err != nil {
			return nil, err
		}
		return o, nil
	})
	if err != nil {
		return nil, nil, err
	}
	o := res.(out)
	return o.rows, o.parsed, nil
}

// checkStreamMatchesParse requires the streamed records to be the leaf
// records of Parse's result, ignoring order.
func checkStreamMatchesParse(t *testing.T, rows []streamRow, parsed interface{}) {
	t.Helper()
	var got, want []string
	streamed := map[string]bool{}
	for _, r := range rows {
		b, _ := json.Marshal(r.Record)
		got = append(got, string(b))
		streamed[string(b)] = true
	}
	var leaves []map[string]interface{}
	collectLeafRecords(parsed, streamed, &leaves)
	for _, m := range leaves {
		b, _ := json.Marshal(m)
		want = append(want, string(b))
	}
	sort.Strings(got)
	sort.Strings(want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("ParseStream records differ from Parse's\nParse:       %v\nParseStream: %v", want, got)
	}
}

// collectLeafRecords collects the records of groups without children: every
// map equal to a streamed record, and every other non-empty map whose values
// hold no maps (directly or in a list). The first rule keeps a record whose
// value is a dict (a default({...}), say) from being taken apart; a record
// Parse has and the stream doesn't still turns up through the second.
func collectLeafRecords(v interface{}, streamed map[string]bool, out *[]map[string]interface{}) {
	switch t := v.(type) {
	case []interface{}:
		for _, item := range t {
			collectLeafRecords(item, streamed, out)
		}
	case map[string]interface{}:
		if b, err := json.Marshal(t); err == nil && len(t) > 0 && streamed[string(b)] {
			*out = append(*out, t)
			return
		}
		leaf := true
		for _, val := range t {
			switch vt := val.(type) {
			case map[string]interface{}:
				leaf = false
			case []interface{}:
				for _, e := range vt {
					if _, ok := e.(map[string]interface{}); ok {
						leaf = false
					}
				}
			}
		}
		if leaf {
			if len(t) > 0 {
				*out = append(*out, t)
			}
			return
		}
		for _, val := range t {
			collectLeafRecords(val, streamed, out)
		}
	}
}

// renderStreamSnapshot writes the callbacks (or the error) as JSON, one
// callback per row.
func renderStreamSnapshot(rows []streamRow, streamErr error) []byte {
	var b bytes.Buffer
	b.WriteString("{\n")
	if streamErr != nil {
		e, _ := json.Marshal(streamErr.Error())
		fmt.Fprintf(&b, " \"error\": %s\n}\n", e)
		return b.Bytes()
	}
	b.WriteString(" \"records\": [")
	for i, r := range rows {
		if i > 0 {
			b.WriteString(",")
		}
		l, _ := json.Marshal(r)
		fmt.Fprintf(&b, "\n  %s", l)
	}
	if len(rows) > 0 {
		b.WriteString("\n ")
	}
	b.WriteString("]\n}\n")
	return b.Bytes()
}
