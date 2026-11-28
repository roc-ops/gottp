# TTP 0.10.1 test suite

Python TTP 0.10.1's own test suite (`test/pytest` in [dmulyalin/ttp](https://github.com/dmulyalin/ttp)), replayed through gottp. This is the compliance measure for the baseline (#60).

Current result (baseline clusters 6 and 2: #68, #136, #121, #139):

```
go test ./test/ttpsuite -v | grep -E 'suite:|repros:|parity:|snapshots:'
```

```
TTP 0.10.1 suite: pass 123 / skipped-by-issue 71 / design 6 of 200
gottp repros: pass 193 / skipped-by-issue 2 / design 3 of 198
source-map parity: same 401 / diverge 0 of 401
stream snapshots: 206 streamable of 401 cases
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
| `issue33_trailing_ignore_with_trailing_whitespace` | passes; #33 is fixed |
| `issue33_vlan_name_keeps_trailing_space` | passes; a greedy `re()` keeps the line's trailing space (#33) |
| `issue33_re_dotstar_keeps_trailing_spaces` | passes; `re(".*")` keeps trailing spaces (#33) |
| `issue33_exact_space_trailing_whitespace` | passes; an `_exact_space_` line allows trailing whitespace (#33) |
| `issue33_untagged_indented_first_line_matches` | passes; a template with no tags keeps its first line's indent (#33) |
| `issue33_untagged_indented_first_line_no_unindented_match` | passes; ...so it doesn't match the line without the indent (#33) |
| `issue33_untagged_leading_blank_lines_keep_indent` | passes; leading blank lines are dropped, the first real line keeps its indent (#33) |
| `issue34_split_joinmatches` | #34 |
| `issue35_strip_characters_argument` | #35 |
| `issue36_unknown_function_falls_back_to_str_method` | passes; #36 is fixed (refactor F2: Python type methods) |
| `sourcemap_void_group` | passes; covers `void` for source-map parity (#108) |
| `issue60_first_line_starts_record_after_start_indicator` | passes; the first line starts a record even after a `_start_` line (refactor B, #60) |
| `issue60_table_tail_line_is_ordinary` | passes; a `method="table"` line after a child group is not a start pattern (#60) |
| `issue60_tail_start_line_starts_own_record` | passes; a `_start_` line after a child group starts its own record (#60) |
| `issue128_end_line_values_are_saved` | passes; #128 is fixed |
| `issue128_end_line_values_no_children` | passes; #128 in a group without children |
| `issue64_default_no_leading_space` | passes; a `default()` variable doesn't capture the space before it (#64) |
| `issue64_start_end_spacing` | passes; `_start_`/`_end_` lines with extra spaces (#64) |
| `issue64_default_only_line_and_blank_lines` | passes; a line of only `default()` variables doesn't match blank lines (#64, #162) |
| `issue64_default_last_column_empty` | `design:`; a trailing `default("")` matches an empty last column, TTP drops the line (#64) |
| `issue64_default_matches_text_with_spaces` | `design:`; a `default()` variable without a regex matches text with spaces (#64) |
| `issue65_default_filled_when_line_absent` | passes; #65 is fixed |
| `issue154_set_line_without_extra_text` | #154; a `set()` variable matches text TTP's regex doesn't allow |
| `issue162_line_matches_whitespace_only` | passes; `_line_` matches a line of only spaces (#162) |
| `issue162_joinmatches_line_keeps_blank` | passes; `_line_ \| joinmatches` keeps a blank line (#162) |
| `issue162_re_spaces_matches_blank` | passes; `re("[ ]+")` matches a blank line (#162) |
| `issue162_re_dotstar_matches_empty_line` | passes; `re(".*")` matches empty lines (#162) |
| `issue162_indented_start_blank_line` | passes; an indented `{{ _start_ }}` keeps its indent and sees the blank line as it is (#162) |
| `issue68_headers_basic` | passes; the issue's example: fixed-width columns, an empty middle column (#68) |
| `issue68_headers_exclude_substring` | passes; a row with a header's name inside that header's column is dropped, as is the header row (#68) |
| `issue68_headers_whitespace_rows` | passes; a row of only spaces matches with empty values; an empty line doesn't (#68) |
| `issue68_headers_short_and_long_rows` | passes; short rows fill the optional columns with ""; the last column takes the rest; a too-long value spills into the next column (#68) |
| `issue68_headers_text_after_indicator` | passes; text after `{{ _headers_ }}` is dropped (#68) |
| `issue68_headers_tab_indent` | passes; a tab in the header row's indent counts as 4 characters (#68) |
| `issue68_headers_columns_all` | passes; `columns(N)` above the header count makes every column mandatory (#68) |
| `issue68_headers_columns_zero` | passes; `columns(0)` is read as `columns(1)` (#68) |
| `issue68_headers_after_other_line` | passes; a `_headers_` line that isn't the first line still starts records (#68) |
| `issue68_headers_unicode` | passes; column widths count characters, not bytes (#68) |
| `issue121_indented_end_needs_indented_blank` | passes; an indented `{{ _end_ }}` matches only blank lines with that indent (#121) |
| `issue136_capture_then_start_matches_only_that_text` | passes; `interface {{ name }} {{ _start_ }}` doesn't match a line with more text (#136) |
| `issue136_capture_after_start_is_cut` | passes; a variable after `{{ _start_ }}` is cut from the regex (#136) |
| `issue136_start_first_on_line_matches_blank` | passes; `{{ _start_ }} interface {{ name }}` matches blank lines only (#136) |
| `issue136_end_cuts_text_after_it` | passes; `exit {{ _end_ }} now` matches `exit`, not `exit later` (#136) |
| `issue136_start_after_two_spaces` | passes; the whitespace before `{{ _start_ }}` is dropped (#136) |
| `issue136_start_exact_space_keeps_space` | passes; with `_exact_space_` the space before `{{ _start_ }}` stays (#136) |
| `issue136_start_re_extension` | `design:`; `{{ _start_ \| re(...) }}` matches the `re()`, TTP ignores it (#136) |
| `issue136_capture_after_end_gets_whole_line` | passes; on `exit {{ _end_ }} {{ w }}`, `w` and `_end_` get the whole matched line, newline first (#136) |
| `issue136_end_then_vars_get_whole_line` | passes; every variable after `{{ _end_ }}` gets the whole matched line (indent and trailing spaces kept), its functions run on it, and `default()` fills a record without the line (#136) |
| `issue136_mixed_blank_before_start` | passes; a space and a tab before `{{ _start_ }}` are cut as one run (#136) |
| `mixed_blank_run_is_one_space` | passes; a run of mixed spaces and tabs in a template line is one `[ \t]+`, as in TTP |
| `issue139_line_after_last_end_dropped` | passes; `_line_` records more than one line past the last `_end_` are dropped; the line right after it is kept (#139) |
| `issue139_line_joinmatches_after_last_end_dropped` | passes; the same with `_line_ \| joinmatches` (#139) |
| `issue139_line_after_last_end_with_child` | passes; the same in a group with a child group, which dropped the line right after the end too (#139) |
| `issue139_second_end_moves_the_cut` | passes; only the last `_end_` cuts (#139) |
| `issue139_start_after_end_keeps_lines` | passes; a `_start_` match after the `_end_` means nothing is cut (#139) |
| `issue139_ordinary_line_after_end_dropped` | passes; an ordinary line past the last `_end_` is not added to the kept `_line_` record (#139) |
| `issue139_end_condition_fails_still_cuts` | passes; an `_end_` match whose `equal()` condition fails still cuts the group, in `Parse` and `ParseStream` (#139) |
| `issue139_end_contains_fails_still_cuts` | passes; the same with `contains()` (#139) |
| `issue139_end_condition_fails_with_start` | passes; the same in a group with a `_start_` line (#139) |
| `issue139_no_trailing_newline` | passes; the cut on input without a trailing newline (#139) |
| `issue139_end_last_line_no_newline` | passes; an `_end_` line that is the last line, without a newline, doesn't match, as in TTP (#139) |
| `issue139_two_end_patterns` | passes; with two `_end_` lines, the later match cuts, whichever pattern it is (#139) |
| `issue139_two_end_patterns_later_second` | passes; the same with the patterns' matches in the other order (#139) |

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
| `issue136_start_only_line_matches_trailing_text` | passes; `text {{ _start_ }}` matches only that exact text (#136) |
| `issue136_startempty_tie_with_start_pattern` | passes; the same, where a start pattern also matches the line (#136) |
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

Related issues with nested groups (refactor C2), each a minimal repro from the issue; all pass (`issue88_itemize_path_argument`, the path argument, since E1): `issue88_itemize_child_group`, `issue88_itemize_path_argument`, `issue89_group_with_only_child_groups`, `issue89_anonymous_child_groups`, `issue91_child_group_defaults_without_match`, `issue99_record_and_set_across_groups`, `issue100_void_sibling_startempty_joinmatches` (a reduction of `test_same_config_different_contexts`), `issue118_joinmatches_stops_at_sibling_start`, `issue119_second_start_line_stays_in_own_section`, `issue120_sibling_groups_same_name`.

Fixes on top of C2, each recorded from TTP 0.10.1; all pass:

| Repro | Rule |
|---|---|
| `issue60_c2_child_macro_runs_on_last_record` | a child `macro=` runs on every record, also once the parent record holds a key named like the macro |
| `issue60_c2_child_vars_exclude_parent_record_contains_val` | a child's `contains_val=` compares with template variables, not the parent record's values |
| `issue60_c2_child_vars_exclude_parent_record_default` | a child's `default="desc"` is the string, not the parent's `desc` |
| `issue60_c2_absolute_child_path` | `name="/vlans_abs"` saves at the results root, one list for every parent record |
| `issue60_c2_absolute_grandchild_path` | the same for a grandchild |
| `issue60_c2_absolute_child_of_anonymous_group` | under an anonymous top group, `{"vl": [...]}` is one more item of the result list |
| `issue60_c2_absolute_grandchild_of_anonymous_group` | the same for a grandchild |
| `issue60_c2_root_child_path` | `name="/"` saves at `_anonymous_*`: the records are items of the result list |
| `issue60_c2_absolute_dynamic_child_path` | an absolute path with a `{{ variable }}` |
| `issue60_c2_absolute_child_with_relative_child` | a relative child of an absolute group goes below it |
| `issue60_c2_void_top_keeps_children` | the children of a `void` top group are kept |
| `issue60_c2_record_target_no_merge` | `record="x, tgt"` records `x` as `tgt` and merges no records |
| `issue60_c2_record_same_value_no_merge` | two records with the same `record=` value stay two records |
| `issue60_c2_record_after_contains` | `contains="z" record="x"`: a record `contains=` drops records nothing |
| `issue60_c2_child_record_after_contains` | the same in a child group |
| `issue60_c2_group_functions_attribute_order` | group function attributes run in template order: `del="z" contains="z"` drops every record |

Group-function order pins for refactor E (#149), recorded from TTP 0.10.1. TTP's `processgrp` (`ttp.py:3240`) runs per record, when the record is saved: it fills in the defaults, then runs every group function in attribute order (`macro=`, `void=` and `functions=`/`chain=` included, each at its own position), and the first rejection drops the record. E1 ports that pipeline (`internal/compiled/pipeline.go`), and every one of these now passes. The *Before E1* column records which ones failed on E0.

| Repro | Rule | Before E1 |
|---|---|---|
| `issue149_macro_runs_before_contains` | `macro="m" contains="mk"`, where `m` adds `mk`: the records are kept | fails |
| `issue149_macro_runs_before_record` | `macro="m" record="desc, rd"`: a later group's `set="rd, seen"` gets the macro's value | fails |
| `issue149_void_runs_before_record` | `void="" record="desc, rd"`: the record is voided first, so nothing is recorded | fails |
| `issue149_functions_run_per_record` | `record="desc, rd" set="rd, desc"` on A (`desc z1`), B (no desc), C (`desc z`) gives `z1, z1, z`: every function runs on one record before the next | fails |
| `issue149_child_functions_run_per_record` | the same in a child group | passes |
| `issue149_itemize_then_contains_child` | `itemize="name" contains="mk"` on a child group: `contains=` still runs and drops every record | fails |
| `issue149_functions_attr_before_record` | `functions="contains('z')" record="x"`: the dropped record isn't recorded | fails |
| `issue149_functions_attr_after_record` | `record="x" functions="contains('z')"`: the record is recorded before it is dropped | passes |
| `issue149_chain_attr_before_record` | `chain="ch" record="x"`, the same with a `<vars>` chain | fails |
| `issue149_chain_attr_after_record` | `record="x" chain="ch"` | passes |
| `issue149_variable_default_before_contains_val` | a `default()` value is filled in before the functions, so `contains_val=` sees it | fails |
| `issue149_set_default_before_contains_val` | the same for a `{{ desc \| set("dflt") }}` default | fails |
| `issue149_group_default_before_record` | the same for group `default=`, seen by `record=` | fails |
| `issue149_contains_then_del_keeps_records` | `contains="z" del="z"` keeps the records with `z` and then deletes `z` (the reverse of `issue60_c2_group_functions_attribute_order`) | passes |
| `issue149_record_without_merge` | `record="x"` over three records, one without `z`: three records, and the last value is recorded | passes |
| `issue88_itemize_path_keyword` | `itemize="key='name', path='info.names'"`, the keyword form with a dotted path | fails |
| `issue88_itemize_in_functions_with_path` | `functions="itemize(key='name', path='ifs')"` | fails |
| `issue88_itemize_dynamic_path_argument` | `itemize="name, 'ifs.{{ kind }}'"`, a path with a variable | fails |
| `issue147_macro_named_like_var_top_group` | `macro="fan"` with `<vars>fan = "zzz"</vars>` runs the macro | fails |
| `issue147_macro_named_like_var_child_group` | the same on a child group | fails |

Fixes from the review of E1 (#153), each recorded from TTP 0.10.1; all pass:

| Repro | Rule |
|---|---|
| `issue149_default_names_template_var` | `default("hostname")` with `<vars>hostname="R1"</vars>` fills `"R1"` (TTP's `update_groups_runs`) |
| `issue149_set_default_names_template_var` | the same for an unconditional `{{ hn \| set("hostname") }}` line |
| `issue149_default_names_template_var_contains` | `contains="hn"` compares with the resolved default, so it keeps no record |
| `issue149_default_names_template_var_exclude` | `exclude="hn"` keeps every record, with the resolved default |
| `issue149_default_lowercase_literals` | `default(none)`, `default(true)`, `default(false)` and `default(NONE)` are `null`/`true`/`false` (TTP's `_UndefSubst`) |
| `issue149_starlark_macro_false_drops` | a Starlark `macro=` returning `False` drops the record |
| `issue149_starlark_macro_false_drops_in_chain` | the same for `macro()` in `chain=` |
| `issue149_sformat_keywords_in_functions` | `functions="sformat(string='{name}-{vrf}', add_field='nv')"` saves under `nv` |
| `issue149_items2dict_missing_key_drops` | `items2dict=` drops a record without one of its keys |

Function-call pins for refactor F (#36), recorded from TTP 0.10.1. F1 gives match, group, input and output functions one call module (`internal/call`), which splits a chain on every `|` as TTP does and reads arguments as Python literals; F2 ports the type-method fallback of TTP's `check_matches` (`ttp.py:2464`). When a match function isn't a TTP function, TTP runs `getattr(value, name)`: a callable is called with the arguments, `True`/`False` keeps or drops the match, and any other result replaces the value. Since F2 they all pass. `issue36_unknown_function_falls_back_to_str_method` (`swapcase`, above) belongs to this set too. What TTP only logs (unknown-name warnings, #144's hang and #146's `TypeError`) is pinned by Go tests in `test/function_calls_test.go`.

| Repro | Rule | Status |
|---|---|---|
| `issue36_str_title` | `title` gives `Hello-World` | passes (F2) |
| `issue36_str_zfill` | `zfill(9)` gives `000000123` | passes (F2) |
| `issue36_str_center` | `center(12, "*")` gives `****abc*****` | passes (F2) |
| `issue36_str_casefold` | `casefold` gives `strasse` | passes (F2) |
| `issue36_str_removesuffix` | `removesuffix(",")` | passes (F2) |
| `issue36_str_partition_returns_tuple` | `partition(",")` returns a tuple, saved as the list `["a", ",", "b,c"]` | passes (F2) |
| `issue36_str_find_returns_int` | `find("2")` saves the int `2` | passes (F2) |
| `issue36_str_isalpha_filters` | `isalpha` returning `False` drops the match | passes (F2) |
| `issue36_str_isupper_filters` | `isupper` returning `False` drops the match | passes (F2) |
| `issue36_str_isdigit_filters` | `isdigit` (a TTP match function) filters the same way | passes |
| `issue36_int_bit_length` | `to_int \| bit_length` gives `8` | passes (F2) |
| `issue36_int_imag_attribute` | `to_int \| imag`, an attribute that isn't callable, gives `0` | passes (F2) |
| `issue36_list_index` | `split(",") \| index("b")` gives `1` (TTP's own `count` shadows `list.count`) | passes (F2) |
| `issue36_unknown_name_keeps_value` | `nosuchfn`, a method on nothing, keeps the value | passes |
| `issue36_builtin_upper` | `upper`, the str method since F2 | passes |
| `issue36_builtin_strip_characters` | `strip("x")`, the str method since F2 | passes |
| `issue36_builtin_replace` | `replace("a", "b")`, the str method since F2 | passes |
| `issue36_builtin_split` | `split(",")`, the str method since F2 | passes |
| `issue36_builtin_join` | `join("-")` after `split` (TTP's own `join`) | passes |
| `issue36_raise_before_first_start_not_evaluated` | `rindex("-")` would raise on a line before the first start line, where run_re runs no regex but the start patterns | passes (F2 review) |
| `issue36_raise_without_start_line_not_evaluated` | the same with no start line at all: nothing but the start patterns runs | passes (F2 review) |
| `issue36_raise_before_child_start_not_evaluated` | a child group's line before the child's first start line, inside the parent's region | passes |
| `issue36_raise_child_end_line_not_evaluated` | a child's ordinary pattern would match (and raise on) the line of its last `_end_` match: TTP's region ends before that line's newline, which the regex needs | passes (F2 review) |
| `issue23_re_pipe_splits_chain` | `re("UP\|DOWN")` is split at the `\|` as TTP splits it, so `--` matches too (decision on #23: keep the result, add a warning) | passes |
| `issue23_re_noncapturing_pipe_splits_chain` | the same for `re("(?:UP\|DOWN)")` | passes |
| `issue23_sformat_pipe_in_quotes` | `sformat("a\|{}")`: both halves fail in TTP, so the value is kept | passes (F1) |
| `issue23_sformat_comma_in_quotes` | `sformat("{}, x")`: a comma inside quotes | passes |
| `issue23_sformat_equals_in_quotes` | `sformat("v={}")`: an `=` inside quotes isn't a keyword argument | passes (F1) |
| `issue151_re_double_backslash_argument` | inline `re("\\d+")` is a Python string, so it matches `\d+` | passes (F1) |
| `issue151_re_raw_string_argument` | inline `re(r"\d+")` is a Python raw string (also #67) | passes (F1) |
| `issue157_unicode_name_group_contains` | `contains="Москва"`: a bare name may use any letters | passes |
| `issue157_unicode_name_contains` | `contains(abcé)` | passes |
| `issue157_unicode_name_sformat` | `sformat(é)` | passes |
| `issue157_whitespace_group_contains` | `contains=" "` is `contains( )`, a call without arguments, which drops the record | passes |
| `issue157_group_functions_contains_no_args` | the same for `functions="contains()"` | passes |
| `issue157_equal_true_is_not_string` | `equal(true)` compares `"True" == True`, which is false | passes |
| `issue157_equal_int_is_not_string` | `equal(1)` doesn't match the text `1` | passes |
| `issue157_equal_to_int_matches_int` | `to_int \| equal(1)` matches the int | passes |
| `issue157_notequal_int_is_not_string` | `notequal(1)` keeps the text `1` | passes |
| `issue157_group_equal_int_is_not_string` | group `equal="x, 1"` doesn't match the text `1` | passes |
| `issue157_group_equal_string` | group `equal="x, foo"` keeps only `foo` | passes |
| `issue157_re_no_args_is_literal_re` | `re()` is the regex `re`, TTP's fallback to the function name | passes |
| `issue157_re_keyword_only_is_literal_re` | the same for `re(x=1)` | passes |
| `issue157_re_char_keyword_is_literal_re` | the same for `re(char="-")` | passes |

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
| #97 | 3 |  | A group macro that returns None loses its in-place changes, and the next chained macro gets None |
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
| #102 | 1 |  | format_attributes is ignored (tabulate tablefmt) |
| #114 | 1 |  | Group validate= attribute is not applied |

#62 (top-level anonymous group loses one list level) was the first difference in 65 cases. Its fix made 26 of them pass; the other 39 are listed under the issue they hit next, which for 11 of them is one of four new issues (#110, #112, #113, #114).

## Python macros (design)

23 cases use `<macro>`. gottp runs `<macro>` code as Starlark unless the tag says `language="python"` (`internal/parser/template.go`, `parseMacro`), and TTP's templates never set it. 6 of the 23 pass (`test_group_macro_returns_none`, `test_group_macro_returns_true`, `test_group_macro_returns_false`, `test_group_chain_attribute_string_var`, `test_group_chain_attribute_string_inline` and `test_issue_52_answer`).

- **5 are skipped as `design: python macro`**: their macro code isn't valid Starlark (`import ipaddress`, f-strings, `bin()`, `_ttp_["match"]["to_ip"]` with the `ipaddress` object API).
- The other 12 fail first on a gottp bug and are tracked by that issue: #80 (`validate_yangson`, 4), #97 (a group macro that returns `None` loses its changes, 3), and one each for #64, #67, #68, #76 and #112. `test_misc/test_tail_match_no_group_functions_processing` also contains Python-only macro code that would fail next (a generator expression inside `any()`).

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
