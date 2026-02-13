---
id: UR-005
title: Custom function registration for all scopes (match, group, input, output, macro)
created_at: 2026-02-13T20:19:10Z
requests: [REQ-012, REQ-013]
word_count: 192
---

# Custom Function Registration for All Scopes

## Summary

User wants to add custom function registration to goTTP, mirroring Python TTP's `add_function(fun, scope, name)` API. goTTP already has comprehensive built-in functions (59 match, 18 group, 11 output, 2 input) but no public API for users to register custom functions except Go macros (group-level only). The feature should support all 5 scopes (match, group, input, output, macro) via both ParseOptions (per-parse dynamic) and CompileOptions (compile-time baked-in). Function signatures should match the existing internal registry signatures for each scope. ParseOptions functions override CompileOptions functions, which override built-ins — same precedence pattern as lookups/vars. Existing `RegisterGoMacro` should be unified into the new API with backward compatibility.

## Extracted Requests

| ID | Title | Summary |
|----|-------|---------|
| REQ-012 | Runtime custom function injection via ParseOptions | Add Functions field to ParseOptions for per-parse function injection across all 5 scopes |
| REQ-013 | Compile-time custom function registration via CompileOptions | Add CompileOptions with Functions field + unify macro registration |

## Batch Constraints

- Maximum parity/portability with Python TTP's `add_function(fun, scope, name)` API
- All 5 scopes: match, group, input, output, macro
- Function signatures must match existing internal registry signatures (not a new universal signature)
- ParseOptions functions override CompileOptions functions override built-ins
- Existing `RegisterGoMacro` continues to work (backward compat) but new unified API is the primary path
- Same API pattern as lookups/vars injection (ParseOptions extension, non-breaking)
- User is consuming this from another project — API ergonomics and portability matter

## Full Verbatim Input

Okay, I know we talked about it on a last request about doing functions as well. Now we want to do kind of the same thing we just did with lookups for functions. You'll need to look at the original Python TTP library because there are several different kinds of functions. These would less speed the functions that would go in the macro library. That's one option, but these would be more to add functionality specific tags like output tag or vars tag or something like that where you can create a tag and have it do something. I'm sorry, create an attribute on a tag, an XML tag and have it do something by running a function on it. Those are the kind of ones for mostly functions on it, but we can also do macro ones too. Via the go TTP version, those are less important because it's not just a time compiled one anymore. Now it's a compiled template, so these would be more for add extra functionality to it. So if you have any questions, please ask me. Review the Python TTP functions definitions to get an idea of what kinds there are. And then come back and ask me some questions so we can make sure we're on the same page before we create the request.

### Design Discussion Q&A

**Q: Which function scopes should support custom registration?**
A: All four (match, group, input, output)

**Q: Where should custom functions be registered — ParseOptions, CompileOptions, or both?**
A: Both options — CompileOptions for static functions baked into compiled template, ParseOptions for dynamic per-parse functions

**Q: For function signatures, should they match Go conventions, mirror Python TTP, or match existing internals?**
A: Match existing internals — use whatever signatures the internal registries already use

**Q: Should ParseOptions functions override CompileOptions functions with the same name?**
A: Yes — maximum parity with Python TTP where user-registered functions shadow built-ins

**Q: Should existing Go macro registration be unified into the new API or kept separate?**
A: Unify — Python TTP treats macro as just another scope in add_function(). Keep RegisterGoMacro for backward compat but new unified API is primary.

**Guiding principle (user's words):** "what would create the greatest parity / portability between gottp and python ttp?"

---
*Captured: 2026-02-13T20:19:10Z*
