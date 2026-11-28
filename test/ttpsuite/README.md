# TTP 0.10.1 test suite

Python TTP 0.10.1's own test suite (`test/pytest` in [dmulyalin/ttp](https://github.com/dmulyalin/ttp)), replayed through gottp. This is the compliance measure for the baseline (#60).

```
go test ./test/ttpsuite -v -run TestTTPSuite | grep 'suite:'
```

Current result (gottp `718c77b`):

```
TTP 0.10.1 suite: pass 22 / skipped-by-issue 174 / design 4 of 200
```

The Go test needs no Python, so it runs in plain `go test ./...` and in CI.

## How it works

- `testdata/cases/<module>.json` holds 200 cases, one file per TTP test module. Each case has the template, the inputs, the TTP test it came from (`source`) and the result Python TTP 0.10.1 returned (`expected`).
- `ttpsuite_test.go` runs each case as a subtest through gottp's Python-style API (`NewParser`, `AddTemplate`, `AddInput`, `Parse`, `Result`) with the working directory set to `testdata/pytest`, so the templates' `./assets/...` paths resolve. The result is compared **strictly**: there is no list-unwrapping normalization (unlike `test/comparison`), because that normalization hides #62.
- `known_failures.json` lists every case that doesn't match yet. `issue` is the issue its first strict difference belongs to, `also` lists issues the case hits after that, and `reason` is one line. A reason starting with `design:` marks a deliberate difference.
- The test fails when a case differs and isn't listed, when a listed case passes ("remove it from known_failures.json"), and when the table names a case that doesn't exist. So the table can't go stale: fixing an issue makes its cases fail until their entries are removed.
- `TTPSUITE_REPORT=/tmp/report.json go test ./test/ttpsuite` writes every case's outcome, first difference and gottp result, for triage.

## Regenerating the data

Never edit `testdata/cases/` by hand. `record.py` writes it from Python TTP, pinned in `../comparison/requirements.txt`:

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
| #62 | 65 |  | Top-level anonymous group loses one list level (and drops other inputs) |
| #68 | 13 |  | _headers_ (and columns) indicator is unimplemented |
| #28 | 9 |  | Parent group whose _start_ line has no variables drops its child groups |
| #80 | 9 |  | Missing output features |
| #67 | 8 |  | Raw-string arguments r"…" silently match nothing |
| #30 | 6 |  | Indented template lines match data at any indentation |
| #74 | 6 |  | <extend> inside a <group> is ignored |
| #93 | 6 |  | <extend>: <vars> from the extended file and the groups=/vars=/inputs=/lookups= filters are not applied |
| #75 | 5 |  | Group-level lookup= attribute is ignored (replace, add_field, update) |
| #88 | 4 |  | itemize on a child group drops its values, and the path argument is ignored |
| #70 | 3 |  | Several inline <input load="text"> tags without names: only the first is parsed |
| #73 | 3 |  | Getters: gethostname through set(), and get_date/get_time/getfilename/get_timestamp* missing |
| #91 | 3 | 1 | A group with no matching lines does not emit its default values |
| #94 | 3 |  | <input url=...> pointing at a directory loads nothing |
| #97 | 3 | 1 | A group macro that returns None loses its in-place changes, and the next chained macro gets None |
| #98 | 3 |  | Formatter output from <output> gets an extra list level |
| #100 | 3 |  | Nested-section templates from TTP answers diverge; root cause not isolated |
| #35 | 2 |  | strip/lstrip/rstrip ignore their characters argument |
| #76 | 2 |  | A dynamic-path value containing . is split into path segments |
| #77 | 2 |  | Nested <template> children are merged into one result |
| #79 | 2 |  | _line_ with joinmatches also captures the _start_ line |
| #89 | 2 |  | A group that holds only child groups returns nothing |
| #96 | 2 |  | include= on <vars> and on a CSV <lookup> does not load the file |
| #99 | 2 |  | record()/set= across groups: the recorded value is not set and the first child record is lost |
| #63 | 1 | 7 | set(True) / set(False) (and any literal argument) is a string |
| #64 | 1 | 5 | A variable with default() captures the leading space |
| #87 | 1 |  | Group path with several {{var}} segments written without spaces is not substituted |
| #90 | 1 |  | A multi-line <vars> dict with a trailing comma is not parsed |
| #92 | 1 |  | to_ip does not accept "address netmask" |
| #95 | 1 |  | A template given as a file name is not loaded from TTP_TEMPLATES_DIR |
| #101 | 1 |  | <output> with its settings in the tag body is ignored |
| #102 | 1 |  | format_attributes is ignored (tabulate tablefmt) |
| #66 | 0 | 1 | copy() is a stub |
| #69 | 0 | 3 | A _start_ line rejected by a condition doesn't close the previous record |
| #71 | 0 | 1 | let() on a matched line is overridden by a later set() |
| #72 | 0 | 2 | Group to_int has no float fallback |

Of the 65 #62 cases, 25 fail on #62 alone; the other 40 fail on something else as well once the list level is fixed (the `reason` says what).

## Python macros (design)

23 cases use `<macro>`. gottp runs `<macro>` code as Starlark unless the tag says `language="python"` (`internal/parser/template.go`, `parseMacro`), and TTP's templates never set it. None of the 23 passes.

- **4 are skipped as `design: python macro`**: their macro code isn't valid Starlark (`import re`/`import ipaddress`, f-strings, `_ttp_["match"]["to_ip"]` with the `ipaddress` object API).
- The other 19 fail first on a gottp bug and are tracked by that issue: #62 for most, #97 (a group macro that returns `None` loses its changes), #80 (`validate_yangson`) and others. Two of them also contain Python-only macro code that would fail next: `bin()` in `test_answers_and_docs/test_interface_template_not_collecting_all_data_solution` and a generator expression inside `any()` in `test_misc/test_tail_match_no_group_functions_processing`.

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
