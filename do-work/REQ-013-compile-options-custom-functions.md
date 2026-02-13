---
id: REQ-013
title: "Compile-time custom function registration via CompileOptions"
status: pending
created_at: 2026-02-13T20:19:10Z
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
