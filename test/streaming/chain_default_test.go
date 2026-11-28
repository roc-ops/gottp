package streaming_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/roc-ops/gottp"
)

type streamedRecord struct {
	record    map[string]interface{}
	srcRange  [2]int
	groupPath string
}

// parseAndStream runs Parse and ParseStream on the same input and returns
// Parse's records (flattened) and the streamed ones.
func parseAndStream(t *testing.T, c *gottp.CompiledTemplate, input map[string]string, vars map[string]interface{}, opts *gottp.ParseOptions) ([]map[string]interface{}, []streamedRecord) {
	t.Helper()
	copyInputs := func() gottp.Inputs {
		in := gottp.Inputs{}
		for k, v := range input {
			in[k] = v
		}
		return in
	}
	parsed, err := c.Parse(copyInputs(), vars, opts)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var got []streamedRecord
	if err := c.ParseStream(copyInputs(), vars, opts, func(m map[string]interface{}, r [2]int, p string) error {
		got = append(got, streamedRecord{m, r, p})
		return nil
	}); err != nil {
		t.Fatalf("ParseStream: %v", err)
	}
	return flattenRecords(parsed), got
}

func recordsOf(rs []streamedRecord) []map[string]interface{} {
	var out []map[string]interface{}
	for _, r := range rs {
		out = append(out, r.record)
	}
	return out
}

func assertSameRecords(t *testing.T, want []map[string]interface{}, got []streamedRecord) {
	t.Helper()
	g := recordsOf(got)
	if len(want) == 0 && len(g) == 0 {
		return
	}
	if !reflect.DeepEqual(want, g) {
		t.Errorf("records differ\nParse:       %v\nParseStream: %v", want, g)
	}
}

// A chain= naming a template variable is checked against the variable's
// value, as processGroupFunctions resolves it. Before, the gate took the
// variable name ("c1") for a function and refused the template.
func TestParseStream_ChainNamesTemplateVariable(t *testing.T) {
	tmpl := `<vars>
c1 = "contains('d')"
</vars>
<group name="i" chain="c1">
interface {{ interface | _start_ }}
 description {{ d }}
</group>`
	c, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if ok, reasons := gottp.WhyNotStreamable(c); !ok {
		t.Fatalf("WhyNotStreamable: %v", reasons)
	}
	input := map[string]string{"Default_Input": "interface a\n description x\ninterface b\ninterface c\n description z\n"}
	want, got := parseAndStream(t, c, input, nil, nil)
	if len(want) != 2 {
		t.Fatalf("Parse gave %d records, want 2 (the chain filters b): %v", len(want), want)
	}
	assertSameRecords(t, want, got)
}

// The parse's vars can override the template's <vars>, so ParseStream
// checks the chain again with them and refuses a non-allowlisted function
// instead of streaming records that differ from Parse.
func TestParseStream_ChainVariableOverriddenAtParseTime(t *testing.T) {
	tmpl := `<vars>
c1 = "contains('d')"
</vars>
<group name="i" chain="c1">
interface {{ interface | _start_ }}
 description {{ d }}
</group>`
	c, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	input := gottp.Inputs{"Default_Input": "interface a\n description x\n"}
	cases := []struct {
		name string
		vars map[string]interface{}
		opts *gottp.ParseOptions
	}{
		{"ParseOptions.Vars string", nil, &gottp.ParseOptions{Vars: map[string]interface{}{"c1": "contains('d') | itemize(key='interface')"}}},
		{"ParseOptions.Vars list", nil, &gottp.ParseOptions{Vars: map[string]interface{}{"c1": []interface{}{"contains('d')", "expand"}}}},
		{"vars argument", map[string]interface{}{"c1": "itemize(key='interface')"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			err := c.ParseStream(input, tc.vars, tc.opts, func(map[string]interface{}, [2]int, string) error {
				called = true
				return nil
			})
			var gateErr *gottp.TemplateNotStreamableError
			if !errors.As(err, &gateErr) || !errors.Is(err, gottp.ErrTemplateNotStreamable) {
				t.Fatalf("ParseStream error = %v, want *TemplateNotStreamableError", err)
			}
			if !anyContains(gateErr.Reasons, "allowlist") {
				t.Errorf("reasons = %q, want one naming the non-allowlisted function", gateErr.Reasons)
			}
			if called {
				t.Error("fn was called although the chain is not streamable")
			}
		})
	}

	// An override to another allowlisted chain still streams.
	opts := &gottp.ParseOptions{Vars: map[string]interface{}{"c1": "del('d')"}}
	want, got := parseAndStream(t, c, map[string]string{"Default_Input": "interface a\n description x\n"}, nil, opts)
	assertSameRecords(t, want, got)
}

