---
id: REQ-004
title: "Fix TestSet/single_arg failure"
status: pending
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
