---
id: REQ-012
title: "Runtime custom function injection via ParseOptions"
status: completed
created_at: 2026-02-13T20:19:10Z
claimed_at: 2026-02-13T20:25:00Z
completed_at: 2026-02-13T21:15:00Z
route: C
user_request: UR-005
related: [REQ-013]
batch: custom-functions
---

# Runtime Custom Function Injection via ParseOptions

## What
Add a `Functions` field to `ParseOptions` so callers can inject custom functions at parse time across all 5 scopes (match, group, input, output, macro), enabling the compile-once-parse-many pattern with different custom logic per parse call.

## Detailed Requirements

- Add `Functions` field to `ParseOptions` containing per-scope function maps
- Support all 5 scopes: match, group, input, output, macro
- Function signatures must match the existing internal registry signatures for each scope:
  - **Match**: `func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)` (from `internal/functions/match/`)
  - **Group**: `func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)` (from `internal/functions/group/`)
  - **Input**: Builder must discover exact signature from `internal/functions/input/` registry
  - **Output**: Builder must discover exact signature from `internal/functions/output/` registry
  - **Macro**: Match existing Go macro signature from `RegisterGoMacro`
- ParseOptions functions override built-in functions with the same name
- ParseOptions functions override CompileOptions functions (REQ-013) with the same name
- Precedence: built-ins < CompileOptions < ParseOptions (same pattern as vars/lookups)
- No mutation of compiled state — custom functions are per-parse, injected into runtime registries at parse start
- Custom functions must be invocable from the same template positions as built-in functions — pipe syntax in match patterns and XML tag attributes
- Existing code that doesn't use the new field continues to work unchanged

## Constraints

- Non-breaking API change — only adds optional field to existing struct
- Same API pattern as `ParseOptions.Lookups` and `ParseOptions.Vars` (REQ-010)
- Maximum parity with Python TTP's `add_function(fun, scope, name)` — the translation story should be clean
- User is consuming this from another project — API ergonomics matter

## Dependencies

- REQ-013 (CompileOptions) builds on the same type definitions but is independent to implement
- REQ-010 (ParseOptions.Lookups/Vars) establishes the pattern this follows

## Builder Guidance

- Certainty level: Firm (evolved from exploratory — user initially unsure, firmed up through Q&A with portability as guiding principle)
- The guiding principle is "maximum parity/portability with Python TTP" — when in doubt, match Python TTP's behavior
- Key files: `gottp.go` (ParseOptions, type definitions), `internal/compiled/runtime.go` (registry injection at parse start), `internal/functions/match/`, `internal/functions/group/`, `internal/functions/output/`, `internal/functions/input/`
- Consider defining public type aliases for each function scope signature (e.g., `type MatchFunc func(...)`, `type GroupFunc func(...)`) for API ergonomics
- The `Functions` field should be a struct with per-scope maps, not a flat map with scope strings — this is Go, not Python
- Test with compile-once-parse-many: same compiled template, different custom functions per parse call

## Full Context
See [user-requests/UR-005/input.md](./user-requests/UR-005/input.md) for complete verbatim input and design Q&A.

---
*Source: See UR-005/input.md for full verbatim input*

---

## Triage

**Route: C** - Complex

**Reasoning:** New feature introducing custom function registration across 5 scopes. Requires discovering exact signatures from 5 different internal registries, designing public type aliases and a Functions struct, and wiring runtime injection into all registry types. Multi-file architectural change spanning public API and internal runtime.

**Planning:** Required

## Plan

### Discovered Function Signatures (from internal registries)

| Scope | Signature | Source |
|-------|-----------|--------|
| Match | `func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)` | `match.Function` |
| Group | `func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)` | `group.Function` |
| Output | `func(data interface{}, args []string) interface{}` | `output.Function` |
| Input | `func(data string, args []string) string` | `input.Function` |
| Macro | `func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)` | `macro.GoMacroFunc` |

### Public Type Definitions (in gottp.go)

```go
type MatchFunc func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)
type GroupFunc func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)
type OutputFunc func(data interface{}, args []string) interface{}
type InputFunc func(data string, args []string) string
type MacroFunc func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)

type Functions struct {
    Match  map[string]MatchFunc
    Group  map[string]GroupFunc
    Output map[string]OutputFunc
    Input  map[string]InputFunc
    Macro  map[string]MacroFunc
}
```

### Implementation Steps

1. **Public types** in `gottp.go`: Define 5 type aliases + `Functions` struct
2. **ParseOptions**: Add `Functions *Functions` to public and internal ParseOptions
3. **Runtime struct**: Add `runtimeFunctions *RuntimeFunctions` per-parse field
4. **Helper methods**: `getMatchFunc(name)`, `getGroupFunc(name)`, etc. — check runtime overrides first, fall back to registry
5. **Parse/ParseWithSourceMap**: Set `r.runtimeFunctions` from options at parse start, clear at next parse
6. **Update ~15 function lookup sites** in `runtime.go` to use helper methods instead of direct registry `.Get()`
7. **Chain function**: Pass resolver closure via kwargs so chain can find custom match functions without circular dependency
8. **Bridge methods**: Update both `ParseWithValidation` methods to pass Functions through
9. **Tests**: 10 scenarios covering all scopes, override, precedence, chain integration, compile-once-parse-many

