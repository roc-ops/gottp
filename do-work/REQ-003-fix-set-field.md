---
id: REQ-003
title: "Fix TestSet/set_field failure"
status: pending
created_at: 2026-02-12T17:32:54Z
user_request: UR-002
related: [REQ-002, REQ-004, REQ-005, REQ-006]
batch: test-failures-and-inner-group
---

# Fix TestSet/set_field failure

## What
The `set()` group function does not produce the expected `new_field` in the result. Test `TestSet/set_field` in `internal/functions/group/functions_test.go:63` fails.

## Context
From test suite run: the `set()` group function is expected to create/set a field called `new_field` in the group result, but the field is not present in the output. This is an existing test that was written but the implementation doesn't satisfy it yet.

---
*Source: Existing test failure from test suite run (REQ-001)*
