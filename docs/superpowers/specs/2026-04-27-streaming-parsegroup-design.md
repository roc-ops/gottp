# Streaming `parseGroup` mode (`ParseStream`)

**Date:** 2026-04-27
**Related issues:** roc-ops/gottp#18 (closed by v0.1.10), roc-ops/gottp#20 (open — streaming candidates + Class 2 regex pathology)
**DH side:** roc-ops/DH360-Device-Discovery#44

## Background

Discovery's production captures from a Vyve Casa C100G CMTS (~9000 modems, 4 Class 1 streaming candidate templates) showed peak parse-time heap up to 831 MB on a 24 MB input due to per-record allocation churn. The v0.1.10 work cut transient allocations by ~15%, but the core problem persists: the result struct itself is held in memory until parse completes, dragging GC behind the collector and blocking DH's downstream pipeline from running concurrently with parse.

The streamable templates have a clean repeating record boundary already encoded in their structure (`_start_` indicators or fully line-anchored patterns). The runtime currently ignores that boundary and accumulates all matches before merging. This design adds a streaming variant that drives the same merge state machine record-by-record and emits each completed record via a callback, dropping intermediates between records.

## Goals & success criteria

**Primary goal.** Bound peak heap usage when parsing large CLI outputs from streamable templates, by emitting one record at a time via a callback instead of accumulating the full result in memory.

**Success criteria for v1:**

1. **Functional.** `compiled.ParseStream(inputs, vars, options, callback)` invokes the callback once per top-level-group match for streamable templates, with `(match map[string]interface{}, srcRange [2]int, groupPath string)`. Strict auto-detect; non-streamable templates return an error naming the reason without invoking the callback.
2. **Behavioral parity.** For every Class 1 fixture, the multiset of records emitted via `ParseStream` equals the multiset produced by `Parse` (sorted on key fields, since order is not guaranteed). Verified by automated parity tests.
3. **Memory.** Peak `HeapInuse` during `ParseStream` on `show_cable_modem_phy.txt` (24 MB / ~256K records, currently 778 MB peak) drops to within ~2× of single-record size — concretely **<20 MB peak above pre-parse baseline**, vs the current 752 MB peak delta.
4. **Throughput.** Parse time for streamable templates does not regress more than 10% vs current `Parse`. We expect parity or slight improvement from removed buffering, but explicitly bound the regression risk.
5. **Diagnostic.** `gottp.WhyNotStreamable(compiled)` correctly classifies all 4 Class 1 fixtures as streamable and returns coherent reasons for any non-streamable template (`joinmatches`, nested groups, no record boundary, aggregating group function).

**Non-goal for v1.** End-to-end DH pipeline measurement. That's deploy + DH sync + observe-next-prod-profile (Phase D), not part of the gottp implementation scope.

## Approaches considered

**Approach 1 (chosen).** Separate `ParseStream` API + dedicated streaming `parseGroup` variant. New API, runtime variant, strict auto-detect gates entry, existing `Parse` untouched. Clear surface, isolated diff, one template at a time migration on DH's side.

**Approach 2 (rejected).** Internal streaming under existing `Parse`. `Parse` would internally route streamable templates through bounded code, but still return the full result. Less code duplication but invasive to existing `Parse`, and DH said they explicitly want to opt in per template rather than have streaming happen invisibly. Also doesn't deliver the peak-heap win — DH still gets a fully-built result struct at the end.

**Approach 3 (rejected).** Iterator/channel pattern (`ParseIter`). DH explicitly asked for callback (their pipeline is `func handleMatch(...) error { ... }` shape, not iterator). Iterators using `iter.Pull` for cancellation-via-error add ceremony without benefit for the consumer.

## Architecture & components

### File-level changes

1. **`internal/pattern/engine.go`** — already has `HasJoinMatches` cached on both `CompiledPattern` and `MatchVariable` from the v0.1.10 work; the streamability check has the signal it needs. No new fields at the pattern layer.

