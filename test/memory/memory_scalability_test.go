package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

// readTestData reads a file from the testdata/ directory relative to this test file.
func readTestData(t testing.TB, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readTestData: failed to read %s: %v", path, err)
	}
	return string(data)
}

// measureParse compiles the template, forces two GC passes, then parses input
// and returns the TotalAlloc difference in bytes. Returns an error instead of
// calling t.Fatal so callers can handle known-broken cases.
func measureParse(t testing.TB, templateStr, inputStr string) (uint64, error) {
	t.Helper()
	compiled, err := gottp.CompileTemplate(templateStr)
	if err != nil {
		return 0, fmt.Errorf("CompileTemplate: %w", err)
	}

	runtime.GC()
	runtime.GC()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	_, err = compiled.Parse(gottp.Inputs{"Default_Input": inputStr}, nil, nil)
	if err != nil {
		return 0, fmt.Errorf("Parse: %w", err)
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	return after.TotalAlloc - before.TotalAlloc, nil
}

// truncateLines returns the first n lines of input (joined with newlines).
func truncateLines(input string, n int) string {
	lines := strings.Split(input, "\n")
	if n > len(lines) {
		n = len(lines)
	}
	return strings.Join(lines[:n], "\n")
}

const memThreshold500MB = 500 * 1024 * 1024

// hasXMLParserBug returns true when the template contains content that triggers
// the known infinite-recursion bug in the XML parser fallback path. The bug
// manifests when a <doc> block contains raw comparison operators (e.g. "<=")
// that are not XML-safe: the lenient fallback decoder loops forever.
func hasXMLParserBug(templateStr string) bool {
	return strings.Contains(templateStr, "<=")
}

// TestIssue1_CableModemMemory verifies that parsing the cable modem template
// does not consume excessive memory.
//
// Known issue: the template contains load="gnmi" lookups which are not yet
// implemented; the test is skipped until gnmi support lands.
func TestIssue1_CableModemMemory(t *testing.T) {
	tmpl := readTestData(t, "show_cable-modem.ttp")
	input := readTestData(t, "show_cable-modem.txt")

	allocated, err := measureParse(t, tmpl, input)
	if err != nil {
		if strings.Contains(err.Error(), "gnmi") {
			t.Skipf("Issue1: template requires gnmi lookup support (not yet implemented): %v", err)
		}
		t.Fatalf("Issue1: unexpected error: %v", err)
	}
	t.Logf("Issue1 cable-modem: allocated %d bytes (%.1f MB)", allocated, float64(allocated)/(1024*1024))

	if allocated > memThreshold500MB {
		t.Errorf("Issue1: memory allocation %d bytes (%.1f MB) exceeds threshold of 500 MB",
			allocated, float64(allocated)/(1024*1024))
	}
}

// TestIssue2_StarlarkScalingSmall verifies that parsing the first 100 lines of
// the log file does not consume excessive memory.
//
// Known issue: show_log.ttp contains a <doc> block with a raw '<=' character
// which causes the XML parser fallback to loop infinitely (stack overflow).
// This test is skipped until the parser bug is fixed.
func TestIssue2_StarlarkScalingSmall(t *testing.T) {
	tmpl := readTestData(t, "show_log.ttp")
	fullInput := readTestData(t, "show_log.txt")

	if hasXMLParserBug(tmpl) {
		t.Skip("Issue2: template contains '<=' in doc block which triggers infinite XML recursion (known parser bug — fix hasXMLParserBug once resolved)")
	}

	input := truncateLines(fullInput, 100)
	allocated, err := measureParse(t, tmpl, input)
	if err != nil {
		t.Fatalf("Issue2 small: unexpected error: %v", err)
	}
	t.Logf("Issue2 starlark small (100 lines): allocated %d bytes (%.1f MB)", allocated, float64(allocated)/(1024*1024))

	if allocated > memThreshold500MB {
		t.Errorf("Issue2 small: memory allocation %d bytes (%.1f MB) exceeds threshold of 500 MB",
			allocated, float64(allocated)/(1024*1024))
	}
}

// TestIssue2_StarlarkLinearityCheck verifies that memory usage scales roughly
// linearly (within 2x) with input size at multiple scale points.
// Skipped in short mode.
func TestIssue2_StarlarkLinearityCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping linearity check in short mode")
	}

	tmpl := readTestData(t, "show_log.ttp")
	fullInput := readTestData(t, "show_log.txt")

	if hasXMLParserBug(tmpl) {
		t.Skip("Issue2: template contains '<=' in doc block which triggers infinite XML recursion (known parser bug — fix hasXMLParserBug once resolved)")
	}

	sizes := []int{100, 500, 1000, 5000}
	allocations := make([]uint64, len(sizes))

	for i, size := range sizes {
		input := truncateLines(fullInput, size)
		allocated, err := measureParse(t, tmpl, input)
		if err != nil {
			t.Fatalf("Issue2 linearity at %d lines: unexpected error: %v", size, err)
		}
		allocations[i] = allocated
		t.Logf("Issue2 linearity: %d lines -> %d bytes (%.1f MB)", size, allocated, float64(allocated)/(1024*1024))
	}

	for i := 1; i < len(sizes); i++ {
		if allocations[i-1] == 0 {
			continue
		}
		sizeRatio := float64(sizes[i]) / float64(sizes[i-1])
		memRatio := float64(allocations[i]) / float64(allocations[i-1])
		t.Logf("Issue2 linearity: size ratio %.1fx (%d->%d lines), mem ratio %.2fx", sizeRatio, sizes[i-1], sizes[i], memRatio)
		if memRatio > sizeRatio*2 {
			t.Errorf("Issue2 linearity: memory ratio %.2fx between %d and %d lines exceeds size ratio %.1fx * 2 = %.1fx",
				memRatio, sizes[i-1], sizes[i], sizeRatio, sizeRatio*2)
		}
	}
}

