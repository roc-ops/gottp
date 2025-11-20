# Compatibility Test Suite

This directory contains compatibility tests based on the original Python TTP test suite.

## Test Structure

Tests are organized to match the Python TTP test structure:

- `compatibility_test.go` - Basic parsing and structure tests
- `regexes_test.go` - Regex pattern matching tests
- `match_functions_test.go` - Match function tests
- `group_name_test.go` - Group name attribute tests
- `anonymous_group_test.go` - Anonymous group tests
- `extend_tag_test.go` - Template extension tests

## Running Tests

```bash
# Run all compatibility tests
go test ./test -v

# Run specific test
go test ./test -v -run TestBasicParsing

# Run with detailed output
go test ./test -v -json
```

## Test Assets

Test assets (template files, data files) are stored in `test/assets/` directory, matching the structure of the original Python TTP tests.

## Status

Tests are being converted from Python TTP and validated against GoTTP. Some tests may need adjustments due to differences in result structure or implementation details.

