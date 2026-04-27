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

func TestParseStream_MemoryBound_Phy(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join(casaChassisB851, "show_cable_modem_phy.ttp"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.ReadFile(filepath.Join(prodCaptureRoot, "show_cable_modem_phy.txt"))
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
	fmt.Printf("ParseStream show_cable_modem_phy: %d records, peak_delta=%.2f MB\n", count, deltaMB)

	// Pre-streaming Parse baseline on this fixture: 752 MB peak. Live working
	// set in streaming mode is well under 20 MB; the limit here is set above
	// that to absorb transient regex/map garbage between 50 ms GC sweep
	// samples (the burst, not the steady state).
	const limitMB = 32.0
	if deltaMB > limitMB {
		t.Errorf("peak heap delta %.2f MB exceeds limit %.2f MB", deltaMB, limitMB)
	}
}
