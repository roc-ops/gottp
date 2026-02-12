---
id: REQ-008
title: Fix multi-template child element loss
status: pending
created_at: 2026-02-12T17:35:29Z
user_request: UR-003
related: [REQ-002, REQ-007, REQ-009]
batch: app-reported-defects
---

# Fix multi-template child element loss

## What
Child `<group>` elements are discarded in multi-template documents. When a document contains multiple `<template>` sections, child group elements within them are lost during parsing.

## Detailed Requirements
- Investigate how multi-template documents parse child `<group>` elements
- Fix so that child groups are preserved across all templates in a multi-template document
- This is a real defect reported from an application using the library

## Dependencies
- May be partially or fully resolved by REQ-002 (inner group TTP compatibility fix) — investigate after REQ-002 is complete
- If REQ-002 resolves this, mark as duplicate/resolved and document why

## Builder Guidance
- Certainty level: Mixed — user says "potential defects" and "might be solved by the previous request"
- Check after REQ-002 is done whether this is still an issue before implementing a fix
- Add regression test regardless of outcome

---
*Source: "Multi-template child element loss — child <group> elements were discarded in multi-template documents"*
