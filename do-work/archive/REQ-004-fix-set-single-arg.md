---
id: REQ-004
title: "Fix TestSet/single_arg failure"
status: completed
claimed_at: 2026-02-12T18:26:00Z
completed_at: 2026-02-12T18:26:00Z
route: A
created_at: 2026-02-12T17:32:54Z
user_request: UR-002
related: [REQ-002, REQ-003, REQ-005, REQ-006]
batch: test-failures-and-inner-group
---

# Fix TestSet/single_arg failure

## What
The `set()` group function with a single argument should return an error but returns `nil`. Test `TestSet/single_arg` in `internal/functions/group/functions_test.go:52` expects `wantErr: true` but gets `error = <nil>`.

## Context
From test suite run: `set()` requires at least two arguments (field name and value). When called with only one argument, it should return a validation error but currently does not.

---
*Source: Existing test failure from test suite run (REQ-001)*

---

## Triage

**Route: A** - Simple

**Reasoning:** Already fixed as part of REQ-003. The minimum args change from 1 to 2 resolves both issues.

**Planning:** Not required

## Plan

**Planning not required** - Route A: Direct implementation

Rationale: Already resolved by REQ-003's fix to set() minimum args validation.

*Skipped by work action*

## Implementation Summary

- Already fixed in REQ-003 commit (9d6eaea): minimum args changed from 1 to 2, single arg now returns proper error

*Completed by work action (Route A) — resolved by REQ-003*

## Testing

**Tests run:** `go test ./internal/functions/group/ -run "TestSet/single_arg" -v`
**Result:** PASS

*Verified by work action*