2. **`internal/compiler/compiled.go`** — additive fields on `CompiledGroup`:
   - `Streamable bool` — set during compile after analysis
   - `NonStreamableReasons []string` — empty if streamable, else human-readable explanations
   - `NormalizedPath string` — group's `Name` with trailing `*` stripped, used as `groupPath` in callback

   And on `CompiledTemplate`:
   - `Streamable bool` — true iff every top-level group is streamable

3. **`internal/compiler/streamability.go`** (new) — `analyzeStreamability(g *CompiledGroup)` runs the strict check; `validateGroupPathCollisions(t *CompiledTemplate) error` runs the `foo` vs `foo*` collision check at compile time.

4. **`internal/compiled/runtime.go`** — new method `(r *Runtime) ParseStream(...)`. Internally calls a new package-level helper `parseGroupStream(group, inputData, vars, callback)` that mirrors the structure of the existing `parseGroup` but emits each completed record via callback instead of appending to `allMatches`. The merge-state-machine (the `currentMatch` / `_start_` / `containsall` filter logic at runtime.go ~2200-2300) is factored out into a small helper used by both `parseGroup` and `parseGroupStream`, so behavior is shared.

5. **`gottp` package (top-level)** — thin wrapper `(c *CompiledTemplate) ParseStream(...)` calls `Runtime.ParseStream`. Plus `WhyNotStreamable(c *CompiledTemplate) (streamable bool, reasons []string)` for the diagnostic.

6. **`test/comparison/`** — new `streaming_parity_test.go` runs each Class 1 fixture through both `Parse` and `ParseStream`, sorts emitted records by key fields, asserts equality.

### What deliberately does NOT change

`Parse`, `ParseWithSourceMap`, the existing returners, output formatters, lookups, macros (semantics — implementation may need engine-reuse verification per the open items), YANG validation. Streaming reads from the same compiled template; the only new compile-time work is the streamability analysis pass.

### Implicit invariants

- `Streamable` and `NonStreamableReasons` are computed once at compile time, never mutated at parse time.
- `groupPath` collision check fires at compile, not at `ParseStream` call — affects all templates including those used only with `Parse`, since the collision is an authoring bug regardless of which API is used.
- Macros and lookups continue to work in streaming mode (verified by parity tests).

## Data flow

Call path for a streaming parse:

```
compiled.ParseStream(inputs, vars, options, callback)
  │
  ├─► precondition: if !compiled.Streamable → return wrapped error
  │   listing each non-streamable top-level group with its reason(s).
  │   No partial work performed; callback never invoked.
  │
  ├─► r := compiled.NewRuntime()
  │
  ├─► r.processInputFunctions(inputs)   [existing — input functions, lookups, macros init]
  │
  └─► for each top-level group, in template order:
        for each input the group is bound to:
          parseGroupStream(r, group, inputData, vars, callback)
              │
              ├─► precompute lineOffsets (one pass, already hoisted)
              │
              ├─► for each pattern in group.Patterns (sequential):
              │     │
              │     ├─► run pattern over input (line-anchored line-by-line, or
              │     │   non-anchored FindAllStringSubmatchIndex)
              │     │
              │     └─► for each match:
              │          ├─ feed match into the merge state machine
              │          │  (currentMatch / _start_ boundary tracking — shared
              │          │   helper between parseGroup and parseGroupStream)
              │          │
              │          └─ if state machine yields a completed record:
              │               ├─ apply group macro (Starlark/Go)
              │               ├─ apply group filter (containsall, etc.)
              │               ├─ if record passes filter:
              │               │   compute srcRange = [firstMatchedLineStart,
              │               │                       lastMatchedLineEnd)
              │               │   err := callback(record, srcRange, group.NormalizedPath)
              │               │   if err != nil → return err immediately,
              │               │     no further callbacks, no buffered work to flush
              │               └─ drop record reference (no buffering)
              │
              └─► at end-of-input: flush any in-flight in-progress record through
                  the same path (final _start_-bounded block, or last
                  non-anchored match, depending on shape)
```

### Key data-structural difference from `parseGroup`

