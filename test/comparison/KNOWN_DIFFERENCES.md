# Known Differences Between GoTTP and Python TTP

This document tracks known differences between GoTTP and Python TTP implementations that may cause comparison tests to fail or behave differently.

**Last Updated**: Based on comprehensive comparison test suite results

> **TTP's own test suite:** the compliance measure for Python TTP 0.10.1 is now [`test/ttpsuite`](../ttpsuite/README.md), which replays the 200 auto-portable cases from TTP's `test/pytest` and compares strictly (no list unwrapping). Every case that differs there is listed in `test/ttpsuite/known_failures.json` with its tracking issue, so that table, not this file, is the current list of known differences. Python `<macro>` code running as Starlark is recorded there as `design: python macro`.

## Test Coverage Summary

The comparison test suite includes:
- **102 passing tests** covering core functionality (100% pass rate)
- **13 skipped tests** for known differences or external dependencies
- **0 failing tests** - All tests either pass or are skipped for known differences

**Test Categories:**
- Match Functions: All 52 functions tested (all passing)
- Group Functions: 21 functions tested (1 skipped - Cerberus validation)
- Input Loaders: 8 loaders tested (5 skipped - external dependencies)
- Output Formatters: 10 formatters tested (4 passing, 6 skipped - external libraries)
- Template Features: All major features tested (all passing)
- Lookups: All lookup features tested (all passing)
- Macros: Starlark and match function macros tested (1 skipped - JavaScript)

## Features Requiring External Dependencies

### GeoIP Lookup
- **Status**: Partial implementation
- **Reason**: Full GeoIP2 support requires `github.com/oschwald/geoip2-golang` and a GeoIP2 database file
- **Current Behavior**: Returns empty/default values
- **Test Status**: Skipped in comparison tests

### Database Loaders
- **Status**: Implemented but requires database drivers
- **Reason**: Requires importing database drivers (e.g., `_ "github.com/lib/pq"` for PostgreSQL)
- **Test Status**: Skipped in comparison tests unless database is available

### URL Loader
- **Status**: Implemented
- **Reason**: May have network dependencies and timing differences
- **Test Status**: Skipped in comparison tests unless network is available

## Output Format Differences

### YAML Formatter
- **Status**: Implemented
- **Reason**: Requires `pyyaml` library in Python TTP
- **Test Status**: Skipped in comparison tests (marked as known difference)

### Table Formatter
- **Status**: Implemented
- **Reason**: Complex nested list structure differences in how Python TTP's table formatter handles data traversal and flattening
- **Test Status**: Skipped in comparison tests (marked as known difference)
- **Note**: GoTTP's table formatter works correctly but has structural differences in nested data handling

### Tabulate Formatter
- **Status**: Implemented
- **Reason**: Requires `tabulate` library in Python TTP
- **Test Status**: Skipped in comparison tests (marked as known difference)

### Excel Formatter
- **Status**: Implemented
- **Reason**: Requires `openpyxl` library in Python TTP
- **Test Status**: Skipped in comparison tests (marked as known difference)

### Jinja2 Formatter
- **Status**: Implemented
- **Reason**: Requires `jinja2` library in Python TTP
- **Test Status**: Skipped in comparison tests (marked as known difference)

### N2G Formatter
- **Status**: Implemented
- **Reason**: Requires `n2g` library in Python TTP
- **Test Status**: Skipped in comparison tests (marked as known difference)

## Type Conversion Differences

### Float Precision
- **Status**: Handled in normalization
- **Reason**: Python and Go may handle float precision differently
- **Solution**: Results are normalized to 6 decimal places before comparison

### Numeric Types
- **Status**: Handled in normalization
- **Reason**: Python may use different numeric types (int vs int64)
- **Solution**: Types are normalized during JSON comparison

## String Handling

### Unicode
- **Status**: No-op in Go
- **Reason**: Go strings are UTF-8 by default, Python 2 had unicode handling
- **Test Status**: Should pass (no-op returns original value)

