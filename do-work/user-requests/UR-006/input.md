---
id: UR-006
title: Update editor for new goTTP features (lookups, vars, custom functions)
created_at: 2026-02-13T22:00:00Z
requests: [REQ-014, REQ-015, REQ-016]
word_count: 100
---

# Update Editor for New goTTP Features

## Summary

User wants to update the WASM-based web editor (`editor/`) to take advantage of the new goTTP features added in UR-004 (runtime lookup/vars injection via ParseOptions) and UR-005 (custom function registration via ParseOptions and CompileOptions). The editor currently compiles goTTP to WebAssembly and runs in-browser. Two concerns: (1) rebuild the WASM binary with the latest goTTP code, and (2) update the editor UI and WASM bridge to expose the new features.

## Extracted Requests

| ID | Title | Summary |
|----|-------|---------|
| REQ-014 | Rebuild editor WASM binary | Rebuild the editor's goTTP WASM binary with latest goTTP code including all new features |
| REQ-015 | Update editor WASM bridge for new APIs | Update wasm/main.go to expose ParseOptions.Lookups, ParseOptions.Vars, ParseOptions.Functions, and CompileTemplateWithOptions |
| REQ-016 | Update editor UI for new features | Add UI panels/controls for lookup tables, vars, and custom functions in the web editor |

## Batch Constraints

- The editor is a sub-project at `editor/` within the goTTP repo
- Editor uses WASM (Go compiled to GOOS=js GOARCH=wasm)
- Must rebuild WASM first before UI changes can reference new APIs
- Editor already has some lookup/vars support — need to understand what exists vs what's new
- Custom functions (Go closures) may not translate directly to browser JS — need to assess feasibility

## Full Verbatim Input

We've added several new pieces of functionality to GoTTP, but I don't think we've rebuilt the editor or adjusted the editor to match the new functionality. Can we take a look at the editor and see what it would take to first rebuild using the new version of GoTTP that we just built, and then secondly how we can take advantage of the new features like custom lookup functions, custom functions in general, etc. Because those are not present in the editor yet.

---
*Captured: 2026-02-13T22:00:00Z*
