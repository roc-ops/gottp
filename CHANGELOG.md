# Changelog

All notable changes to GoTTP will be documented in this file.

## [Unreleased]

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

### Tests
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