### Carriage returns in CRLF input (deliberate)
- **Python TTP**: keeps the `\r` of a CRLF line in the text it matches, so a capture at the end of a line whose pattern can match `\r` includes it: `{{ x | re(".*") }}` gives `"a  \r"` and `{{ x | _line_ }}` gives `"v c \r"`. `PHRASE`/`ORPHRASE`/`WORD` captures don't include it in either engine, and neither does the last line of input when it has no newline.
- **GoTTP**: trims `\r` from each data line before matching (trailing spaces and tabs are kept, as in TTP, since #33), so the same capture has no `\r`.
- **Reason**: a stray `\r` in values is never wanted, and the result doesn't depend on the input's line endings.

## Ordering Differences

### Dictionary/Map Key Ordering
- **Status**: Handled in normalization
- **Reason**: Go maps have random iteration order, Python dicts maintain insertion order (3.7+)
- **Solution**: Keys are sorted alphabetically before comparison

### List Ordering
- **Status**: Preserved
- **Reason**: Order matters for lists in both implementations
- **Test Status**: Should match exactly

## Error Handling

### Error Messages
- **Status**: May differ
- **Reason**: Different error message formats between Python and Go
- **Test Status**: Error presence is checked, exact message may differ

## Macro Execution

### Starlark
- **Status**: Fully implemented
- **Note**: Should match Python TTP behavior exactly

### JavaScript
- **Status**: Implemented using goja
- **Note**: Python TTP has issues parsing JavaScript macros (syntax errors), so it returns original values. GoTTP correctly executes JavaScript macros.
- **Test Status**: Skipped in comparison tests (known difference)

### Python
- **Status**: Optional, requires build tag
- **Note**: Requires CGO and Python development headers
- **Test Status**: Skipped unless Python macro support is enabled

### Group functions after `itemize` without a path
- **gottp:** after `itemize` without a path, the record is the itemized value, and the functions after it run on an empty record. They can only drop the item: `contains`, `containsall` and another `itemize` always drop it, while `del`, `set` or a macro have no effect.
- **Python TTP 0.10.1:** the functions after it get the value itself. For a string value, `contains('x')` tests whether `x` is a substring, and functions that expect a dict raise an error.
- **Test status:** `gottp-repros/issue149_itemize_then_contains_child` pins the case where both drop every item.

### Output macro lists (gottp extension)
- **gottp:** `<output macro="a, b">` and `<output functions="macro('a', 'b')">` run each macro in order, each on the result of the one before.
- **Python TTP 0.10.1:** both raise `TypeError`, because `macro=` becomes the call `macro(data, "a", "b")` and TTP's output `macro()` takes one name. Use `functions="macro('a') | macro('b')"` in templates that must run in both.
- **Test status:** no comparison case; covered by `TestOutputMacro_SeveralNamesRunInOrder` in `test/output_macro_test.go`.

### Several text-only `<extend>` tags (deliberate)
- **gottp:** `<extend template="a.txt"/>` followed by `<extend template="b.txt"/>`, where both files are plain text, gives the implicit group with both files' text.
- **Python TTP 0.10.1:** gives `{}`. Its `handle_extend` removes children while iterating over them, so it skips the second extend, which leaves a tag in the template, so no implicit group is made. gottp keeps the sensible result.

### `default()` without a regex (gottp extension, #64)
- **gottp:** a match variable with `default()` and no regex of its own uses `.*?`, so it matches any text, also an empty or missing last column and text with spaces. `{{ port }} {{ state }} {{ descr | default("") }}` matches `Gi2 down` (`descr` is `""`) and `Gi3 up core link` (`descr` is `"core link"`). The whitespace between the text and the variable is not captured, unless the template puts the text right against the variable (`ip foo{{ x | default("a") }}` on `ip foo bar` gives `" bar"`). A line made only of such variables doesn't match blank lines, as in TTP. Two such variables separated only by whitespace don't split the text as TTP does: the first captures `""` and the second takes the rest (#166).
- **Python TTP 0.10.1:** gives such a variable the `WORD` regex (`\S+`), so it drops both lines. The default value is used only when no line matches.
- **Why:** DH360 templates use a trailing `{{ col | default("") }}` to make the last column optional, and TTP's behaviour would drop those rows. Add a regex (`{{ col | WORD | default("") }}`) to get TTP's behaviour.
- **Side effect:** the text before the variable doesn't need a space after it, so `ip address {{ ip | default("none") }}` also matches `ip addressX` (`ip` is `"X"`), where TTP needs `ip address X`.
- **Test status:** `gottp-repros/issue64_default_last_column_empty` and `issue64_default_matches_text_with_spaces` (listed as `design:` in `test/ttpsuite/known_failures.json`); `TestDefaultExtension_ParseAndStream` in `test/streaming`.

### `{{ _start_ | re(...) }}` (gottp extension, #136)
- **gottp:** a line's regex ends at a `{{ _start_ }}` or `{{ _end_ }}` variable, as in TTP, unless the variable has its own `re()`. That variable then matches its regex: `interface {{ _start_ | re("Gi\\S+") }}` matches `interface Gi1` but not `interface` or `interface Te1`.
- **Python TTP 0.10.1:** ignores the `re()`. Its `regex_deleteVar` cuts the regex at `_start_`, so the line matches only `interface`.
- **Why:** 3 DH templates use `{{ _start_ | re(...) }}` and need the regex. Drop the `re()` to get TTP's behaviour.
- **Test status:** `gottp-repros/issue136_start_re_extension` (listed as `design:` in `test/ttpsuite/known_failures.json`); `TestGenerateRegex_CutAtStartEnd` in `internal/pattern`.

### Variables after `{{ _start_ }}` or `{{ _end_ }}` (deliberate, #136)
- **gottp:** a variable after the indicator on the same line captures nothing, as the regex ends at the indicator. `GetWarnings()` names it when the template compiles.
- **Python TTP 0.10.1:** doesn't capture it from the data either. But when the regex has no named group left (`exit {{ _end_ }} {{ w }}`), TTP's `check_matches` gives every variable of the line the whole match text, which starts with the newline: `{"_end_": "\nexit", "w": "\nexit"}`. A variable after the indicator also makes the line an ordinary line instead of an end line, in both engines.
- **Why:** the values are an artifact of TTP's internal `\n` line anchors, not data; the compile warning tells the author to move the indicator.
- **Test status:** `gottp-repros/issue136_capture_after_end_gets_no_value` (listed as `design:`); `TestCutVariableWarning` in `internal/compiler`.

### Unquoted words Python TTP can't evaluate (gottp extension)
- **gottp:** an unquoted argument such as `set(auto-decision)`, `containsall="af-marker"` or `exclude(-VM-)` is its raw text, `"auto-decision"`. `GetWarnings()` says that Python TTP fails on it and gives the quoted call, `set("auto-decision")`, which works in both.
- **Python TTP 0.10.1:** evaluates the argument as Python (`auto - decision`, with each unknown name as its own string) and exits: `unsupported operand type(s) for -: 'str' and 'str'`.
- **Why:** such templates exist in real use (8 DH templates). Refusing them broke working gottp templates for no benefit.
- **Test status:** `TestRawTextArguments` in `test/function_calls_test.go`; `TestParseRawArgs` in `internal/call`.

### Arguments that are Python expressions (D1, deliberate)
- **gottp:** an argument that is a Python expression rather than a literal, such as `set(1+2)`, `sformat("a"+"b")`, `x.upper()`, an f-string, `b"x"`, `1e400`, `"\N{...}"`, a set, `*args` or a comprehension, is passed as its raw source text (`"1+2"`). `GetWarnings()` says that Python TTP evaluates it and gottp passes the raw text.
- **Python TTP 0.10.1:** evaluates it with `eval`, so `set(1+2)` is `3`.
- **Why:** gottp reads literals, not Python. Only broken syntax (an unterminated quote, unbalanced brackets) and the calls TTP raises `TypeError` on are compile errors.
- **Test status:** `TestRawTextArguments` in `test/function_calls_test.go`; `TestParseRawArgs` in `internal/call`.

### Type methods that change the value in place (F2, deliberate)
- **gottp:** a match variable function that is a Python list or dict method changing the value in place (`sort`, `reverse`, `pop`, `extend`, `insert`, `remove`, `clear`, `update`, `setdefault`, `popitem`) isn't ported. `GetWarnings()` names it at compile time and the value is kept unchanged.
- **Python TTP 0.10.1:** calls the method through its `getattr` fallback (`ttp.py:2464`). The method returns `None`, which TTP saves as the value: `{{ v | split(",") | sort }}` saves `null`.
- **Why:** the fallback is meant for methods that return a new value; saving `None` is never what a template wants.
- **Test status:** `TestTypeMethodWarnings` in `test/function_calls_test.go`; `TestListMethods` and `TestDictMethods` in `internal/pymethod`.

### Type method results with no JSON form (F2)
- **gottp:** `str.encode`, `int.to_bytes` (bytes), and the class methods `int.from_bytes`, `float.fromhex` and `dict.fromkeys` aren't ported; they warn at compile time and keep the value. `keys()`, `values()` and `items()` on a dict (an `ip_info` value) give lists, `items()` as `[key, value]` lists, in key order (gottp's maps keep no order).
- **Python TTP 0.10.1:** saves the bytes or the `dict_keys`/`dict_values`/`dict_items` object, in insertion order. Its JSON and YAML outputs can't write them (`TypeError: Object of type dict_keys is not JSON serializable`).
- **Test status:** `TestDictMethods` in `internal/pymethod`.

### `get(x)` as a match variable function (F2)
- **gottp:** runs `dict.get(x)` on a dict value; on any other value it is skipped (the value is kept).
- **Python TTP 0.10.1:** reads `get(x)` as `re(x)`: its pattern lookup (`ttp/patterns/get_pattern.py`) looks the name up in its own module, where `get` is the lookup function, so `x` becomes the variable's regex. `get` without arguments fails with `TypeError`. `dict.get` is never reached.
- **Test status:** `TestDictMethods` in `internal/pymethod`.

### Type method arguments (F2)
- **gottp:** a dict literal's keys are strings, so `translate({97: "x"})` maps the key `"97"` (the code point 97) and `dict.get(1)` finds nothing.
- **Python TTP 0.10.1:** keeps int keys.

## Recent Fixes and Improvements

### Fixed During Testing (2024)

1. **Dynamic Path Variables**: Variables used in dynamic paths (e.g., `interfaces.{{ interface }}`) are now correctly removed from match results to match Python TTP behavior.

2. **Output Formatter Functions**: The `traverse()` function now correctly handles simple path arguments like `traverse('interfaces')` and is processed before formatting.

3. **Output Formatter Path Attribute**: The `path` attribute in output formatters now correctly extracts data from the specified path before formatting.

4. **Start/End/Line Indicators**: All three indicators (`_start_`, `_end_`, `_line_`) are now correctly recognized and handled, including when they appear on their own lines without `{{ }}`.

5. **Nested Groups**: Nested groups are correctly processed within their parent context and not as top-level groups.

6. **Method Attribute**: The `method="table"` attribute now correctly treats all patterns as start patterns, with each pattern match saved as a separate result entry (not merged).

7. **Set Function**: The `set` group function now uses the correct argument order (source, target) matching Python TTP's behavior.

8. **Resub Function**: The `resub` match function now correctly replaces only the first occurrence by default (matching Python's `re.sub` with count=1).

9. **CSV and PPrint Formatters**: String representation now matches Python TTP's output, including Python-like dict/list formatting and key ordering.

## Future Improvements

1. Add tolerance for whitespace differences in formatted output
2. Implement XML-aware comparison for N2G output
3. Add database test fixtures for database loader tests
4. Add network mock for URL loader tests
5. Improve error message comparison
6. Resolve table formatter nested list structure differences

