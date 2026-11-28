# Changelog

All notable changes to GoTTP will be documented in this file.

## [Unreleased]

### Fixed (outputs)
- **Output macros run**: `<output macro="name"/>` and `<output functions="macro('name')"/>` were ignored, so the results came back unchanged. They now run the macro on the whole results, like Python TTP 0.10.1's output `macro()`: the macro gets the per_input list of results (or the per_template result), and what it returns replaces them, `True` and `False` included; `None` keeps the results with any in-place change. `macro=` and `functions=` run in attribute order, before the formatter. An undefined macro is skipped; a macro that fails, and `macro="a, b"` (a `TypeError` in TTP), are parse errors. Native Go macros work on per_template results; they take a map, so on the per_input list they are an error. A per_input result that an output macro or function turns into a non-list (a dict, `False`) is no longer wrapped in one more list; formatter output keeps its wrap (#98). (#142)

### Changed (breaking: record assembly)
A group without child groups is now assembled by a port of Python TTP 0.10.1's `_results_class` (refactor C1, #60; `internal/compiled/assembler.go`). `Parse`, source maps and `ParseStream` all use it. Groups with children keep the old merge until refactor C2. Templates that relied on these gottp-only behaviours change results:
- **One template line per data line (first match wins).** When several template lines match the same data line, TTP uses one of them: a `{{ _start_ }}` line first, then other start lines, then ordinary and `_end_` lines, then `_line_` lines. Within a tier the first template line wins, with `_end_` lines ahead of ordinary ones. gottp used to fill the record from every line that matched. A line made only of `ignore` variables still takes its data line. Python TTP turns digits in a line's text into `\d+`, so `Field-01 {{ a }}` and `Field-02 {{ b }}` both match `Field-07 x`, and the first one now takes every such line. Give such lines distinct text. (#32)
- **Values on an `_end_` line are discarded.** The line only ends the record. gottp used to save them. (#128, for groups without children)
- **joinmatches keeps value types.** Strings are joined with the join character, lists are extended (`split(",") | joinmatches` gives one list), and any other value becomes a list (`to_int | joinmatches` gives `[5, 6, 7]`, or `5` for one match). gottp used to stringify everything into one string. On a joinmatches line, TTP also joins the line's other variables. (#34)
- **A repeated start line starts a new record**, including one with a joinmatches variable and a joinmatches first line. gottp used to keep merging into the current record. (#29)
- **A start line rejected by a condition (`exclude`, `contains`, ...) ends the current record.** The lines after it are dropped until the next start line. gottp used to attach them to the previous record. (#69)
- **`_line_` loses its data line to every other template line**, so it no longer captures the start line. `_line_` joins lines even without joinmatches. A `_line_` line rejected by a condition is skipped instead of stopping later lines from joining. (#79)
- **No line-gap limit.** A line more than 100 lines after its record's start line is added to the record, as in TTP. gottp used to drop it (`maxGapLines`, a gottp-only rule). The limit is also gone from the merge used for groups with children.
- **Ties between two start lines on one data line follow TTP.** Before a group's first record, the last matching start line wins. After that, the first one does. A rejected start line that is the last candidate ends the group, as in TTP.
- **A `{{ _line_ }}` variable keeps its value.** A line that is only `{{ _line_ }}` compiled to a regex without a capture group, so the joined lines never reached the record; they are now saved under `_line_`, as in TTP.
- `ParseStream` `srcRange` follows the new records. A record closed by `_end_` ends at the end of that line, not at the next record's start, and a record ended by a rejected start line ends where that line starts.

### Changed (breaking: ParseStream)
- **`ParseStream` refuses templates it used to stream incorrectly**: its gate accepted group features the stream path doesn't implement, so those templates streamed without an error and gave records that differ from `Parse`. `ParseStream` now returns `ErrTemplateNotStreamable` (as `*TemplateNotStreamableError`, with one reason per feature) before calling the callback when a top-level group uses any of these: `chain=`/`functions=` functions outside the streamable allowlist, including after a `|` and in a `<vars>` value a chain names; group functions given as attributes (`contains=`, `containsall=`, `del=`/`delete=`, `exclude=`, `excludeall=`, `equal=`, `to_int=`, `contains_val=`, `exclude_val=`, `sformat=`, `itemize=`, `set=`, `expand=`, `items2dict=`, `to_ip=`, `record=`), or an attribute naming a custom group function from `CompileOptions` or `ParseOptions`; `default=`; `void`; `set()` defaults on a line with no other text; `method="table"`; a dynamic path (`{{ }}` in `name=`); `ignore()` with a template variable; a pattern that isn't line-anchored (this failed during the parse before). Detect it with `errors.Is(err, gottp.ErrTemplateNotStreamable)` and fall back to `Parse`, or check up front with `gottp.WhyNotStreamable`, which lists the compile-time reasons. Custom group function attributes, and a chain variable overridden by the parse's vars, are known only when `ParseStream` starts, so only its error reports them. A template affected by this change already gave wrong records. (#130)

### Fixed (ParseStream)
- **A start line with `default()` streams the record of defaults** `Parse` gives when the group matches nothing, with `srcRange` `[0, 0]` and the group's path. (#130)
- **A record that its chain functions empty is dropped**, as `Parse` drops it; the stream gave `{}`. (#130)
- **`chain=` naming a template variable is checked against the variable's value** (a string chain or a list of functions), not the variable's name. (#130)

### Changed (breaking: result shape)
- **Top-level anonymous groups now match Python TTP's result shape**: a template whose top-level group has no `name` (an unnamed `<group>`, or a template with no group tags at all) used its list of matches as the *whole* per-input result list, so the result was one list level short (`[[{...},{...}]]` from `NewParser().Result()` instead of TTP's `[[[{...},{...}]]]`), every input after the first was dropped, and named groups next to the anonymous one were lost. Each input's result is now built the way TTP's `form_results` does it: the anonymous matches are that input's result, other top-level results in the same input are appended as one more item, and every input keeps its own entry. An unnamed `<group>` gives a list even for one match; a template without group tags is an implicit group named `_anonymous_` (as in TTP), so one match gives a dict. The same applies to `results="per_template"` and to the source-map path (`EnableSourceMap`), whose `per_template` result kept a literal `_anonymous_` key. The `table` formatter flattens the per-input lists into one table as before. Code that reads `result[0]` as a match dict for an anonymous-group template must now read `result[0][0]`. Source maps report `GroupName: "_anonymous_"` for the implicit group; `ParseStream` still reports an empty `groupPath` for it. (#62)

### Fixed
- **`strip`/`lstrip`/`rstrip` characters argument**: `strip("chars")` ignored its argument and only removed whitespace, and `lstrip`/`rstrip` weren't registered, so all three returned the value unchanged. They now behave like Python's `str.strip/lstrip/rstrip(chars)`: the argument is a set of characters removed from both ends, the start or the end, and without an argument they remove whitespace. The `chain()` fallback does the same. A quoted argument made only of the other quote character (`strip('"')`) also reached the function as an empty string; quote characters inside a quoted argument are now kept. (#35)

### Changed (breaking: matching)
- **Indented template lines match the exact indent**: a template line's leading whitespace was turned into `[ \t]+`, so a line indented by two spaces also matched data indented by one space, four spaces or a tab. Python TTP rebuilds the indent verbatim, and gottp now does the same: two spaces match exactly two spaces, and a tab matches only a tab. Unindented lines are unchanged, and so is whitespace inside a line, which still matches any run of spaces and tabs. Templates that relied on the loose match need their indent to match the data. Templates saved with `SaveCompiledTemplate` keep the regex they were compiled with until they are compiled again. (#30)

### Changed (stricter YANG validation)
These fixes make `ParseWithValidation` report problems it used to pass, so output that was `Valid=true` can now be `Valid=false`. Parsed data is unchanged; only `ValidationResults` differ.
- **Integer leaves reject fractional numbers**: `12.5` no longer validates as a `uint8` (RFC 7950 9.2). A whole `float64` such as `12.0` (what a JSON round trip makes of an int) is still accepted. (#50)
- **`pattern` is enforced**: a string leaf must match all its patterns as a whole value, with XSD regex semantics (RFC 7950 9.4.5), and `modifier invert-match` is honoured. Patterns RE2 can't express (`\i`, `\c`, `\p{IsBlock}`, class subtraction) give a warning, once per group, instead of passing silently. (#55)
- **Enumeration and identityref membership is checked**: an enum leaf takes only its declared names, and an identityref only an identity derived from its base, in the RFC 7951 `module:identity` form (the module may be left out for an identity in the leaf's own module). A `union` with an enum member no longer accepts any string. (#56)
- **A `yang=` path at the wrong level is an error**: when none of a group's fields is defined under its path (for example the container instead of the list each match maps to), the result is `Valid=false` with one error that suggests the right path, instead of `Valid=true` with only warnings. (#57)
- **Leaves under `choice`/`case` are validated**: they were reported as "not defined" warnings and never type-checked. (#59)

### Fixed (source maps on)
`ParseOptions{EnableSourceMap: true}` (the editor's source-map toggle, and `ParseWithValidation` callers that set it) ran its own copy of the parse orchestration, which had drifted from the one used with source maps off. Both now run one pipeline (refactor A1, #60), so turning source maps on no longer changes data, errors, key fields or validation results:
- **A group without `input=` parses every input**, not only `Default_Input`, and `input="a, b"` is split on commas. Inputs embedded with `<input load="text">` or passed under other names gave an empty result. (#105)
- **Group errors keep their `failed to parse group <name>:` prefix.** (#106)
- **The `ignore("<vars name>")` empty-record special case applies**, so the group comes back as `{}` instead of being dropped (as with source maps off; #31 tracks matching Python TTP). (#107)
- **`void` groups are left out of the results.** (#108)
- **Input functions run** (`<input>` `functions=`, `macro=`, `commands=`), and **`KeyFields` is filled** from `keys=`.
- **Top-level groups are validated once.** A second YANG validation of top-level groups, which replaced the result `parseGroup` recorded, is gone; a group that only returns its start-line defaults no longer gets a validation result that it doesn't get with source maps off. (#52)
- `<vars name="...">` tags are stored the same way as with source maps off: in each input's results with `results="per_input"` (with getters such as `gethostname` resolved), and not at all with `per_template`.

### Changed (source map content)
The source map is now recorded from the matches the parser actually uses, instead of a second matching pass after parsing (refactor A2, #60). Parsed data doesn't change. The `SourceMap` JSON shape doesn't change, but its content does:
- **`ResultPath` is the record's real location in the results**, found by following each record through `record=`, group functions, macros and storing. Every level gets its list index: `routers[0].interfaces[1].addresses[0]`, where it used to be `routers*.interfaces[1].addresses`. Dynamic paths are resolved (`vlans.10` instead of `vlans.{{ id }}[0]`), and a repeated group with one match gives `interfaces[0]` instead of `interfaces*`. The old pass looked the path up by prefix in random map order and recorded only two nesting levels.
- **Only matches a record used are recorded.** A line a pattern matches but no record takes (before the first start line, after a rejected start line, a redundant `_start_`/`_end_` match) is no longer marked matched.
- **Matches of records that don't reach the results** (filtered out by `contains=` and similar, `void` groups, a child group whose `itemize` result isn't stored) are still recorded, with an empty `ResultPath`.
- **Children at every depth are recorded**, including a line that two groups' records both consumed.
- With source maps on, parsing is 24–32% faster and allocates 2–20% less, because the second matching pass is gone.

### Fixed (streaming)
- **`ParseStream` refuses templates it would parse differently from `Parse`.** Its streamability gate accepted group features the stream path ignores, so those templates streamed without error and gave different records. `WhyNotStreamable` now reports, and `ParseStream` returns `*TemplateNotStreamableError` for, these features: `chain=` functions outside the allowlist (only `functions=` was checked); a non-allowlisted function after a `|` (the names were split on commas only, so `contains('ip') | itemize(...)` passed); group functions given as attributes (`contains=`, `record=`, `expand=`, ...); `default=`; `void`; `set()` defaults; `method="table"`; dynamic paths; `default()` on the start line; `ignore()` with a template variable; a pattern that isn't line-anchored (it used to fail at runtime); and an attribute naming a custom group function (checked when `ParseStream` starts). A streamed record emptied by its group functions is now dropped, as `Parse` drops it. (#130)

### Tests
- **Record-assembler unit tests** (`internal/compiled/assembler_test.go`) cover each `_results_class` rule at the assembler seam. `TestLargeGroupWithStartMergesAllFields` now gives its 60 field lines distinct text, because under TTP's first-match rule `Field-01`…`Field-60` are one regex.
- **Record-assembly pins** (refactor C0, #60): 41 new minimal repros in `test/ttpsuite/testdata/gottp-repros`, recorded from Python TTP 0.10.1, cover each record-assembly rule of TTP's `_results_class` (one winning match per position, `startempty`, `_line_`, end then restart, add without overwrite, typed joinmatches) and each related issue (#69, #79, #88, #89, #91, #99, #100, #118, #119, #120, #128, and the new #136). The 29 gottp gets wrong are in `known_failures.json`. `TestStreamSnapshots` pins what `ParseStream` emits (record, `srcRange`, `groupPath`) for the 114 cases it streams, and checks the records against `Parse`.
- **Source-map snapshots and parity pins** (refactor A0, #60): `test/ttpsuite` now pins the source map of all 217 source-map parity cases as snapshot files (`TestSourceMapSnapshots`, rewritten with `-update-sourcemap-snapshots`), and `TestSourceMapParity` also compares `KeyFields`. Three new gottp-only parity cases (`testdata/sourcemap-parity`) record that source maps on skips input functions and leaves `KeyFields` empty. `test/comparison` has benchmarks with source maps on (`*SourceMap`) next to the existing ones.

## [v0.1.13] - 2026-09-03

### Fixed
- **SourceMap ResultPath for repeated groups**: `MatchMapping.ResultPath` for a repeated group (`name="interfaces*"`) always came back as the literal, unresolved group-name pattern (`"interfaces*"`) for every match, instead of resolving per-instance (`"interfaces[0]"`, `"interfaces[1]"`, ...) as documented. Per-match instance boundaries are now derived from the already-collected, position-sorted match list, with a safe fallback to the previous behavior for merge modes (table method, joinmatches) the boundary heuristic doesn't model. (#24)
- **Silently-swallowed group macro failures**: a group macro that failed — whether its source didn't compile as Starlark at all (e.g. `del data[key]`, unsupported by this version of go.starlark.net) or it threw at call time on a specific record's data — was silently treated the same as an unregistered macro name, with the match falling back to completely unmodified data and no error or warning anywhere. `ValidateMacroSource` now actually attempts to compile macro source as Starlark, surfacing genuine syntax errors as a compile warning; `processGroupMacro` now distinguishes "macro not registered" (still silently skipped, matching Python TTP behavior) from a real execution failure of a macro that *was* found (now propagates as a parse error). (#26)
- **Stale SourceMap.Variables after macro field rename**: a group macro that renamed a field (`data["name"] = data["ifname"]`) left `SourceMap.Variables` only keyed by the pre-macro name, with nothing connecting the renamed field in the final output back to the source span it came from. Renamed fields are now reconciled by value per resolved instance and aliased to their original span; macro-fabricated fields with no matching pre-macro value correctly still get no provenance entry. (#25)

## [v0.1.12] - 2026-07-25

### Performance
- **parseGroup Parent Ranges**: Removed two O(parents × matches) rescans of the match list in `parseGroup`'s parent-range computation. `allMatches` is already sorted by position, so both lookups are now binary searches. Parse time is linear in record count again instead of quadratic: on an 80,000-row input, ~1986 ms → ~130 ms (15×). Production profiles had this block at ~17% of aggregate CPU and ~50% within a single 30-second window.
- **Parent Span Bookkeeping**: Parent-to-match index lists are stored as `{lo, hi}` ranges rather than materialized `[]int` slices, cutting allocations on the same 80,000-row input from 4.32M to 1.28M (−70%) and 426 MB to 164 MB (−62%).
- **Dynamic Path Resolution**: The `{{ variable }}` regex in `PathResolver` and `ResultManager.FormPath` is compiled once at package scope instead of on every call, and placeholder substitution no longer compiles a second regex per placeholder. This accounted for ~7% of all allocations in production profiles and affects every template with a named group, not just dynamic paths.

### Fixed
- **Dynamic Path Values Containing `$`**: A parsed value containing `$` was silently truncated when substituted into a dynamic group name — `{{ hostname }}` resolving to `r$1x` produced `r`. Substitution used a regex replacement, which expanded `$1` / `${name}` sequences inside the replacement text; it now inserts the value verbatim.

### Also included (previously unreleased)

These entries had been accumulating under `[Unreleased]` and ship with this tag.

- **Start Pattern Detection**: Fixed issue where non-start patterns (like `route-policy`) were matching even when the `_start_` pattern didn't match. Now, when `_start_` is present but `_end_` is not:
  - Patterns with `_start_` are always start patterns
  - Patterns before the first `_start_` pattern can also be start patterns (if they have non-special variables)
  - Patterns after the first `_start_` pattern are not start patterns and only merge into matches started by `_start_` patterns
- **Empty Match Filtering**: Added logic to filter out empty matches from `mergedMatches` before returning from `parseGroup`, preventing empty maps from appearing in results
- **Traverse Function**: Fixed `traverse` function to correctly wrap dict results in lists when traversing paths in `[{interfaces: {...}}]` format, ensuring consistent return types

### Changed
- **Start Pattern Behavior**: Refined start pattern detection logic to be more precise about which patterns can initiate matches vs. which patterns only merge

### Documentation
- Updated `traverse` function documentation with argument order details and examples
- Updated `_start_` indicator documentation with refined behavior explanation
- Added examples showing how patterns before and after `_start_` behave differently

