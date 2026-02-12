---
id: REQ-005
title: "Fix TestResub/multiple_matches failure"
status: completed
claimed_at: 2026-02-12T18:27:00Z
completed_at: 2026-02-12T18:30:00Z
route: A
created_at: 2026-02-12T17:32:54Z
user_request: UR-002
related: [REQ-002, REQ-003, REQ-004, REQ-006]
batch: test-failures-and-inner-group
---

# Fix TestResub/multiple_matches failure

## What
The `resub()` match function only replaces the first match instead of all matches. Test `TestResub/multiple_matches` in `internal/functions/match/more_test.go:325` expects `aXbXcX` (all digits replaced with `X`) but gets `aXb2c3` (only the first digit replaced).

## Context
From test suite run: `resub()` should perform a global replacement (all occurrences) but currently only replaces the first match. This is a regex substitution function that should behave like Python's `re.sub()` which replaces all matches by default.

---
*Source: Existing test failure from test suite run (REQ-001)*

---

## Triage

**Route: A** - Simple

**Reasoning:** Single default value change. Test specifies exact expected behavior.

**Planning:** Not required

## Plan

**Planning not required** - Route A: Direct implementation

Rationale: Simple default value bug. Python's `re.sub()` uses `count=0` (replace all) by default.

*Skipped by work action*

## Implementation Summary

- **Correction:** Python TTP comparison test confirmed `resub` replaces first match only (count=1) by default — `resuball` replaces all. The unit test expectation was wrong.
- Reverted `internal/functions/match/more.go` default count back to `1` (matching Python TTP's resub behavior)
- Fixed unit test to expect `"aXb2c3"` (first match only) instead of `"aXbXcX"` (all matches)

*Completed by work action (Route A) — corrected after Python TTP comparison test*

## Testing

**Tests run:** `go test ./... `
**Result:** All tests passing including comparison tests

*Verified by work action*
