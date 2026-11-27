# gottp vs Python TTP 0.10.1: compatibility research

Status: research only. Nothing here changes gottp code. Written 2026-11-27 against gottp `27b1e4f` (v0.1.13) for the baseline tracked in roc-ops/gottp#60 (milestone "Baseline v0.1.14").

Notation:

- `T:` means a Python TTP 0.10.1 source location, at the **release commit** `1da3a76` (see §2.1), e.g. `T:ttp/ttp.py:530` = <https://github.com/dmulyalin/ttp/blob/1da3a76c998603afd2d647f43420243c1e26cf99/ttp/ttp.py#L530>.
- `G:` means a gottp location in this repo at `27b1e4f`.
- "Repro N" refers to §5. Each repro was run through Python TTP 0.10.1 (PyPI wheel) and through gottp's Python-style API (`gottp.NewParser()`, `G:gottp.go:600`).

## TL;DR

1. **The `0.10.1` git tag is not the 0.10.1 release.** The tag points at `1816d93` (pyproject still says 0.10.0). The PyPI 0.10.1 wheel matches commit `1da3a76` ("bunp to 0.10.1"), which adds one fix: nested named templates loaded with `<extend>` (§2.1).
2. **0.9.5 → 0.10.1 changes almost nothing in parsing behaviour.** Apart from that `<extend>` fix, the diff removes Python 2 code paths (§2.2). On TTP's own test suite, 0.9.5 passes 278 of 288 tests and 0.10.1 passes 281. The 3-test difference is exactly the new nested-extend tests.
3. **The comparison suite already runs against 0.10.x when `ttp-original/` exists, and skips when TTP is missing.** `python_runner.py` puts the gitignored `ttp-original/` checkout first on `sys.path`, so the installed TTP doesn't matter when that directory is present. Without TTP, every comparison test calls `t.Skip` and `go test` reports `ok` (§3).
4. **Results under 0.9.5 vs 0.10.1:** with a clean TTP cache, both give 105 pass, 0 fail, 14 skip (top-level tests). The "0.9.5" install on this machine is broken by a stale `ttp_dict_cache.pickle` left behind by a 0.10.x install, which produces 33 false failures. One test, `TestGroupFunctionSetExample`, is flaky in every configuration because gottp's output order is nondeterministic (§3.4).
5. **The suite overstates compatibility.** I replayed 200 of TTP's own tests through gottp automatically. 21 pass strictly, 48 pass after the suite's own normalization, and 145 fail (§4). The suite's `normalizeJSON` unwraps single-element lists, which hides a structural difference in every result with a top-level anonymous group (Repro 1).
6. **I found 19 incompatibilities that aren't filed** (§5). Each has a minimal repro. All 11 open template-engine issues (#23, #27–#36) still reproduce. However, for **#23, Python TTP 0.10.1 gives the same output as gottp**, so #23 is not a TTP divergence (§5.0).

---

## 1. Feature matrix

Legend: ✅ supported (exercised, and output matches TTP) · ⚠️ differs · ❌ missing · ➖ present but not verified here. "Filed" names the open issue that covers the difference.

### 1.1 Template tags

| Tag | TTP 0.10.1 | gottp | Status | Notes |
|---|---|---|---|---|
| `<template>` (+ `name`, `base_path`, `results`, `pathchar`) | `T:ttp/ttp.py:1102-1120` | `G:internal/parser/template.go:396-403` | ⚠️ | Nested child `<template>`s are merged into one result; TTP returns one result per child template (Repro 16). Unwrapped multi-group templates lose groups: **filed #27**. |
| `<group>` / `g` / `grp` | `T:ttp/ttp.py:1137-1139` | `G:internal/parser/template.go:447` | ⚠️ | See §1.2 and §1.3. |
| `<input>` / `i` / `in` | `T:ttp/ttp.py:1143-1145` | `G:internal/parser/template.go:454` | ⚠️ | Several unnamed inline inputs: TTP keeps every one (`T:ttp/ttp.py:1044`, `:508-516`); gottp parses only the first (Repro 9). |
| `<output>` / `o` / `out` | `T:ttp/ttp.py:1140-1142` | `G:internal/parser/template.go:461` | ⚠️ | See §1.6. |
| `<vars>` / `v` / `variables` | `T:ttp/ttp.py:1134-1136` | `G:internal/parser/template.go:442` | ⚠️ | Getters are mostly missing (§1.8). `set(True)` gives a string (Repro 2). |
| `<lookup>` | `T:ttp/ttp.py:1146` | `G:internal/parser/template.go:468` | ⚠️ | Match-level `lookup` passes the suite. The group-level `lookup=` attribute is ignored (Repro 14). |
| `<macro>` | `T:ttp/ttp.py:1158` (`parse_macro`, Python `exec`) | `G:internal/parser/template.go:475`, `G:internal/macro/starlark.go`, `python_stub.go` | ⚠️ | gottp runs macros as Starlark by default, and Python only with `-tags python` (`G:docs/PYTHON_MACROS.md`). 14 of the replayed TTP tests that use Python macros fail, and 4 error out (§4.3). This is a deliberate design difference. |
| `<doc>` | `T:ttp/ttp.py:1149` | `G:internal/parser/template.go:482` | ✅ | Ignored on both sides (§5 "doc-tag" check). |
| `<extend>` | `T:ttp/ttp.py:1315-1385` | `G:internal/parser/template.go:485`, `G:internal/compiler/extend.go:36` | ⚠️ | `<extend>` inside a `<group>` is ignored. `G:internal/parser/template.go:536-546` only recurses into child groups (Repro 13). 14 of 18 replayed extend tests fail. TTP 0.10.1 added nested named templates through extend (§2.1). |
| `answers` tag | does not exist. The tag table at `T:ttp/ttp.py:1133-1150` has no `answers`. `test_answers_and_docs.py` is only a test-file name. | — | n/a | |

