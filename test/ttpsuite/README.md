# TTP 0.10.1 test suite

Python TTP 0.10.1's own test suite (`test/pytest` in [dmulyalin/ttp](https://github.com/dmulyalin/ttp)), replayed through gottp. This is the compliance measure for the baseline (#60).

Current result (gottp `4065098` plus the #62 fix):

```
go test ./test/ttpsuite -v | grep -E 'suite:|repros:|parity:'
```

```
TTP 0.10.1 suite: pass 48 / skipped-by-issue 148 / design 4 of 200
gottp repros: pass 1 / skipped-by-issue 9 / design 0 of 10
source-map parity: same 199 / diverge 11 of 210
```

The Go tests need no Python, so they run in plain `go test ./...` and in CI. There are three:

- `TestTTPSuite`: TTP's own 200 cases against TTP 0.10.1's results (below).
- `TestGottpRepros`: gottp bug repros against TTP 0.10.1's results ([gottp repros](#gottp-repros)).
- `TestSourceMapParity`: every case from both, parsed with source maps off and on, must give the same data ([Source-map parity](#source-map-parity)).

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
| `issue28_parent_start_line_without_variables` | #28 |
| `issue29_joinmatches_on_start_line` | #29 |
| `issue30_indented_line_matches_any_indentation` | #30 |
| `issue31_ignore_vars_name` | #31 |
| `issue32_one_line_fills_several_template_lines` | #32 |
| `issue33_trailing_ignore_with_trailing_whitespace` | #33 |
| `issue34_split_joinmatches` | #34 |
| `issue35_strip_characters_argument` | #35 |
| `issue36_unknown_function_falls_back_to_str_method` | #36 |
| `sourcemap_void_group` | passes; covers `void` for source-map parity (#108) |

## Source-map parity

`CompiledTemplate.Parse` runs `Runtime.Parse` when source maps are off, and a separate copy of the orchestration (`Runtime.ParseWithSourceMap` and `parseGroupWithSourceMap`) when `ParseOptions{EnableSourceMap: true}`. The editor always turns source maps on. `TestSourceMapParity` parses every case (suite and repros) through `CompiledTemplate.ParseWithValidation` both ways and requires the same data, or the same error. The source map itself isn't compared, and it doesn't matter whether the data matches TTP.

`sourcemap_divergences.json` lists the cases that differ, with the same rules as `known_failures.json`: an unlisted divergence, a listed case at parity and a stale entry all fail the test. `TTPSUITE_SOURCEMAP_REPORT=/tmp/sm.json` writes both results of every diverging case.

| Issue | Cases | Title |
|---|---|---|
| #105 | 5 | Source map: a group without input= parses only Default_Input (other inputs give an empty result) |
| #106 | 4 | Source map: group errors lose the "failed to parse group <name>" context |
| #107 | 1 | Source map: the ignore("<vars name>") empty-record special case is missing, so the group is dropped |
| #108 | 1 | Source map: void groups are not dropped from the results |

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
| #67 | 10 |  | Raw-string arguments r"…" silently match nothing |
| #28 | 9 |  | Parent group whose _start_ line has no variables drops its child groups |
| #80 | 9 | 2 | Missing output features |
| #63 | 8 |  | set(True) / set(False) (and any literal argument) is a string |
| #30 | 6 |  | Indented template lines match data at any indentation |
| #64 | 6 |  | A variable with default() captures the leading space |
| #74 | 6 |  | <extend> inside a <group> is ignored |
| #93 | 6 |  | <extend>: <vars> from the extended file and the groups=/vars=/inputs=/lookups= filters are not applied |
| #98 | 6 |  | Formatter output from <output> gets an extra list level |
| #75 | 5 |  | Group-level lookup= attribute is ignored (replace, add_field, update) |
| #70 | 4 | 1 | Several inline <input load="text"> tags without names: only the first is parsed |
| #88 | 4 |  | itemize on a child group drops its values, and the path argument is ignored |
| #97 | 4 |  | A group macro that returns None loses its in-place changes, and the next chained macro gets None |
| #110 | 4 |  | Group-specific output= results are merged into the main result instead of added as a separate item |
| #112 | 4 |  | A chain stored in <vars> as a list is ignored (group chain= and chain()) |
| #69 | 3 |  | A _start_ line rejected by a condition doesn't close the previous record |
| #73 | 3 | 1 | Getters: gethostname through set(), and get_date/get_time/getfilename/get_timestamp* missing |
| #79 | 3 |  | _line_ with joinmatches also captures the _start_ line |
| #91 | 3 | 1 | A group with no matching lines does not emit its default values |
| #94 | 3 |  | <input url=...> pointing at a directory loads nothing |
| #99 | 3 |  | record()/set= across groups: the recorded value is not set and the first child record is lost |
| #100 | 3 |  | Nested-section templates from TTP answers diverge; root cause not isolated |
| #35 | 2 |  | strip/lstrip/rstrip ignore their characters argument |
| #72 | 2 |  | Group to_int has no float fallback |
| #76 | 2 |  | A dynamic-path value containing . is split into path segments |
| #77 | 2 |  | Nested <template> children are merged into one result |
| #89 | 2 |  | A group that holds only child groups returns nothing |
| #96 | 2 |  | include= on <vars> and on a CSV <lookup> does not load the file |
| #113 | 2 | 1 | macro() inside a group chain= is not run |
| #66 | 1 |  | copy() is a stub |
| #71 | 1 |  | let() on a matched line is overridden by a later set() |
| #87 | 1 |  | Group path with several {{var}} segments written without spaces is not substituted |
| #90 | 1 |  | A multi-line <vars> dict with a trailing comma is not parsed |
| #92 | 1 |  | to_ip does not accept "address netmask" |
| #95 | 1 |  | A template given as a file name is not loaded from TTP_TEMPLATES_DIR |
| #101 | 1 | 1 | <output> with its settings in the tag body is ignored |
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
