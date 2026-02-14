---
id: REQ-015
title: "Update editor WASM bridge for new goTTP APIs"
status: pending
created_at: 2026-02-13T22:00:00Z
user_request: UR-006
related: [REQ-014, REQ-016]
batch: editor-update
---

# Update Editor WASM Bridge for New goTTP APIs

## What
Update `editor/wasm/main.go` to expose the new goTTP APIs through the WASM bridge so JavaScript can use them: ParseOptions.Lookups, ParseOptions.Vars, ParseOptions.Functions, CompileTemplateWithOptions, and CompileOptions.Functions.

## Detailed Requirements

- Expose `ParseOptions.Lookups` — allow JavaScript to pass lookup tables when parsing
- Expose `ParseOptions.Vars` — allow JavaScript to pass dynamic variables when parsing
- Expose `CompileTemplateWithOptions` — allow JavaScript to compile with CompileOptions
- Assess feasibility of custom functions (Go closures) in WASM context:
  - Custom functions are Go closures — can JavaScript provide these via WASM?
  - If not directly possible, document the limitation
  - Consider alternatives: predefined function sets, Starlark macros via template, etc.
- Update the `parseTemplate()` WASM function to accept lookups and vars parameters
- Assess whether goTTP loader helpers (`LoadLookupFromJSON`, `LoadLookupFromYAML`, `LoadLookupFromCSV` from REQ-011) can be exposed through WASM for in-browser lookup table loading from user-uploaded files
- Maintain backward compatibility — existing calls without new parameters still work

## Builder Guidance

- Certainty level: Exploratory — the custom functions aspect may have WASM limitations
- The WASM bridge currently exports 7 functions via `syscall/js` — new functionality may add new exports or extend existing ones
- Key file: `editor/wasm/main.go`
- Key reference: `editor/js/wasm-bridge.js` (JavaScript side of the bridge)
- Lookups/vars should be straightforward (JSON data). Custom functions may not be feasible in WASM — assess and document.

## Dependencies

- Depends on REQ-014 (WASM binary must be rebuilt with new goTTP first)

---
*Source: See UR-006/input.md for full verbatim input*
