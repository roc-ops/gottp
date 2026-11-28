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

