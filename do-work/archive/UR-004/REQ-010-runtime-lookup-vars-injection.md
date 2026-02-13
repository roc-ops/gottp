---
id: REQ-010
title: "Runtime lookup and vars injection via ParseOptions"
status: completed
created_at: 2026-02-13T19:12:17Z
claimed_at: 2026-02-13T19:15:00Z
route: C
completed_at: 2026-02-13T19:45:00Z
user_request: UR-004
related: [REQ-011]
batch: dynamic-injection
---

# Runtime Lookup and Vars Injection via ParseOptions

## What
Add `Lookups` and `Vars` fields to `ParseOptions` so that callers can inject dynamic lookup tables and variables at parse time without recompiling the template.

## Detailed Requirements

- Add `Lookups` field to `ParseOptions` with type `map[string]map[string]interface{}` (lookup name -> key-value table)
- Add `Vars` field to `ParseOptions` with type `map[string]interface{}`
- When `ParseOptions.Lookups` is provided, merge with compiled lookups at runtime; runtime lookups override compiled lookups with the same name
- When `ParseOptions.Vars` is provided, merge with the top-level `vars` parameter passed to `Parse()`; `ParseOptions.Vars` takes precedence on key conflicts
- The `CompiledTemplate` must remain stateless — no mutation of compiled state; all injection is per-parse via `ParseOptions`
- Existing code that doesn't use the new fields must continue to work unchanged (non-breaking addition)
- The lookup registry (`internal/lookup/table.go`) already supports `AddTable()` and `Lookup()` — wire the new `ParseOptions.Lookups` into this mechanism at runtime
- Vars are already partially supported via the top-level `Parse()` parameter — ensure the `ParseOptions.Vars` merge happens correctly with the existing var resolution path

## Constraints

- Non-breaking API change — only adds optional fields to an existing struct
- CompiledTemplate stays stateless; PythonParser API is not in scope (it already has `AddVars`)
- User is consuming this from another project, so the API must be clean and well-documented

## Dependencies

- REQ-011 (loader helpers) builds on top of this — loaders produce maps that feed into `ParseOptions.Lookups`
- This is the core mechanism; REQ-011 is convenience

## Builder Guidance

- Certainty level: Firm (evolved from exploratory — user initially unsure of approach, firmed up through design Q&A)
- The user explored Python TTP's `add_lookup()` / `add_vars()` API but explicitly chose a different design (ParseOptions extension) suited to goTTP's compiled model
- Key files to modify: `gottp.go` (ParseOptions struct), `internal/compiled/runtime.go` (lookup/var resolution), `internal/lookup/table.go` (registry integration)
- Test with compile-once-parse-many pattern: same compiled template, different lookups/vars per parse call

## Full Context
See [user-requests/UR-004/input.md](./user-requests/UR-004/input.md) for complete verbatim input and design Q&A.

---
*Source: See UR-004/input.md for full verbatim input*

---

## Triage

**Route: C** - Complex

**Reasoning:** New feature introducing runtime injection concept to the compiled template model. Touches public API (ParseOptions), runtime lookup resolution, var merging paths, and lookup registry. Multi-file architectural change requiring careful planning to maintain statelessness.

**Planning:** Required

## Plan

### Architecture Overview

Two layers of `ParseOptions`:
1. **Public API** (`gottp.ParseOptions` in `gottp.go` line 296): external callers
2. **Internal** (`compiled.ParseOptions` in `internal/compiled/runtime.go` line 87): runtime engine

Translation happens in `CompiledTemplate.ParseWithValidation()` (line 168) and `Runtime.ParseWithValidation()` (line 352) in `gottp.go`.

### Design Decisions

**Runtime lookups threading:** Store `runtimeLookups` as per-parse field on `Runtime` (consistent with existing `validationResults`, `recordedVars` pattern). Set at parse start, read during parse.

**Var merge precedence (3 layers):**
1. `compiled.Vars` (from `<vars>` tag) — base
2. `vars` parameter to `Parse()` — overrides compiled
3. `ParseOptions.Vars` — highest precedence

**Lookup merge:** Compiled lookups first, runtime lookups override same-name entries.

### Implementation Steps

1. **Internal ParseOptions struct** (`runtime.go` line 87): Add `Lookups map[string]map[string]interface{}` and `Vars map[string]interface{}`
2. **Runtime struct** (`runtime.go` line 23): Add `runtimeLookups map[string]map[string]interface{}` per-parse field
3. **Runtime.Parse()** (`runtime.go` line 99): Set/clear `runtimeLookups`, add `ParseOptions.Vars` merge after existing var merge
4. **Runtime.ParseWithSourceMap()** (`runtime.go` line 592): Same pattern as step 3
5. **processFunctions()** (`runtime.go` line 5331): Merge `r.runtimeLookups` into lookup tables map after compiled lookups
6. **Public ParseOptions** (`gottp.go` line 296): Add `Lookups` and `Vars` fields with doc comments
7. **Translation** (`gottp.go` lines 168, 352): Thread `Lookups` and `Vars` to internal struct
8. **Tests** (`test/runtime_injection_test.go`): 9 test scenarios covering basic usage, override semantics, precedence, compile-once-parse-many, regression

