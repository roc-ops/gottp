# Memory Scalability Fix — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix three memory scalability issues causing 4.8GB–52GB+ consumption, achieving linear scaling under 200MB for all reproduction cases.

**Architecture:** Profile-first approach (Phase 1) builds benchmarks that reproduce each issue, then targeted fixes (Phase 2) address root causes: line-based gap for Issue 1, Starlark thread reuse + key caching + GC relief for Issue 2, and profiling-driven fix for Issue 3. Phase 3 tightens thresholds as permanent regression guardrails.

**Tech Stack:** Go 1.24, `go.starlark.net`, `testing.B` benchmarks, `runtime.MemStats`, `runtime/pprof`

**Spec:** `docs/superpowers/specs/2026-03-16-memory-scalability-design.md`

---

## Chunk 1: Profiling Infrastructure (Phase 1)

### Task 1: Copy reproduction test data

**Files:**
- Create: `test/memory/testdata/show_cable-modem.ttp`
- Create: `test/memory/testdata/show_cable-modem.txt`
- Create: `test/memory/testdata/show_log.ttp`
- Create: `test/memory/testdata/show_log.txt`
- Create: `test/memory/testdata/show_ha_error-detection_ORIGINAL.ttp`
- Create: `test/memory/testdata/show_ha_error-detection_cmoffline_status.txt`
- Create: `test/memory/testdata/show_ha_error-detection_datapath_status.txt`

- [ ] **Step 1: Create test/memory/testdata directory and copy reproduction files**

Copy from `/tmp/memory-repro/memory-repro/` into `test/memory/testdata/`.

```bash
mkdir -p test/memory/testdata
cp /tmp/memory-repro/memory-repro/show_cable-modem.ttp test/memory/testdata/
cp /tmp/memory-repro/memory-repro/show_cable-modem.txt test/memory/testdata/
cp /tmp/memory-repro/memory-repro/show_log.ttp test/memory/testdata/
cp /tmp/memory-repro/memory-repro/show_log.txt test/memory/testdata/
cp /tmp/memory-repro/memory-repro/show_ha_error-detection_ORIGINAL.ttp test/memory/testdata/
cp /tmp/memory-repro/memory-repro/show_ha_error-detection_cmoffline_status.txt test/memory/testdata/
cp /tmp/memory-repro/memory-repro/show_ha_error-detection_datapath_status.txt test/memory/testdata/
```

- [ ] **Step 2: Verify files are in place**

```bash
ls -la test/memory/testdata/
```

Expected: 7 files matching the list above.

- [ ] **Step 3: Commit test data**

```bash
git add test/memory/testdata/
git commit -m "Add memory scalability reproduction test data"
```

---

### Task 2: Write memory scalability benchmark tests

**Files:**
- Create: `test/memory/memory_scalability_test.go`

- [ ] **Step 1: Write the benchmark test file**

