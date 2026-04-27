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

// streamingMemoryLimits sets per-fixture peak heap-delta ceilings for
// ParseStream over the Class 1 production captures.
//
// Baseline context:
//   - show_cable_modem_phy: pre-streaming Parse peaked at 752 MB; streaming
//     measured at 23.74 MB peak_delta in C1 — limit set to 32 MB to absorb
//     transient burst between 50 ms GC samples.
//   - show_cable_modem_verbose: 46 MB input, ~80 fields/record, ~3× record
//     size of phy. 60 MB is a realistic ceiling; aspirational plan target
//     was 25 MB (pre-measurement guess).
//   - show_iftable_detail / show_cable_modem_fec: smaller inputs; 20 MB each.
var streamingMemoryLimits = []struct {
	name     string
	template string
	input    string
	limitMB  float64
}{
	// ~80 fields/record × ~64K records, 46 MB input
	{"show_cable_modem_verbose", "show_cable_modem_verbose.ttp", "show_cable_modem_verbose.txt", 60.0},
	// ~10 fields/record × ~256K records, 24 MB input — C1 baseline: 23.74 MB
	{"show_cable_modem_phy", "show_cable_modem_phy.ttp", "show_cable_modem_phy.txt", 32.0},
	// smaller mixed fixture
	{"show_iftable_detail", "show_iftable_detail.ttp", "show_iftable_detail.txt", 20.0},
	// 3.2 MB input, smaller record count
	{"show_cable_modem_fec", "show_cable_modem_fec.ttp", "show_cable_modem_fec.txt", 20.0},
}

func TestParseStream_MemoryBound_All(t *testing.T) {
	for _, lc := range streamingMemoryLimits {
		lc := lc // capture for t.Run closure
		t.Run(lc.name, func(t *testing.T) {
			tmpl, err := os.ReadFile(filepath.Join(casaChassisB851, lc.template))
			if err != nil {
				t.Fatal(err)
			}
			input, err := os.ReadFile(filepath.Join(prodCaptureRoot, lc.input))
			if err != nil {
				t.Fatal(err)
			}
			c, err := gottp.CompileTemplate(string(tmpl))
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			inputs := gottp.Inputs{"Default_Input": string(input)}

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

			count := 0
			err = c.ParseStream(inputs, nil, nil,
				func(m map[string]interface{}, sr [2]int, gp string) error {
					count++
					return nil
				})
			close(stop)
			<-done
			if err != nil {
				t.Fatalf("ParseStream: %v", err)
			}

			peak := atomic.LoadUint64(&peakHeap)
			delta := int64(peak) - int64(msBefore.HeapInuse)
			deltaMB := float64(delta) / 1024.0 / 1024.0
			fmt.Printf("ParseStream %s: %d records, peak_delta=%.2f MB (limit=%.0f MB)\n",
				lc.name, count, deltaMB, lc.limitMB)

			if deltaMB > lc.limitMB {
				t.Errorf("peak heap delta %.2f MB exceeds limit %.2f MB", deltaMB, lc.limitMB)
			}
		})
	}
}
