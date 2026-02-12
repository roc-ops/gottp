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

- Fixed `internal/functions/match/more.go` line 521: changed default `count` from `1` to `0` to match Python's `re.sub(count=0)` behavior (replace all occurrences)
- The existing `count <= 0` branch already called `re.ReplaceAllString()`, so only the default needed fixing

*Completed by work action (Route A)*

## Testing

**Tests run:** `go test ./internal/functions/match/ -run "TestResub" -v`
**Result:** All 5 TestResub and 12 TestResuball subtests passing

*Verified by work action*
