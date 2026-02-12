---
id: REQ-009
title: "Fix table + _start_ conflict"
status: pending
created_at: 2026-02-12T17:35:29Z
user_request: UR-003
related: [REQ-002, REQ-007, REQ-008]
batch: app-reported-defects
---

# Fix table + _start_ conflict

## What
`method="table"` unconditionally marks ALL patterns as record-starters, overriding explicit `_start_` boundaries. This means when a group has both `method="table"` and an explicit `_start_` indicator on a specific pattern, the table method ignores the `_start_` and treats every pattern line as a record boundary.

## Detailed Requirements
- Investigate how `method="table"` interacts with `_start_` indicators
- Fix so that when `_start_` is explicitly set on a pattern, `method="table"` respects it instead of overriding it
- `_start_` should take precedence — it's the user's explicit declaration of where records begin
- This is a real defect reported from an application using the library

## Dependencies
- May be partially or fully resolved by REQ-002 (inner group TTP compatibility fix) — investigate after REQ-002 is complete
- If REQ-002 resolves this, mark as duplicate/resolved and document why
- Directly relevant to the UR-002 template which uses both `method="table"` and `_start_` on `ifIndex`

## Builder Guidance
- Certainty level: Mixed — user says "potential defects" and "might be solved by the previous request"
- Check after REQ-002 is done whether this is still an issue before implementing a fix
- Add regression test regardless of outcome
- The UR-002 template (`ifTable` with `method="table"` + `_start_` on `ifIndex`) is a natural test case for this

---
*Source: "Table + _start_ conflict — method=\"table\" unconditionally marked ALL patterns as record-starters, overriding _start_ boundaries"*
