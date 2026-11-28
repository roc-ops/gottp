# TTP 0.10.1 test suite

Python TTP 0.10.1's own test suite (`test/pytest` in [dmulyalin/ttp](https://github.com/dmulyalin/ttp)), replayed through gottp. This is the compliance measure for the baseline (#60).

Current result (refactor C2, #60):

```
go test ./test/ttpsuite -v | grep -E 'suite:|repros:|parity:|snapshots:'
```

```
TTP 0.10.1 suite: pass 72 / skipped-by-issue 124 / design 4 of 200
gottp repros: pass 49 / skipped-by-issue 6 / design 0 of 55
source-map parity: same 258 / diverge 0 of 258
stream snapshots: 114 streamable of 258 cases
```

The Go tests need no Python, so they run in plain `go test ./...` and in CI. There are five:

- `TestTTPSuite`: TTP's own 200 cases against TTP 0.10.1's results (below).
- `TestGottpRepros`: gottp bug repros against TTP 0.10.1's results ([gottp repros](#gottp-repros)).
- `TestSourceMapParity`: every case from both, plus the gottp-only cases in `testdata/sourcemap-parity`, parsed with source maps off and on, must give the same data and key fields ([Source-map parity](#source-map-parity)).
- `TestSourceMapSnapshots`: the source map of every parity case must match its snapshot in `testdata/sourcemap-snapshots` ([Source-map snapshots](#source-map-snapshots)).
- `TestStreamSnapshots`: what `ParseStream` emits for every parity case it accepts must match its snapshot in `testdata/stream-snapshots`, and the streamed records must be the records `Parse` returns ([Stream snapshots](#stream-snapshots)).

## How it works

- `testdata/cases/<module>.json` holds 200 cases, one file per TTP test module. Each case has the template, the inputs, the TTP test it came from (`source`) and the result Python TTP 0.10.1 returned (`expected`).
- `ttpsuite_test.go` runs each case as a subtest through gottp's Python-style API (`NewParser`, `AddTemplate`, `AddInput`, `Parse`, `Result`) with the working directory set to `testdata/pytest`, so the templates' `./assets/...` paths resolve. The result is compared **strictly**: there is no list-unwrapping normalization, because that normalization hid #62. (`test/comparison` is strict too now, except for the tests in its `looseListLevels` table.)
- `known_failures.json` lists every case that doesn't match yet. `issue` is the issue its first strict difference belongs to, `also` lists issues the case hits after that, and `reason` is one line. A reason starting with `design:` marks a deliberate difference.
- The test fails when a case differs and isn't listed, when a listed case passes ("remove it from known_failures.json"), and when the table names a case that doesn't exist. So the table can't go stale: fixing an issue makes its cases fail until their entries are removed.
- `TTPSUITE_REPORT=/tmp/report.json go test ./test/ttpsuite` writes every case's outcome, first difference and gottp result, for triage (`TTPSUITE_REPROS_REPORT` does the same for the repros).

## gottp repros

`testdata/gottp-repros/` holds minimal repros of gottp bugs: `NAME.ttp` (template), `NAME.txt` (the `Default_Input` data) and `NAME.expected.json` (Python TTP 0.10.1's result, written by `record.py expect`). The case name is `gottp-repros/NAME`; `NAME` starts with the issue number. `TestGottpRepros` runs them like the suite cases, through the Python-style API, and they share `known_failures.json`, so each one turns on when its issue is fixed. To add one, write the `.ttp` and `.txt` and run `record.py expect`.

| Repro | Issue |
|---|---|
| `issue28_parent_start_line_without_variables` | passes; #28 is fixed |
| `issue29_joinmatches_on_start_line` | #29 |
| `issue30_indented_line_matches_any_indentation` | passes; #30 is fixed |
| `issue31_ignore_vars_name` | #31 |
| `issue32_one_line_fills_several_template_lines` | #32 |
| `issue33_trailing_ignore_with_trailing_whitespace` | #33 |
| `issue34_split_joinmatches` | #34 |
| `issue35_strip_characters_argument` | #35 |
| `issue36_unknown_function_falls_back_to_str_method` | #36 |
| `sourcemap_void_group` | passes; covers `void` for source-map parity (#108) |
| `issue60_first_line_starts_record_after_start_indicator` | passes; the first line starts a record even after a `_start_` line (refactor B, #60) |
| `issue60_table_tail_line_is_ordinary` | passes; a `method="table"` line after a child group is not a start pattern (#60) |
| `issue60_tail_start_line_starts_own_record` | passes; a `_start_` line after a child group starts its own record (#60) |
| `issue128_end_line_values_are_saved` | passes; #128 is fixed |
| `issue128_end_line_values_no_children` | passes; #128 in a group without children |

Record-assembly pins for refactor C (#60). Each is a minimal template for one rule of TTP's `_results_class` (one winning match per position, start/startempty/add/join/end), recorded from TTP 0.10.1. Since refactor C1 gottp assembles records with a port of `_results_class` (`internal/compiled/assembler.go`), and since C2 a group with children and all its descendants go through one assembler (`internal/compiled/tree.go`), so most of them pass. Those gottp still gets wrong are in `known_failures.json` under the issue named by their prefix; `issue60_asm_*` cases without a dedicated issue are listed under #60.

| Repro | Rule |
|---|---|
| `issue28_empty_record_after_saved_record` | a record with no values is saved once the path already holds a record (#28) |
| `issue29_joinmatches_on_first_line` | a joinmatches first line is a start pattern, so each match starts a record (#29) |
| `issue32_start_pattern_wins_position` | a start pattern beats an ordinary one on the same line (#32) |
| `issue32_first_ordinary_pattern_wins_position` | of two ordinary patterns on one line, the first template line wins (#32) |
| `issue32_end_pattern_wins_position` | an end pattern beats an ordinary one on the same line, and its values are dropped (#32) |
| `issue32_start_tie_before_and_after_first_record` | two start patterns on one line: the last wins before any record, the record's own group after (#32) |
| `issue32_ignore_only_pattern_wins_position` | a line of only `ignore` variables still wins its position (#32) |
| `issue34_joinmatches_keeps_value_types` | joinmatches keeps `to_int` values: one value stays an int, several make a list (#34) |
| `issue34_joinmatches_char_and_lists` | joinmatches with a join character, and lists that extend (#34) |
| `issue34_join_line_joins_every_variable` | on a joinmatches line, every variable of the line is joined (#34) |
| `issue69_failed_start_condition_ends_record` | a start line rejected by a condition ends the record (#69) |
| `issue79_line_joinmatches_skips_start_line` | `_line_` loses its position to the start line (#79) |
| `issue79_line_without_joinmatches_joins_lines` | `_line_` joins lines even without joinmatches (#79) |
| `issue136_start_only_line_matches_trailing_text` | `text {{ _start_ }}` matches only that exact text (#136) |
| `issue136_startempty_tie_with_start_pattern` | the same, where a start pattern also matches the line (#136) |
| `issue60_asm_startempty_records` | `{{ _start_ }}` starts an empty record |
| `issue60_asm_end_then_restart` | after `_end_`, lines are dropped until the next start |
| `issue60_asm_add_without_overwrite` | a repeated ordinary line keeps its first value |
| `issue60_asm_lines_before_first_start` | lines before the first start are dropped |
| `issue60_asm_repeated_identical_start` | a repeated identical start line starts a new record |
| `issue60_asm_two_start_patterns` | two `_start_` lines take turns starting records |
| `issue60_asm_table_records` / `issue60_asm_table_first_pattern_wins` | `method="table"` |
| `issue60_asm_failed_second_start_pattern` | a rejected `_start_` line ends the record, and the next start reopens the group |
| `issue60_asm_end_pattern_same_text_as_start` | an `_end_` line with the same text as the start line |
| `issue60_asm_join_after_add` | joinmatches after an ordinary line set the variable |
| `issue60_asm_joinmatches_to_list` | `to_list | joinmatches` |
| `issue60_asm_no_line_gap_limit` | no limit on the distance from the start line (gottp's `maxGapLines` is gottp-only) |
| `issue60_asm_bare_line_variable` | a `{{ _line_ }}` variable keeps its joined lines |
| `issue60_asm_line_condition_skips_line` | a `_line_` line rejected by a condition is skipped, not the rest |

Related issues with nested groups (refactor C2), each a minimal repro from the issue; all pass since C2 except `issue88_itemize_path_argument` (the path argument): `issue88_itemize_child_group`, `issue88_itemize_path_argument`, `issue89_group_with_only_child_groups`, `issue89_anonymous_child_groups`, `issue91_child_group_defaults_without_match`, `issue99_record_and_set_across_groups`, `issue100_void_sibling_startempty_joinmatches` (a reduction of `test_same_config_different_contexts`), `issue118_joinmatches_stops_at_sibling_start`, `issue119_second_start_line_stays_in_own_section`, `issue120_sibling_groups_same_name`.

## Source-map parity

`CompiledTemplate.Parse` and `ParseWithValidation` run one parse pipeline (`Runtime.parse`, `internal/compiled/orchestrator.go`) whether source maps are on or off; with `ParseOptions{EnableSourceMap: true}` it also records the source map, from the matches `parseGroup` actually uses (`internal/compiled/sourcemap_observer.go`). Before refactor A1 (#60) source maps on ran a separate copy of the orchestration that had drifted, and before A2 the source map came from a second, out-of-step matching pass. The editor turns source maps on when its source-map toggle is on. `TestSourceMapParity` parses every case (suite, repros and `testdata/sourcemap-parity`) through `CompiledTemplate.ParseWithValidation` both ways and requires the same data and the same `KeyFields`, or the same error. The source map itself isn't compared here, and it doesn't matter whether the data matches TTP.

`testdata/sourcemap-parity/` holds gottp-only cases for paths the TTP suite doesn't reach: `NAME.ttp` (template) and `NAME.txt` (the `Default_Input` data), named `sourcemap-parity/NAME`. They have no Python TTP expectation; they cover gottp's `keys=` attribute (`KeyFields`) and input functions (an input `macro=`).

`sourcemap_divergences.json` lists the cases that differ, with the same rules as `known_failures.json`: an unlisted divergence, a listed case at parity and a stale entry all fail the test. `TTPSUITE_SOURCEMAP_REPORT=/tmp/sm.json` writes both results of every diverging case.

The table is empty: refactor A1 (#60) fixed the 14 divergences it used to list (#105, #106, #107, #108, plus input functions and `KeyFields`).

## Source-map snapshots

`TestSourceMapSnapshots` parses every source-map parity case with `EnableSourceMap` and compares the source map with `testdata/sourcemap-snapshots/<case>.json`, byte for byte. A snapshot holds one row per input line (`LineNumber`, `Matched`, `Matches` exactly as the public `gottp.SourceMap` marshals to JSON, which is what the editor reads), or the parse error. Any difference fails, and so does a snapshot that names no case. After a deliberate change, rewrite them and review the diff:

```
go test ./test/ttpsuite -run TestSourceMapSnapshots -update-sourcemap-snapshots
git diff test/ttpsuite/testdata/sourcemap-snapshots
```

## Stream snapshots

`TestStreamSnapshots` runs `ParseStream` on every source-map parity case and, for each one the streaming gate accepts, compares the callbacks (record, `srcRange`, `groupPath`, in order) with `testdata/stream-snapshots/<case>.json`. A case the gate rejects has no snapshot, so a snapshot left for one fails, as does a missing snapshot. It also requires the streamed records to be the records `Parse` returns, ignoring order, except for the cases in `streamParityExceptions`, whose `Parse` result also holds outputs, `<vars name>` or lookup data that `ParseStream` never emits. After a deliberate change:

```
go test ./test/ttpsuite -run TestStreamSnapshots -update-stream-snapshots
git diff test/ttpsuite/testdata/stream-snapshots
```

## Regenerating the data

Never edit `testdata/cases/` or the `*.expected.json` files by hand. `record.py` writes them from Python TTP, pinned in `../comparison/requirements.txt`. It needs every package in `requirements.txt` (it exits if one is missing): without the optional formatter and validation packages TTP returns different results for some cases.

```
python3 -m venv /tmp/ttpvenv && /tmp/ttpvenv/bin/pip install -r test/ttpsuite/requirements.txt
/tmp/ttpvenv/bin/python -I test/ttpsuite/record.py expect --check   # exit 1 if an expectation would change
/tmp/ttpvenv/bin/python -I test/ttpsuite/record.py expect           # rewrite the expectations
```

`record.py extract --ttp-src DIR` rebuilds `testdata/` from scratch from a TTP checkout: it runs TTP's pytest suite under the `ttprec.py` recorder, keeps the auto-portable records and copies the assets they read. Use the PyPI 0.10.1 release, which is commit `1da3a76` of dmulyalin/ttp (the git tag `v0.10.1` points at a different commit). Run `expect --check` afterwards; it confirms that replaying the cases reproduces the recorded results.

A record is **auto-portable** when the test uses one `ttp` object, calls only `__init__`/`add_input`/`parse`/`result`, calls `result()` with no arguments, adds string inputs with unique names, and gets a JSON-serializable result.

## Failures by issue

Cases per issue (first strict difference), and how many more cases hit the issue later (`also`):

| Issue | Cases | Also | Title |
|---|---|---|---|
| #68 | 13 |  | _headers_ (and columns) indicator is unimplemented |
| #67 | 12 |  | Raw-string arguments r"…" silently match nothing |
| #63 | 10 | 1 | set(True) / set(False) (and any literal argument) is a string |
| #80 | 9 | 2 | Missing output features |
| #64 | 6 |  | A variable with default() captures the leading space |
| #74 | 6 |  | <extend> inside a <group> is ignored |
| #93 | 6 |  | <extend>: <vars> from the extended file and the groups=/vars=/inputs=/lookups= filters are not applied |
| #98 | 6 |  | Formatter output from <output> gets an extra list level |
| #75 | 5 |  | Group-level lookup= attribute is ignored (replace, add_field, update) |
| #70 | 4 | 1 | Several inline <input load="text"> tags without names: only the first is parsed |
| #97 | 4 |  | A group macro that returns None loses its in-place changes, and the next chained macro gets None |
| #110 | 4 |  | Group-specific output= results are merged into the main result instead of added as a separate item |
| #112 | 4 |  | A chain stored in <vars> as a list is ignored (group chain= and chain()) |
| #73 | 3 | 1 | Getters: gethostname through set(), and get_date/get_time/getfilename/get_timestamp* missing |
| #88 | 3 |  | itemize on a child group drops its values, and the path argument is ignored |
| #94 | 3 |  | <input url=...> pointing at a directory loads nothing |
| #91 | 2 | 1 | A group with no matching lines does not emit its default values |
| #113 | 2 | 1 | macro() inside a group chain= is not run |
| #72 | 2 |  | Group to_int has no float fallback |
| #76 | 2 |  | A dynamic-path value containing . is split into path segments |
| #77 | 2 |  | Nested <template> children are merged into one result |
| #96 | 2 |  | include= on <vars> and on a CSV <lookup> does not load the file |
| #100 | 2 |  | Nested-section templates from TTP answers diverge; root cause not isolated |
| #65 | 1 | 2 | The default() value isn't filled in when the line is absent |
| #101 | 1 | 1 | <output> with its settings in the tag body is ignored |
| #27 | 1 |  | Without a <template> wrapper, only the first top-level group's results are returned |
| #66 | 1 |  | copy() is a stub |
| #71 | 1 |  | let() on a matched line is overridden by a later set() |
| #87 | 1 |  | Group path with several {{var}} segments written without spaces is not substituted |
| #90 | 1 |  | A multi-line <vars> dict with a trailing comma is not parsed |
| #92 | 1 |  | to_ip does not accept "address netmask" |
| #95 | 1 |  | A template given as a file name is not loaded from TTP_TEMPLATES_DIR |
| #99 | 1 |  | record()/set= across groups: the recorded value is not set and the first child record is lost |
| #102 | 1 |  | format_attributes is ignored (tabulate tablefmt) |
| #114 | 1 |  | Group validate= attribute is not applied |

#62 (top-level anonymous group loses one list level) was the first difference in 65 cases. Its fix made 26 of them pass; the other 39 are listed under the issue they hit next, which for 11 of them is one of four new issues (#110, #112, #113, #114).

## Python macros (design)

23 cases use `<macro>`. gottp runs `<macro>` code as Starlark unless the tag says `language="python"` (`internal/parser/template.go`, `parseMacro`), and TTP's templates never set it. 2 of the 23 pass (`test_group_macro_returns_none` and `test_group_macro_returns_true`).

- **4 are skipped as `design: python macro`**: their macro code isn't valid Starlark (`import re`/`import ipaddress`, f-strings, `_ttp_["match"]["to_ip"]` with the `ipaddress` object API).
- The other 17 fail first on a gottp bug and are tracked by that issue: #97 (a group macro that returns `None` loses its changes), #80 (`validate_yangson`), #113 (`macro()` inside a group `chain=`) and others. Two of them also contain Python-only macro code that would fail next: `bin()` in `test_answers_and_docs/test_interface_template_not_collecting_all_data_solution` and a generator expression inside `any()` in `test_misc/test_tail_match_no_group_functions_processing`.

The `python` build tag doesn't help as it stands. `go build -tags python ./...` fails: cgo can't use CPython's type-check macros (`could not determine what C.PyBool_Check refers to` and five more in `internal/macro/python.go`), and `python_init.go` redefines `initPythonEngine`, which `starlark.go` already defines. Even when it builds, these templates would still run as Starlark, because they don't set `language="python"`. Whether TTP-style `<macro>` should run as Python is a policy decision for #60.

## Not ported

TTP 0.10.1's `test/pytest` has 288 tests. The recorder writes 286 records (2 tests are skipped upstream); 200 are auto-portable and live here. The other 86 need parser APIs or arguments the Python-style API doesn't have:

| Bucket | Records |
|---|---|
| `result(structure=…)` / `returner=` / `format=` kwargs | 15 |
| no `result()` call (tests that assert errors or logs) | 15 |
| `add_input(template_name=…/groups=…)` | 13 |
| `get_input_load` / `get_template` / `clear_result` / `set_input` / `add_template` | 12 |
| `add_function` (Python callables) | 11 |
| same input name added twice | 6 |
| several parser objects | 5 |
| failing upstream (need `ttp_templates` or the filesystem) | 5 |
| `base_path` / `dynamic_path_key_to_int` constructor arguments | 2 |
| non-string data | 1 |
| `parse()` arguments | 1 |

(The research report counts 16 for the first bucket and 3 for "other"; one test has both `parse()` arguments and `result()` kwargs and is counted once here.) With the packages in `requirements.txt` only, `extract` sees 283 records because 3 upstream tests skip without `ttp_templates`; the 200 auto-portable cases are the same.

## Licence

The templates, inputs and assets in `testdata/` come from TTP's test suite, which is MIT-licensed (Copyright (c) 2020 apraksim). The licence is in `testdata/LICENSE-TTP`.
