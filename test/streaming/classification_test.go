package streaming_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/roc-ops/gottp"
)

// dh360Root is the DH360_Device_Discovery checkout whose templates these
// tests read. Set GOTTP_DH360_ROOT to point at it; the tests skip when the
// templates aren't there.
func dh360Root() string {
	if root := os.Getenv("GOTTP_DH360_ROOT"); root != "" {
		return root
	}
	return "/Users/jasonpatterson/DH360_Device_Discovery"
}

const casaTemplates = "data/hardware_platforms/casa-systems/casa-chassis"

var class1Templates = []string{
	"8.8.3.5_build_b851/field-mappings/templates/show_cable_modem_verbose.ttp",
	"8.8.3.5_build_b851/field-mappings/templates/show_cable_modem_phy.ttp",
	"8.8.3.5_build_b851/field-mappings/templates/show_iftable_detail.ttp",
	"8.8.3.5_build_b851/field-mappings/templates/show_cable_modem_fec.ttp",
	// default() on the start line (a record of defaults when nothing
	// matches); rejected by the first #130 gate, streamable again since.
	"8.8.3.6_build_c4e8-a01/field-mappings/templates/show_cable_modem_cpe.ttp",
	"8.8.3.6_build_c4e8-a01/field-mappings/templates/show_cable_modem_summary.ttp",
}

func readDH360Template(t *testing.T, name string) *gottp.CompiledTemplate {
	t.Helper()
	path := filepath.Join(dh360Root(), casaTemplates, name)
	tmplBytes, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("template not available at %s (set GOTTP_DH360_ROOT): %v", path, err)
	}
	c, err := gottp.CompileTemplate(string(tmplBytes))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return c
}

func TestClass1TemplatesAreStreamable(t *testing.T) {
	for _, name := range class1Templates {
		t.Run(filepath.Base(name), func(t *testing.T) {
			c := readDH360Template(t, name)
			streamable, reasons := gottp.WhyNotStreamable(c)
			if !streamable {
				t.Fatalf("expected %s to be streamable; reasons: %v", name, reasons)
			}
			// On an empty input the stream gives what Parse gives (for a
			// start line with default(), one record of defaults).
			want, got := parseAndStream(t, c, map[string]string{"Default_Input": ""}, nil, nil)
			assertSameRecords(t, want, got)
		})
	}
}
