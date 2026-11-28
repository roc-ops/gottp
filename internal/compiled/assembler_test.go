package compiled

import (
	"reflect"
	"testing"

	"github.com/roc-ops/gottp/internal/compiler"
)

// Table-driven tests at the assembler seam: hand-built patterns and matches
// in, records out. Each rule is TTP 0.10.1's _results_class behaviour; the
// gottp-repros in test/ttpsuite pin the same rules end to end against live
// Python TTP.

// tp builds an asmPattern. role is "start", "end", "re" or "none" (TTP's
// start_re, end_re, re, or no regex); rank follows from it.
func tp(role string, action compiler.PatternAction, opts ...func(*asmPattern)) func(i, n int) asmPattern {
	return func(i, n int) asmPattern {
		p := asmPattern{action: action}
		switch role {
		case "start":
			p.rank = i
		case "end":
			p.rank = n + i
		case "re":
			p.rank = 2*n + i
		default:
			p.rank = 3*n + i
			p.none = true
		}
		for _, o := range opts {
			o(&p)
		}
		return p
	}
}

func isLine(p *asmPattern) { p.isLine = true }

func joinVar(name, char string) func(*asmPattern) {
	return func(p *asmPattern) { p.joinVars = append(p.joinVars, asmJoinVar{name: name, char: char}) }
}

func testGroup(nested bool, pats ...func(i, n int) asmPattern) *asmGroup {
	g := &asmGroup{name: "g", parentID: -1}
	if nested {
		g.parentID = 1
	}
	for i, p := range pats {
		g.patterns = append(g.patterns, p(i, len(pats)))
	}
	return g
}

// m is a match of pattern p on the line starting at pos (lines are 10 bytes
// long, so the match ends at pos+9). A nil result is a rejected start line.
func m(p, pos int, result map[string]interface{}) asmMatch {
	return asmMatch{pattern: p, start: pos, end: pos + 9, result: result}
}

type v = map[string]interface{}

type wantRec struct {
	result v
	used   []int
	src    [2]int
}

