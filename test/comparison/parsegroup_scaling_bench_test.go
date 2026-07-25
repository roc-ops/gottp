package comparison

import (
	"fmt"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// benchParentScaling parses a flat group over `rows` lines. Every line produces
// one parent match, so this benchmark is dominated by parseGroup's per-parent
// range computation.
//
// That computation used to rescan the whole allMatches slice once per parent
// match, making it O(parents x matches): on production inputs (~5.6 MB, ~100K
// matches) it was ~17% of total CPU and up to ~50% in a single 30s window.
// Run the three sizes together and check that ns/op grows roughly linearly
// with rows -- superlinear growth means the quadratic rescan is back.
//
//	go test ./test/comparison -run='^$' -bench=BenchmarkParentScaling -benchtime=3x
func benchParentScaling(b *testing.B, rows int) {
	template := `<group name="cm*">
{{mac}} {{ip}} {{us}} {{ds}} {{status}}
</group>`

	var sb strings.Builder
	for i := 0; i < rows; i++ {
		fmt.Fprintf(&sb, "0000.1111.%04x 10.0.0.%d us%d ds%d online\n", i%65536, i%250, i%8, i%8)
	}

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		b.Fatalf("compile: %v", err)
	}
	inputs := gottp.Inputs{"Default_Input": sb.String()}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compiled.Parse(inputs, nil, nil); err != nil {
			b.Fatalf("parse: %v", err)
		}
	}
}

func BenchmarkParentScaling5k(b *testing.B)  { benchParentScaling(b, 5000) }
func BenchmarkParentScaling20k(b *testing.B) { benchParentScaling(b, 20000) }
func BenchmarkParentScaling80k(b *testing.B) { benchParentScaling(b, 80000) }

// BenchmarkDynamicPathGroup exercises PathResolver.ResolvePath, which runs once
// per match when a group name contains a `{{ var }}` placeholder. It used to
// compile two regexes per call (the placeholder scanner plus one per
// substitution), which showed up as ~7% of all allocations in production
// profiles.
func BenchmarkDynamicPathGroup(b *testing.B) {
	template := `<group name="devices.{{ host }}.interfaces.{{ intf }}">
{{ host }} {{ intf }} {{ status }}
</group>`

	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&sb, "router%d Gi0/%d up\n", i, i%48)
	}

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		b.Fatalf("compile: %v", err)
	}
	inputs := gottp.Inputs{"Default_Input": sb.String()}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compiled.Parse(inputs, nil, nil); err != nil {
			b.Fatalf("parse: %v", err)
		}
	}
}
