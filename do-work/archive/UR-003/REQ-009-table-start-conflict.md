---
id: REQ-009
title: "Fix table + _start_ conflict"
status: completed
created_at: 2026-02-12T17:35:29Z
completed_at: 2026-02-12T19:00:00Z
user_request: UR-003
related: [REQ-002, REQ-007, REQ-008]
batch: app-reported-defects
resolution: no-fix-needed
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

## Research Findings

### Python TTP Behavior (Confirmed via Source Code and Testing)

In Python TTP (`ttp.py` lines 1854-1871), the regex classification logic is:

1. If `index == 0` (first pattern): always added to `start_re`
2. If `method == "table"`: ALL remaining patterns are added to `start_re` (ACTION set to "start")
3. If pattern has `_start_` indicator: added to `start_re`
4. Otherwise: added to `re` (normal, non-start patterns)

Key insight: **`method="table"` overrides `_start_`, not the other way around.** When `method="table"` is set, ALL patterns become start patterns regardless of `_start_`. Each pattern match creates a separate, independent record. `_start_` has no special precedence.

### The UR-002 Template Pattern

The UR-002 template uses `method="table"` on the **outer** group with `_start_` inside an **unnamed inner** group:

```xml
<group name="yang.if-mib:IF-MIB.ifTable.ifEntry*" method="table">
<group>
ifIndex: {{ ifIndex | _start_ }}
...other patterns...
</group>
</group>
```

In Python TTP, the inner group inherits `method="group"` (the default), so `_start_` works normally within the inner group. The outer `method="table"` only affects the outer group's own patterns (which has none -- all patterns are in the inner group). This produces merged records, which is the correct behavior.

### gottp Behavior (Already Correct)

gottp already matches Python TTP in both scenarios:

1. **`method="table"` + `_start_` without inner group**: All patterns are start patterns, each match is a separate record (6 records for 3 patterns x 2 data blocks). Matches Python TTP.

2. **`method="table"` + `_start_` with unnamed inner group**: Inner group uses default method, `_start_` works normally, records are merged (2 complete records). Matches Python TTP.

### Resolution

**No code fix was needed.** The original concern in the request was based on an incorrect assumption that `_start_` should take precedence over `method="table"`. In Python TTP, `method="table"` always takes precedence -- it unconditionally makes all patterns into start patterns.

The UR-002 template works correctly because the `_start_` is inside an unnamed inner group (which uses `method="group"` by default), not directly in the `method="table"` group. REQ-002 (commit d1759cf) already fixed the unnamed inner group handling, which was the actual issue.

## Tests Added

### Standalone Regression Tests (`test/method_attribute_test.go`)
- `TestMethodTableWithStartIndicator` - verifies `method="table"` + `_start_` without inner group produces 6 separate records (matching Python TTP)
- `TestMethodTableWithStartInnerGroup` - verifies `method="table"` with `_start_` in unnamed inner group produces 2 merged records (matching Python TTP)

### Comparison Tests (`test/comparison/groups_attributes_test.go`)
- `TestGroupAttributeMethodTableWithStart` - validates gottp matches Python TTP for `method="table"` + `_start_` without inner group
- `TestGroupAttributeMethodTableWithStartInnerGroup` - validates gottp matches Python TTP for `method="table"` + `_start_` with inner group

All tests pass (including full test suite: `go test ./...`).
