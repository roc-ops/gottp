---
id: REQ-006
title: "Fix TestStartEndIndicatorTogether failure"
status: pending
created_at: 2026-02-12T17:32:54Z
user_request: UR-002
related: [REQ-002, REQ-003, REQ-004, REQ-005]
batch: test-failures-and-inner-group
---

# Fix TestStartEndIndicatorTogether failure

## What
The combined start/end indicator test fails because the parser produces an empty result (`{}`) instead of the expected structure containing a `ptp_peers` key. Test `TestStartEndIndicatorTogether` in `test/start_end_indicator_test.go:250,269` shows the result is an empty object.

## Context
From test suite run: when both `_start_` and `_end_` indicators are used together in a template, the parser fails to extract any data. The expected output contains a `ptp_peers` structure but the actual output is `{}`.

---
*Source: Existing test failure from test suite run (REQ-001)*
