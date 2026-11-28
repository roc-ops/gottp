package compiled

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/parser"
)

// Unit tests for the single-stream tree (tree.go). Each expectation is what
// Python TTP 0.10.1's _results_class does; test/ttpsuite pins the same
// rules end to end against live TTP.

func pp(specs ...string) []pathPart {
	var out []pathPart
	for _, s := range specs {
		key, formatter := s, ""
		for len(key) > 0 && key[len(key)-1] == '*' {
			key = key[:len(key)-1]
			formatter += "*"
		}
		out = append(out, pathPart{key: key, formatter: formatter})
	}
	return out
}

func asJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSaveCurElements(t *testing.T) {
	type save struct {
		path []pathPart
		data interface{}
		want saveResult
	}
	tests := []struct {
		name  string
		saves []save
		want  string
	}{
		{
			name:  "first record is the value, a second makes a list",
			saves: []save{{pp("c"), v{"a": "1"}, saveStored}, {pp("c"), v{"a": "2"}, saveStored}},
			want:  `{"c":[{"a":"1"},{"a":"2"}]}`,
		},
		{
			name:  "a second record makes a list even when it is empty; after that empty records are skipped",
			saves: []save{{pp("c"), v{"a": "1"}, saveStored}, {pp("c"), v{}, saveStored}, {pp("c"), v{}, saveSkipped}},
			want:  `{"c":[{"a":"1"},{}]}`,
		},
		{
			name:  "a path ending in * is a list from the start",
			saves: []save{{pp("c*"), v{"a": "1"}, saveStored}, {pp("c*"), v{}, saveSkipped}},
			want:  `{"c":[{"a":"1"}]}`,
		},
		{
			name:  "an empty first record is filled by the next",
			saves: []save{{pp("c"), v{}, saveStored}, {pp("c"), v{"a": "1"}, saveStored}},
			want:  `{"c":{"a":"1"}}`,
		},
		{
			name:  "** merges into one dict",
			saves: []save{{pp("c**"), v{"a": "1"}, saveMerged}, {pp("c**"), v{"b": "2"}, saveMerged}},
			want:  `{"c":{"a":"1","b":"2"}}`,
		},
		{
			name:  "intermediate lists are entered at their last item",
			saves: []save{{pp("c"), v{"a": "1"}, saveStored}, {pp("c"), v{"a": "2"}, saveStored}, {pp("c", "d"), v{"x": "y"}, saveStored}},
			want:  `{"c":[{"a":"1"},{"a":"2","d":{"x":"y"}}]}`,
		},
		{
			name:  "missing levels are created, with a list for *",
			saves: []save{{pp("c*", "d"), v{"x": "y"}, saveStored}},
			want:  `{"c":[{"d":{"x":"y"}}]}`,
		},
		{
			name:  "a _ segment merges into the record above it",
			saves: []save{{pp("c"), v{"a": "1"}, saveStored}, {pp("c", "_"), v{"b": "2"}, saveStored}},
			want:  `{"c":{"a":"1","b":"2"}}`,
		},
		{
			name:  "itemized values form a list",
			saves: []save{{pp("c*"), "x", saveStored}, {pp("c*"), "z", saveStored}},
			want:  `{"c":["x","z"]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := map[string]interface{}{}
			for i, s := range tt.saves {
				if got := saveCurElements(root, s.path, s.data); got != s.want {
					t.Errorf("save %d: result %d, want %d", i, got, s.want)
				}
			}
			if got := asJSON(t, root); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSaveCurElementsKeepsRecordIdentity(t *testing.T) {
	root := map[string]interface{}{}
	rec := map[string]interface{}{"a": "1"}
	saveCurElements(root, pp("c", "d"), rec)
	if mapID(root["c"].(map[string]interface{})["d"].(map[string]interface{})) != mapID(rec) {
		t.Fatal("the record saved at a new path is not the stored map; the source map finds records by identity")
	}
}

func TestTopSaver(t *testing.T) {
	tests := []struct {
		path  string
		empty []bool
		want  []topAction
	}{
		{"g", []bool{false, false, false}, []topAction{topAppend, topAppend, topAppend}},
		{"g", []bool{true, false, true, true}, []topAction{topAppend, topMerge, topAppend, topSkip}},
		{"g", []bool{true, true, false}, []topAction{topAppend, topMerge, topMerge}},
		{"g*", []bool{true, false, true}, []topAction{topSkip, topAppend, topSkip}},
		{"a.{{ x }}", []bool{true, false}, []topAction{topSkip, topAppend}},
		{"", []bool{true}, []topAction{topSkip}},
	}
	for _, tt := range tests {
		s := newTopSaver(tt.path)
		var got []topAction
		for _, e := range tt.empty {
			got = append(got, s.add(e))
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q %v: got %v, want %v", tt.path, tt.empty, got, tt.want)
		}
	}
}

func compileTop(t *testing.T, tmpl string) *compiler.CompiledGroup {
	t.Helper()
	pt, err := parser.ParseTemplate(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	ct, err := compiler.NewCompiler().CompileTemplate(pt)
	if err != nil {
		t.Fatal(err)
	}
	return ct.Groups[0]
}

func TestBuildTreePaths(t *testing.T) {
	top := compileTop(t, `<template>
<group name="a.b*">
x {{ x }}
<group name="c">
y {{ y }}
<group>
z {{ z }}
</group>
</group>
<group name="/a.b*">
w {{ w }}
</group>
<group name="_">
u {{ u }}
</group>
</group>
</template>`)
	_, groups := buildTree(top)
	type want struct {
		path, rel []string
		name      string
		parentID  int
	}
	wants := []want{
		{path: []string{"a", "b*"}, name: "a.b*", parentID: -1},
		{path: []string{"a", "b*", "c"}, rel: []string{"c"}, name: "a.b*.c", parentID: 0},
		// An anonymous group has its parent's path (and so its PATH).
		{path: []string{"a", "b*", "c"}, rel: []string{"c"}, name: "a.b*.c._anonymous_", parentID: 1},
		// An absolute path that names the top group's keys saves at its path.
		{path: []string{"a", "b*"}, rel: []string{}, name: "a.b*", parentID: 0},
		{path: []string{"a", "b*", "_"}, rel: []string{"_"}, name: "a.b*._", parentID: 0},
	}
	if len(groups) != len(wants) {
		t.Fatalf("got %d groups, want %d", len(groups), len(wants))
	}
	for i, w := range wants {
		g := groups[i]
		if !reflect.DeepEqual(g.path, w.path) || len(g.rel) != len(w.rel) || (len(w.rel) > 0 && !reflect.DeepEqual(g.rel, w.rel)) {
			t.Errorf("group %d: path %v rel %v, want %v %v", i, g.path, g.rel, w.path, w.rel)
		}
		if g.asm.name != w.name || g.asm.parentID != w.parentID || g.asm.id != i {
			t.Errorf("group %d: name %q parent %d id %d, want %q %d %d", i, g.asm.name, g.asm.parentID, g.asm.id, w.name, w.parentID, i)
		}
	}
	if groups[1].asm.path != groups[2].asm.path {
		t.Error("an anonymous child must have its parent's PATH")
	}
}

// TestBuildTreeAbsolutePaths pins TTP's extract_name for absolute child
// names, and which groups save from the input's results (abs).
func TestBuildTreeAbsolutePaths(t *testing.T) {
	type want struct {
		path []string
		abs  bool
	}
	cases := []struct {
		name  string
		tmpl  string
		wants []want
	}{
		{"outside the top group", `<group name="intf">
a {{ a }}
<group name="/vlans">
b {{ b }}
<group name="names">
c {{ c }}
</group>
<group name="/top.n*">
d {{ d }}
</group>
</group>
</group>`, []want{
			{path: []string{"intf"}},
			{path: []string{"vlans"}, abs: true},
			{path: []string{"vlans", "names"}, abs: true},
			{path: []string{"top", "n*"}, abs: true},
		}},
		{"slash alone", `<group name="intf">
a {{ a }}
<group name="/">
b {{ b }}
</group>
</group>`, []want{
			{path: []string{"intf"}},
			{path: []string{"_anonymous_*"}, abs: true},
		}},
		// "_anonymous_*" is not the segment "_anonymous_", so an anonymous
		// top group's absolute child is not prefixed (TTP compares list items).
		{"anonymous top group", `<group>
a {{ a }}
<group name="/vl">
b {{ b }}
</group>
<group name="/">
c {{ c }}
</group>
</group>`, []want{
			{path: []string{"_anonymous_*"}},
			{path: []string{"vl"}, abs: true},
			{path: []string{"_anonymous_*"}},
		}},
		{"parent path with an _anonymous_ segment", `<group name="_anonymous_">
a {{ a }}
<group name="/vl">
b {{ b }}
</group>
</group>`, []want{
			{path: []string{"_anonymous_"}},
			{path: []string{"_anonymous_*", "vl"}},
		}},
		{"void top group", `<group name="intf" void="">
a {{ a }}
<group name="ips">
b {{ b }}
</group>
</group>`, []want{
			{path: []string{"intf"}},
			{path: []string{"intf", "ips"}, abs: true},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, groups := buildTree(compileTop(t, "<template>\n"+c.tmpl+"\n</template>"))
			if len(groups) != len(c.wants) {
				t.Fatalf("got %d groups, want %d", len(groups), len(c.wants))
			}
			for i, w := range c.wants {
				g := groups[i]
				if !reflect.DeepEqual(g.path, w.path) || g.abs != w.abs {
					t.Errorf("group %d: path %v abs %v, want %v %v", i, g.path, g.abs, w.path, w.abs)
				}
				if g.abs && !reflect.DeepEqual(g.rel, g.path) {
					t.Errorf("group %d: an abs group's rel %v must be its path %v", i, g.rel, g.path)
				}
			}
		})
	}
}

// treeAsm builds an assembler over a parent group (id 0, patterns: start,
// ordinary, end) and a child group (id 1, patterns: start, ordinary).
func treeAsm(t *testing.T, childPath string) (*assembler, *[]asmRecord) {
	t.Helper()
	parent := &asmGroup{name: "p", path: "p", id: 0, parentID: -1, patterns: []asmPattern{
		{action: compiler.ActionStart, rank: 0},
		{action: compiler.ActionAdd, rank: 1},
		{action: compiler.ActionEnd, rank: 2},
	}}
	child := &asmGroup{name: "p.c", path: childPath, id: 1, parentID: 0, patterns: []asmPattern{
		{action: compiler.ActionStart, rank: 10},
		{action: compiler.ActionAdd, rank: 11},
	}}
	var got []asmRecord
	a := newTreeAssembler([]*asmGroup{parent, child}, false, func(r asmRecord) error {
		got = append(got, r)
		return nil
	})
	return a, &got
}

func tm(group, pattern, pos int, result v) asmMatch {
	return asmMatch{idx: -1, group: group, pattern: pattern, start: pos, end: pos + 9, result: result}
}

type treeRec struct {
	Group int
	Merge bool
	R     v
}

func runTree(t *testing.T, a *assembler, got *[]asmRecord, ms ...asmMatch) []treeRec {
	t.Helper()
	for _, m := range ms {
		if err := a.add(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.finish(1000); err != nil {
		t.Fatal(err)
	}
	var out []treeRec
	for _, r := range *got {
		out = append(out, treeRec{Group: r.group.id, Merge: r.mergeWithLast, R: r.result})
	}
	return out
}

func TestAssemblerTree(t *testing.T) {
	t.Run("a parent's line after a child record is merged with the parent's last record", func(t *testing.T) {
		a, got := treeAsm(t, "p.c")
		out := runTree(t, a, got,
			tm(0, 0, 0, v{"name": "A"}),
			tm(1, 0, 10, v{"ip": "1"}),
			tm(0, 1, 20, v{"desc": "d"}),
		)
		want := []treeRec{{0, false, v{"name": "A"}}, {1, false, v{"ip": "1"}}, {0, true, v{"desc": "d"}}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("got %v, want %v", out, want)
		}
	})
	t.Run("a child's lines after its parent ended are skipped", func(t *testing.T) {
		a, got := treeAsm(t, "p.c")
		out := runTree(t, a, got,
			tm(0, 0, 0, v{"name": "A"}),
			tm(1, 0, 10, v{"ip": "1"}),
			tm(0, 2, 20, v{}),
			tm(1, 0, 30, v{"ip": "99"}),
			tm(0, 0, 40, v{"name": "B"}),
		)
		want := []treeRec{{0, false, v{"name": "A"}}, {1, false, v{"ip": "1"}}, {0, false, v{"name": "B"}}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("got %v, want %v", out, want)
		}
	})
	t.Run("a group with its parent's PATH adds to the parent's record", func(t *testing.T) {
		// An anonymous child shares its parent's PATH, so its ordinary
		// lines fill the parent's record.
		a, got := treeAsm(t, "p")
		out := runTree(t, a, got,
			tm(0, 0, 0, v{"name": "A"}),
			tm(1, 1, 10, v{"mac": "x"}),
		)
		want := []treeRec{{0, false, v{"name": "A", "mac": "x"}}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("got %v, want %v", out, want)
		}
	})
	t.Run("at one position a child start wins over the parent's ordinary line", func(t *testing.T) {
		a, got := treeAsm(t, "p.c")
		out := runTree(t, a, got,
			tm(0, 0, 0, v{"name": "A"}),
			tm(0, 1, 10, v{"desc": "d"}),
			tm(1, 0, 10, v{"ip": "1"}),
		)
		want := []treeRec{{0, false, v{"name": "A"}}, {1, false, v{"ip": "1"}}}
		if !reflect.DeepEqual(out, want) {
			t.Errorf("got %v, want %v", out, want)
		}
	})
}
