# Macro engine reuse audit

Date: 2026-04-27
Branch: feat/parsestream-v1
Purpose: Determine whether macro engines are safe to reuse across per-record
emissions in streaming mode, or whether B2/B3 must add explicit caching.

---

## Starlark

- **Engine created at:** `internal/compiled/runtime.go:57`
  `macroRegistry: macro.NewMacroRegistry()` — called once inside
  `NewRuntimeWithFunctions`. The `MacroRegistry` holds a single
  `*StarlarkEngine` for the lifetime of the `Runtime`.

- **Macro source compiled at:** `internal/macro/starlark.go:51-52`
  (`RegisterMacro`) and `internal/macro/starlark.go:119-120`
  (`RegisterMacroSource`). Both call `starlark.ExecFile` exactly once at
  registration time and cache the resulting `starlark.Callable` in
  `e.funcCache` (a `map[string]starlark.Callable`).

- **Per-invocation thread created at:**
  - Fast path (cache hit): `internal/macro/starlark.go:254` — a single
    lightweight `*starlark.Thread` is created per `ExecuteMacroStarlark` call.
  - Batch path: `internal/macro/starlark.go:350` — one thread is created per
    `ExecuteMacroStarlarkBatch` call and **reused for all macros in the chain**.
  - The `*starlark.Callable` itself is never re-created; only the thread is new
    each invocation.

- **Reused across records: YES (callable) / lightweight per-call (thread)**
  The compiled `starlark.Callable` lives in `funcCache` and is looked up by
  name (`internal/macro/starlark.go:213-215`, `288-291`). The only per-call
  overhead is creating a `*starlark.Thread` struct (cheap) and calling
  `starlark.Call`. Source recompilation only occurs on the first call if the
  macro was not registered via `RegisterMacro` / `RegisterMacroSource` (the
  `else` branch at `internal/macro/starlark.go:223-250`) — this is a fallback
  that should not be hit in normal operation.

- **Invocation site in runtime:** `internal/compiled/runtime.go:6149`
  (`ExecuteMacroStarlarkBatch`) and `6190` (`ExecuteMacro`) — both called from
  `processGroupMacro` which is called per-group-result batch, not per record.
  In streaming mode it will be called once per emitted record.

- **If streaming were to hit a recompile path:** The else-branch at
  `starlark.go:223` calls `starlark.ExecFile` (full source recompile). This
  only fires when `funcCache[name]` is empty — i.e., the macro source was
  registered but the function name was not found during registration. This is
  a bug path, not the normal path. Result: caching is effective after the
  first invocation even if the registration path missed the function.

---

## JavaScript (goja)

- **Engine created at:** `internal/macro/starlark.go:648` via
  `NewMacroRegistry` → `NewJavaScriptEngine()` (`internal/macro/javascript.go:19`).
  Single `*goja.Runtime` is created there and stored in `JavaScriptEngine.vm`.

- **Registered at:** `internal/macro/javascript.go:34` (`RegisterMacro`) —
  source stored in `e.macros[name]` map, validated via `vm.RunString` once.

- **Per-invocation overhead:** `internal/macro/javascript.go:66` —
  `ExecuteMacro` creates a **brand-new `*goja.Runtime` (VM) on every call**
  (`vm := goja.New()`), then calls `vm.RunString(source)` to re-execute the
  macro source before invoking the function. This is full recompilation per
  call — the registered `e.vm` is never used for execution; only the stored
  source string is used.

- **Reused across records: NO** — every `ExecuteMacro` call allocates a new
  goja VM and re-parses + re-executes the source JS. For 256K records this
  would create 256K VMs.

- **Impact for Phase B:** JavaScript macros are a per-call cliff.
  However: the streamability classifier (`analyzeStreamability`) should mark
  groups with JS macros as non-streamable (macro language check), preventing
  streaming mode from being activated for those templates. Verify that
  `analyzeStreamability` rejects templates with `Macro != ""` that reference
  a JS-language macro. If it does, the JS recompile path is never reached in
  streaming mode and no fix is needed here.

---

## Native Go

- **Registered at:** `internal/compiled/runtime.go:81`
  (`compileFns.Macro` loop), `248`, `255`, `790`, `797` — all call
  `r.macroRegistry.RegisterGoMacro(name, fn)` which stores the Go function
  pointer in `MacroRegistry.goMacros` (`internal/macro/starlark.go:699`).

- **Per-invocation overhead:** `internal/macro/starlark.go:725`
  (`ExecuteMacro` Go branch) — a direct Go function call:
  `goMacro(dataMap, nil, ttpContext)`. No VM, no compilation, no reflection.
  The function pointer is looked up once from the map at call time.

- **Reused across records: YES** — the function pointer is stable for the
  lifetime of the `MacroRegistry`. Each invocation is a plain Go call with
  map-typed arguments.

---

## Summary

| Engine    | Callable cached | Per-call cost                  | Safe for streaming? |
|-----------|-----------------|-------------------------------|---------------------|
| Starlark  | YES (funcCache) | New `*starlark.Thread` (cheap)| YES                 |
| Native Go | YES (func ptr)  | Direct Go call                | YES                 |
| JavaScript| NO              | New goja.Runtime + RunString  | NO (but gated)      |
| Python    | Unknown (stub)  | —                             | Unknown             |

---

## Verdict

**Starlark macros are reused across records. Native Go macros are reused.
JavaScript macros are NOT reused (new VM per call), but streaming mode must
already be gated behind the streamability classifier which should reject JS
macro groups.**

**Phase B action required:**

1. Confirm `analyzeStreamability` (Phase A) rejects groups whose macro field
   references a JavaScript-language macro. If yes, no JS fix needed.
2. No changes to the Starlark engine or Go macro paths are needed for B2/B3.
3. The one footgun: `processGroupMacro` (runtime.go:6097) calls
   `r.macroRegistry.GetStarlarkEngine()` and converts the whole match to
   Starlark before calling macros. In streaming mode (per-record), this
   Go→Starlark→Go round-trip happens for every emitted record. The conversion
   is O(fields-per-record), not O(source-length), so it is acceptable — no
   compile work is being repeated. Document this in B3 if the per-record
   conversion becomes a profiling target.
