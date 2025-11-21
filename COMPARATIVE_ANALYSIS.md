# Comparative Analysis: Python TTP vs GoTTP

## Executive Summary

This document provides a detailed comparative analysis between Python TTP and GoTTP implementations, focusing on logic and data structure differences that may be causing test failures. The analysis covers architecture, data structures, result processing, pattern matching, and function execution.

**Key Finding**: While GoTTP maintains high compatibility with Python TTP, there are several architectural and implementation differences in how results are processed, stored, and transformed that could lead to subtle behavioral differences.

---

## 1. Architecture Overview

### Python TTP Architecture

**Stateful Design:**
- Maintains internal state between parse calls
- Uses a `_parser_class` that holds parsing state
- Results are accumulated in `main_results` dictionary
- Groups maintain `runs` dictionaries for default values
- Uses a `_results_class` to build results structure incrementally

**Key Components:**
1. **Parser Class** (`_parser_class`): Handles regex matching and result collection
2. **Results Class** (`_results_class`): Builds final results structure using `start()`, `add()`, `end()`, `join()` methods
3. **Group Class** (`_group_class`): Manages group patterns, defaults, and functions
4. **Template Class** (`_template_class`): Manages template parsing and group organization

**Data Flow:**
```
Template → Groups → Parser → Raw Results → Results Class → Final Results
```

### GoTTP Architecture

**Stateless Design:**
- Compiled templates are immutable
- Each `Parse()` call is independent
- No internal state between calls
- Results are built in a single pass

**Key Components:**
1. **Runtime** (`Runtime`): Executes compiled templates
2. **MatchCollector** (`MatchCollector`): Collects and merges pattern matches
3. **PathResolver** (`PathResolver`): Resolves dynamic paths
4. **CompiledTemplate** (`CompiledTemplate`): Immutable compiled template structure

**Data Flow:**
```
CompiledTemplate → Runtime → MatchCollector → Path Resolution → Final Results
```

### Key Architectural Differences

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **State Management** | Stateful - maintains state between calls | Stateless - each call is independent |
| **Result Building** | Incremental - uses `start()`, `add()`, `end()`, `join()` methods | Single-pass - builds results in one iteration |
| **Group Processing** | Groups processed sequentially with state tracking | Groups processed independently per input |
| **Default Values** | Stored in group `runs` dictionary, updated during parsing | Applied during result formation |
| **Path Resolution** | Resolved during result saving in `save_curelements()` | Resolved before storing results |

---

## 2. Result Processing and Storage

### Python TTP Result Processing

**Result Building Process:**

1. **Raw Results Collection** (`_parser_class.parse()`):
   - Collects matches in `results` dictionary keyed by `span_start`
   - Each entry: `{span_start: [(regex, match_data), ...]}`
   - Results are sorted by `span_start` before processing

2. **Result Formation** (`_results_class.make_results()`):
   - Iterates through sorted raw results
   - Uses action-based methods: `start()`, `add()`, `end()`, `join()`
   - Maintains a `record` dictionary with current state:
     ```python
     {
         "result": {},      # Current match data
         "PATH": [],        # Current path
         "DEFAULTS": {},    # Group default values
         "FUNCTIONS": [],   # Group functions
         "GRP_ID": None,    # Current group ID
     }
     ```

3. **Path Resolution**:
   - Paths are resolved when saving: `save_curelements(result_data, result_path)`
   - Uses `dict_by_path()` to navigate/create nested structure
   - Handles `*` and `**` formatters during path navigation
   - Dynamic paths resolved using `form_path()` method

4. **Result Storage**:
   - Single match → stored as dictionary
   - Multiple matches → converted to list using `value_to_list()`
   - Path with `*` → ensures list storage
   - Path with `**` → updates existing dict (merge behavior)

**Key Behavior:**
- Top-level groups: single match = dict, multiple matches = list
- Nested groups: always stored as list if path has `*`
- Anonymous groups: stored as `_anonymous_*` (always list)

### GoTTP Result Processing

**Result Building Process:**

1. **Match Collection** (`parseGroup()`):
   - Collects all matches first
   - Merges matches based on pattern types and indicators
   - Returns list of match dictionaries

2. **Result Storage** (`Runtime.Parse()`):
   - Processes each group independently
   - Resolves dynamic paths before storing
   - Uses `storeAtPath()` to store results

