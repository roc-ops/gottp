# Memory Scalability Fix — Design Spec

**Date:** 2026-03-16
**Status:** Approved
**Approach:** C — Profile + Architectural Guardrails

## Summary

Three memory issues cause catastrophic memory consumption (4.8GB–52GB+) under specific conditions in the goTTP runtime. This design addresses all three through profiling-driven targeted fixes plus permanent regression guardrails.

**Constraints:**
- Output must be bit-identical for existing templates (minor tolerance on Issue 1 grouping if more correct)
- Python TTP output parity remains the goal
- Target: linear memory scaling, ideally under 100MB for all reproduction cases

## Issue Overview

| Issue | Trigger | Input | Peak Memory | Root Cause |
|---|---|---|---|---|
| 1 | `maxGap = 10000` | 4 CM entries (636B) | 40–52GB | Character-based gap causes combinatorial merge explosion |
| 2 | Starlark macro on large input | 12,951 syslog lines (2MB) | 16GB+ | Per-entry Go/Starlark conversion + no GC relief |
| 3 | Dual groups + exclude() + Starlark | 4 lines (316B) | 4.8GB+ | Suspected regex overlapping matches or exclude() overhead (profiling required) |

## Phase 1: Profiling Infrastructure

**New file:** `test/memory/memory_scalability_test.go`

Three benchmark tests using the downstream user's reproduction files (stored in `test/memory/testdata/`):

- `BenchmarkIssue1_MaxGapExplosion` — cable modem template, 4 entries
- `BenchmarkIssue2_StarlarkScaling` — show_log template at 100/500/1000/5000/12951 lines
- `BenchmarkIssue3_DualGroupExclude` — show_ha_error-detection template, 4 lines

Each benchmark:
1. Runs `CompileTemplate` then `Parse` on the reproduction input
2. Records `runtime.MemStats.TotalAlloc` and `HeapInuse` before/after
3. Fails if peak memory exceeds threshold (initially 500MB, tightened after fixes)
4. Supports `-memprofile` flag for `pprof` heap profile dumps

Issue 2 also runs at multiple input sizes to confirm whether scaling is linear or superlinear.

These are `testing.B` benchmarks so `go test -bench -benchmem` gives allocation counts for free.

## Phase 2: Targeted Fixes

### Issue 1: Line-Based Gap Replacement

**File:** `internal/compiled/runtime.go`

**Current:** `const maxGap = 500` — character distance between pattern match positions determines whether adjacent matches belong to the same group instance.

**Change:** Replace with `const maxGapLines = 30` — line-based gap counting.

**Mechanics:**
- Pre-compute a position-to-line lookup using the existing `lineOffsets` array (line 1227)
- All gap comparisons (`match.spanStart - currentStartPos < maxGap`) become `lineOf(match.spanStart) - lineOf(currentStartPos) < maxGapLines`
- Affects 5 comparison sites at lines ~2065, 2071, 2086, 2113, and the finalization path

**Why 30 lines:** Most CLI record blocks are under 30 lines. The old 500-char limit was ~8-10 lines for typical 60-char-wide CLI output. 30 lines is generous enough for multi-line records but prevents cross-record merging.

**Output parity:** Identical for most templates (500 chars ~ 8-10 lines < 30). The only change: records where a single long line (300+ chars) pushed the next field past 500 chars but stayed within 30 lines — this produces *more correct* grouping.

**Why this eliminates the explosion:** With character-based gaps, `maxGap=10000` meant nearly every match position in a small input was "within gap" of every other position, creating combinatorial merge combinations. Line-based gaps don't have this property — `maxGapLines=10000` on a 4-entry input still only has ~30 lines, so the gap check behaves identically to `maxGapLines=30`.

### Issue 2: Starlark Execution Memory

**Files:** `internal/macro/starlark.go`, `internal/compiled/runtime.go`

Three layers of fix:

#### Layer 1: Thread Reuse

`ExecuteMacroStarlark` (line 254) creates a `new starlark.Thread` on every call. Add a reusable thread to `StarlarkEngine`:

```go
type StarlarkEngine struct {
    // ...existing fields...
    execThread *starlark.Thread  // reusable for sequential macro execution
}
```

Initialize once in `NewStarlarkEngine()`, reuse in `ExecuteMacroStarlark` and `ExecuteMacro`. Starlark threads are lightweight state containers — reuse is safe for sequential execution.

