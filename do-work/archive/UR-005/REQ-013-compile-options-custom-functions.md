---
id: REQ-013
title: "Compile-time custom function registration via CompileOptions"
status: completed
completed_at: 2026-02-13T21:45:00Z
created_at: 2026-02-13T20:19:10Z
claimed_at: 2026-02-13T21:20:00Z
route: C
user_request: UR-005
related: [REQ-012]
batch: custom-functions
---

# Compile-time Custom Function Registration via CompileOptions

## What
Add a `CompileOptions` struct with a `Functions` field so callers can register custom functions at compile time that get baked into the compiled template. Also unify the existing `RegisterGoMacro` API into the new function registration system.

## Detailed Requirements

- Create a `CompileOptions` struct with a `Functions` field (same type as `ParseOptions.Functions` from REQ-012)
- Add `CompileTemplateWithOptions(templateText string, options *CompileOptions) (*CompiledTemplate, error)` or modify existing `CompileTemplate` to accept options
- CompileOptions functions are baked into the compiled template — available for all subsequent `Parse()` calls without re-specifying
- CompileOptions functions override built-in functions with the same name
- ParseOptions functions (REQ-012) override CompileOptions functions with the same name
- Precedence: built-ins < CompileOptions < ParseOptions
- Unify existing `RegisterGoMacro` into the new API:
  - The macro scope in `CompileOptions.Functions` replaces the need for `runtime.GetMacroRegistry().RegisterGoMacro()`
  - Keep `RegisterGoMacro` working for backward compatibility (don't remove it)
  - Document migration path from old API to new unified API
- Support all 5 scopes: match, group, input, output, macro
- Custom functions must be invocable from the same template positions as built-in functions — pipe syntax in match patterns and XML tag attributes

## Constraints

- `CompileTemplate(templateText)` without options must continue to work unchanged (non-breaking)
- The compiled template must store custom functions so they're available at parse time
- User is consuming this from another project — API ergonomics and portability matter
- Maximum parity with Python TTP's `add_function()` which is called before template loading

## Dependencies

- Uses the same function type definitions as REQ-012 (shared types)
- Logically paired with REQ-012 but can be implemented independently
- The precedence chain (built-ins < CompileOptions < ParseOptions) requires both REQs to be complete for full functionality

## Builder Guidance

- Certainty level: Firm — user confirmed "both options" (CompileOptions + ParseOptions)
- The guiding principle is "maximum parity/portability with Python TTP"
- Python TTP's `add_function()` is called before template loading — `CompileOptions` is the goTTP analog for that timing
- For the `RegisterGoMacro` unification: the macro scope in `Functions` is the replacement, but the old API stays as a convenience wrapper
- Key files: `gottp.go` (new CompileOptions, CompileTemplateWithOptions), `internal/compiler/` (function storage), `internal/compiled/runtime.go` (function loading)

## Full Context
See [user-requests/UR-005/input.md](./user-requests/UR-005/input.md) for complete verbatim input and design Q&A.

---
*Source: See UR-005/input.md for full verbatim input*

---

## Triage

**Route: C** - Complex

**Reasoning:** New public API surface (`CompileOptions`, `CompileTemplateWithOptions`), compile-time function storage on `CompiledTemplate`, 3-layer precedence chain (built-ins < CompileOptions < ParseOptions), `RegisterGoMacro` unification. Multi-file architectural change spanning public API, compiler, and runtime.

**Planning:** Required

## Plan

### Architecture

- Compile-time functions stored on public `gottp.CompiledTemplate` wrapper (unexported `compileFunctions` field), not on internal `compiler.CompiledTemplate` (which is serializable — closures are not)
- Reuse existing `compiled.RuntimeFunctions` type for both compile-time and parse-time functions
- Three-layer precedence: `runtimeFunctions` (ParseOptions) > `compileFunctions` (CompileOptions) > registry (built-ins)
- `CompileTemplateWithOptions` as new function (not modifying `CompileTemplate` signature) for backward compat
- `NewRuntime` delegates to `NewRuntimeWithFunctions(compiled, nil)` for backward compat

### Implementation Steps

1. **runtime.go**: Add `compileFunctions *RuntimeFunctions` field to `Runtime`, create `NewRuntimeWithFunctions(compiled, compileFns)` constructor, update `NewRuntime` to delegate
2. **runtime.go**: Extend 4 helper methods (`getMatchFunc`, `getGroupFunc`, `getOutputFunc`, `getInputFunc`) to check `compileFunctions` between `runtimeFunctions` and registry
3. **runtime.go**: Update `Parse`/`ParseWithSourceMap` to re-register compile-time macros at each parse start (before per-parse macros) for correct precedence across multiple Parse calls
4. **gottp.go**: Add `CompileOptions` struct, `CompileTemplateWithOptions` function, add `compileFunctions` field to public `CompiledTemplate`, update `ParseWithValidation` and `NewRuntime` to pass `compileFunctions`
5. **gottp.go**: Add deprecation doc comments to `RegisterGoMacro` with migration examples
6. **Tests**: ~10 scenarios covering all scopes, 3-layer precedence, compile-once-parse-many, nil safety, macro unification, chain integration

### Key Design Decisions

- **Macros registered eagerly**: Unlike match/group/input/output (lazy lookup via helpers), macros go into `MacroRegistry.goMacros` eagerly. Compile-time macros registered in constructor, re-registered each `Parse` call before per-parse macros.
- **Serialization**: Serialized/deserialized templates lose compile-time functions — expected since functions are code, not data.

*Generated by Plan agent*

## Exploration

### Key Locations in gottp.go
- `CompiledTemplate` struct: lines 71-72 (add `compileFunctions` field)
- `CompileTemplate` function: lines 46-57 (pattern for `CompileTemplateWithOptions`)
- `ParseWithValidation` (CompiledTemplate): lines 156-209 (line 157 creates runtime — pass compileFunctions)
- `ParseWithValidation` (Runtime): lines 386-440 (line 408 — similar update)
- `NewRuntime` (CompiledTemplate): lines 445-448 (use `NewRuntimeWithFunctions`)
- `convertFunctions()` helper: lines 559-595 (reuse for compile-time conversion)
- `Functions` struct + type aliases: lines 304-329 (already defined from REQ-012)

### Key Locations in runtime.go
- `Runtime` struct: lines 23-37 (add `compileFunctions` field)
- `NewRuntime` constructor: lines 40-57 (create `NewRuntimeWithFunctions`, delegate)
- Helper methods: `getMatchFunc` (89-96), `getGroupFunc` (99-106), `getOutputFunc` (109-116), `getInputFunc` (119-126) — extend to 3-layer precedence
- `RuntimeFunctions` type: lines 127-133 (reuse for compile-time)
- `Parse` macro registration: lines 180-185 (add compile-time macros before runtime macros)
- `ParseWithSourceMap` macro registration: lines 717-722 (same)

### Macro Registry (internal/macro/starlark.go)
- `GoMacroFunc` type: line 627
- `MacroRegistry` struct: lines 630-637 (has `goMacros map[string]GoMacroFunc`)
- `RegisterGoMacro`: lines 696-701 (acquires lock, sets `goMacros[name] = fn`)
- `ExecuteMacro`: lines 710-752 (checks goMacros first, then Starlark/JS/Python)

### api/python/parser.go
- Line 71: `compiled.NewRuntime(template)` — backward compat via delegation, no changes needed

*Generated by Explore agent*

## Testing

| # | Test | What it verifies |
|---|------|-----------------|
| 1 | TestCompileOptions_MatchScope | Custom match function via CompileOptions transforms values |
| 2 | TestCompileOptions_GroupScope | Custom group function via CompileOptions filters results |
| 3 | TestCompileOptions_MacroScope | Custom macro via CompileOptions (replaces RegisterGoMacro) |
| 4 | TestCompileOptions_OverrideBuiltin | CompileOptions "upper" overrides built-in |
| 5 | TestCompileOptions_ParseOptionsOverrides | ParseOptions overrides CompileOptions with same name |
| 6 | TestCompileOptions_PersistAcrossParses | Compile once, parse 3 times, functions always available |
| 7 | TestCompileOptions_NilOptions | `CompileTemplateWithOptions(text, nil)` behaves like `CompileTemplate(text)` |
| 8 | TestCompileOptions_MacroOverridePrecedence | Parse-time macro overrides compile-time macro, then restores |
| 9 | TestCompileOptions_ThreeLayerPrecedence | Full 3-layer: built-in < CompileOptions < ParseOptions, with restore |
| 10 | TestCompileOptions_MatchInChain | Compile-time match function works inside `chain()` |

**Result:** All 10 tests pass. All existing tests continue to pass (`go test ./...` — 0 failures).

## Implementation Summary

### Files Modified

| File | Changes |
|------|---------|
| `gottp.go` | Added `CompileOptions` struct, `CompileTemplateWithOptions` function, `compileFunctions` field on `CompiledTemplate`, updated `ParseWithValidation` and `NewRuntime` to use `NewRuntimeWithFunctions`, added deprecation notes to `RegisterGoMacro` |
| `internal/compiled/runtime.go` | Added `compileFunctions` field to `Runtime`, created `NewRuntimeWithFunctions` constructor, extended 4 helper methods for 3-layer precedence, added compile-time macro re-registration in `Parse`/`ParseWithSourceMap`, fixed custom group function attribute lookup to use `getGroupFunc()` |

### Files Created

| File | Description |
|------|-------------|
| `test/compile_options_test.go` | 10 tests covering all scopes, precedence, persistence, nil safety, macro unification, chain |

### Deviations from Plan

1. **Custom group function attribute lookup**: Plan didn't account for the runtime group function attribute block directly accessing `r.runtimeFunctions.Group[attrName]` instead of `getGroupFunc()`. Fixed to use helper method for proper 3-layer precedence and extended guard to also trigger for compile-time group functions.

*Completed by work action (Route C)*
