package streaming_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/roc-ops/gottp"
)

const class1TemplateRoot = "/Users/jasonpatterson/DH360_Device_Discovery/data/hardware_platforms/casa-systems/casa-chassis/8.8.3.5_build_b851/field-mappings/templates"

var class1Templates = []string{
	"show_cable_modem_verbose.ttp",
	"show_cable_modem_phy.ttp",
	"show_iftable_detail.ttp",
	"show_cable_modem_fec.ttp",
}

func TestClass1TemplatesAreStreamable(t *testing.T) {
	for _, name := range class1Templates {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(class1TemplateRoot, name)
			tmplBytes, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("template not available at %s: %v", path, err)
			}
			c, err := gottp.CompileTemplate(string(tmplBytes))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			streamable, reasons := gottp.WhyNotStreamable(c)
			if !streamable {
				t.Errorf("expected %s to be streamable; reasons: %v", name, reasons)
			}
		})
	}
}