// TTP's own chain= cases that name a <vars> string or list stream the
// records Parse gives.
func TestParseStream_TTPSuiteChainVariableCases(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "ttpsuite", "testdata", "cases", "test_group_chain_attribute.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name     string `json:"name"`
		Template string `json:"template"`
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	wantNames := map[string]bool{
		"test_group_chain_attribute/test_group_chain_attribute_string_var":      true,
		"test_group_chain_attribute/test_group_chain_attribute_list":            true,
		"test_group_chain_attribute/test_group_chain_attribute_list_multi_func": true,
	}
	ran := 0
	for _, tc := range cases {
		if !wantNames[tc.Name] {
			continue
		}
		ran++
		t.Run(filepath.Base(tc.Name), func(t *testing.T) {
			c, err := gottp.CompileTemplate(tc.Template)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if ok, reasons := gottp.WhyNotStreamable(c); !ok {
				t.Fatalf("WhyNotStreamable: %v", reasons)
			}
			// The input is the template's own <input load="text">.
			want, got := parseAndStream(t, c, map[string]string{}, nil, nil)
			if len(want) == 0 {
				t.Fatal("Parse gave no records; the case no longer tests anything")
			}
			assertSameRecords(t, want, got)
		})
	}
	if ran != len(wantNames) {
		t.Fatalf("ran %d of the %d cases; were they renamed?", ran, len(wantNames))
	}
}

// A start line with default() gives a record of defaults when the group
// matches nothing; ParseStream gives the same record, with srcRange [0, 0]
// and the group's path.
func TestParseStream_StartLineDefaultMatchesParse(t *testing.T) {
	tmpl := `<group name="e*" functions="containsall('mac')">
{{ ip }} {{ mac | re("[0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4}") }} {{ kind }} {{ tag | default("") }}
</group>`
	c, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if ok, reasons := gottp.WhyNotStreamable(c); !ok {
		t.Fatalf("WhyNotStreamable: %v", reasons)
	}
	cases := []struct {
		name, input  string
		wantDefaults bool
	}{
		{"empty input", "", true},
		{"no match", "IP MAC Kind Tag\n", true},
		{"matched", "IP MAC Kind Tag\n10.0.0.1 aaaa.bbbb.cccc cpe t1\n10.0.0.2 aaaa.bbbb.dddd emta\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, got := parseAndStream(t, c, map[string]string{"Default_Input": tc.input}, nil, nil)
			if tc.wantDefaults {
				if len(want) != 1 || !reflect.DeepEqual(want[0], map[string]interface{}{"tag": ""}) {
					t.Fatalf("Parse = %v, want one record of defaults", want)
				}
				if len(got) == 1 && (got[0].srcRange != [2]int{0, 0} || got[0].groupPath != "e") {
					t.Errorf("defaults record srcRange=%v groupPath=%q, want [0 0] \"e\"", got[0].srcRange, got[0].groupPath)
				}
			} else if len(want) != 2 {
				t.Fatalf("Parse = %v, want 2 records", want)
			}
			assertSameRecords(t, want, got)
		})
	}
}

// Parse makes the record of defaults only when no record survives the
// chain and macro: a record the chain drops leaves room for it, a record the
// chain empties (then dropped as empty) does not.
func TestParseStream_StartLineDefaultAfterChain(t *testing.T) {
	cases := []struct {
		name, template, input string
	}{
		{"all dropped by chain", `<group name="e*" chain="contains('ip')">
interface {{ name | default("none") }}
 ip {{ ip }}
</group>`, "interface a\ninterface b\n"},
		{"emptied by chain", `<group name="e*" chain="del('name')">
interface {{ name | default("none") }}
</group>`, "interface a\n"},
		{"defaults from two lines", `<group name="e*">
interface {{ name | default("none") }}
 ip {{ ip | default("0") }}
</group>`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := gottp.CompileTemplate(tc.template)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if ok, reasons := gottp.WhyNotStreamable(c); !ok {
				t.Fatalf("WhyNotStreamable: %v", reasons)
			}
			input := map[string]string{"Default_Input": tc.input}
			want, got := parseAndStream(t, c, input, nil, nil)
			assertSameRecords(t, want, got)
		})
	}
}
