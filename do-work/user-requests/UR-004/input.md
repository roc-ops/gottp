---
id: UR-004
title: Runtime lookup and variable injection for CompiledTemplate API
created_at: 2026-02-13T19:12:17Z
requests: [REQ-010, REQ-011]
word_count: 183
---

# Runtime Lookup and Variable Injection for CompiledTemplate API

## Summary

User is working with goTTP in another project and identified a gap: goTTP has no equivalent of Python TTP's `add_lookup()` / `add_vars()` for dynamic data injection at parse time. Since goTTP compiles templates ahead-of-time (vs Python TTP's JIT), lookups and vars are baked in at compile time. The user wants a way to compile once and parse many times with different dynamic data. After back-and-forth discussion, agreed on extending `ParseOptions` with `Lookups` and `Vars` fields, plus loader helpers.

## Extracted Requests

| ID | Title | Summary |
|----|-------|---------|
| REQ-010 | Runtime lookup and vars injection via ParseOptions | Add Lookups and Vars fields to ParseOptions for runtime injection |
| REQ-011 | Lookup loader helpers | LoadLookupFromJSON, LoadLookupFromCSV, LoadLookupFromYAML convenience functions |

## Batch Constraints

- The CompiledTemplate API must remain stateless — injection happens per-parse via ParseOptions, not mutating compiled state
- Runtime values override compiled values with the same name (lookups and vars both)
- ParseOptions.Vars merges with the top-level `vars` parameter; ParseOptions takes precedence on conflict
- Must be non-breaking — existing code that doesn't use the new fields continues to work unchanged
- User is taking this back to another project, so API ergonomics matter

## Full Verbatim Input

Okay, I'm working with goTTP in another project and an interesting point came up that I don't think we have Discussed or worked on or anything like that. So one of the key differences between goTTP and Python TTP is GoTTP is all compiled from the get go You don't know just in time compiling as it goes. So that has some downsides too was specifically for variable and lookup injection so right now we don't have the equivalent of add lookup or addvairs or Any of the stuff that Python had to make it kind of dynamic so that you could have a generic template and then just inject some dynamic data we need to Add that to our API. I'm not sure what the right way to do it is since ours is invokes differently than Python TTP You know the add underscore lookup or add underscore vars or any of those functions may not be applicable But let's do some exploration and see if we can come up with a solution that I can take back to them so that they can use in the other project before we required the center request, though, I want us to do some back and forth. You ask me some questions, I'll answer you, etc., so we can get the proper request put in there for this feature.

### Design Discussion Q&A

**Q: Which API surface — PythonParser (stateful) or CompiledTemplate (stateless)?**
A: CompiledTemplate (stateless)

**Q: Lookups only, or also vars?**
A: Both lookups and vars

**Q: Core use case?**
A: Compile once, parse many times with different dynamic data

**Q: How to inject — new method or extend ParseOptions?**
A: Add to ParseOptions (non-breaking change)

**Q: Override behavior when runtime lookup/var name matches compiled one?**
A: Runtime overrides compiled

**Q: Lookup format — raw maps only, or also loader helpers?**
A: Support both raw maps and loader helpers (JSON, CSV, YAML)

**Q: Add Vars to ParseOptions too for consistency?**
A: Yes, add Vars to ParseOptions for consistency (merges with top-level vars param, ParseOptions takes precedence)

---
*Captured: 2026-02-13T19:12:17Z*