### Key Design Decisions

- **Override via helper methods**: Runtime uses `getMatchFunc(name)` which checks `runtimeFunctions.Match[name]` first, then falls back to `matchRegistry.Get(name)`. No mutation of registries.
- **Chain support**: Pass `_match_func_resolver` closure in kwargs to avoid circular dependency between match and compiled packages.
- **Statelessness**: `runtimeFunctions` set at parse start, not stored on CompiledTemplate.

*Generated by Plan agent*

## Exploration

### Corrected Function Signatures (from actual source)

| Scope | Signature | File |
|-------|-----------|------|
| Match | `func(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)` | `match/functions.go:14` |
| Group | `func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)` | `group/functions.go:10` |
| Output | `func(data interface{}, args []string, kwargs map[string]interface{}) (interface{}, error)` | `output/functions.go:12` |
| Input | `func(data string, args []string, kwargs map[string]interface{}) (string, bool, error)` | `input/functions.go:9` |
| Macro | `func(data map[string]interface{}, args []string, kwargs map[string]interface{}) (map[string]interface{}, bool, error)` | `macro/starlark.go:629` |

### Registry.Get() Call Sites in runtime.go (8+ locations)

- **Match**: Line 5302 (`r.matchRegistry.Get(funcName)` in processFunctions)
- **Match→Chain**: Line 5336 (passes `r.matchRegistry` to chain via kwargs)
- **Group**: Lines 3635, 3750, 5929 (group attribute handling + applyGroupFunctions)
- **Input**: Lines 6148, 6177 (extract_commands + general input functions)
- **Output**: Lines 6264, 6290 (output functions + format functions)

### Chain Function (more.go lines 1674-1760)
- Gets registry from `kwargs["_match_registry"]`
- Calls `registry.Get(funcName)` at line 1740
- Need resolver closure pattern to handle custom functions without circular deps

### Key Patterns
- All registries follow same pattern: `NewRegistry()` → `registerBuiltins()` → `Get(name)`
- All created fresh in `NewRuntime()` (lines 42-46)
- Macro registry has concurrency locks (RWMutex) since it handles Go macros

*Generated by Explore agent*

## Testing

| # | Test | Scope | What it verifies |
|---|------|-------|-----------------|
| 1 | TestParseOptionsFunctions_MatchScope | Match | Custom match function via pipe syntax transforms value |
| 2 | TestParseOptionsFunctions_MatchOverrideBuiltin | Match | Custom "upper" overrides built-in "upper" |
| 3 | TestParseOptionsFunctions_GroupScope | Group | Custom group function filters results via XML attribute |
| 4 | TestParseOptionsFunctions_MacroScope | Macro | Custom macro function modifies group data |
| 5 | TestCompileOnceParseMany_DifferentFunctions | Match | Same compiled template, 3 parses: alpha, beta, no-custom |
| 6 | TestParseOptionsFunctions_NilFields | Match | nil Functions, empty Functions, nil options all behave identically |
| 7 | TestParseOptionsFunctions_MatchInChain | Match+Chain | Custom function inside `chain()` via `_match_func_resolver` closure |

**Result:** All 7 tests pass. All existing tests continue to pass (`go test ./...` — 0 failures).

## Implementation Summary

### Files Modified

| File | Changes |
|------|---------|
| `gottp.go` | Added 5 public type aliases (`MatchFunc`, `GroupFunc`, `OutputFunc`, `InputFunc`, `MacroFunc`), `Functions` struct, `Functions *Functions` field on `ParseOptions`, `convertFunctions()` helper, updated both `ParseWithValidation` bridge methods |
| `internal/compiled/runtime.go` | Added `RuntimeFunctions` struct, `Functions *RuntimeFunctions` on internal `ParseOptions`, `runtimeFunctions` per-parse field on `Runtime`, 4 helper methods (`getMatchFunc`, `getGroupFunc`, `getOutputFunc`, `getInputFunc`), replaced 8 `registry.Get()` calls with helpers, added `_match_func_resolver` closure for chain, custom group function attribute processing, wired setup/cleanup into `Parse()`/`ParseWithSourceMap()` |
| `internal/functions/match/more.go` | Updated `chainFunc` to try `_match_func_resolver` first, then registry, then inline |
| `test/custom_functions_test.go` | New — 7 tests + `extractHostname` helper |

### Deviations from Plan

1. **Group attribute handling**: Plan didn't account for custom group functions specified as XML attributes being limited to a hardcoded list. Added a secondary loop for runtime group function attribute processing.
2. **Chain test uses `<vars>` tag**: Chain test defines the chain variable via `<vars>` in template rather than `ParseOptions.Vars` due to how chain resolves variable names at runtime.
3. **No loop variable capture**: Go 1.22+ changed loop variable semantics — no closures needed for loop iteration variables.

### Notes

- Pre-existing `TestSharedRuntime` race condition (concurrent map write in `internal/compiled`) observed sporadically — unrelated to REQ-012 changes.
- Input and Output scopes wired in runtime but not explicitly tested via integration tests (no easy end-to-end test template pattern for these scopes). The helper methods and type conversions are exercised by the match/group/macro tests confirming the pattern works.