func runAssembler(t *testing.T, g *asmGroup, matches []asmMatch, end int) []wantRec {
	t.Helper()
	var got []wantRec
	a := newAssembler(g, true, func(r asmRecord) error {
		got = append(got, wantRec{result: r.result, used: r.used, src: r.srcRange})
		return nil
	})
	for i, mt := range matches {
		mt.idx = i
		if err := a.add(mt); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.finish(end); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestAssembler(t *testing.T) {
	const (
		start      = compiler.ActionStart
		startEmpty = compiler.ActionStartEmpty
		add        = compiler.ActionAdd
		join       = compiler.ActionJoin
		end        = compiler.ActionEnd
	)
	cases := []struct {
		name    string
		group   *asmGroup
		matches []asmMatch
		want    []wantRec
	}{
		{
			name:  "start then add without overwrite; srcRange runs to the next start and to finish",
			group: testGroup(false, tp("start", start), tp("re", add)),
			matches: []asmMatch{
				m(0, 0, v{"h": "1"}), m(1, 10, v{"v": "a"}), m(1, 20, v{"v": "b", "w": "c"}),
				m(0, 30, v{"h": "2"}),
			},
			want: []wantRec{
				{v{"h": "1", "v": "a", "w": "c"}, []int{0, 1, 2}, [2]int{0, 30}},
				{v{"h": "2"}, []int{3}, [2]int{30, 40}},
			},
		},
		{
			name:    "matches before the first start are dropped",
			group:   testGroup(false, tp("start", start), tp("re", add)),
			matches: []asmMatch{m(1, 0, v{"v": "early"}), m(0, 10, v{"h": "1"}), m(1, 20, v{"v": "a"})},
			want:    []wantRec{{v{"h": "1", "v": "a"}, []int{1, 2}, [2]int{10, 40}}},
		},
		{
			name:    "startempty starts an empty record",
			group:   testGroup(false, tp("start", startEmpty), tp("re", add)),
			matches: []asmMatch{m(0, 0, v{}), m(1, 10, v{"a": "1"}), m(0, 20, v{}), m(0, 30, v{})},
			want: []wantRec{
				{v{"a": "1"}, []int{0, 1}, [2]int{0, 20}},
				{v{}, []int{2}, [2]int{20, 30}},
				{v{}, []int{3}, [2]int{30, 40}},
			},
		},
		{
			name:    "a start pattern beats an ordinary one at a position",
			group:   testGroup(false, tp("start", start), tp("re", add)),
			matches: []asmMatch{m(0, 0, v{"id": "1"}), m(1, 0, v{"kind": "id"}), m(1, 10, v{"kind": "x"})},
			want:    []wantRec{{v{"id": "1", "kind": "x"}, []int{0, 2}, [2]int{0, 40}}},
		},
		{
			name:  "startempty beats start",
			group: testGroup(false, tp("start", start), tp("start", startEmpty), tp("re", add)),
			matches: []asmMatch{
				m(0, 0, v{"a": "1"}), m(1, 0, v{}), m(2, 10, v{"v": "x"}),
			},
			want: []wantRec{{v{"v": "x"}, []int{1, 2}, [2]int{0, 40}}},
		},
		{
			name:  "of two ordinary patterns the first template line wins",
			group: testGroup(false, tp("start", start), tp("re", add), tp("re", add)),
			matches: []asmMatch{
				m(0, 0, v{"h": "1"}), m(2, 10, v{"c": "p"}), m(1, 10, v{"a": "p"}),
			},
			want: []wantRec{{v{"h": "1", "a": "p"}, []int{0, 2}, [2]int{0, 40}}},
		},
		{
			name: "an end pattern ranks before ordinary ones, ends the record and drops its values",
			// Template order: start, ordinary, end; TTP lists end_re before re.
			group: testGroup(false, tp("start", start), tp("re", add), tp("end", end)),
			matches: []asmMatch{
				m(0, 0, v{"h": "1"}), m(1, 10, v{"x": "z"}), m(2, 10, v{"y": "z"}),
				m(1, 20, v{"x": "late"}),
			},
			want: []wantRec{{v{"h": "1"}, []int{0, 2}, [2]int{0, 19}}},
		},
		{
			name:  "after an end, the next start reopens the group",
			group: testGroup(false, tp("start", start), tp("re", add), tp("end", end)),
			matches: []asmMatch{
				m(0, 0, v{"s": "1"}), m(1, 10, v{"v": "a"}), m(2, 20, v{"e": "now"}), m(1, 30, v{"v": "b"}),
				m(0, 40, v{"s": "2"}), m(1, 50, v{"v": "c"}),
			},
			want: []wantRec{
				{v{"s": "1", "v": "a"}, []int{0, 1, 2}, [2]int{0, 29}},
				{v{"s": "2", "v": "c"}, []int{4, 5}, [2]int{40, 40}},
			},
		},
		{
			name:  "_line_ loses to every other tier",
			group: testGroup(false, tp("start", start), tp("re", join, isLine)),
			matches: []asmMatch{
				m(0, 0, v{"h": "a"}), m(1, 0, v{"rest": "host a"}),
				m(1, 10, v{"rest": "foo"}), m(1, 20, v{"rest": "bar"}),
			},
			want: []wantRec{{v{"h": "a", "rest": "foo\nbar"}, []int{0, 2, 3}, [2]int{0, 40}}},
		},
		{
			name:  "two start patterns: the last wins before any record, the record's group after",
			group: testGroup(false, tp("start", start), tp("start", start), tp("re", add)),
			matches: []asmMatch{
				m(0, 0, v{"a": "1"}), m(1, 0, v{"b": "1"}), m(2, 10, v{"v": "x"}),
				m(0, 20, v{"a": "2"}), m(1, 20, v{"b": "2"}), m(2, 30, v{"v": "y"}),
			},
			want: []wantRec{
				{v{"b": "1", "v": "x"}, []int{1, 2}, [2]int{0, 20}},
				{v{"a": "2", "v": "y"}, []int{3, 5}, [2]int{20, 40}},
			},
		},
		{
			name:  "a nested group's parent has started, so its first start tie goes to the first pattern",
			group: testGroup(true, tp("start", start), tp("start", start)),
			matches: []asmMatch{
				m(0, 0, v{"a": "1"}), m(1, 0, v{"b": "1"}),
			},
			want: []wantRec{{v{"a": "1"}, []int{0}, [2]int{0, 40}}},
		},
		{
			name:  "a rejected start line ends the record; lines up to the next start are dropped",
			group: testGroup(false, tp("start", start), tp("re", add)),
			matches: []asmMatch{
				m(0, 0, v{"name": "Gi1"}), m(0, 10, nil), m(1, 20, v{"d": "two"}),
				m(0, 30, v{"name": "Gi2"}), m(1, 40, v{"d": "three"}),
			},
			want: []wantRec{
				{v{"name": "Gi1"}, []int{0}, [2]int{0, 10}},
				{v{"name": "Gi2", "d": "three"}, []int{3, 4}, [2]int{30, 40}},
			},
		},
		{
			name:  "a rejected start loses its position to a valid one once the group has a record",
			group: testGroup(false, tp("start", start), tp("start", start), tp("re", add)),
			matches: []asmMatch{
				m(0, 0, v{"a": "1"}), m(0, 10, v{"a": "2"}), m(1, 10, nil), m(2, 20, v{"v": "x"}),
			},
			want: []wantRec{
				{v{"a": "1"}, []int{0}, [2]int{0, 10}},
				{v{"a": "2", "v": "x"}, []int{1, 3}, [2]int{10, 40}},
			},
		},
		{
			// TTP's second pass leaves the last candidate tried as the
			// winner even when it was skipped as rejected (checked against
			// TTP 0.10.1 with exclude() on the second _start_ line).
			name:    "before any record, a rejected last candidate wins and ends the group",
			group:   testGroup(false, tp("start", start), tp("start", start), tp("re", add)),
			matches: []asmMatch{m(0, 0, v{"a": "1"}), m(1, 0, nil), m(2, 10, v{"v": "x"})},
			want:    nil,
		},
		{
			name:  "join: strings grow by the join character, lists extend, other values become a list",
			group: testGroup(false, tp("start", start), tp("re", join, joinVar("s", ",")), tp("re", join, joinVar("l", "\n")), tp("re", join, joinVar("n", "\n"))),
			matches: []asmMatch{
				m(0, 0, v{"h": "1"}),
				m(1, 10, v{"s": "a"}), m(1, 20, v{"s": "b"}), m(1, 30, v{"s": "c"}),
				m(2, 40, v{"l": []interface{}{"1", "2"}}), m(2, 50, v{"l": "3"}), m(2, 60, v{"l": []interface{}{"4"}}),
				m(3, 70, v{"n": 5}), m(3, 80, v{"n": 6}), m(3, 90, v{"n": 7}),
			},
			want: []wantRec{{v{"h": "1", "s": "a,b,c", "l": []interface{}{"1", "2", "3", "4"}, "n": []interface{}{5, 6, 7}}, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, [2]int{0, 40}}},
		},
		{
			name:    "join joins every variable of the line, not only the joinmatches one",
			group:   testGroup(false, tp("start", start), tp("re", join, joinVar("v", "\n"))),
			matches: []asmMatch{m(0, 0, v{"h": "1"}), m(1, 10, v{"k": "a", "v": "1"}), m(1, 20, v{"k": "b", "v": "2"})},
			want:    []wantRec{{v{"h": "1", "k": "a\nb", "v": "1\n2"}, []int{0, 1, 2}, [2]int{0, 40}}},
		},
		{
			name:    "a joinmatches start line starts a record on every match",
			group:   testGroup(false, tp("start", start, joinVar("x", "\n")), tp("re", add)),
			matches: []asmMatch{m(0, 0, v{"x": "a"}), m(1, 10, v{"v": "1"}), m(0, 20, v{"x": "b"})},
			want: []wantRec{
				{v{"x": "a", "v": "1"}, []int{0, 1}, [2]int{0, 20}},
				{v{"x": "b"}, []int{2}, [2]int{20, 40}},
			},
		},
		{
			name:    "a line TTP has no regex for doesn't compete and only fills missing values",
			group:   testGroup(false, tp("start", start), tp("none", add), tp("re", add)),
			matches: []asmMatch{m(0, 0, v{"h": "1"}), m(1, 10, v{"x": "R1"}), m(2, 10, v{"v": "2"}), m(1, 20, v{"h": "other"})},
			want:    []wantRec{{v{"h": "1", "x": "R1", "v": "2"}, []int{0, 2, 1, 3}, [2]int{0, 40}}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runAssembler(t, tc.group, tc.matches, 40)
			if len(tc.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("records\n got: %#v\nwant: %#v", got, tc.want)
			}
		})
	}
}

// Joining a list must not write into the slice a match result holds (the
// source map keeps match values).
func TestAssemblerJoinDoesNotAliasMatchValues(t *testing.T) {
	g := testGroup(false, tp("start", compiler.ActionStart), tp("re", compiler.ActionJoin, joinVar("l", "\n")))
	first := make([]interface{}, 1, 8)
	first[0] = "1"
	m0 := v{"h": "1", "l": first}
	runAssembler(t, g, []asmMatch{m(0, 0, m0), m(1, 10, v{"l": []interface{}{"2"}})}, 40)
	if got := m0["l"].([]interface{}); len(got) != 1 || got[:2][1] != nil {
		t.Errorf("start match's list was modified: %v", got[:2])
	}
}

// Without trackUsed the assembler keeps no match indices (ParseStream).
func TestAssemblerUntrackedKeepsNoIndices(t *testing.T) {
	g := testGroup(false, tp("start", compiler.ActionStart), tp("re", compiler.ActionAdd))
	var used [][]int
	a := newAssembler(g, false, func(r asmRecord) error { used = append(used, r.used); return nil })
	for _, mt := range []asmMatch{m(0, 0, v{"h": "1"}), m(1, 10, v{"v": "a"})} {
		if err := a.add(mt); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.finish(20); err != nil {
		t.Fatal(err)
	}
	if len(used) != 1 || used[0] != nil {
		t.Errorf("used = %v, want one record with nil indices", used)
	}
}
