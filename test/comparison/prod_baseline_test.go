//go:build prodbaseline

package comparison

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

// Run with:
//
//	go test -tags prodbaseline -run TestProdBaseline -v -timeout 30m ./test/comparison/
//
// Captures parse time, allocations, and peak HeapInuse for the templates
// from issue #20 against the Vyve production captures in
// /tmp/vyve-prod-captures/.
//
// Class 1: streaming candidates (large output, parse rate proportional to size).
//          We expect high heap and high alloc count — the streaming target.
// Class 2: regex pathology (small input, pathological parse rate). We expect
//          LOW heap, LOW alloc count, but HIGH parse_time-per-byte. Confirms
//          the working hypothesis that streaming wouldn't help these — they
//          need template-side regex review.

const (
	casaChassisB851 = "/Users/jasonpatterson/DH360_Device_Discovery/data/hardware_platforms/casa-systems/casa-chassis/8.8.3.5_build_b851/field-mappings/templates"
	casaVccapC192   = "/Users/jasonpatterson/DH360_Device_Discovery/data/hardware_platforms/casa-systems/casa-vccap/10.8.1_build_c192/field-mappings/templates"
	prodCaptureRoot = "/tmp/vyve-prod-captures"
)

type prodCase struct {
	class    int // 1 = streaming candidate, 2 = regex pathology
	name     string
	template string // absolute path
	input    string // basename within prodCaptureRoot
}

var prodCases = []prodCase{
	// Class 1 — streaming candidates
	{1, "show_cable_modem_verbose", filepath.Join(casaChassisB851, "show_cable_modem_verbose.ttp"), "show_cable_modem_verbose.txt"},
	{1, "show_cable_modem_phy", filepath.Join(casaChassisB851, "show_cable_modem_phy.ttp"), "show_cable_modem_phy.txt"},
	{1, "show_iftable_detail", filepath.Join(casaChassisB851, "show_iftable_detail.ttp"), "show_iftable_detail.txt"},
	{1, "show_cable_modem_fec", filepath.Join(casaChassisB851, "show_cable_modem_fec.ttp"), "show_cable_modem_fec.txt"},
	// Class 2 — regex pathology
	{2, "show_cpuinfo", filepath.Join(casaChassisB851, "show_cpuinfo.ttp"), "show_cpuinfo.txt"},
	{2, "show_cable_modem", filepath.Join(casaChassisB851, "show_cable_modem.ttp"), "show_cable_modem.txt"},
	{2, "show_cable_downstream_channel_counter", filepath.Join(casaVccapC192, "show_cable_downstream_channel_counter.ttp"), "show_cable_downstream_channel_counter.txt"},
	{2, "show_upstream_signal-quality", filepath.Join(casaChassisB851, "show_upstream_signal-quality.ttp"), "show_upstream_signal-quality.txt"},
	{2, "show_cable_modem_bonding", filepath.Join(casaChassisB851, "show_cable_modem_bonding.ttp"), "show_cable_modem_bonding.txt"},
	{2, "show_upstream_fec", filepath.Join(casaChassisB851, "show_upstream_fec.ttp"), "show_upstream_fec.txt"},
	{2, "show_controller_upstream", filepath.Join(casaChassisB851, "show_controller_upstream.ttp"), "show_controller_upstream.txt"},
}

func TestProdBaseline(t *testing.T) {
	for _, tc := range prodCases {
		t.Run(tc.name, func(t *testing.T) {
			runProdBaseline(t, tc)
		})
	}
}

func runProdBaseline(t *testing.T, tc prodCase) {
	tmplBytes, err := os.ReadFile(tc.template)
	if err != nil {
		t.Fatalf("read template %s: %v", tc.template, err)
	}
	inputBytes, err := os.ReadFile(filepath.Join(prodCaptureRoot, tc.input))
	if err != nil {
		t.Fatalf("read input %s: %v", tc.input, err)
	}
	inputSize := len(inputBytes)
	inputStr := string(inputBytes)
	inputBytes = nil // release the byte slice; the string is what Parse uses

	compiled, err := gottp.CompileTemplate(string(tmplBytes))
	if err != nil {
		t.Fatalf("compile %s: %v", tc.template, err)
	}

	inputs := gottp.Inputs{"Default_Input": inputStr}

	// Settle the runtime so pre-parse stats reflect just our held data.
	runtime.GC()
	runtime.GC()
	time.Sleep(150 * time.Millisecond)

	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)

	var peakHeap uint64 = msBefore.HeapInuse
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		var ms runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				runtime.ReadMemStats(&ms)
				for {
					cur := atomic.LoadUint64(&peakHeap)
					if ms.HeapInuse <= cur {
						break
					}
					if atomic.CompareAndSwapUint64(&peakHeap, cur, ms.HeapInuse) {
						break
					}
				}
			}
		}
	}()

	start := time.Now()
	result, err := compiled.Parse(inputs, nil, nil)
	elapsed := time.Since(start)
	close(stop)
	<-done

	if err != nil {
		t.Fatalf("parse %s: %v", tc.name, err)
	}

	var msAfter runtime.MemStats
	runtime.ReadMemStats(&msAfter)

	// Force a GC and re-read to see what's actually retained vs. just held
	// pending the next GC.
	runtime.GC()
	runtime.GC()
	var msAfterGC runtime.MemStats
	runtime.ReadMemStats(&msAfterGC)

	allocs := msAfter.Mallocs - msBefore.Mallocs
	allocBytes := msAfter.TotalAlloc - msBefore.TotalAlloc
	peak := atomic.LoadUint64(&peakHeap)
	peakDelta := int64(peak) - int64(msBefore.HeapInuse)

	mb := func(n uint64) float64 { return float64(n) / 1024.0 / 1024.0 }
	mbi := func(n int64) float64 { return float64(n) / 1024.0 / 1024.0 }

	msPerKB := float64(elapsed.Milliseconds()) / (float64(inputSize) / 1024.0)

	fmt.Printf("\n=== [class %d] %s ===\n", tc.class, tc.name)
	fmt.Printf("  input          : %d bytes (%.2f MB)\n", inputSize, mb(uint64(inputSize)))
	fmt.Printf("  parse_time     : %v\n", elapsed)
	fmt.Printf("  ms_per_KB      : %.3f\n", msPerKB)
	fmt.Printf("  allocs         : %d\n", allocs)
	fmt.Printf("  alloc_bytes    : %.2f MB total (sum of every allocation, includes GC'd)\n", mb(allocBytes))
	fmt.Printf("  pre_parse_heap : %.2f MB\n", mb(msBefore.HeapInuse))
	fmt.Printf("  peak_heap      : %.2f MB (absolute, sampled @50ms)\n", mb(peak))
	fmt.Printf("  peak_delta     : %.2f MB (peak above pre-parse baseline)\n", mbi(peakDelta))
	fmt.Printf("  post_parse_heap: %.2f MB (no forced GC)\n", mb(msAfter.HeapInuse))
	fmt.Printf("  retained_heap  : %.2f MB (after forced GC)\n", mb(msAfterGC.HeapInuse))
	fmt.Println()

	// Touch result to keep it live across the measurement (so the GC can't
	// scavenge it before we read the post-parse stats).
	_ = result
}