### Key Invariant: `internal/lookup/table.go` NOT modified
The runtime already builds `lookupTables` as raw `map[string]map[string]interface{}` bypassing the Registry. Runtime lookups merge into this same map.

### Test Scenarios
- Basic lookups via ParseOptions (no compiled lookup)
- Runtime lookups override compiled lookups
- Basic vars via ParseOptions
- ParseOptions.Vars > Parse() vars > compiled vars
- Compile-once-parse-many with different lookups
- Compile-once-parse-many with different vars
- Nil Lookups/Vars regression
- Combined lookups + vars

*Generated by Plan agent*

## Exploration

### Key Structures

- **Public `ParseOptions`** (`gottp.go:295`): Currently has `YANGModules *YANGModules` and `EnableSourceMap bool`
- **Internal `ParseOptions`** (`runtime.go:87`): Currently has `YANGModuleSet *yang.ModuleSet` and `EnableSourceMap bool`
- **`Runtime` struct** (`runtime.go:23`): Has `validationResults` and `recordedVars` as per-parse mutable fields
- **`CompiledTemplate.Lookups`**: Type `[]*CompiledLookup` where each has `.Name`, `.Data` (interface{})
- **`CompiledTemplate.Vars`**: Type `map[string]interface{}`

### Translation Points (public → internal ParseOptions)

Two locations in `gottp.go`:
1. `CompiledTemplate.ParseWithValidation()` (line ~168): Creates `compiled.ParseOptions{EnableSourceMap: options.EnableSourceMap}`
2. `Runtime.ParseWithValidation()` (line ~352): Same pattern

### Var Merge (exists in two places)

- `Runtime.Parse()` (line ~110): compiled.Vars base, then Parse() vars override
- `Runtime.ParseWithSourceMap()` (line ~614): Same pattern duplicated

### Lookup Table Building (`processFunctions` line ~5330)

```go
if funcName == "lookup" || funcName == "gpvlookup" || funcName == "rlookup" {
    lookupTables := make(map[string]map[string]interface{})
    if r.compiled.Lookups != nil {
        for _, lookup := range r.compiled.Lookups {
            if lookup.Data != nil {
                if dataMap, ok := lookup.Data.(map[string]interface{}); ok {
                    lookupTables[lookup.Name] = dataMap
                }
            }
        }
    }
    kwargs["_lookup_tables"] = lookupTables
}
```

### Test Patterns

Tests use `gottp.CompileTemplate(template)` → `compiled.Parse(gottp.Inputs{...}, nil, nil)` → JSON marshal for inspection. Existing `test/lookups_test.go` and `test/template_variables_test.go` provide patterns to follow.

*Generated by Explore agent*

## Implementation Summary

- Modified `internal/compiled/runtime.go`:
  - Added `Lookups` and `Vars` fields to internal `ParseOptions` struct
  - Added `runtimeLookups` per-parse field to `Runtime` struct
  - Added runtime lookups setup and `ParseOptions.Vars` merge to `Parse()` method
  - Added same pattern to `ParseWithSourceMap()` method
  - Added runtime lookups merge into `processFunctions()` lookup table building
- Modified `gottp.go`:
  - Added `Lookups` and `Vars` fields with doc comments to public `ParseOptions` struct
  - Added field translation in both `ParseWithValidation` methods
- Created `test/runtime_injection_test.go` with 8 tests

No deviations from plan.

*Completed by work action (Route C)*

## Testing

**Tests run:** `go test ./...`
**Result:** All packages PASS, zero failures

**New tests added** (`test/runtime_injection_test.go`):
- TestParseOptionsLookups_Basic — lookup resolves from ParseOptions without compiled lookup
- TestParseOptionsLookups_OverrideCompiled — runtime lookups override compiled lookup data
- TestParseOptionsVars_Basic — ParseOptions.Vars overrides compiled template vars
- TestParseOptionsVars_PrecedenceOverParseVars — ParseOptions.Vars > Parse() vars parameter
- TestCompileOnceParseMany_Lookups — compile once, 3 parses with different lookups
- TestCompileOnceParseMany_Vars — compile once, 3 parses with different vars
- TestParseOptions_NilFields_Regression — nil fields identical to nil options
- TestParseOptions_LookupsAndVarsCombined — both features used together

**Existing tests verified:** All 14 test packages pass unchanged

*Verified by work action*