### 1.2 Group attributes

TTP special attributes: `T:ttp/ttp.py:1780-1788` (`method`, `input`, `output`, `name`, `default`, `chain`, `functions`). Any other attribute is a group function (`T:ttp/ttp.py:1790-1794`). gottp: `G:internal/parser/template.go:511-528` and `G:internal/compiled/runtime.go:3661`, `:3885-3892`.

| Attribute | Status | Evidence |
|---|---|---|
| `name` (paths, `*`, `**`, dynamic `{{ }}`) | ⚠️ | A dynamic-path value that contains `.` is split on the dot (Repro 15). |
| `method="table"` | ➖ | Suite passes. |
| `input`, `output` | ⚠️ | Groups that read named inputs loaded from files or dirs fail in the replay (`test_inputs.py`: 3 of 4 fail, §4). Not isolated further. |
| `default` | ➖ | The suite passes `TestGroupAttributeDefault*`. Replays of `test_group_default_referencing_variable_*` fail. Not isolated. |
| `chain` / `functions` | ⚠️ | All 4 replayed chain tests fail, and 3 of them use Python macros. |
| `void`, `del`, `contains`, `containsall`, `exclude`, `excludeall`, `equal`, `contains_val`, `exclude_val`, `itemize`, `expand`, `items2dict`, `record`, `set`, `sformat`, `to_ip`, `validate`/`cerberus`, `macro` | ➖ / ✅ | All registered (`G:internal/functions/group/functions.go:40-58`). `del` and `contains` match TTP (§5 checks). |
| `to_int` | ⚠️ | No float fallback: TTP turns `"9200.0"` into `9200.0` (`T:ttp/group/to_converters.py:10-28`), but gottp leaves the string (Repro 11). |
| `lookup` (group function) | ❌ | `T:ttp/group/lookup.py:6`. gottp has no group-scope `lookup` (`G:internal/functions/group/functions.go:40-58`), and unknown attributes fall through silently (`G:internal/compiled/runtime.go:3885-3892`) (Repro 14). |
| `str_to_unicode` | ❌ (no-op in TTP for py3) | `T:ttp/group/to_converters.py:6`. Not registered in gottp, but it has no effect on Python 3. |

### 1.3 Match-variable functions and indicators

The full name-by-name cross-check of `T:ttp/match/*.py` (including `_name_map_` aliases) against gottp's registry is in §6. All 53 TTP match functions have a registered name in gottp. Differences in behaviour:

