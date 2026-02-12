---
id: REQ-007
title: Fix container group early return
status: completed
claimed_at: 2026-02-12T18:46:00Z
completed_at: 2026-02-12T18:46:00Z
route: A
created_at: 2026-02-12T17:35:29Z
user_request: UR-003
related: [REQ-002, REQ-008, REQ-009]
batch: app-reported-defects
---

# Fix container group early return

## What
`parseGroup()` skips nested subgroups when the outer group has no direct patterns. This means container groups (groups that exist only to hold child groups, with no match patterns of their own) cause their children to be silently ignored.

## Detailed Requirements
- Investigate `parseGroup()` to find where it returns early when no direct patterns are found
- Fix so that outer groups without direct patterns still process their nested subgroups
- This is a real defect reported from an application using the library

## Dependencies
- May be partially or fully resolved by REQ-002 (inner group TTP compatibility fix) — investigate after REQ-002 is complete
- If REQ-002 resolves this, mark as duplicate/resolved and document why

## Builder Guidance
- Certainty level: Mixed — user says "potential defects" and "might be solved by the previous request"
- Check after REQ-002 is done whether this is still an issue before implementing a fix
- Add regression test regardless of outcome

---
*Source: "Container group early return — parseGroup() skipped nested subgroups when outer group had no direct patterns"*

---

## Triage

**Route: A** - Simple

**Reasoning:** Already resolved by REQ-002. The fix added delegation to unnamed nested groups when outer group has no patterns, which is exactly this issue.

**Planning:** Not required

## Plan

**Planning not required** - Route A: Direct implementation

Rationale: Resolved by REQ-002's fix (commit d1759cf). Verified by TestUnnamedInnerGroupSimple test.

*Skipped by work action*

## Implementation Summary

- Already fixed in REQ-002 (commit d1759cf): added delegation to unnamed nested groups in parseGroup early return path
- TestUnnamedInnerGroupSimple confirms container groups correctly process nested children

*Completed by work action (Route A) — resolved by REQ-002*

## Testing

**Tests run:** `go test ./test/ -run "TestUnnamedInnerGroupSimple" -v`
**Result:** PASS — container group correctly delegates to nested subgroups

*Verified by work action*