`parseGroup` accumulates `[]patternMatch` (every match across every pattern), then walks that slice through the merge state machine to build `[]map[string]interface{}` (the records list). `parseGroupStream` doesn't accumulate either — it drives the same merge state machine record-by-record and emits each completed record before producing the next match. Peak heap is bounded by `(in-flight match buffer of one record's matches) + (one completed record)`, plus whatever the callback chooses to retain (the caller's responsibility).

### Cross-record state preservation

- **`record()` function:** persists vars across records. Streaming processes records in the same order `Parse` does (sequential by group, sequential within a group), so any `record()` updates from earlier records are visible to later records exactly as today.
- **Lookups:** tables load once at parse start. Per-record `lookup(...)` invocations work identically.
- **Macros:** applied per record at extract time. Engine reuse verified during Phase A; if currently per-record alloc, fixed in Phase B.

### `srcRange` computation

- **Line-anchored single-line records** (modem-phy, fec): `[lineOffset[i], lineOffset[i] + len(line))`
- **`_start_`-bounded multi-line records** (verbose, iftable): `[lineOffset[firstMatchedLine], lineOffset[lastMatchedLine] + len(lastLine))` — half-open span covering every line that contributed to the record

Half-open by Go convention.

### What stays buffered (intentionally)

- `lineOffsets` slice — needed for srcRange and pattern dispatch, O(input lines), ~bytes per line. For the 46 MB verbose input that's ~1 MB. Acceptable.
- The full input string — `Parse` already requires it as a string in memory. Streaming-the-input is a separate, larger redesign; out of scope for v1.

## API surface

### Public additions (`gottp` top-level package)

```go
// ParseStream invokes fn for each record produced by the streamable
// groups in this template, dropping intermediate state between records
// to bound peak heap usage.
//
// Returns *TemplateNotStreamableError (also matches gottp.ErrTemplateNotStreamable
// via errors.Is) if any top-level group fails the streamability check;
// in that case fn is never invoked.
//
// Calling order:
//   - groups are processed in template definition order
//   - within a group, matches in input scan order
// No cross-group ordering guarantee is made.
//
// If fn returns a non-nil error, parsing aborts immediately: that error
// is wrapped and returned; no further fn invocations occur; no buffered
// work is flushed. Already-emitted records remain with the caller.
func (c *CompiledTemplate) ParseStream(
    inputs Inputs,
    vars map[string]interface{},
    options *ParseOptions,
    fn func(match map[string]interface{}, srcRange [2]int, groupPath string) error,
) error

// WhyNotStreamable reports whether the template is streamable; if not,
// returns one human-readable reason per non-streamable top-level group.
// Useful for template-readiness audits without round-tripping through
// ParseStream + error inspection.
func WhyNotStreamable(c *CompiledTemplate) (streamable bool, reasons []string)
```

### Error types

```go
var ErrTemplateNotStreamable = errors.New("template is not streamable")

type TemplateNotStreamableError struct {
    Reasons []string // one human-readable string per non-streamable group
}
func (e *TemplateNotStreamableError) Error() string  // "template is not streamable: <reasons joined by '; '>"
func (e *TemplateNotStreamableError) Is(target error) bool { return target == ErrTemplateNotStreamable }
func (e *TemplateNotStreamableError) Unwrap() error  { return ErrTemplateNotStreamable }

// Returned by CompileTemplate when two groups normalize to the same path.
type GroupPathCollisionError struct {
    NormalizedPath string
    GroupNames     []string // the literal Name values that collide
}
func (e *GroupPathCollisionError) Error() string  // "group path collision: 2 groups normalize to "foo": foo, foo*"
```

DH can either match the error generically with `errors.Is(err, gottp.ErrTemplateNotStreamable)` or assert to `*TemplateNotStreamableError` to inspect `.Reasons`.

### Compiler-internal additions (`internal/compiler/compiled.go`)

```go
type CompiledGroup struct {
    // ... existing fields ...

    Streamable           bool     // passed the strict streamability check
    NonStreamableReasons []string // empty iff Streamable
    NormalizedPath       string   // Name with trailing "*" stripped
}

