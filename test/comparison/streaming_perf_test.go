//go:build prodbaseline

package comparison

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

func TestParseStream_NoRegression_Verbose(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join(casaChassisB851, "show_cable_modem_verbose.ttp"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile(filepath.Join(prodCaptureRoot, "show_cable_modem_verbose.txt"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := gottp.CompileTemplate(string(tmpl))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	inputs := gottp.Inputs{"Default_Input": string(input)}

	c.Parse(inputs, nil, nil)

	t0 := time.Now()
	_, err = c.Parse(inputs, nil, nil)
	parseTime := time.Since(t0)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	t0 = time.Now()
	count := 0
	err = c.ParseStream(inputs, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			count++
			return nil
		})
	streamTime := time.Since(t0)
	if err != nil {
		t.Fatalf("ParseStream: %v", err)
	}

	ratio := float64(streamTime) / float64(parseTime)
	fmt.Printf("Parse=%v ParseStream=%v ratio=%.2f\n", parseTime, streamTime, ratio)

	const maxRatio = 1.10
	if ratio > maxRatio {
		t.Errorf("ParseStream is %.2fx slower than Parse (limit %.2fx)", ratio, maxRatio)
	}
}
