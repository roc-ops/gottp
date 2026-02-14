---
id: REQ-016
title: "Update editor UI for lookups, vars, and custom functions"
status: pending
created_at: 2026-02-13T22:00:00Z
user_request: UR-006
related: [REQ-014, REQ-015]
batch: editor-update
---

# Update Editor UI for Lookups, Vars, and Custom Functions

## What
Add UI panels and controls to the web editor so users can manage lookup tables, dynamic variables, and (if feasible) custom functions through the browser interface.

## Detailed Requirements

- **Lookup tables**: The editor already has a "Lookup Tables" UI panel — determine whether it wires through to the new `ParseOptions.Lookups` field or uses an older mechanism, and update accordingly rather than building from scratch
  - Support JSON format for lookup table input
  - Support loading lookup tables from files (JSON, YAML, CSV via loader helpers if available in WASM)
- **Dynamic variables**: The editor already has a "Global Variables" UI panel — determine whether it wires through to the new `ParseOptions.Vars` field or uses an older mechanism, and update accordingly rather than building from scratch
- **Custom functions** (if feasible per REQ-015 assessment):
  - If WASM can support JS→Go function callbacks, add UI for defining custom functions
  - If not, document limitation and consider alternative approaches
- All new UI should match the existing editor's dark theme and design patterns
- New features should be accessible from the existing menu/panel system

## Builder Guidance

- Certainty level: Exploratory — scope depends on REQ-015's feasibility findings for custom functions
- The editor already has a "Global Variables" feature and "Lookup Tables" feature — need to assess how these interact with the new goTTP ParseOptions fields
- Key files: `editor/js/app.js` (main UI logic, 79KB), `editor/index.html`, `editor/css/main.css`
- Scope cue from user: "see what it would take" — this is investigative, assess what's possible/useful

## Dependencies

- Depends on REQ-015 (WASM bridge must expose new APIs first)
- Depends on REQ-014 (WASM binary must be rebuilt first)

---
*Source: See UR-006/input.md for full verbatim input*
