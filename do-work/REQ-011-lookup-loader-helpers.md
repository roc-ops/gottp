---
id: REQ-011
title: "Lookup loader helpers"
status: pending
created_at: 2026-02-13T19:12:17Z
user_request: UR-004
related: [REQ-010]
batch: dynamic-injection
---

# Lookup Loader Helpers

## What
Provide convenience functions to load lookup tables from common file formats (JSON, CSV, YAML), returning maps that can be passed directly to `ParseOptions.Lookups`.

## Detailed Requirements

- `LoadLookupFromJSON(name string, data []byte) (map[string]map[string]interface{}, error)` — parse JSON into a named lookup table
- `LoadLookupFromCSV(name string, data []byte) (map[string]map[string]interface{}, error)` — parse CSV into a named lookup table (first row as headers, key column configurable)
- `LoadLookupFromYAML(name string, data []byte) (map[string]map[string]interface{}, error)` — parse YAML into a named lookup table
- Each function returns a single-entry map (`name -> table`) that can be merged into `ParseOptions.Lookups`
- Match Python TTP's loader patterns for feature parity where applicable
- Handle common error cases: malformed input, empty data, missing key column (CSV)

## Constraints

- These are convenience functions — raw `map[string]map[string]interface{}` must also work without loaders
- Depends on REQ-010 (the ParseOptions.Lookups field must exist for these to be useful)
- User wants both raw maps AND loader helpers, not one or the other
- User is consuming this from another project — API ergonomics matter

## Dependencies

- Depends on: REQ-010 (runtime injection mechanism) — loaders produce data that feeds into `ParseOptions.Lookups`

## Builder Guidance

- Certainty level: Firm — user explicitly requested "support both raw maps and loader helpers"
- These are public API functions, likely in the `gottp` package or a `gottp/lookup` sub-package
- CSV loader may need a key column option (e.g., first column as lookup key by default)
- Consider whether file-path variants (LoadLookupFromJSONFile, etc.) are needed or if []byte is sufficient

## Open Questions

- Input format: Should loaders accept `[]byte`, file paths (`string`), `io.Reader`, or all three? The Q&A confirmed JSON/CSV/YAML formats but didn't specify the Go input mechanism. Builder should decide based on API ergonomics.

## Full Context
See [user-requests/UR-004/input.md](./user-requests/UR-004/input.md) for complete verbatim input and design Q&A.

---
*Source: See UR-004/input.md for full verbatim input*