3. **Path Resolution**:
   - Dynamic paths resolved using `PathResolver.ResolvePath()`
   - Path variables extracted and removed from match data
   - Path resolution happens **before** storing results

4. **Result Storage Logic**:
   ```go
   // Single match → map
   if len(matches) == 1 {
       valueToStore = matches[0]
   } else {
       // Multiple matches → list
       valueToStore = resultList
   }
   ```

**Key Differences:**

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **Result Building** | Incremental with state tracking | Single-pass, all matches collected first |
| **Path Resolution Timing** | During result saving | Before result storing |
| **Match Merging** | Handled in `add()` method based on PATH | Handled in `parseGroup()` before storage |
| **Default Values** | Applied during `start()`/`add()` via DEFAULTS | Applied during group function execution |
| **Group Functions** | Applied after result formation via `processgrp()` | Applied during result processing |

### Critical Differences in Result Structure

1. **Top-Level Group Storage:**
   - **Python TTP**: Single match stored as dict, multiple as list
   - **GoTTP**: Same behavior, but path resolution happens earlier

2. **Dynamic Path Variables:**
   - **Python TTP**: Path variables may remain in result if not explicitly removed
   - **GoTTP**: Path variables are explicitly removed via `removePathVars()`

3. **Anonymous Groups:**
   - **Python TTP**: Stored as `_anonymous_*` (always list)
   - **GoTTP**: Same, but merged into root for `per_template` results method

4. **Nested Group Handling:**
   - **Python TTP**: Nested groups processed within parent context
   - **GoTTP**: Nested groups processed recursively within parent

---

## 3. Pattern Matching and Merging

### Python TTP Pattern Matching

**Match Collection:**
- Uses regex `finditer()` to find all matches
- Stores matches in dictionary: `{span_start: [(regex, match_data), ...]}`
- Sorts by `span_start` before processing