#### Layer 2: Reduce Conversion Churn

The batch path (lines 6218-6256) calls `GoToStarlark`/`StarlarkToGo` per entry. For macros where keys are the same every time (e.g., `_raw_line`), add a key string cache:

```go
type StarlarkEngine struct {
    // ...existing fields...
    keyCache map[string]starlark.String  // reuse Starlark key strings
}
```

In `goToStarlark` for `map[string]interface{}`, look up keys in cache before allocating. Cuts string allocations ~50% (keys reused, only values fresh).

#### Layer 3: GC Pressure Relief

After every N entries (e.g., 1000), call `runtime.GC()` to force collection of previous batch's Starlark temporaries. The Go GC pacer falls behind when allocation rate is high and the `result` slice retains all final maps (high liveness ratio).

**Expected impact:** Thread reuse + key caching reduces per-entry allocation ~40-60%. GC relief prevents runaway accumulation. 12K entries should stay under 200MB.

### Issue 3: Dual Group + Exclude + Macro Explosion

**Files:** TBD based on profiling results

**Investigation plan:** Run the 4-line reproduction case under `pprof` to identify whether allocations are in:

- **(a) Regex engine** — `FindAllStringSubmatch` producing overlapping matches on multi-column patterns without anchors
- **(b) `extractMatchResult`/`applyFunctions`** — exclude() creating intermediate `ConditionResult` allocations even for rejected matches
- **(c) Starlark conversion** — same as Issue 2
- **(d) Something else**

**Fix plan by finding:**

**(a) Regex overlapping matches:** Add implicit `^`/`$` anchors to multi-variable patterns that don't already have them. Multi-variable patterns (5+ capture groups) are intended to match complete lines, not substrings. Validate against full test suite. Alternative: force line-by-line matching when first variable has `exclude()`.

**(b) Exclude overhead:** Short-circuit `extractMatchResult` — if the first variable's `exclude()` rejects, skip processing remaining variables entirely.

**(c) Starlark:** Addressed by Issue 2 fixes (Layers 1-3).

**(d) Unknown:** Profiling data will determine the fix.

**Key principle:** Phase 1 profiling must complete before committing to a specific Issue 3 fix.

## Phase 3: Regression Guardrails

The Phase 1 benchmarks become permanent regression tests with tightened post-fix thresholds:

| Test Case | Input | Threshold |
|---|---|---|
| Issue 1: maxGap stress | cable-modem.txt, 4 entries | < 50MB |
| Issue 2: Starlark scaling | show_log.txt, 12,951 lines | < 200MB |
| Issue 2: linearity check | show_log.txt at 100/500/1000/5000 lines | ratio < 3x between sizes |
| Issue 3: dual group + exclude | ha_error-detection, 4 lines | < 50MB |

The **linearity check** is the most important guardrail — it catches superlinear regressions even if absolute memory stays under threshold.

**Test structure:**
- `go test` runs fast subset (100-line inputs, < 50MB assertions)
- `go test -bench` runs full benchmarks with `runtime.MemStats` assertions
- `-memprofile` flag available for future debugging
- No CI pipeline changes needed — standard Go test files

## Reproduction Files

Stored in `test/memory/testdata/` (from downstream user's reproduction zip):

| File | Purpose |
|---|---|
| `show_cable-modem.ttp` | Issue 1 template |
| `show_cable-modem.txt` | Issue 1 input (4 CM entries) |
| `show_log.ttp` | Issue 2 template (Starlark macro) |
| `show_log.txt` | Issue 2 input (12,951 syslog lines) |
| `show_ha_error-detection_ORIGINAL.ttp` | Issue 3 template |
| `show_ha_error-detection_cmoffline_status.txt` | Issue 3 input (4 lines) |
| `show_ha_error-detection_datapath_status.txt` | Issue 3 alt input (8 lines) |

## Risk Assessment

| Change | Risk | Mitigation |
|---|---|---|
| Line-based gap (Issue 1) | Low-medium: grouping heuristic changes | Full test suite + output comparison |
| Thread reuse (Issue 2) | Low: sequential execution, no concurrency concern | Existing thread safety tests |
| Key caching (Issue 2) | Very low: read-only cache of immutable strings | Unit test cache correctness |
| GC calls (Issue 2) | Very low: correctness-preserving, only affects timing | Benchmark confirms improvement |
| Issue 3 fix | TBD: depends on profiling findings | Profiling-first approach minimizes risk |