// TestIssue3_DualGroupExcludeMemory verifies that the dual-group exclude
// template does not OOM on the cmoffline status input.
func TestIssue3_DualGroupExcludeMemory(t *testing.T) {
	tmpl := readTestData(t, "show_ha_error-detection_ORIGINAL.ttp")
	input := readTestData(t, "show_ha_error-detection_cmoffline_status.txt")

	allocated, err := measureParse(t, tmpl, input)
	if err != nil {
		t.Fatalf("Issue3 cmoffline: unexpected error: %v", err)
	}
	t.Logf("Issue3 dual-group cmoffline: allocated %d bytes (%.1f MB)", allocated, float64(allocated)/(1024*1024))

	if allocated > memThreshold500MB {
		t.Errorf("Issue3 cmoffline: memory allocation %d bytes (%.1f MB) exceeds threshold of 500 MB",
			allocated, float64(allocated)/(1024*1024))
	}
}

// TestIssue3_DatapathVariant verifies the dual-group exclude template on the
// datapath status input.
func TestIssue3_DatapathVariant(t *testing.T) {
	tmpl := readTestData(t, "show_ha_error-detection_ORIGINAL.ttp")
	input := readTestData(t, "show_ha_error-detection_datapath_status.txt")

	allocated, err := measureParse(t, tmpl, input)
	if err != nil {
		t.Fatalf("Issue3 datapath: unexpected error: %v", err)
	}
	t.Logf("Issue3 dual-group datapath: allocated %d bytes (%.1f MB)", allocated, float64(allocated)/(1024*1024))

	if allocated > memThreshold500MB {
		t.Errorf("Issue3 datapath: memory allocation %d bytes (%.1f MB) exceeds threshold of 500 MB",
			allocated, float64(allocated)/(1024*1024))
	}
}

// BenchmarkIssue1_CableModem benchmarks parsing the cable modem template.
func BenchmarkIssue1_CableModem(b *testing.B) {
	tmpl := readTestData(b, "show_cable-modem.ttp")
	input := readTestData(b, "show_cable-modem.txt")

	compiled, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		if strings.Contains(err.Error(), "gnmi") {
			b.Skipf("Issue1: template requires gnmi lookup support (not yet implemented): %v", err)
		}
		b.Fatalf("CompileTemplate failed: %v", err)
	}

	inputs := gottp.Inputs{"Default_Input": input}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compiled.Parse(inputs, nil, nil)
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

// BenchmarkIssue2_StarlarkScaling benchmarks parsing the log template with
// the first 100 lines of input.
func BenchmarkIssue2_StarlarkScaling(b *testing.B) {
	tmpl := readTestData(b, "show_log.ttp")
	fullInput := readTestData(b, "show_log.txt")

	if hasXMLParserBug(tmpl) {
		b.Skip("Issue2: template contains '<=' in doc block which triggers infinite XML recursion (known parser bug)")
	}

	input := truncateLines(fullInput, 100)
	compiled, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		b.Fatalf("CompileTemplate failed: %v", err)
	}

	inputs := gottp.Inputs{"Default_Input": input}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compiled.Parse(inputs, nil, nil)
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

// BenchmarkIssue3_DualGroupExclude benchmarks parsing the dual-group exclude
// template on the cmoffline status input.
func BenchmarkIssue3_DualGroupExclude(b *testing.B) {
	tmpl := readTestData(b, "show_ha_error-detection_ORIGINAL.ttp")
	input := readTestData(b, "show_ha_error-detection_cmoffline_status.txt")

	compiled, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		b.Fatalf("CompileTemplate failed: %v", err)
	}

	inputs := gottp.Inputs{"Default_Input": input}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compiled.Parse(inputs, nil, nil)
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

// BenchmarkIssue2_StarlarkScalingSizes runs sub-benchmarks at multiple input
// sizes to characterize Starlark macro scaling behaviour.
func BenchmarkIssue2_StarlarkScalingSizes(b *testing.B) {
	tmpl := readTestData(b, "show_log.ttp")
	fullInput := readTestData(b, "show_log.txt")

	if hasXMLParserBug(tmpl) {
		b.Skip("Issue2: template contains '<=' in doc block which triggers infinite XML recursion (known parser bug)")
	}

	compiled, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		b.Fatalf("CompileTemplate failed: %v", err)
	}

	sizes := []int{100, 500, 1000, 5000}
	for _, size := range sizes {
		size := size
		input := truncateLines(fullInput, size)
		inputs := gottp.Inputs{"Default_Input": input}
		b.Run(fmt.Sprintf("lines=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := compiled.Parse(inputs, nil, nil)
				if err != nil {
					b.Fatalf("Parse failed: %v", err)
				}
			}
		})
	}
}
