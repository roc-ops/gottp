package comparison

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// The benchmarks in this file parse with ParseOptions{EnableSourceMap: true},
// the path the editor takes when its source-map toggle is on. Each one mirrors
// a source-maps-off benchmark next to it, so the two can be compared:
//
//	go test ./test/comparison -run='^$' -bench='CableModem|GoTTPParse$|ParentScaling20k|DynamicPathGroup|SourceMap' -count=6

func benchSourceMap(b *testing.B, template, data string) {
	b.Helper()
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		b.Fatalf("compile: %v", err)
	}
	opts := &gottp.ParseOptions{EnableSourceMap: true}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res, err := compiled.ParseWithValidation(gottp.Inputs{"Default_Input": data}, nil, opts)
		if err != nil {
			b.Fatalf("parse: %v", err)
		}
		if res.SourceMap == nil {
			b.Fatal("no source map")
		}
	}
}

// BenchmarkParseGroupCableModemSourceMap is BenchmarkParseGroupCableModem
// with source maps on.
func BenchmarkParseGroupCableModemSourceMap(b *testing.B) {
	template := `<group name="show_cable_modem*">
{{mac-address | MAC}} {{ip-address | IP}}         {{us-intf}}      {{ds-intf}}     {{status}}     {{prim-sid}}    {{rx-power}}    {{timing-offset}}      {{num-cpes}}    {{bpi-enabled}}  {{rphy-node}}    {{mac-domain}}
</group>`
	root, err := getProjectRoot()
	if err != nil {
		b.Fatalf("getProjectRoot: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "test", "comparison", "fixtures", "cable_modem_input.txt"))
	if err != nil {
		b.Fatalf("read fixture: %v", err)
	}
	benchSourceMap(b, template, string(data))
}

// BenchmarkGoTTPParseSourceMap is BenchmarkGoTTPParse with source maps on.
func BenchmarkGoTTPParseSourceMap(b *testing.B) {
	template := `<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
 description {{ description | default("") }}
</group>`
	data := `interface Loopback0
 ip address 192.168.0.1/24
 description Management interface
interface Vlan100
 ip address 10.0.0.1/24
 description User VLAN
interface GigabitEthernet0/0
 ip address 172.16.0.1/24
interface GigabitEthernet0/1
 ip address 172.16.1.1/24
`
	benchSourceMap(b, template, data)
}

// BenchmarkParentScaling20kSourceMap is BenchmarkParentScaling20k with
// source maps on.
func BenchmarkParentScaling20kSourceMap(b *testing.B) {
	template := `<group name="cm*">
{{mac}} {{ip}} {{us}} {{ds}} {{status}}
</group>`
	var sb strings.Builder
	for i := 0; i < 20000; i++ {
		fmt.Fprintf(&sb, "0000.1111.%04x 10.0.0.%d us%d ds%d online\n", i%65536, i%250, i%8, i%8)
	}
	benchSourceMap(b, template, sb.String())
}

// BenchmarkNestedGroupsSourceMap covers nested groups, whose matches the
// source map records too. BenchmarkNestedGroups is its source-maps-off twin.
func BenchmarkNestedGroupsSourceMap(b *testing.B) {
	benchSourceMap(b, nestedBenchTemplate, nestedBenchInput(500))
}

// BenchmarkNestedGroups is BenchmarkNestedGroupsSourceMap with source maps off.
func BenchmarkNestedGroups(b *testing.B) {
	compiled, err := gottp.CompileTemplate(nestedBenchTemplate)
	if err != nil {
		b.Fatalf("compile: %v", err)
	}
	data := nestedBenchInput(500)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := compiled.Parse(gottp.Inputs{"Default_Input": data}, nil, nil); err != nil {
			b.Fatalf("parse: %v", err)
		}
	}
}

const nestedBenchTemplate = `<group name="interfaces*">
interface {{ name }}
 description {{ description | ORPHRASE }}
 <group name="addresses*">
 ip address {{ ip }} {{ mask }}
 </group>
!{{ _end_ }}
</group>`

func nestedBenchInput(n int) string {
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "interface Gi0/%d\n description link %d\n ip address 10.%d.0.1 255.255.255.0\n ip address 10.%d.1.1 255.255.255.0\n!\n", i, i, i%250, i%250)
	}
	return sb.String()
}
