package streaming_test

import (
	"reflect"
	"testing"

	"github.com/roc-ops/gottp"
)

// A default() variable without a regex of its own is a deliberate gottp
// extension (#64): it uses .*? where Python TTP uses WORD (\S+), so a
// trailing {{ col | default("") }} makes the last column optional and may
// hold text with spaces. It doesn't capture the space before it, and a line
// made only of default() variables doesn't match blank lines (#162). Parse
// and ParseStream agree.
func TestDefaultExtension_ParseAndStream(t *testing.T) {
	cases := []struct {
		name, template, input string
		want                  []map[string]interface{}
	}{
		{
			"optional last column",
			`<group name="g*">
{{ port }} {{ state }} {{ descr | default("") }}
</group>`,
			"Gi1 up uplink\nGi2 down\nGi3 up core link\nGi4 up   \n",
			[]map[string]interface{}{
				{"port": "Gi1", "state": "up", "descr": "uplink"},
				{"port": "Gi2", "state": "down", "descr": ""},
				{"port": "Gi3", "state": "up", "descr": "core link"},
				{"port": "Gi4", "state": "up", "descr": ""},
			},
		},
		{
			"default-only line skips blank lines",
			`<group name="g*">
interface {{ name }}
{{ rest | default("none") }}
</group>`,
			"interface A\n\n   \ninterface B\n",
			[]map[string]interface{}{
				{"name": "A", "rest": "none"},
				{"name": "B", "rest": "none"},
			},
		},
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
			want, got := parseAndStream(t, c, map[string]string{"Default_Input": tc.input}, nil, nil)
			if !reflect.DeepEqual(want, tc.want) {
				t.Fatalf("Parse = %v, want %v", want, tc.want)
			}
			assertSameRecords(t, want, got)
		})
	}
}
