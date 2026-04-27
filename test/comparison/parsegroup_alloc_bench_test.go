package comparison

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/roc-ops/gottp"
)

// BenchmarkParseGroupCableModem exercises the parseGroup / extractMatchResult
// hot path against the cable_modem_input.txt fixture (~6235 rows). Tracks both
// time and allocations so we can compare before/after for perf changes
// targeting GC pressure (see issue #18).
func BenchmarkParseGroupCableModem(b *testing.B) {
	template := `<group name="show_cable_modem*">
{{mac-address | MAC}} {{ip-address | IP}}         {{us-intf}}      {{ds-intf}}     {{status}}     {{prim-sid}}    {{rx-power}}    {{timing-offset}}      {{num-cpes}}    {{bpi-enabled}}  {{rphy-node}}    {{mac-domain}}
</group>`

	root, err := getProjectRoot()
	if err != nil {
		b.Fatalf("getProjectRoot: %v", err)
	}
	fixture := filepath.Join(root, "test", "comparison", "fixtures", "cable_modem_input.txt")
	bytes, err := os.ReadFile(fixture)
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	data := string(bytes)

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		b.Fatalf("compile: %v", err)
	}

	inputs := gottp.Inputs{"Default_Input": data}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compiled.Parse(inputs, nil, nil); err != nil {
			b.Fatalf("parse: %v", err)
		}
	}
}
