---
id: UR-003
title: Application-reported defects in group parsing and table method
created_at: 2026-02-12T17:35:29Z
requests: [REQ-007, REQ-008, REQ-009]
word_count: 85
---

# Application-reported defects in group parsing and table method

## Summary

User reports 3 potential defects from an application using the gottp library. User notes these may overlap with the inner group TTP compatibility fix (REQ-002/UR-002) and wants them investigated after that fix is done.

## Extracted Requests

| ID | Title | Summary |
|----|-------|---------|
| REQ-007 | Fix container group early return | parseGroup() skips nested subgroups when outer group has no direct patterns |
| REQ-008 | Fix multi-template child element loss | Child group elements discarded in multi-template documents |
| REQ-009 | Fix table + _start_ conflict | method="table" overrides _start_ boundaries by marking all patterns as record-starters |

## Batch Constraints

- These may be partially or fully resolved by REQ-002 (inner group TTP compatibility fix) — user wants REQ-002 done first, then these investigated
- All three are reported from a real application using the library, so they represent production-impacting issues
- Sequencing: process after REQ-002 completes; some may become invalid/duplicate at that point

## Full Verbatim Input

An application that's using this library is reported as three potential defects. They might be solved by the previous request I sent in as we fix that template so that it matches. That may solve some of this problem, but after that's done I want you to look into these issues as well. 1. Container group early return — parseGroup() skipped nested subgroups when outer group had no direct patterns
  2. Multi-template child element loss — child <group> elements were discarded in multi-template documents
  3. Table + _start_ conflict — method="table" unconditionally marked ALL patterns as record-starters, overriding
   _start_ boundaries

---
*Captured: 2026-02-12T17:35:29Z*