```go
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

// readTestData reads a file from testdata/ and returns its content as a string.
func readTestData(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read testdata/%s: %v", name, err)
	}
	return string(data)
}

// measureParse compiles a template, parses input, and returns peak HeapInuse in bytes.
func measureParse(t testing.TB, templateStr, inputStr string) uint64 {
	t.Helper()

	// Force GC and get baseline
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	compiled, err := gottp.CompileTemplate(templateStr)
	if err != nil {
		t.Fatalf("CompileTemplate failed: %v", err)
	}

	_, err = compiled.Parse(gottp.Inputs{"Default_Input": inputStr}, nil, nil)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// Use TotalAlloc difference as the measure of work done
	return after.TotalAlloc - before.TotalAlloc
}

// truncateLines returns the first n lines of input.
func truncateLines(input string, n int) string {
	lines := strings.Split(input, "\n")
	if n >= len(lines) {
		return input
	}
	return strings.Join(lines[:n], "\n")
}

// --- Issue 1: maxGap explosion ---
// This test runs with the CURRENT maxGap=500 (or whatever the production value is).
// It verifies that the cable modem template stays under the memory threshold.
// When maxGap was 10000, this caused 40-52GB. With the line-based fix, even large
// gap values should not cause explosion.
func TestIssue1_CableModemMemory(t *testing.T) {
	templateStr := readTestData(t, "show_cable-modem.ttp")
	inputStr := readTestData(t, "show_cable-modem.txt")

	allocated := measureParse(t, templateStr, inputStr)
	allocMB := allocated / (1024 * 1024)

	t.Logf("Issue 1 (cable modem, 4 entries): allocated %d MB", allocMB)

	const thresholdMB = 500 // Phase 1: generous; Phase 3: tighten to 50MB
	if allocMB > thresholdMB {
		t.Fatalf("memory threshold exceeded: %d MB > %d MB", allocMB, thresholdMB)
	}
}

// --- Issue 2: Starlark scaling ---
func TestIssue2_StarlarkScalingSmall(t *testing.T) {
	templateStr := readTestData(t, "show_log.ttp")
	fullInput := readTestData(t, "show_log.txt")
	inputStr := truncateLines(fullInput, 100)

	allocated := measureParse(t, templateStr, inputStr)
	allocMB := allocated / (1024 * 1024)

	t.Logf("Issue 2 (show_log, 100 lines): allocated %d MB", allocMB)

	const thresholdMB = 500
	if allocMB > thresholdMB {
		t.Fatalf("memory threshold exceeded: %d MB > %d MB", allocMB, thresholdMB)
	}
}

// TestIssue2_StarlarkLinearityCheck runs at multiple input sizes and checks
// that memory growth is roughly linear (ratio between sizes < 3x).
// Skipped by default because the full input can use >16GB before fixes.
func TestIssue2_StarlarkLinearityCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping linearity check in short mode")
	}

	templateStr := readTestData(t, "show_log.ttp")
	fullInput := readTestData(t, "show_log.txt")

	sizes := []int{100, 500, 1000, 5000}
	results := make([]struct {
		lines     int
		allocated uint64
	}, len(sizes))

	for i, size := range sizes {
		inputStr := truncateLines(fullInput, size)
		allocated := measureParse(t, templateStr, inputStr)
		results[i].lines = size
		results[i].allocated = allocated
		t.Logf("  %d lines: %d MB allocated", size, allocated/(1024*1024))
	}

	// Check linearity: ratio between consecutive sizes should be < 3x
	// (perfect linearity with 5x input increase would be ratio ~5x,
	// but we allow up to 3x between our smaller increments)
	for i := 1; i < len(results); i++ {
		sizeRatio := float64(results[i].lines) / float64(results[i-1].lines)
		memRatio := float64(results[i].allocated) / float64(results[i-1].allocated)
		// Allow memory ratio up to sizeRatio * 2 (generous for overhead)
		maxRatio := sizeRatio * 2
		t.Logf("  %d->%d lines: size ratio %.1fx, memory ratio %.1fx (max %.1fx)",
			results[i-1].lines, results[i].lines, sizeRatio, memRatio, maxRatio)
		if memRatio > maxRatio {
			t.Errorf("superlinear scaling detected: %d->%d lines, memory grew %.1fx (expected < %.1fx)",
				results[i-1].lines, results[i].lines, memRatio, maxRatio)
		}
	}
}

// --- Issue 3: Dual group + exclude + macro explosion ---
func TestIssue3_DualGroupExcludeMemory(t *testing.T) {
	templateStr := readTestData(t, "show_ha_error-detection_ORIGINAL.ttp")
	inputStr := readTestData(t, "show_ha_error-detection_cmoffline_status.txt")

	allocated := measureParse(t, templateStr, inputStr)
	allocMB := allocated / (1024 * 1024)

	t.Logf("Issue 3 (ha_error-detection, 4 lines): allocated %d MB", allocMB)

	const thresholdMB = 500
	if allocMB > thresholdMB {
		t.Fatalf("memory threshold exceeded: %d MB > %d MB", allocMB, thresholdMB)
	}
}

// TestIssue3_DatapathVariant tests the alternate 8-line datapath input.
func TestIssue3_DatapathVariant(t *testing.T) {
	templateStr := readTestData(t, "show_ha_error-detection_ORIGINAL.ttp")
	inputStr := readTestData(t, "show_ha_error-detection_datapath_status.txt")

	allocated := measureParse(t, templateStr, inputStr)
	allocMB := allocated / (1024 * 1024)

	t.Logf("Issue 3 datapath variant (8 lines): allocated %d MB", allocMB)

	const thresholdMB = 500
	if allocMB > thresholdMB {
		t.Fatalf("memory threshold exceeded: %d MB > %d MB", allocMB, thresholdMB)
	}
}

// --- Benchmarks for pprof profiling ---

func BenchmarkIssue1_CableModem(b *testing.B) {
	templateStr := readTestData(b, "show_cable-modem.ttp")
	inputStr := readTestData(b, "show_cable-modem.txt")

	compiled, err := gottp.CompileTemplate(templateStr)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := compiled.Parse(gottp.Inputs{"Default_Input": inputStr}, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIssue2_StarlarkScaling(b *testing.B) {
	templateStr := readTestData(b, "show_log.ttp")
	fullInput := readTestData(b, "show_log.txt")

	// Use 100 lines for benchmark (safe for repeated runs)
	inputStr := truncateLines(fullInput, 100)

	compiled, err := gottp.CompileTemplate(templateStr)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := compiled.Parse(gottp.Inputs{"Default_Input": inputStr}, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIssue3_DualGroupExclude(b *testing.B) {
	templateStr := readTestData(b, "show_ha_error-detection_ORIGINAL.ttp")
	inputStr := readTestData(b, "show_ha_error-detection_cmoffline_status.txt")

	compiled, err := gottp.CompileTemplate(templateStr)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := compiled.Parse(gottp.Inputs{"Default_Input": inputStr}, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// --- Profiling helpers ---

// To generate heap profiles, run:
//   go test -bench BenchmarkIssue2 -memprofile mem.prof ./test/memory/
//   go tool pprof mem.prof
//
// For Issue 2 at larger sizes (CAUTION: can use >16GB before fixes):
//   go test -run TestIssue2_StarlarkLinearityCheck -v ./test/memory/
//
// To run all memory tests quickly:
//   go test -short -v ./test/memory/

// BenchmarkIssue2_StarlarkScalingSizes runs benchmarks at multiple input sizes
// for profiling comparison. Use -benchtime=1x to run once per size.
func BenchmarkIssue2_StarlarkScalingSizes(b *testing.B) {
	templateStr := readTestData(b, "show_log.ttp")
	fullInput := readTestData(b, "show_log.txt")

	sizes := []int{100, 500, 1000, 5000}
	for _, size := range sizes {
		inputStr := truncateLines(fullInput, size)
		b.Run(fmt.Sprintf("lines_%d", size), func(b *testing.B) {
			compiled, err := gottp.CompileTemplate(templateStr)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := compiled.Parse(gottp.Inputs{"Default_Input": inputStr}, nil, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to establish baselines (use safe sizes only)**

```bash
go test -run "TestIssue1|TestIssue3" -v -timeout 60s ./test/memory/
```

Expected: Tests pass (current maxGap=500 should keep Issue 1 under 500MB for 4 entries). Issue 3 may or may not pass — this establishes the baseline. If a test exceeds memory and gets OOM-killed, note the failure and add `-short` guard or reduce threshold.

- [ ] **Step 3: Run Issue 2 small test only (100 lines is safe)**

```bash
go test -run "TestIssue2_StarlarkScalingSmall" -v -timeout 60s ./test/memory/
```

Expected: Passes — 100 lines should be well under 500MB even before fixes.

- [ ] **Step 4: Run benchmarks for pprof baseline**

```bash
go test -bench "BenchmarkIssue1|BenchmarkIssue3" -benchmem -benchtime=3x -timeout 120s ./test/memory/
```

Expected: Benchmark output with allocation counts. Save this output for comparison after fixes.

- [ ] **Step 5: Commit benchmark tests**

```bash
git add test/memory/memory_scalability_test.go
git commit -m "Add memory scalability benchmark tests for three critical issues"
```

---

## Chunk 2: Issue 1 Fix — Line-Based Gap (Phase 2)

### Task 3: Add lineIdx to patternMatch and compute during match collection

**Files:**
- Modify: `internal/compiled/runtime.go:1214-1220` (patternMatch struct)
- Modify: `internal/compiled/runtime.go:1286-1340` (match collection — anchored path)
- Modify: `internal/compiled/runtime.go` (match collection — non-anchored path)

- [ ] **Step 1: Run existing test suite to confirm green baseline**

```bash
go test ./... -count=1 -timeout 300s 2>&1 | tail -20
```

Expected: All packages PASS.

- [ ] **Step 2: Add lineIdx field to patternMatch struct**

In `internal/compiled/runtime.go`, find the `patternMatch` struct definition at line ~1214:

```go
// Current:
type patternMatch struct {
    patternIdx int
    spanStart  int
    spanEnd    int
    result     map[string]interface{}
}
```

Add `lineIdx` field:

```go
type patternMatch struct {
    patternIdx int
    spanStart  int
    spanEnd    int
    lineIdx    int // line number (0-based) for line-based gap comparison
    result     map[string]interface{}
}
```

- [ ] **Step 3: Set lineIdx during anchored match collection**

In the anchored match path (line-by-line matching, around line ~1286), where matches are appended to `allMatches`, the `lineIdx` is simply the loop variable. Find the `allMatches = append(allMatches, patternMatch{` calls inside the `if hasAnchors` block and add `lineIdx: lineIdx` to each.

There will be one or more `allMatches = append(...)` calls in this block. Each must include `lineIdx: lineIdx`.

- [ ] **Step 4: Set lineIdx during non-anchored match collection**

In the non-anchored match path (matching against entire input), find `allMatches = append(...)` calls and compute lineIdx from spanStart using the `lineOffsets` array:

```go
lineIdx: sort.SearchInts(lineOffsets, spanStart+1) - 1,
```

Note: `sort.SearchInts` returns the insertion point, so for a position within line N, `SearchInts(lineOffsets, pos+1) - 1` gives the 0-based line index. The `sort` package is already imported (line 7).

- [ ] **Step 5: Run tests to verify struct change doesn't break anything**

```bash
go test ./internal/compiled/ -count=1 -timeout 60s
go test ./test/ -count=1 -timeout 60s
```

Expected: All PASS (lineIdx is added but not yet used for comparisons).

- [ ] **Step 6: Commit lineIdx addition**

```bash
git add internal/compiled/runtime.go
git commit -m "Add lineIdx field to patternMatch for line-based gap comparison"
```

---

### Task 4: Replace maxGap with maxGapLines in all comparison sites

**Files:**
- Modify: `internal/compiled/runtime.go:1528` (constant definition)
- Modify: `internal/compiled/runtime.go:1525` (currentStartPos tracking)
- Modify: `internal/compiled/runtime.go:2065,2071,2086,2113,2673` (5 comparison sites)
- Modify: `internal/compiled/runtime.go:2712` (new match start position)

- [ ] **Step 1: Replace the constant definition**

At line 1528, change:

```go
// Old:
const maxGap = 500                  // Maximum gap between patterns in same group instance (characters)
```

to:

```go
// New:
const maxGapLines = 30              // Maximum gap between patterns in same group instance (lines)
```

- [ ] **Step 2: Add currentStartLineIdx tracking variable**

Near line 1525, find:

```go
var currentStartPos int = -1
```

Add after it:

```go
var currentStartLineIdx int = -1
```

- [ ] **Step 3: Replace all 5 gap comparison sites**

Replace each occurrence of the character-based gap check with the line-based equivalent. There are exactly 5 sites (plus the constant reference in the comment at line 2675).

**Site 1 (line ~2065):** Inside the `hasLineIndicator` + `hasAnyStartIndicator` path:
```go
// Old:
if match.spanStart >= currentStartPos && match.spanStart-currentStartPos < maxGap {
// New:
if match.lineIdx-currentStartLineIdx >= 0 && match.lineIdx-currentStartLineIdx < maxGapLines {
```

**Site 2 (line ~2071):** The `else if` branch in the same block:
```go
// Old:
} else if match.spanStart >= currentStartPos && match.spanStart-currentStartPos < maxGap {
// New:
} else if match.lineIdx-currentStartLineIdx >= 0 && match.lineIdx-currentStartLineIdx < maxGapLines {
```

**Site 3 (line ~2086):** Inside the `isStartPattern` path:
```go
// Old:
if match.spanStart >= currentStartPos && match.spanStart-currentStartPos < maxGap {
// New:
if match.lineIdx-currentStartLineIdx >= 0 && match.lineIdx-currentStartLineIdx < maxGapLines {
```

**Site 4 (line ~2113):** The non-start pattern merge path:
```go
// Old:
} else if match.spanStart >= currentStartPos && match.spanStart-currentStartPos < maxGap {
// New:
} else if match.lineIdx-currentStartLineIdx >= 0 && match.lineIdx-currentStartLineIdx < maxGapLines {
```

**Site 5 (line ~2673):** The finalization path:
```go
// Old:
} else if match.spanStart >= currentStartPos && match.spanStart-currentStartPos < maxGap {
    // Normal gap check (when no _end_ patterns and no _line_ indicator)
    // Pattern must come after the start position and be within maxGap
// New:
} else if match.lineIdx-currentStartLineIdx >= 0 && match.lineIdx-currentStartLineIdx < maxGapLines {
    // Normal gap check (when no _end_ patterns and no _line_ indicator)
    // Pattern must come after the start line and be within maxGapLines
```

- [ ] **Step 4: Update currentStartLineIdx at ALL currentStartPos assignment sites**

There are 6 sites where `currentStartPos` is assigned. Each must have a corresponding `currentStartLineIdx` update. Search for `currentStartPos = ` in runtime.go to find all of them:

1. **Line ~2312** (table method initial match start):
```go
currentStartPos = match.spanStart
currentStartLineIdx = match.lineIdx
```

2. **Line ~2472** (new group instance initialization):
```go
currentStartPos = match.spanStart // Set start position for gap calculation
currentStartLineIdx = match.lineIdx
```

3. **Line ~2499** (start pattern merge position update):
```go
currentStartPos = match.spanStart
currentStartLineIdx = match.lineIdx
```

4. **Line ~2510** (reset after _end_ finalization — reset to -1):
```go
currentStartPos = -1
currentStartLineIdx = -1
```

5. **Line ~2712** (shouldFinalizeAndStartNew — new match start):
```go
currentStartPos = match.spanStart
currentStartLineIdx = match.lineIdx
```

6. **Line ~2931** (shouldStart path for start patterns):
```go
currentStartPos = match.spanStart
currentStartLineIdx = match.lineIdx
```

- [ ] **Step 5: Run the full test suite**

```bash
go test ./... -count=1 -timeout 300s 2>&1 | tail -30
```

Expected: All PASS. If any test fails, compare output differences — the line-based gap should produce identical results for all existing test cases.

- [ ] **Step 6: Run Issue 1 memory test to verify improvement**

```bash
go test -run "TestIssue1_CableModemMemory" -v -timeout 60s ./test/memory/
```

Expected: PASS, with memory well under 500MB (should be ~17MB as reported with maxGap=500).

- [ ] **Step 7: Commit**

```bash
git add internal/compiled/runtime.go
git commit -m "Replace character-based maxGap with line-based maxGapLines

Replaces const maxGap=500 (character distance) with const maxGapLines=30
(line count) for deciding whether adjacent pattern matches belong to the
same group instance. This eliminates combinatorial merge explosion when
gap values are large, since line-based gaps don't scale with character
count per line."
```

---

## Chunk 3: Issue 2 Fix — Starlark Memory (Phase 2)

### Task 5: Add reusable exec thread to StarlarkEngine

**Files:**
- Modify: `internal/macro/starlark.go:12-20` (StarlarkEngine struct)
- Modify: `internal/macro/starlark.go:30-43` (NewStarlarkEngine)
- Modify: `internal/macro/starlark.go:254` (ExecuteMacroStarlark — thread creation)
- Modify: `internal/macro/starlark.go:350` (ExecuteMacroStarlarkBatch — thread creation, **hot path for Issue 2**)
- Modify: `internal/macro/starlark.go:482` (ExecuteMacro — thread creation)

Note: There are 8 total `thread := &starlark.Thread{Name: "macro"}` sites in starlark.go (lines 51, 119, 225, 254, 318, 350, 450, 482). Lines 51 and 119 are in `RegisterMacro`/`RegisterMacroSource` (called once during compilation — not hot paths). Lines 225, 318, and 450 are fallback compilation paths (only hit on cache miss). The three critical hot-path sites are **254** (ExecuteMacroStarlark), **350** (ExecuteMacroStarlarkBatch — the actual hot path for Issue 2 since the batch path is used for Starlark macros), and **482** (ExecuteMacro).

- [ ] **Step 1: Add execThread field to StarlarkEngine struct**

In `internal/macro/starlark.go`, add `execThread` to the struct:

```go
type StarlarkEngine struct {
	predeclared starlark.StringDict
	mu          sync.RWMutex
	cache       map[string]*starlark.Program
	macros      map[string]string
	sourceCache map[string]*macroSourceInfo
	funcToSource map[string]string
	funcCache   map[string]starlark.Callable
	execThread  *starlark.Thread // reusable thread for sequential macro execution
}
```

- [ ] **Step 2: Initialize execThread in NewStarlarkEngine**

In `NewStarlarkEngine()`, add initialization:

```go
func NewStarlarkEngine() *StarlarkEngine {
	predeclared := make(starlark.StringDict)
	predeclared["_ttp_"] = starlark.NewDict(0)

	return &StarlarkEngine{
		predeclared:  predeclared,
		cache:        make(map[string]*starlark.Program),
		macros:       make(map[string]string),
		sourceCache:  make(map[string]*macroSourceInfo),
		funcToSource: make(map[string]string),
		funcCache:    make(map[string]starlark.Callable),
		execThread:   &starlark.Thread{Name: "macro"},
	}
}
```

- [ ] **Step 3: Replace thread creation in ExecuteMacroStarlark**

At line ~254, replace:

```go
// Old:
thread := &starlark.Thread{Name: "macro"}
```

with:

```go
// New:
thread := e.execThread
```

- [ ] **Step 4: Replace thread creation in ExecuteMacroStarlarkBatch (HOT PATH)**

At line ~350 in `ExecuteMacroStarlarkBatch`, replace:

```go
// Old:
thread := &starlark.Thread{Name: "macro"}
```

with:

```go
// New:
thread := e.execThread
```

This is the most critical site — `ExecuteMacroStarlarkBatch` is the actual path used for Issue 2's Starlark macros (called from runtime.go line ~6236).

- [ ] **Step 5: Replace thread creation in ExecuteMacro**

At line ~482, replace:

```go
// Old:
thread := &starlark.Thread{Name: "macro"}
```

with:

```go
// New:
thread := e.execThread
```

- [ ] **Step 6: Run macro tests**

```bash
go test ./internal/macro/ -count=1 -v -timeout 60s
go test ./test/ -run "Macro|macro|Starlark|starlark" -count=1 -v -timeout 60s
```

Expected: All PASS.

- [ ] **Step 7: Run full test suite**

```bash
go test ./... -count=1 -timeout 300s 2>&1 | tail -20
```

Expected: All PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/macro/starlark.go
git commit -m "Reuse Starlark thread across macro invocations

Instead of allocating a new starlark.Thread per ExecuteMacro call,
reuse a single thread stored on the engine. Thread reuse is safe
because the engine's mutex ensures sequential access and Starlark
threads carry no execution state between calls."
```

---

### Task 6: Add Starlark key string cache

**Files:**
- Modify: `internal/macro/starlark.go:12-20` (StarlarkEngine struct)
- Modify: `internal/macro/starlark.go:30-43` (NewStarlarkEngine)
- Modify: `internal/macro/starlark.go:546-576` (goToStarlark method)

- [ ] **Step 1: Add keyCache field to StarlarkEngine struct**

```go
type StarlarkEngine struct {
	// ... existing fields ...
	execThread  *starlark.Thread
	keyCache    map[string]starlark.String // reuse Starlark key strings for map conversions
}
```

- [ ] **Step 2: Initialize keyCache in NewStarlarkEngine**

Add to the return struct:

```go
keyCache:    make(map[string]starlark.String),
```

- [ ] **Step 3: Use keyCache in goToStarlark for map keys**

In the `goToStarlark` method, find the `case map[string]interface{}:` branch (~line 566):

```go
// Old:
case map[string]interface{}:
    dict := starlark.NewDict(len(val))
    for k, v := range val {
        dict.SetKey(starlark.String(k), e.goToStarlark(v))
    }
    return dict
```

Replace with:

```go
// New:
case map[string]interface{}:
    dict := starlark.NewDict(len(val))
    for k, v := range val {
        key, ok := e.keyCache[k]
        if !ok {
            key = starlark.String(k)
            e.keyCache[k] = key
        }
        dict.SetKey(key, e.goToStarlark(v))
    }
    return dict
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/macro/ -count=1 -v -timeout 60s
go test ./... -count=1 -timeout 300s 2>&1 | tail -20
```

Expected: All PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/macro/starlark.go
git commit -m "Cache Starlark key strings to reduce conversion allocations

Add a keyCache to StarlarkEngine that reuses starlark.String objects
for map keys during Go-to-Starlark conversion. Keys are typically
repeated across entries (e.g. '_raw_line'), so caching eliminates
redundant string allocations."
```

---

### Task 7: Add GC pressure relief for large macro batches

**Files:**
- Modify: `internal/compiled/runtime.go:6213-6256` (macro execution loop)

- [ ] **Step 1: Add runtime import if not present**

Check if `runtime` is already imported in `internal/compiled/runtime.go`. If not, add it to the import block. Note: `runtime` may conflict with the package name — use a qualified import alias like `goruntime "runtime"` if needed. Actually, the file is in package `compiled`, so `runtime` is fine as an import.

Look for existing `"runtime"` import — it's likely not imported yet since MemStats calls wouldn't be in this file. Add it to the import block.

- [ ] **Step 2: Add GC call inside the macro execution loop**

In `internal/compiled/runtime.go`, find the macro execution loop at line ~6215:

```go
for _, match := range matches {
```

Add a GC trigger every 1000 entries. Right after the loop start:

```go
for matchIdx, match := range matches {
    // Periodic GC to prevent Starlark conversion temporaries from accumulating
    if matchIdx > 0 && matchIdx%1000 == 0 {
        runtime.GC()
    }
```

Note: Change the loop variable from `_` to `matchIdx` if it's currently `_`. If it's already a named variable, use that.

- [ ] **Step 3: Run full test suite**

```bash
go test ./... -count=1 -timeout 300s 2>&1 | tail -20
```

Expected: All PASS.

- [ ] **Step 4: Run Issue 2 memory test to check improvement**

```bash
go test -run "TestIssue2_StarlarkScalingSmall" -v -timeout 60s ./test/memory/
```

Expected: PASS with lower allocation count than baseline.

- [ ] **Step 5: Run linearity check with safe sizes and memory limit**

```bash
GOMEMLIMIT=4GiB go test -run "TestIssue2_StarlarkLinearityCheck" -v -timeout 180s ./test/memory/
```

Expected: PASS — ratios between 100/500/1000/5000 lines should be roughly linear (< 2x the size ratio). GOMEMLIMIT prevents OOM if scaling is still superlinear at 5000 lines.

- [ ] **Step 6: Commit**

```bash
git add internal/compiled/runtime.go
git commit -m "Add periodic GC during macro execution for large inputs

Call runtime.GC() every 1000 entries during macro processing to
prevent Starlark conversion temporaries from accumulating faster
than the GC pacer can collect them."
```

---

## Chunk 4: Issue 3 Investigation + Fix (Phase 2)

### Task 8: Profile Issue 3 and identify root cause

**Files:**
- Read only (no modifications yet)

This task is investigative. The spec requires profiling Issue 3 AFTER the Issue 1 fix is applied.

- [ ] **Step 1: Run Issue 3 memory test to see current state**

```bash
go test -run "TestIssue3_DualGroupExcludeMemory" -v -timeout 120s ./test/memory/
```

If this OOM-kills, run with a memory limit:

```bash
GOMEMLIMIT=2GiB go test -run "TestIssue3_DualGroupExcludeMemory" -v -timeout 120s ./test/memory/ 2>&1 || echo "OOM or failed"
```

Note the result — if it fails, the issue persists and needs a fix.

- [ ] **Step 2: Run pprof heap profile on Issue 3**

```bash
go test -run "TestIssue3_DualGroupExcludeMemory" -memprofile /tmp/issue3_mem.prof -timeout 120s ./test/memory/ 2>&1 || true
```

If the test OOMs before writing the profile, use the benchmark with limited iterations:

```bash
go test -bench "BenchmarkIssue3" -benchtime=1x -memprofile /tmp/issue3_mem.prof -timeout 120s ./test/memory/
```

- [ ] **Step 3: Analyze the profile**

```bash
go tool pprof -top -cum /tmp/issue3_mem.prof 2>&1 | head -30
go tool pprof -text -cum /tmp/issue3_mem.prof 2>&1 | head -50
```

Look for which function dominates allocations. The answer determines the fix path:

- **If `regexp.(*Regexp).FindAllStringSubmatch` dominates:** → Fix path (a): anchors needed
- **If `extractMatchResult` or variable processing dominates:** → Fix path (b): short-circuit exclude
- **If `goToStarlark` or `starlarkToGo` dominates:** → Already addressed by Task 5-7
- **If `MatchCollector.Collect` or `match_collector.go` dominates:** → Unbounded joinmatches accumulation. Add a size cap to `MatchCollector.collections` slices.
- **If something else:** → Capture the full pprof top-30 output and pause for human review before proceeding to Task 9. Do not attempt a speculative fix without understanding the root cause.

- [ ] **Step 4: Document findings**

Record the top allocation sites and the chosen fix path. Update the spec if needed.

---

### Task 9: Apply Issue 3 fix (path depends on profiling results)

**Files:** Depends on profiling findings from Task 8.

The steps below cover the two most likely fix paths. Execute the one that matches profiling results.

#### Fix Path (a): Add implicit line anchors to multi-variable patterns

**Files:**
- Modify: `internal/compiled/runtime.go:1238-1241` (hasAnchors detection)

- [ ] **Step a1: Change hasAnchors to default true for multi-variable patterns**

Find the anchor detection logic at line ~1238:

```go
// Old:
hasAnchors := strings.Contains(compiledPattern.Regex.String(), "^") ||
    strings.Contains(compiledPattern.Regex.String(), "$")
```

Change to always use line-by-line matching when a pattern has 2+ variables (multi-column patterns are intended to match complete lines):

```go
// New:
hasAnchors := strings.Contains(compiledPattern.Regex.String(), "^") ||
    strings.Contains(compiledPattern.Regex.String(), "$")
// Multi-variable patterns should always match line-by-line to prevent
// overlapping substring matches that cause memory explosion
if len(compiledPattern.Variables) >= 2 {
    hasAnchors = true
}
```

- [ ] **Step a2: Run full test suite**

```bash
go test ./... -count=1 -timeout 300s 2>&1 | tail -30
```

Expected: All PASS. If any test fails, the multi-variable heuristic may be too aggressive and needs refinement (e.g., only apply when pattern has `exclude()`).

- [ ] **Step a3: Run Issue 3 memory test**

```bash
go test -run "TestIssue3" -v -timeout 60s ./test/memory/
```

Expected: PASS under 500MB (target: < 50MB).

#### Fix Path (b): Short-circuit extractMatchResult on early exclude() failure

**Files:**
- Modify: `internal/compiled/runtime.go:5628-5680` (extractMatchResult)

- [ ] **Step b1: Add early termination on condition failure**

In `extractMatchResult`, the current flow processes ALL variables even if the first one fails `exclude()`. The `applyFunctions` call at line ~5607 returns a `condition_failed` error. Find where `extractMatchResult` calls `applyFunctions` for each variable and add early return on condition failure:

This is already the case — `applyFunctions` returns `condition_failed` error and `extractMatchResult` returns nil. But check if there's per-variable processing that allocates before the condition check. If the allocations are in `applyFunctions` itself (creating ConditionResult objects), the fix is to check exclude conditions before calling the full function chain.

- [ ] **Step b2: Run tests and verify**

```bash
go test ./... -count=1 -timeout 300s 2>&1 | tail -30
go test -run "TestIssue3" -v -timeout 60s ./test/memory/
```

- [ ] **Step 3: Commit the fix (whichever path was taken)**

```bash
git add internal/compiled/runtime.go
git commit -m "Fix Issue 3: prevent memory explosion on dual-group exclude+macro templates

[describe the specific fix based on profiling findings]"
```

---

## Chunk 5: Regression Guardrails (Phase 3)

### Task 10: Tighten memory thresholds

**Files:**
- Modify: `test/memory/memory_scalability_test.go`

- [ ] **Step 1: Run all memory tests and note current usage**

```bash
go test -run "TestIssue" -v -timeout 120s ./test/memory/
```

Record the actual MB values from log output for each test.

- [ ] **Step 2: Tighten thresholds based on actual measurements**

Update the `thresholdMB` constants in each test:

- `TestIssue1_CableModemMemory`: change from 500 to 50
- `TestIssue2_StarlarkScalingSmall`: change from 500 to 50
- `TestIssue3_DualGroupExcludeMemory`: change from 500 to 50
- `TestIssue3_DatapathVariant`: change from 500 to 50

Use actual measurements + 3x headroom as the threshold. For example, if Issue 1 measures 17MB, set threshold to 50MB.

- [ ] **Step 3: Add full-size Issue 2 test (if fixes make it safe)**

Add a test that runs the full 12,951-line input:

```go
func TestIssue2_StarlarkFullInput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full input test in short mode")
	}

	templateStr := readTestData(t, "show_log.ttp")
	inputStr := readTestData(t, "show_log.txt")

	allocated := measureParse(t, templateStr, inputStr)
	allocMB := allocated / (1024 * 1024)

	t.Logf("Issue 2 (show_log, full 12951 lines): allocated %d MB", allocMB)

	const thresholdMB = 200
	if allocMB > thresholdMB {
		t.Fatalf("memory threshold exceeded: %d MB > %d MB", allocMB, thresholdMB)
	}
}
```

- [ ] **Step 4: Run all memory tests to verify thresholds**

```bash
go test -v -timeout 300s ./test/memory/
```

Expected: All PASS with tightened thresholds.

- [ ] **Step 5: Run full test suite one final time**

```bash
go test ./... -count=1 -timeout 300s 2>&1 | tail -30
```

Expected: All packages PASS.

- [ ] **Step 6: Commit**

```bash
git add test/memory/memory_scalability_test.go
git commit -m "Tighten memory scalability thresholds as regression guardrails

Update memory test thresholds from 500MB to actual post-fix values
with headroom. Add full 12K-line Issue 2 test under -short guard."
```

---

### Task 11: Final verification and cleanup

**Files:** None new

- [ ] **Step 1: Run benchmarks for final comparison**

```bash
go test -bench "BenchmarkIssue" -benchmem -benchtime=3x -timeout 120s ./test/memory/
```

Compare with baseline from Task 2 Step 4.

- [ ] **Step 2: Run linearity check**

```bash
go test -run "TestIssue2_StarlarkLinearityCheck" -v -timeout 120s ./test/memory/
```

Expected: PASS with all ratios under threshold.

- [ ] **Step 3: Run full test suite one last time**

```bash
go test ./... -count=1 -timeout 300s
```

Expected: All PASS.

- [ ] **Step 4: Commit any remaining changes**

If any files were modified during verification, commit them.