type CompiledTemplate struct {
    // ... existing fields ...
    Streamable bool // every top-level group is streamable
}
```

## Streamability rule (formal)

A `CompiledGroup` is `Streamable == true` iff *all* of:

1. `!g.IsNested` (top-level only in v1)
2. `len(g.Groups) == 0` (no nested children in v1)
3. For every pattern `p` in `g.Patterns`, for every variable `v` in `p.Variables`: `!v.HasJoinMatches`
4. Has a record boundary, i.e. *either*:
   a. some variable has `Name == "_start_"` *or* its `Functions` contain literal `"_start_"`, *or*
   b. every pattern in `g.Patterns` has `p.HasAnchors == true`
5. The group's `Functions` attribute parses to only per-record group functions. **Allowlist** (safer than denylist for unknown functions): `containsall`, `containsany`, `equal`, `exclude`, `exclude_equal`, `record`, `set`, `let` — to be finalized against gottp's group-function registry in Phase A. Any unrecognized group function makes the group non-streamable, with a reason that names the function so Discovery knows what to file against.

Each failed condition contributes one entry to `NonStreamableReasons` (so a group with both `joinmatches` and a nested child gets two reasons — better than one).

### `groupPath` collision check

After computing `NormalizedPath` for every group (including nested, even though nested isn't supported for streaming in v1), build a multimap. For any normalized path with multiple distinct literal `Name` values, return `*GroupPathCollisionError`. Multiple groups with *identical* `Name` (the deliberate alternative-pattern synthesis case used by `show_iftable_detail` for `ups-port-virtual-entry*`) do not collide — the rule keys on distinct literal names normalizing to the same path, not on count.

## Error handling & edge cases

### Error precedence (in order of when they can fire)

1. **Compile time** — `*GroupPathCollisionError` from `CompileTemplate` if two groups normalize to the same path. Existing compile errors (regex failures, parse failures, etc.) keep their current types.
2. **`ParseStream` preflight, before any callback** — `*TemplateNotStreamableError` if the template isn't streamable. Existing input-function / lookup-load errors preserved (matches `Parse`).
3. **Callback** — non-nil return from `fn` aborts cleanly; ParseStream wraps with `fmt.Errorf("ParseStream callback aborted: %w", err)` so DH can `errors.Is(err, ctx.Canceled)` for context cancellation.
4. **Mid-parse internal errors** (macro panics, group function failures, lookup misses) — **same semantics as `Parse`**. Not introducing new error paths; whatever `parseGroup` does today (record drop, logged warning, etc.), `parseGroupStream` does identically. This is part of the parity guarantee.

**No buffered work to flush on abort.** Because streaming emits each completed record before producing the next match, there's no half-built results list to clean up. The in-flight match buffer (one record's worth of pattern matches) becomes garbage on return; the GC reclaims it.

### Edge cases

- **Empty input / no matches:** `ParseStream` returns `nil`, `fn` never invoked.
- **Callback panic:** not recovered. Same as a panic in any other Go callback API. Documented in the godoc.
- **Concurrent calls on the same `CompiledTemplate`:** safe. `CompiledTemplate` is read-only post-compile; each `ParseStream` builds its own `Runtime`. The callback `fn` is the caller's responsibility for goroutine safety if they share state.
- **Both `Parse` and `ParseStream` called sequentially on the same template:** safe. No mutation of the compiled template.
- **`vars` mutation across records:** if the template uses `record()` to mutate vars, downstream records see updates in input/group order — same as `Parse`. Documented as an explicit invariant.
- **Last in-flight record at EOF:** for `_start_`-bounded shapes, the final record has no following `_start_` to trigger emission. Streaming flushes it through the same emit path at end-of-input (mirrors how `Parse` handles the trailing record today).

## Testing strategy

Five test layers, each with a clear gate:

1. **Streamability classification** (`internal/compiler/streamability_test.go`) — table-driven test against synthetic templates exercising each rule individually:
   - plain streamable (single group, `_start_`, no `joinmatches`) → `Streamable=true`, no reasons
   - one variable uses `joinmatches` → false, reason names the variable
   - has a nested group → false, reason mentions nesting
   - no record boundary → false
   - aggregating group function in `Functions` → false, reason names the function
   - `foo` + `foo*` siblings → `*GroupPathCollisionError` from `CompileTemplate`
   - same literal name on multiple groups (the show_iftable_detail synthesis pattern) → no error

   Plus all 4 Class 1 prod templates → `Streamable=true` and clean reasons.

2. **Parity tests** (`test/comparison/streaming_parity_test.go`, no build tag — runs in normal `go test ./...`) — for each fixture in a small corpus (synthetic + small samples; not the 46 MB ones):
   - run `Parse`, walk the structured result, flatten to `map[groupPath][]record` keyed by normalized group path
   - run `ParseStream`, collect callbacks into the same shape
   - sort each `[]record` by the group's declared key fields
   - assert deep-equal

   Catches any divergence in record content, group filter behavior, macro application, lookup resolution, or `srcRange` semantics. Runs fast enough to live in the default test suite.

3. **Memory-bound test** (build-tagged `prodbaseline`, like the existing baseline harness in `test/comparison/prod_baseline_test.go`) — runs `ParseStream` against `/tmp/vyve-prod-captures/show_cable_modem_phy.txt` with the heap sampler, asserts `peak_delta < 20 MB` over the pre-parse baseline. Single test with the explicit success criterion from Goals.

4. **Performance regression guard** (also build-tagged) — runs both `Parse` and `ParseStream` over the same fixture and asserts `streamElapsed < 1.10 * parseElapsed`. Prevents accidental slowdown from streaming overhead.

5. **API contract tests** (in the top-level `gottp` package, normal suite):
   - `ParseStream` on non-streamable template → returns `*TemplateNotStreamableError` with non-empty reasons; `fn` never invoked
   - callback returns error → `ParseStream` returns wrapped error; verify `errors.Is(err, ctx.Canceled)` works for the cancellation use case
   - empty input → `ParseStream` returns nil, callback never invoked
   - `WhyNotStreamable(streamableTemplate)` returns `(true, nil)` and `WhyNotStreamable(nonStreamableTemplate)` returns `(false, reasons)` matching the underlying error reasons

### Phase exit criteria

- **Phase A** (compile-time analysis + API surface): layers 1 and 5 green.
- **Phase B** (streaming `parseGroup` variant): layer 2 green on the synthetic + small prod corpus.
- **Phase C** (full integration): layers 3 and 4 green on the at-scale prod fixture.
- **Phase D** (deploy + DH sync + observe): tracked outside this design as the validation step.

## Out of scope (v1)

- Nested groups inside streamable parents (deferred to v2).
- Source maps in streaming mode (`ParseWithSourceMap` style not exposed for `ParseStream` — error if both requested).
- Adapting existing output formatters/returners to consume streams (DH calls `ParseStream` directly, doesn't need a returner).
- `sync.Pool` for transient slices/maps (separate optimization track).
- Multi-input streaming with cross-input ordering guarantees (DH is single-input-per-template, no demand).
- Cancellation via `context.Context` parameter (DH uses callback-returns-error for cancellation, no `context.Context` needed in v1).

## Open items / risks

- **Macro engine reuse across records.** Verify in Phase A that Starlark and native Go macros reuse their execution engine across records inside a single `ParseStream` call. If the current code reallocates per call, fix as part of Phase B; otherwise a 256K-record template like `show_cable_modem_phy` becomes unnecessarily slow from engine-setup overhead.
- **Group function allowlist finalization.** The exact list of streamable group functions is finalized against gottp's group-function registry in Phase A. Any unrecognized group function makes the group non-streamable with a reason that names the function.
- **DH split-timing data for Class 2 templates.** The Class 2 (regex pathology) investigation depends on Discovery splitting their `parse_convert_ms` metric so we can localize where the cost lives. That track is independent of this design and runs on its own clock.