**Match Merging Logic:**
- Handled in `add()` method of `_results_class`
- Merges if `PATH` matches current `record["PATH"]`
- Uses `result.update(self.record["result"])` to merge
- Preserves existing values (new values don't override)

**Pattern Priority:**
1. `startempty` (_start_ pattern) - highest priority
2. `start` patterns
3. `normal` patterns
4. `line` patterns - lowest priority

**Start/End Pattern Handling:**
- `_start_` pattern starts new group
- Patterns between `_start_` and `_end_` merge into same match
- `_end_` pattern finalizes group
- `_line_` pattern always merges (uses `join` action)

### GoTTP Pattern Matching

**Match Collection:**
- Collects all matches first in `parseGroup()`
- Uses `MatchCollector` to organize matches
- Sorts matches by position

**Match Merging Logic:**
- Handled in `parseGroup()` before returning results
- Complex logic to determine when to merge vs. start new match
- Considers pattern types, indicators, and positions

**Key Merging Rules:**
1. `_line_` indicator: always merges
2. `_start_`/`_end_`: patterns between merge into same match
3. Same pattern appearing again: may start new match or merge based on context
4. Different patterns: merge if within `maxGap`, otherwise start new

**Critical Differences:**

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **Merging Timing** | During result building (`add()` method) | During match collection (`parseGroup()`) |
| **Merge Decision** | Based on PATH equality | Based on pattern type, position, and indicators |
| **Start Pattern Handling** | Starts new group, saves previous | May merge if conditions met |
| **End Pattern Handling** | Finalizes group | Marks match as ended, prevents further merging |

### Potential Issues

1. **Merge Decision Logic:**
   - GoTTP's merge logic is more complex and may differ in edge cases
   - Python TTP's merge is simpler (PATH-based) but may miss some cases

2. **Pattern Priority:**
   - Both handle priority similarly, but GoTTP may have different edge case handling

3. **Start/End Pattern Behavior:**
   - GoTTP has extensive logic for `_start_`/`_end_` handling
   - May differ in cases with multiple `_start_` patterns before `_end_`

---

## 4. Group Functions

### Python TTP Group Functions

**Execution Timing:**
- Group functions executed in `processgrp()` method
- Called after result data is collected but before saving
- Functions receive current `record["result"]` and `record["DEFAULTS"]`

**Function Execution:**
```python
for func in FUNCTIONS:
    result, flag = func(data, *args, **kwargs)
    if flag is False:
        # Filter out result
        return False
    data = result
```

**Key Functions:**
- `set()`: Sets variable from template vars or defaults
- `contains()`: Filters results containing specified keys
- `exclude()`: Filters out results with specified keys
- `record()`: Records variable for later use
- `delete()`: Removes specified keys

**Default Values:**
- Stored in `group.runs` dictionary
- Updated during parsing via `update_groups_runs()`
- Applied to results via `DEFAULTS` in `record`

### GoTTP Group Functions

**Execution Timing:**
- Group functions executed during result processing
- Applied to each match result before storing
- Functions receive match data and return modified data

**Function Execution:**
```go
for _, fn := range group.Functions {
    result, err := fn.Execute(matchData, vars)
    if err != nil || result == nil {
        // Filter out result
        continue
    }
    matchData = result
}
```

**Key Differences:**

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **Execution Timing** | After all matches collected, before saving | During match processing, before storing |
| **Default Values** | Applied via DEFAULTS in record | Applied separately, may differ in timing |
| **Function Context** | Has access to full record state | Only has access to current match |
| **Filtering** | Returns `False` to filter | Returns `nil` to filter |

### Potential Issues

1. **Set Function Argument Order:**
   - **Python TTP**: `set(source, target="_use_source_", default="_no_default_value_")`
   - **GoTTP**: May have different argument handling
   - **Issue**: Argument order or default value handling may differ

2. **Default Value Application:**
   - **Python TTP**: Defaults applied via `DEFAULTS` during `start()`/`add()`
   - **GoTTP**: Defaults may be applied differently
   - **Issue**: Default values may not be applied at the same time or in the same way

3. **Function Execution Context:**
   - **Python TTP**: Functions have access to full record state
   - **GoTTP**: Functions only have access to current match
   - **Issue**: Functions that depend on previous matches may behave differently

---

## 5. Match Functions

### Python TTP Match Functions

**Execution:**
- Executed during regex match processing in `check_matches()`
- Functions receive matched data and return `(data, flag)`
- `flag=False` means validation failed, result is filtered
- `flag=None` means no validation, data is transformed

**Key Functions:**
- `resub()`: Regex substitution with `count=1` by default (replaces first occurrence)
- `resuball()`: Regex substitution replacing all occurrences
- `to_int()`, `to_float()`, `to_str()`: Type conversion
- `lookup()`: Lookup table matching

**Function Signature:**
```python
def function_name(data, *args, **kwargs):
    # Process data
    return data, flag  # flag can be True, False, or None
```

### GoTTP Match Functions

**Execution:**
- Executed during variable processing in `parseGroup()`
- Functions receive matched data and return transformed data
- Error or `nil` return means validation failed

**Key Differences:**

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **Return Value** | `(data, flag)` tuple | `data` or `error` |
| **Validation** | `flag=False` filters result | `error` or `nil` filters result |
| **Default Arguments** | Uses Python defaults | May use Go defaults (different) |

### Potential Issues

1. **Resub Function:**
   - **Python TTP**: `resub(data, old, new, count=1)` - replaces first occurrence by default
   - **GoTTP**: May have different default behavior
   - **Issue**: Default `count` parameter may differ

2. **Type Conversion:**
   - **Python TTP**: Returns Python types (int, float, str)
   - **GoTTP**: Returns Go types (int64, float64, string)
   - **Issue**: Type differences may cause comparison issues

3. **Lookup Function:**
   - **Python TTP**: May have different lookup behavior
   - **GoTTP**: Implementation may differ
   - **Issue**: Lookup matching logic may differ

---

## 6. Path Resolution and Dynamic Paths

### Python TTP Path Resolution

**Path Formation:**
- Paths formed using `form_path()` method
- Handles dynamic variables: `{{ variable }}`
- Resolves paths when saving results in `save_curelements()`

**Dynamic Path Handling:**
- Variables in path are resolved from match data or template vars
- Path variables may remain in result data (not explicitly removed)
- Path resolution happens during result saving

**Path Formatters:**
- `*`: Ensures list storage
- `**`: Merge behavior (updates existing dict)

### GoTTP Path Resolution

**Path Formation:**
- Paths resolved using `PathResolver.ResolvePath()`
- Handles dynamic variables: `{{ variable }}`
- Resolves paths **before** storing results

**Dynamic Path Handling:**
- Variables in path are explicitly extracted and removed from match data
- Uses `removePathVars()` to clean match data
- Path resolution happens before result storage

**Key Differences:**

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **Resolution Timing** | During result saving | Before result storing |
| **Variable Removal** | May remain in result | Explicitly removed |
| **Path Extraction** | Uses `form_path()` | Uses `ExtractVariablesFromPath()` |

### Potential Issues

1. **Path Variable Removal:**
   - **Python TTP**: Path variables may remain in results
   - **GoTTP**: Path variables are explicitly removed
   - **Issue**: Results may have different keys

2. **Path Resolution Timing:**
   - **Python TTP**: Resolved during saving
   - **GoTTP**: Resolved before storing
   - **Issue**: May affect group function execution context

---

## 7. Input Processing

### Python TTP Input Processing

**Input Handling:**
- Inputs processed per group via `input` attribute
- Input functions executed before parsing
- Input data stored in `DATATEXT` and `DATANAME`

**Input Functions:**
- Executed in sequence before parsing
- Modify `DATATEXT` directly
- Can return `False` to stop processing

### GoTTP Input Processing

**Input Handling:**
- Inputs processed per group via `Input` attribute
- Input functions executed before parsing
- Input data passed to `parseGroup()`

**Key Differences:**

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **Input Storage** | Stored in `DATATEXT` | Passed directly to parser |
| **Function Execution** | Modifies `DATATEXT` in place | Returns processed data |
| **Error Handling** | Returns `False` to stop | Returns error |

---

## 8. Template Variables and Defaults

### Python TTP Variables

**Variable Storage:**
- Template variables stored in `vars` dictionary
- Group default values stored in `group.runs`
- Updated during parsing via `update_groups_runs()`

**Variable Application:**
- Applied during `start()`/`add()` via `DEFAULTS`
- Merged into result data
- Can be overridden by match data

### GoTTP Variables

**Variable Storage:**
- Template variables stored in `compiled.Vars`
- Group default values stored in group structure
- Applied during result processing

**Key Differences:**

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **Default Storage** | `group.runs` dictionary | Group structure fields |
| **Application Timing** | During `start()`/`add()` | During result processing |
| **Update Mechanism** | `update_groups_runs()` | Applied directly |

---

## 9. Results Method Handling

### Python TTP Results Methods

**`per_input` Method:**
- Results stored per input name
- Structure: `{input_name: {group_name: results}}`
- Each input processed independently

**`per_template` Method:**
- All results in single structure
- Structure: `{group_name: results}`
- Results from all inputs merged

### GoTTP Results Methods

**`per_input` Method:**
- Results stored per input name
- Structure: `{input_name: {group_name: results}}`
- Similar to Python TTP

**`per_template` Method:**
- All results in single structure
- Anonymous groups merged into root
- May differ in merge behavior

**Key Differences:**

| Aspect | Python TTP | GoTTP |
|--------|-----------|-------|
| **Anonymous Groups** | Stored as `_anonymous_*` | Merged into root for `per_template` |
| **Input Ordering** | Preserves input order | May differ in ordering |

---

## 10. Specific Areas of Concern

### 10.1 Group Function Execution

**Issue**: Group functions may execute at different times or with different context.

**Python TTP:**
- Functions execute after all matches collected
- Have access to full record state
- Default values applied via `DEFAULTS`

**GoTTP:**
- Functions execute during match processing
- Only have access to current match
- Default values may be applied differently

**Recommendation**: Verify that group functions receive the same data and context in both implementations.

### 10.2 Match Merging Logic

**Issue**: Complex merge logic may differ in edge cases.

**Python TTP:**
- Simple PATH-based merging
- Merges if `PATH == record["PATH"]`

**GoTTP:**
- Complex position and pattern-based merging
- Considers pattern types, indicators, and positions

**Recommendation**: Test edge cases with multiple patterns, `_start_`/`_end_` indicators, and `_line_` patterns.

### 10.3 Path Variable Removal

**Issue**: Path variables may be handled differently.

**Python TTP:**
- Path variables may remain in results

**GoTTP:**
- Path variables explicitly removed

**Recommendation**: Verify that results have the same keys after path resolution.

### 10.4 Default Value Application

**Issue**: Default values may be applied at different times.

**Python TTP:**
- Applied during `start()`/`add()` via `DEFAULTS`
- Merged into result data

**GoTTP:**
- Applied during result processing
- May be applied differently

**Recommendation**: Verify that default values are applied correctly and at the right time.

### 10.5 Result Structure Formation

**Issue**: Result structure may differ for edge cases.

**Python TTP:**
- Uses `value_to_list()` to convert dict to list
- Handles `*` and `**` formatters during path navigation

**GoTTP:**
- Determines list vs. dict before storing
- Handles `*` and `**` formatters during path resolution

**Recommendation**: Test cases with single vs. multiple matches, nested groups, and dynamic paths.

---

## 11. Recommendations for Fixing Test Failures

### 11.1 Investigate Group Function Failures

**Tests Affected:**
- `TestGroupFunctionBasic`
- `TestGroupFunctionTransformation`
- `TestGroupFunctionChain`
- `TestGroupFunctionMacro`

**Actions:**
1. Verify group function execution timing
2. Check default value application
3. Verify function argument handling (especially `set()` function)
4. Test function context and available data

### 11.2 Investigate Match Function Failures

**Tests Affected:**
- `TestMatchFunctionString`
- `TestMatchFunctionTypeConversion`
- `TestMatchFunctionIPMAC`
- `TestMatchFunctionConditions`
- `TestMatchFunctionDataManipulation`
- `TestMatchFunctionLookup`
- `TestMatchFunctionChain`
- `TestMatchFunctionToUnicode`

**Actions:**
1. Verify `resub()` default `count` parameter
2. Check type conversion return types
3. Verify lookup function behavior
4. Test function return value handling

### 11.3 Investigate Macro Failures

**Tests Affected:**
- `TestMacroStarlark`
- `TestMacroMatchFunction`
- `TestMacroMultiple`
- `TestMacroWithVars`

**Actions:**
1. Verify macro execution context
2. Check `_ttp_` dictionary availability
3. Verify variable access in macros
4. Test macro return value handling

### 11.4 Investigate Output Formatter Failures

**Tests Affected:**
- `TestOutputFormatterRaw`
- `TestOutputFormatterPPrint`
- `TestOutputFormatterWithFunctions`

**Actions:**
1. Verify formatter input data structure
2. Check formatter function execution
3. Verify output format matching

### 11.5 Investigate Template Test Failures

**Tests Affected:**
- `TestTemplateVariables`
- `TestTemplateExtend`

**Actions:**
1. Verify template variable resolution
2. Check extend tag handling
3. Verify variable merging

---

## 12. Debugging Strategy

### 12.1 Add Detailed Logging

**Areas to Log:**
1. Match collection and merging decisions
2. Path resolution and variable removal
3. Group function execution and results
4. Match function execution and results
5. Result storage and structure formation

### 12.2 Compare Intermediate Results

**Compare:**
1. Raw match results (before merging)
2. Merged match results (after merging)
3. Path-resolved results (after path resolution)
4. Group function results (after function execution)
5. Final results (after storage)

### 12.3 Test with Minimal Cases

**Strategy:**
1. Start with simplest failing test
2. Add logging to identify first difference
3. Fix issue and move to next
4. Build up to complex cases

### 12.4 Use Python TTP as Reference

**Strategy:**
1. Run same template/data through Python TTP
2. Capture intermediate results at each stage
3. Compare with GoTTP intermediate results
4. Identify first point of divergence

---

## 13. Conclusion

The analysis reveals several key areas where Python TTP and GoTTP differ in implementation:

1. **Result Processing**: Python TTP uses incremental state-based building, GoTTP uses single-pass building
2. **Path Resolution**: Python TTP resolves during saving, GoTTP resolves before storing
3. **Group Functions**: Python TTP executes after collection, GoTTP executes during processing
4. **Match Merging**: Python TTP uses simple PATH-based merging, GoTTP uses complex position-based merging
5. **Default Values**: Python TTP applies via DEFAULTS, GoTTP applies during processing

These differences, while maintaining overall compatibility, may cause subtle behavioral differences in edge cases. The recommendations above should help identify and fix the remaining test failures.

---

## Appendix: Key Code Locations

### Python TTP
- Result building: `ttp.py:2690-2833` (`make_results()`)
- Match merging: `ttp.py:3111-3133` (`add()`)
- Path resolution: `ttp.py:2948-3028` (`dict_by_path()`, `save_curelements()`)
- Group functions: `ttp.py:3235-3512` (`processgrp()`)
- Match functions: `ttp.py:2445-2501` (`check_matches()`)

### GoTTP
- Result building: `internal/compiled/runtime.go:90-500` (`Parse()`)
- Match merging: `internal/compiled/runtime.go:600-1500` (`parseGroup()`)
- Path resolution: `internal/compiled/path_resolver.go`
- Group functions: `internal/functions/group/`
- Match functions: `internal/functions/match/`



