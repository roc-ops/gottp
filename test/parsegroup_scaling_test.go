package test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

// TestParseGroupScalesLinearly guards against reintroducing the O(parents ×
// matches) rescans in parseGroup's parent-range computation.
//
// Those rescans made parseGroup quadratic in the number of parent matches: on
// a production 5.6 MB input (~100K matches) they were ~17% of aggregate CPU,
// ~50% within one 30s window.
//
// The assertion is deliberately loose. Across 12 runs of the fixed code
// (including 6 under heavy CPU contention) an 8x row increase cost 6.0x-11.4x;
// across 6 runs of the quadratic code it cost 41.7x-94.5x. The 18x threshold
// sits between those bands with headroom on both sides. Contention makes the
// quadratic version worse, not better, so load widens the gap rather than
// closing it.
//
// Two details matter for stability, both learned from a run that passed
// against known-quadratic code. The small measurement must be large enough not
// to be noise-dominated: inflating it *deflates* the ratio, so a noise burst
// that lands on a short small-measurement hides a real regression. And the
// large size is measured first, so the small size is timed against an already
// warm allocator. Both sizes take the best of several runs for the same
// reason.
//
// If this fails, the parent-range block has almost certainly gone back to
// scanning allMatches once per parent match.
func TestParseGroupScalesLinearly(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped in -short mode")
	}

	template := `<group name="cm*">
{{mac}} {{ip}} {{us}} {{ds}} {{status}}
</group>`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	build := func(rows int) gottp.Inputs {
		var sb strings.Builder
		for i := 0; i < rows; i++ {
			fmt.Fprintf(&sb, "0000.1111.%04x 10.0.0.%d us%d ds%d online\n", i%65536, i%250, i%8, i%8)
		}
		return gottp.Inputs{"Default_Input": sb.String()}
	}

	// Take the best of several runs at each size so a single GC pause or a
	// noisy neighbour cannot fail the build.
	measure := func(rows, runs int) time.Duration {
		inputs := build(rows)
		best := time.Duration(1<<62 - 1)
		for i := 0; i < runs; i++ {
			start := time.Now()
			if _, err := compiled.Parse(inputs, nil, nil); err != nil {
				t.Fatalf("parse %d rows: %v", rows, err)
			}
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}

	const (
		smallRows   = 10000
		largeRows   = smallRows * 8
		maxSlowdown = 18.0
	)

	// Large first: see the note above about warm-up skewing the small number.
	large := measure(largeRows, 3)
	small := measure(smallRows, 5)

	ratio := float64(large) / float64(small)
	t.Logf("%d rows: %v; %d rows: %v; ratio %.1fx (linear would be %dx)",
		smallRows, small, largeRows, large, ratio, largeRows/smallRows)

	if ratio > maxSlowdown {
		t.Errorf("parseGroup scaling regressed: %dx more rows cost %.1fx more time (limit %.0fx). "+
			"A quadratic rescan over allMatches has likely been reintroduced in the "+
			"parent-range computation.", largeRows/smallRows, ratio, maxSlowdown)
	}
}

// TestParseGroupEndPatternRanges pins the parent-range selection that the
// sort.Search rewrite depends on: for each parent match, the range must end at
// the _end_ pattern belonging to that parent, not at some earlier or later
// parent's _end_. Getting this wrong silently mis-slices the input handed to
// nested groups, so it is asserted through nested-group content.
//
// The fourth block deliberately contains two `!` lines. The rewritten lookup is
// a lower bound, i.e. the *first* _end_ at or after the parent's start; a
// version that took the last one instead would swallow the trailing neighbor
// and is only distinguishable when a parent's span holds more than one
// candidate.
func TestParseGroupEndPatternRanges(t *testing.T) {
	template := `<group name="bgp*">
router bgp {{ asn }} {{ _start_ }}
 <group name="neighbors*">
 neighbor {{ neighbor }} remote-as {{ remote_as }}
 </group>
!{{ _end_ }}
</group>`

	input := `router bgp 65001
 neighbor 10.0.0.2 remote-as 65002
 neighbor 10.0.0.3 remote-as 65003
!
router bgp 65010
 neighbor 10.1.0.2 remote-as 65011
!
router bgp 65020
 neighbor 10.2.0.2 remote-as 65021
 neighbor 10.2.0.3 remote-as 65022
 neighbor 10.2.0.4 remote-as 65023
!
router bgp 65030
 neighbor 10.3.0.2 remote-as 65031
!
 neighbor 10.3.0.9 remote-as 65039
!
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	res, err := compiled.Parse(gottp.Inputs{"Default_Input": input}, nil, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	parents := flattenParents(t, res)
	if len(parents) != 4 {
		t.Fatalf("expected 4 parent matches, got %d: %#v", len(parents), parents)
	}

	// Each parent must own exactly the neighbors inside its own _start_.._end_
	// span. A range that ended too early or ran into the next parent would
	// change these counts.
	wantASN := []string{"65001", "65010", "65020", "65030"}
	wantNeighbors := [][]string{
		{"10.0.0.2", "10.0.0.3"},
		{"10.1.0.2"},
		{"10.2.0.2", "10.2.0.3", "10.2.0.4"},
		// Range must stop at the FIRST `!`, so 10.3.0.9 is out of span.
		{"10.3.0.2"},
	}

	for i, parent := range parents {
		if got := parent["asn"]; got != wantASN[i] {
			t.Errorf("parent %d: asn = %v, want %v", i, got, wantASN[i])
		}
		got := neighborAddrs(parent["neighbors"])
		if len(got) != len(wantNeighbors[i]) {
			t.Errorf("parent %d (asn %s): got %d neighbors %v, want %d %v",
				i, wantASN[i], len(got), got, len(wantNeighbors[i]), wantNeighbors[i])
			continue
		}
		for j := range got {
			if got[j] != wantNeighbors[i][j] {
				t.Errorf("parent %d (asn %s): neighbor %d = %s, want %s",
					i, wantASN[i], j, got[j], wantNeighbors[i][j])
			}
		}
	}
}

// flattenParents unwraps the nested []interface{} / []map[string]interface{}
// shapes Parse can return down to a flat list of parent match maps.
func flattenParents(t *testing.T, res interface{}) []map[string]interface{} {
	t.Helper()
	var out []map[string]interface{}
	var walk func(v interface{}, depth int)
	walk = func(v interface{}, depth int) {
		if depth > 6 {
			return
		}
		switch tv := v.(type) {
		case []interface{}:
			for _, item := range tv {
				walk(item, depth+1)
			}
		case []map[string]interface{}:
			for _, item := range tv {
				walk(item, depth+1)
			}
		case map[string]interface{}:
			if _, ok := tv["asn"]; ok {
				out = append(out, tv)
				return
			}
			// Not a parent match itself -- descend through the group-name
			// wrapper levels. Keys are visited in sorted order so the
			// collected order is deterministic.
			keys := make([]string, 0, len(tv))
			for k := range tv {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(tv[k], depth+1)
			}
		}
	}
	walk(res, 0)
	return out
}

// neighborAddrs pulls the neighbor addresses out of whatever shape the nested
// group produced (a single map when there is one neighbor, a list otherwise).
func neighborAddrs(v interface{}) []string {
	var out []string
	switch tv := v.(type) {
	case map[string]interface{}:
		if n, ok := tv["neighbor"].(string); ok {
			out = append(out, n)
		}
	case []interface{}:
		for _, item := range tv {
			out = append(out, neighborAddrs(item)...)
		}
	case []map[string]interface{}:
		for _, item := range tv {
			out = append(out, neighborAddrs(item)...)
		}
	}
	return out
}
