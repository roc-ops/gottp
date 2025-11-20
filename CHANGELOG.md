# Changelog

All notable changes to GoTTP will be documented in this file.

## [Unreleased]

### Fixed
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