| Item | TTP | gottp | Status |
|---|---|---|---|
| Argument parsing | Python `eval` of the argument list (`T:ttp/utils/get_attributes.py:58-60`), so `True`/`False`/numbers are typed and `r"..."` works | `G:internal/compiled/runtime.go:6400-6415`: args are `[]string`, no literal typing, no raw-string prefix | ⚠️ Repro 2, Repro 6 |
| `_start_`, `_end_` | `T:ttp/ttp.py:2123-2124` | `G:internal/compiled/runtime.go:1569` | ⚠️ #28, #29. A `_start_` line rejected by a condition doesn't close the previous record (Repro 8). |
| `_line_` | `T:ttp/ttp.py:2049-2055`, pattern `T:ttp/patterns/get_pattern.py:9` | `G:internal/pattern/engine.go:20` | ⚠️ The start line also feeds `_line_` (Repro 18 / related to **#32**). |
| `_exact_`, `_exact_space_` | `T:ttp/ttp.py:2126-2127`, `:2178-2186` | `G:internal/compiled/runtime.go:4940` | ✅ for `_exact_` (§5 check). Indentation handling: **filed #30**. |
| `_headers_`, `columns` | `T:ttp/ttp.py:2115-2121`, `:2239`, docs `Match Variables/Indicators.rst:382` | **no occurrence anywhere in `internal/`** | ❌ Repro 7. All 13 replayed headers tests return nothing. |
| `ignore` | `T:ttp/ttp.py:2056`, `:2206` | ➖ | ⚠️ `ignore(r"…")` matches nothing (Repro 6). `ignore("<var>")`: **#31**. Trailing `ignore(".*")`: **#33**. |
| `set` | `T:ttp/match/set_.py:4-16` | `G:internal/functions/match/set.go:4` | ⚠️ Booleans come out as strings (Repro 2). `let` after `set` (Repro 10). |
| `default` | `T:ttp/ttp.py:2037-2044` | `G:internal/compiled/runtime.go:3986` (group); match-level ➖ | ⚠️ The capture picks up the leading space, and the default isn't applied when the line is absent (Repro 3, Repro 4). |
| `let` | `T:ttp/match/let.py:1-6` | `G:internal/functions/match/more.go:24` | ⚠️ Repro 10 |
| `copy` | `T:ttp/match/copy_.py:6-7` (returns `new_field`) | `G:internal/functions/match/more.go:634-638`: a "for now, just return the value" stub | ❌ Repro 5 |
| `joinmatches` | `T:ttp/match/joinmatches.py:1` | `G:internal/functions/match/more.go:26` | ⚠️ **#29, #34** |
| `strip`/`lstrip`/`rstrip` and other str methods | `getattr(str, name)` fallback | registry only | ⚠️ **#35, #36** |
| `re("A\|B")` | splits on `\|` without regard to quotes (`T:ttp/utils/get_attributes.py:41`) | same | ✅ The same as TTP. See §5.0 on #23. |
| `contains_re`/`startswith_re`/... with vars | `T:ttp/ttp.py:2135-2144` | `G:internal/functions/match/conditions.go:46-65` | ✅ with plain strings, ⚠️ with `r""` |

### 1.4 Regex shortcuts

`T:ttp/patterns/get_pattern.py:1-11` vs `G:internal/pattern/engine.go:12-22` and `:105-120`. `PHRASE`, `ROW`, `ORPHRASE`, `DIGIT`, `IP`, `PREFIX`, `IPV6`, `PREFIXV6`, `_line_`, `WORD` and `MAC` are all present ✅. gottp makes TTP's capturing groups non-capturing (`(?:…)`). No difference showed up in the replays.

### 1.5 Input functions and loaders

TTP: `T:ttp/input/commands.py:4` (`extract_commands`), `T:ttp/input/test.py:1`, `T:ttp/input/macro.py:1`. gottp: `G:internal/functions/input/functions.go:41-42`, `G:internal/parser/template.go:564-590`. All three are present ➖. Loaders (`load="text|python|yaml|json|ini|csv"`, `url`, `extensions`, `filters`) are present in `G:internal/input/loader.go`. The suite skips `text_loader_with_patterns`, `file_loader_with_patterns`, `directory_loader`, `url_loader` and `database_loader` (`G:test/comparison/comparison_test.go:320-336`).

### 1.6 Outputs: functions, formatters, returners

| Item | TTP | gottp | Status |
|---|---|---|---|
| Output functions `traverse`, `dict_to_list`, `is_equal`, `macro`, `validate` (cerberus) | `T:ttp/output/transform.py:6,44`, `is_equal.py:1`, `validate_cerberus.py:32` | `G:internal/functions/output/functions.go:43-45` | ➖ |
| `deepdiff` | `T:ttp/output/deepdiffer.py:18` | not registered | ❌ The 3 deepdiff tests fail. |
| `validate_yangson` | `T:ttp/output/validate_yangson.py:142` | not registered. gottp uses its own `yang=` validator instead. | ❌ (different design; see YANG issues #50–#59) |
| Formatters `raw`, `json`, `pprint`, `yaml`, `table`, `csv`, `tabulate`, `excel`, `jinja2`, `n2g` | `T:ttp/formatters/*.py` | `G:internal/formatters/*.go` | ⚠️ `csv` renders only the first row when the top group is anonymous, because of Repro 1. `n2g` errors with "unknown output format: n2g". `jinja2` errors with "requires template content". The suite skips 6 formatters. |
| Returners `self`, `file`, `terminal`, `syslog` | `T:ttp/returners/*.py` | `G:internal/returners/*.go` | ➖ (not replayed: the tests pass `returner=` to `result()`) |

### 1.7 Results structures and the Python API

| Item | TTP | gottp | Status |
|---|---|---|---|
| `per_input` (default) | `T:ttp/ttp.py:508-516` | `G:internal/parser/template.go:381`, `G:internal/compiled/runtime.go:664-745` | ⚠️ A top-level anonymous group loses a nesting level and drops the other inputs (Repro 1). |
| `per_template` | `T:ttp/ttp.py:517-528` | `G:internal/compiled/runtime.go:644`, `:739` | ⚠️ With several inline inputs, gottp keeps only the first (Repro 9b). |
| `result(structure="dictionary" \| "flat_list")`, `templates=`, `format=`, `returner=` | `T:ttp/ttp.py:530-665` | `G:api/python/parser.go:92-95`: `Result()` takes no arguments; neither `flat_list` nor `dictionary` occurs anywhere in gottp | ❌ |
| `add_input` with the same input name twice | appends a dataset (`T:ttp/ttp.py:1044`) | overwrites (`G:api/python/parser.go:49-54`) | ⚠️ |
| `add_function`, `add_lookup`, `get_input_load`, `set_input`, `clear_input`, `get_template`, `clear_result(templates=)` | `T:ttp/ttp.py:284,315,378,666,692,713,803` | not in `G:api/python/parser.go`. The Go API has `ParseOptions.Lookups` and `Functions` (`G:gottp.go:463-480`). | ❌ (Python API only) |
| `ttp(..., dynamic_path_key_to_int=True)` | `__init__` kwarg | none | ❌ |

### 1.8 Lookup tables and getters

- Lookup tables (`<lookup>` with `load=ini|csv|python|yaml|json`, `include=`), and the match functions `lookup`, `rlookup` and `gpvlookup`, are present (`G:internal/functions/match/more.go:16,40,44`). The suite passes them under 0.10.1. Replays: `test_lookups.py` 1 of 2 passes (`include` of a CSV fails). Group lookup is ❌ (Repro 14). `geoip2` is partial (`G:test/comparison/KNOWN_DIFFERENCES.md`, "GeoIP Lookup").
- Getters (`T:ttp/variable/*.py`, docs `Template Variables/Getters.rst:12-22`): `gethostname`, `getfilename`, `get_time`, `get_date`, `get_timestamp`, `get_timestamp_ms`, `get_time_ns`, `get_timestamp_iso`. gottp resolves only `gethostname` (`G:internal/compiled/runtime.go:7785`), and only on the named-`<vars>` path (`:621-630`). Through `set(var)` the literal `"gethostname"` comes out. The other 7 getters don't exist ❌ (Repro 12).

---

## 2. 0.9.5 → 0.10.1 changes

### 2.1 Which commit is "0.10.1"?

| Ref | Commit | `pyproject.toml` version | Date |
|---|---|---|---|
| tag `0.9.5` | `638fa9d` | 0.9.5 | — |
| tag `0.10.0` | `c12a4bd` | 0.10.0 | 2025-11-02 |
| tag `0.10.1` | `1816d93` ("fixing docs build") | **0.10.0** | 2025-11-02 |
| PyPI 0.10.1 wheel = commit `1da3a76` ("bunp to 0.10.1") | `1da3a76` | 0.10.1 | 2026-03-21 (PyPI upload 2026-03-21T04:25) |

Evidence: <https://pypi.org/project/ttp/0.10.1/>. `diff -r --strip-trailing-cr` of the installed wheel against `1da3a76:ttp/` is empty. Against tag `0.10.1:ttp/` it differs in `ttp/ttp.py` (the `<extend>` nested-template branch, `T:ttp/ttp.py:1354-1360`) and in black-reformatting of `ttp/output/validate_yangson.py`. Upstream master has 2 more commits after `1da3a76`: `93d743a` (tests/docs) and `c3134c4` (drops Python 3.9).

Local copies: gottp's gitignored `ttp-original/` (`G:.gitignore:76`) is checked out at **`1816d93`, the tag, which is 0.10.0 code**. So the comparison suite is not quite running against 0.10.1.

### 2.2 What changed in behaviour (`git diff 0.9.5 1da3a76 -- ttp`)

1. **Nested named templates through `<extend>`** (`e3e344a`, `T:ttp/ttp.py:1354-1360`). If an extended file is itself a `<template name=…>`, it now becomes a nested template instead of being flattened into the parent. New tests: `test_extend_tag.py::test_extend_tag_from_file_with_template_tag_multiple`, `test_ttp_parser_methods.py::test_input_load_with_extend_and_nested_templates[_with_inputs]`.
2. Python 2 removal (`2076bd9`): `str_to_unicode`/`to_unicode` become unconditional no-ops (`T:ttp/group/to_converters.py:6`, `T:ttp/match/to.py:36`), `load_python_exec_py3.py` is renamed to `load_python_exec.py`, and the py2 branches go from `ttp/utils/loaders.py` (ini loader now only uses `configparser.read_string`), `ttp/match/ip.py` and `ttp/returners/terminal_returner.py`. No behaviour change on Python 3.
3. `ast` `.s` → `.value` in `lazy_import_functions` (`427fbc4`, `T:ttp/ttp.py:148`). This is for the Python 3.12+ deprecation. No behaviour change.
4. Minimum Python is now 3.9, and the optional extras were bumped (jinja2 3.1, deepdiff 8.6, tabulate 0.9, yangson 1.6, n2g 0.3). Formatter output could change through those library versions, but all of those formatters are skipped in gottp's suite.
5. Docs only: `to_int` documents `intlist=True`. That code already existed in 0.9.5.

**Measured:** TTP's own suite (288 tests collected) under 0.9.5 with a clean cache: 278 passed, 8 failed, 2 skipped. Under 0.10.1: 281 passed, 5 failed, 2 skipped. The extra 0.9.5 failures are exactly the 3 nested-extend tests. The 5 failures in both need `ttp_templates` content or depend on the filesystem.

**What matters for gottp:** only item 1. gottp's suite has no nested-named-template extend case, so moving from 0.9.5 to 0.10.1 doesn't change any comparison expectation (§3.3 confirms this empirically).

---

## 3. Comparison-suite health

### 3.1 How it picks TTP

- It uses `python3` from `PATH` (`G:test/comparison/comparison_test.go:33`, `:59`). `python_runner.py` first does `sys.path.insert(0, <repo>/ttp-original)` (`G:test/comparison/python_runner.py:17`). **So when `ttp-original/` exists, it wins over the installed TTP.** The installed version only matters when that directory is missing, as in a fresh clone or CI, since it's gitignored.
- The availability probe runs `python3 -c "…sys.path.insert(0,'ttp-original'); from ttp import ttp"` (`G:test/comparison/comparison_test.go:39-41`).
- Expectations aren't stored. Every test runs Python live (`G:test/comparison/comparison_test.go:351`). So the expectations come from whichever TTP gets imported. Here that's `ttp-original` @ `1816d93` (0.10.0 code).
- `test/comparison/TEST_COVERAGE.md` does not exist. The brief mentions it, but no file by that name is in the repo.

### 3.2 When TTP is missing: skip, not fail

`RunComparison` and the tests that use `pythonTTPAvailable()` directly (124 call sites) call `t.Skip` (`G:test/comparison/comparison_test.go:345-348`). I checked this with a `python3` that has no TTP: every comparison test skips, and `go test ./test/comparison/` prints `ok`. So "`go test ./...` passes" (the #60 body) says nothing about TTP parity unless TTP is importable.

### 3.3 Runs (`go test ./test/comparison/ -count=1 -v`, counts are top-level tests / subtests)

| Configuration | TTP actually imported | PASS | FAIL | SKIP | Notes |
|---|---|---|---|---|---|
| A. This worktree as-is | `ttp-original` @ `1816d93` (0.10.0 code) | 104 / 55 | 1 / 0 | 14 / 2 | The fail is the `TestGroupFunctionSetExample` flake (§3.4). |
| B. Repo copy without `ttp-original`, pyenv `python3` (ttp 0.9.5) | 0.9.5 + **stale cache** | 72 / 51 | **33 / 4** | 14 / 2 | All 33 are "Python TTP failed: exit status 1": `No module named 'ttp.utils.load_python_exec'`. |
| B'. Same, `TTPCACHEFOLDER=<empty dir>` | 0.9.5 (clean) | 105 / 55 | 0 / 0 | 14 / 2 | |
| C. Repo copy without `ttp-original`, `/tmp/ttp0101/bin` first on `PATH` | 0.10.1 (PyPI) | 105 / 55 | 0 / 0 | 14 / 2 | |

**0.9.5 vs 0.10.1: no difference** once the cache problem is removed (B' = C).

Why B fails: the pyenv site-packages has `ttp/ttp_dict_cache.pickle` dated 2025-11-19, written by an earlier 0.10.x install. The 0.9.5 wheel doesn't own that file, so reinstalling 0.9.5 didn't remove it. 0.9.5's `lazy_import_functions` reuses the cache when only the Python major version matches (0.9.5 `ttp/ttp.py:86-99`), and the cache names the 0.10 module `utils.load_python_exec`. That module doesn't exist in 0.9.5, so every template that loads Python-style text (lookups, vars, macros) fails. Any developer machine that has moved between TTP versions can hit the same false failures.

Note for the brief: `/tmp/ttpvenv` is **0.10.1**, not 0.9.5. `importlib.metadata.version('ttp')` reports `0.10.1`. The 0.9.5 install on this machine is the pyenv `python3`.

Skips (identical in every configuration): 13 feature skips from the `knownDiffs` map (`G:test/comparison/comparison_test.go:320-336`), plus `groups_attributes_test.go:244` and the network/database skips.

### 3.4 Masking and flakiness inside the suite

- **`normalizeJSON` unwraps any single-element list whose element is a list** (`G:test/comparison/comparison_test.go:191-198`). It applies that to both sides, recursively. That collapses Python `[[[a,b]]]` and gottp `[[a,b]]` to the same value, which hides Repro 1, the main structural difference. 27 of the 48 TTP tests that "pass" under this normalization fail strictly (§4).
- `runGoTTP` drops the `lookups` argument, while Python receives it (`G:test/comparison/comparison_test.go:140-143` vs `python_runner.py:85-87`).
- `runGoTTP` rewrites the template to `results_method="per_input"` and wraps it in `<template>` when there's none (`G:test/comparison/comparison_test.go:112-120`). That wrapper is the workaround that hides **#27**.
- **`TestGroupFunctionSetExample` is flaky because gottp's output is nondeterministic**, not because of TTP. Running gottp 40 times on that template gave 2 different outputs (34 runs one way, 6 the other): the order of the `VRFs` list flips. Python's output is stable. `-count=30` failed 3/30 (A), 3/30 (C) and 1/30 (B'). Template: `G:test/comparison/groups_functions_comprehensive_test.go:232-243` (Repro 17).

---

## 4. Porting TTP's own test suite

### 4.1 Size

TTP 0.10.1 `test/pytest/` has 24 test modules and **288 collected tests** (2 are skipped upstream), with 252 `assert res == …` expectations. Fixtures: 26 files in `assets/`, 5 in `mock_data/`, 6 in `yang_modules/`.

### 4.2 Method (scripts live in `/tmp`, not in the repo)

1. A pytest plugin wraps the `ttp` class methods and records each test's top-level calls under 0.10.1: constructor args, `add_input`, `parse`, `result` and their return values. Templates and data passed as file paths are read in. 281 tests pass upstream under the recorder, and 286 records were written.
2. A record counts as **auto-portable** when all of these hold:
   - the test uses one parser object;
   - it only calls `__init__`/`add_input`/`parse`/`result`;
   - `result()` is called with no arguments;
   - inputs are strings or files, with unique names;
   - the result is JSON-serializable.
3. Each auto-portable case goes through gottp's Python-style API (`NewParser`/`AddTemplate`/`AddInput`/`Parse`/`Result`) with cwd set to `test/pytest` so relative asset paths resolve. The output is compared strictly, and again after applying the comparison suite's own normalization (§3.4).

Classification of the 286 records:

| Bucket | Count |
|---|---|
| **auto-portable** | **200** (70 % of 286) |
| `result(structure=…)` / `returner=` / `format=` kwargs | 16 |
| `add_input(template_name=…/groups=…)` | 13 |
| no `result()` call (tests that assert errors or logs) | 15 |
| `add_function` (Python callables) | 11 |
| `get_input_load` / `get_template` / `clear_result` / `set_input` / `add_template` | 12 |
| same input name added twice | 6 |
| several parser objects | 5 |
| failing upstream (need `ttp_templates` / filesystem) | 5 |
| other (`base_path`, `dynamic_path_key_to_int`, non-string data) | 3 |

### 4.3 Results on the 200 auto-portable tests

| Outcome | Count |
|---|---|
| pass, strict | 21 |
| pass only after suite-style normalization | 27 |
| fail | 145 |
| gottp error | 7 |

Per module (auto-portable cases only): `test_headers_indicator` 0/11 · `test_group_lookup` 0/7 · `test_group_chain_attribute` 0/4 · `test_output_functions` 0/9 · `test_match_condition_fun` 7/21 (all normalized-only) · `test_extend_tag` 3/18 · `test_group_functions` 10/23 · `test_match_functions` 4/13 · `test_misc` 7/17 · `test_answers_and_docs` 8/50 · `test_output_formatters` 2/8 · `test_ttp_parser_methods` 2/2.

Rough grouping of failures by first difference. A test can fail for several reasons; this counts only the first one hit.

| Cluster | Tests | Covered by |
|---|---|---|
| unclassified first difference | 55 | various |
| Python `<macro>` code run as Starlark (fail 14 + error 4) | 18 | design difference |
| `<extend>` (inside group / filters) | 14 | Repro 13 |
| `joinmatches` / `_line_` | 13 | #29, #34, #32, Repro 18 |
| `_headers_` unsupported | 13 | Repro 7 |
| output tags / formatters / output functions (fail 11 + error 2) | 13 | Repro 1 (csv), §1.6 |
| `set(True/False)` comes out as a string | 8 | Repro 2 |
| lookup / group lookup | 8 | Repro 14 |
| leading space captured on `default()` lines | 6 | Repro 3 |
| quotes not stripped by `strip('"')` | 2 | #35 |

### 4.4 Twenty representative tests (proof table)

Status uses §4.3's terms. Paths in the first-difference column are on the normalized structure.

| # | TTP test (file::name) | gottp | first difference (path: Python → gottp) |
|---|---|---|---|
| 1 | `test_misc.py::test_match_vars_with_hyphen` | pass (normalized only) | — (strict: one list level missing, Repro 1) |
| 2 | `test_anonymous_group.py::test_simple_anonymous_template` | pass (normalized only) | — (strict: Repro 1) |
| 3 | `test_empty_group_results.py::test_empty_group_results` | pass, strict | — |
| 4 | `test_answers_and_docs.py::test_github_issue_22` | fail | `$.len`: `2` → `0` (`ignore(r"\s+")`, Repro 6) |
| 5 | `test_match_condition_fun.py::test_contains_re_from_vars` | fail | `$[0].is_lag`: `true` → `"True"` (Repro 2) |
| 6 | `test_match_condition_fun.py::test_startswith_re_inline` | fail | `$.len`: `1` → `0` (`r"Por\S"`, Repro 6) |
| 7 | `test_match_condition_fun.py::test_exclude_inline` | fail | `$[0].description`: `<missing>` → `"RID"` (Repro 8) |
| 8 | `test_match_functions.py::test_match_variable_default_set_to_string` | fail | `$[0].ip`: `"1.1.1.1/24"` → `" 1.1.1.1/24"` (Repro 3) |
| 9 | `test_match_functions.py::test_let_and_set` | fail | `$[0].admin-status`: `"down"` → `"up"` (Repro 10) |
| 10 | `test_match_functions.py::test_copy` | fail | `$[0].ip_address`: `"1.1.1.123"` → `<missing>` (Repro 5) |
| 11 | `test_headers_indicator.py::test_headers_docs_example` | fail | `$.len`: `3` → `0` (Repro 7) |
| 12 | `test_extend_tag.py::test_extend_tag_within_group` | fail | `$[0].bgp_config.config`: `{"rid":"1.1.1.1"}` → `<missing>` (Repro 13) |
| 13 | `test_group_lookup.py::test_group_lookup_using_lookup_table_action_replace` | fail | `$[0].arp[0].ip`: `"app_1"` → `"10.12.13.2"` (Repro 14) |
| 14 | `test_group_functions.py::test_group_to_int_missing_key_issue_109` | fail | `$[2].mtu`: `9200` → `"9200.0"` (Repro 11) |
| 15 | `test_output_formatters.py::test_csv_formatter_simple` | fail | `$[0]`: 2-row CSV → 1-row CSV (consequence of Repro 1) |
| 16 | `test_structure.py::test_list_structure` | fail | `$[0]`: 2 inputs → 1 input (Repro 9) |
| 17 | `test_misc.py::test_asterisk_issue_97` | fail | `$[0].interfaces`: key `lag-4:1629.*` → `lag-4:1629` › `*` (Repro 15) |
| 18 | `test_answers_and_docs.py::test_vlans_parsing` | fail | `$[0].vlans_active`: present → `<missing>` (**#28**) |
| 19 | `test_match_functions.py::test_multiple_joinmatches` | fail | `$[0].vlans[0].Hostname`: `"SWITCH"` → `"gethostname"` (Repro 12) |
| 20 | `test_answers_and_docs.py::test_issue_50` | fail | `…description`: `BNG-RH201-CORE` → `"BNG-RH201-CORE"` (**#35**) |
| 21 | `test_N2G_formatter.py::test_n2g_formatter` | error | `unknown output format: n2g` |
| 22 | `test_group_functions.py::test_group_macro_chaining` | error | Starlark: `unhandled index operation NoneType[string]` |

**Feasibility:** 200 of 286 recorded tests (70 %) can be replayed with no hand-porting, using only template + inputs + expected result. 16 more need `result(structure=…)`, which gottp lacks (§1.7). The ~29 that rely on Python callables or parser-object APIs would need a Go-side equivalent or would stay Python-only.

---

## 5. Recommendation inputs: new incompatibilities that aren't filed

### 5.0 Facts about filed issues

- All filed template-engine repros (#27, #28, #29, #30, #31, #32, #33, #34, #35, #36) still differ from Python TTP 0.10.1 when replayed through gottp's Python-style API at `27b1e4f`.
- **#23 is not a TTP divergence.** With #23's template, `{{ id | DIGIT }}  {{ direction | re("(?:UP|DOWN)") }}  {{ rest | ROW }}`, Python TTP 0.10.1 gives the same over-matching result as gottp, `direction: "--"` for the `300 -- 1s …` row. It also logs `match variable function 'DOWN)")' failed`. The reason is that TTP splits the filter chain on `|` without regard to quotes (`T:ttp/utils/get_attributes.py:41`). TTP's docs work around this by putting such regexes in `<vars>` (see #31). Fixing #23 as written would make gottp differ from TTP.

### 5.1 New, unfiled (19)

Each repro gives the template, the data, and both outputs. "PY" is Python TTP 0.10.1 `parser.result()`; "GO" is gottp's `NewParser().Result()`.

**Repro 1: Top-level anonymous group loses one list level (and drops other inputs)**
```
template: interface {{ name }}
data:     interface A\ninterface B
PY: [[[{"name":"A"},{"name":"B"}]]]
GO: [[{"name":"A"},{"name":"B"}]]
```
An unnamed `<group>` gives the same result. Cause: `G:internal/compiled/runtime.go:707-712` uses the anonymous list *as the whole per-input result list* and skips the other inputs with `goto`. The same pattern is at `:1351-1380`. TTP makes the list one input's result (`T:ttp/ttp.py:989-1000`). Cascades: the csv formatter renders only the first match (`test_csv_formatter_simple`), and output functions see the wrong shape. The comparison suite masks it (§3.4).

**Repro 2: `set(True)` / `set(False)` (and any literal argument) is a string**
```
<group name="g">
interface {{ name }}
 {{ flag | set(True) }}
 {{ off | set(False) }}
</group>
data: interface A
PY: [[{"g":{"name":"A","flag":true,"off":false}}]]
GO: [[{"g":{"name":"A","flag":"True","off":"False"}}]]
```
TTP evaluates arguments as Python (`T:ttp/utils/get_attributes.py:58-60`). gottp's `parseFunction` returns `[]string` (`G:internal/compiled/runtime.go:6400-6415`). 8 replayed tests hit this.

**Repro 3: A variable with `default()` captures the leading space**
```
<group name="g">
interface {{ name }}
  ip address {{ ip | default("none") }}
</group>
data: interface A\n  ip address 1.1.1.1/24\ninterface B
PY: [[{"g":[{"ip":"1.1.1.1/24","name":"A"},{"name":"B","ip":"none"}]}]]
GO: [[{"g":[{"ip":" 1.1.1.1/24","name":"A"},{"name":"B"}]}]]
```

**Repro 4: The `default()` value isn't filled in when the line is absent** (also visible in Repro 3, record B)
```
<group name="g">
interface {{ name }}
  ip address {{ ip }}
  description {{ d | default("none") }}
</group>
data: interface A\n  ip address 1.1.1.1/24
PY: [[{"g":{"ip":"1.1.1.1/24","name":"A","d":"none"}}]]
GO: [[{"g":{"ip":"1.1.1.1/24","name":"A"}}]]
```
TTP: `T:ttp/ttp.py:2037-2044`. Docs: `Match Variables/Functions.rst:2630`.

**Repro 5: `copy()` is a stub**
```
<group name="g">
ip {{ last | copy("ip") | split(".") | item(-1) }}
</group>
data: ip 1.1.1.123
PY: [[{"g":{"last":"123","ip":"1.1.1.123"}}]]
GO: [[{"g":{"last":"123"}}]]
```
`G:internal/functions/match/more.go:634-638` ("For now, just return the value") vs `T:ttp/match/copy_.py:6-7`.

**Repro 6: Raw-string arguments `r"…"` silently match nothing**
```
<group name="g">
interface {{ name | startswith_re(r"Por\S") }}
</group>
data: interface Port-1\ninterface Lo0
PY: [[{"g":{"name":"Port-1"}}]]
GO: [[{}]]          (same template with "Por\S", no r-prefix: matches)
```
Also `{{ ignore(r"\s+") }}ip address {{ ip }} 255.255.255.255` → PY `{"ip":"1.1.1.1"}`, GO `{}`. This is a common idiom in TTP's docs and answers (`test_github_issue_22`, `test_issue_62`).

**Repro 7: `_headers_` (and `columns`) indicator is unimplemented**
```
<group name="g">
Port      Name    Status {{ _headers_ }}
</group>
data:
Port      Name    Status
Gi0/1     foo     up
Gi0/2             down
PY: [[{"g":[{"Port":"Gi0/1","Name":"foo","Status":"up"},{"Port":"Gi0/2","Name":"","Status":"down"}]}]]
GO: [[{}]]
```
`_headers_` occurs nowhere in `internal/`. TTP: `T:ttp/ttp.py:2115-2121`, `:2239`.

**Repro 8: A `_start_` line rejected by a condition doesn't close the previous record**
```
<group name="g">
interface {{ name | exclude("Loop") }}
  description {{ d }}
</group>
data: interface Gi1\ninterface Loopback0\n  description two
PY: [[{"g":{"name":"Gi1"}}]]
GO: [[{"g":{"d":"two","name":"Gi1"}}]]
```
The Loopback0 block's lines get attached to Gi1.

**Repro 9: Several inline `<input load="text">` tags without names: only the first is parsed**
```
<input load="text">
int A
</input>
<input load="text">
int B
</input>
<group name="g">
int {{ n }}
</group>
PY: [[{"g":{"n":"A"}},{"g":{"n":"B"}}]]
GO: [[{"g":{"n":"A"}}]]
```
9b: same with `<template results="per_template">` and group `a*`. PY `[{"a":[{"x":"1"},{"x":"2"}]}]`, GO `[{"a":[{"x":"1"}]}]`. TTP merges same-name inputs by appending data (`T:ttp/ttp.py:1044`). gottp keys inputs by name (`G:internal/parser/template.go:554`, `G:internal/compiled/runtime.go:302`). The Python-style `AddInput` overwrites in the same way (`G:api/python/parser.go:49-54`).

**Repro 10: `let()` on a matched line is overridden by a later `set()`**
```
<group name="g">
interface {{ name }}
 shutdown {{ enabled | set(False) | let("admin", "down") }}
 {{ admin | set(up) }}
</group>
data: interface A\n shutdown\ninterface B
PY: [[{"g":[{"enabled":false,"admin":"down","name":"A"},{"name":"B","admin":"up"}]}]]
GO: [[{"g":[{"admin":"up","enabled":"False","name":"A"},{"admin":"up","name":"B"}]}]]
```

**Repro 11: Group `to_int` has no float fallback**
```
<group name="g" to_int="">
interface {{ name }}
 mtu {{ mtu }}
</group>
data: interface A\n mtu 9200.0
PY: [[{"g":{"mtu":9200.0,"name":"A"}}]]
GO: [[{"g":{"mtu":"9200.0","name":"A"}}]]
```
`T:ttp/group/to_converters.py:10-28` tries `int()` and then `float()`.

**Repro 12: Getters: `gethostname` through `set()`, and `get_date`/`get_time`/`getfilename`/`get_timestamp*` missing**
```
<vars>
hostname="gethostname"
</vars>
<group name="g">
interface {{ name }}
 {{ host | set(hostname) }}
</group>
data: sw1#show run\ninterface A
PY: [[{"g":{"name":"A","host":"sw1"}}]]
GO: [[{"g":{"host":"gethostname","name":"A"}}]]
```
With `t="get_date"`: PY `"d":"2026-11-27"`, GO `"d":"get_date"`. gottp resolves only `gethostname`, only for named `<vars>` (`G:internal/compiled/runtime.go:621-630`, `:7785`).

**Repro 13: `<extend>` inside a `<group>` is ignored**
```
/tmp/x.txt:   " router-id {{ rid }}"
<group name="bgp">
router bgp {{ asn }}
<extend template="/tmp/x.txt"/>
</group>
data: router bgp 65100\n router-id 1.1.1.1
PY: [[{"bgp":{"rid":"1.1.1.1","asn":"65100"}}]]
GO: [[{"bgp":{"asn":"65100"}}]]
```
`G:internal/parser/template.go:536-546` only looks at child groups. TTP recurses into groups: `T:ttp/ttp.py:1372-1373`.

**Repro 14: Group-level `lookup=` attribute is ignored (`replace`, `add_field`, `update`)**
```
<lookup name="L" load="ini">
[ips]
10.0.0.1 = app_1
</lookup>
<group name="arp" lookup="'ip', name='L.ips', replace=True">
Internet {{ ip }} {{ mac }}
</group>
data: Internet 10.0.0.1 aaaa.bbbb.cccc
PY: [[{"arp":{"ip":"app_1","mac":"aaaa.bbbb.cccc"}}]]
GO: [[{"arp":{"ip":"10.0.0.1","mac":"aaaa.bbbb.cccc"}}]]
```
`add_field='APP'`: PY adds `"APP":"app_1"`, GO adds nothing. `T:ttp/group/lookup.py:6`. There's no group-scope `lookup` in `G:internal/functions/group/functions.go:40-58`.

**Repro 15: A dynamic-path value containing `.` is split into path segments**
```
<group name="saps.{{ sap }}">
sap {{ sap }} create
</group>
data: sap lag-4:1629.* create
PY: [[{"saps":{"lag-4:1629.*":{}}}]]
GO: [[{"saps":{"lag-4:1629":{}}}]]
```
With `**` (as in `test_asterisk_issue_97`), GO gives `{"lag-4:1629":{"*":{…}}}`. TTP splits only the template's path string on `pathchar` (`T:ttp/ttp.py:1729-1736`), not the substituted values.

**Repro 16: Nested `<template>` children are merged into one result**
```
<template>
<template name="t1">
<group name="a">
A {{ x }}
</group>
</template>
<template name="t2">
<group name="b">
B {{ y }}
</group>
</template>
</template>
data: A 1\nB 2
PY: [[{"a":{"x":"1"}}],[{"b":{"y":"2"}}]]
GO: [[{"a":{"x":"1"},"b":{"y":"2"}}]]
```
2 replayed `test_group_lookup` "in template results" tests come back empty for related reasons.

**Repro 17: Nondeterministic output order** (the source of the `TestGroupFunctionSetExample` flake)

Template `G:test/comparison/groups_functions_comprehensive_test.go:232-243` with its data. Over 40 gottp runs there were 2 distinct outputs (34 and 6): the `bgp_config.VRFs` list order flips. Python is stable.

**Repro 18: `_line_` with `joinmatches` also captures the `_start_` line**
```
<group name="g">
host {{ h | _start_ }}
{{ rest | _line_ | joinmatches }}
</group>
data: host a\nfoo bar\nbaz
PY: [[{"g":{"h":"a","rest":"foo bar\nbaz"}}]]
GO: [[{"g":{"h":"a","rest":"host a\nfoo bar\nbaz"}}]]
```
This is close to **#32** (one data line fills several template lines), but #32's repro doesn't cover `_line_`. Whoever triages it should decide whether to fold it into #32.

**Repro 19: Missing output features** (one item, several gaps)

- The output functions `deepdiff` (`T:ttp/output/deepdiffer.py:18`) and `validate_yangson` (`T:ttp/output/validate_yangson.py:142`) aren't registered.
- `format="n2g"` errors with "unknown output format: n2g".
- `format="jinja2"` with an inline template errors with "jinja2 formatter requires template content". The input is `test_output_formatters.py::test_jinja2_formatter`.

**Not counted as defects:** these are deliberate design differences or Python-only API.

- Python `<macro>` code runs as Starlark (§1.1).
- `result(structure=…)`, `add_function`, `add_lookup`, `get_input_load` and the other Python API methods are absent (§1.7).

---

## 6. Appendix: match/group/output/input name cross-check

Generated by scanning `def` and `_name_map_` in `T:ttp/{match,group,output,input}/*.py` and `Register("…")` in `G:internal/functions/**`.

- **match**: all 53 TTP names are registered in gottp (`G:internal/functions/match/conditions.go:46-65`, `functions.go:48-51`, `more.go:13-49`). gottp-only: `startswith`, `endswith`, `notstartswith`, `notendswith`, `to`, `default`, `chain`, `upper`, `lower`, `strip`, `split`, `replace`, `IP`.
- **group**: all registered except `lookup` (❌), `str_to_unicode` (❌, a py3 no-op) and `void`/`macro`, which are handled inline (`G:internal/compiled/runtime.go:459`, `:3887`). gottp-only: `delete`.
- **output**: `traverse`, `dict_to_list` and `is_equal` are registered. `deepdiff` and `validate_yangson` are missing. `validate` resolves to the group cerberus function.
- **input**: `extract_commands` and `test` are registered (`G:internal/functions/input/functions.go:41-42`). `macro` is handled inline.

## 7. Reproducing this

- TTP 0.10.1: `python3 -m venv /tmp/ttp0101 && /tmp/ttp0101/bin/pip install "ttp[full]==0.10.1"`. Source: `git clone https://github.com/dmulyalin/ttp`, then `git worktree add … 1da3a76`.
- TTP 0.9.5: `/tmp/ttp095/bin/pip install ttp==0.9.5 pytest pyyaml jinja2 cerberus deepdiff openpyxl tabulate yangson n2g ttp_templates`. `ttp[full]==0.9.5` doesn't install on Python 3.12.
- When switching TTP versions, set `TTPCACHEFOLDER` to an empty directory, or delete `site-packages/ttp/ttp_dict_cache.pickle`.
- To run the comparison suite against an installed TTP, use a copy of the repo without `ttp-original/`, or remove it from `sys.path`.
- gottp has no `cmd/gottp` at `27b1e4f`. The replay used a small Go program that calls `gottp.NewParser()`.
