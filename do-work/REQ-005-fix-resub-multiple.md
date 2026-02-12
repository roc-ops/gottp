---
id: REQ-005
title: "Fix TestResub/multiple_matches failure"
status: pending
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
