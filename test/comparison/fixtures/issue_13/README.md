# Issue #13 Test Fixtures

This directory contains `gottp-config.json` files exported from the GoTTP editor for testing issue #13 (Inconsistent Output compared to Python TTP).

## How to Add a Test Case

1. Export your editor configuration from the GoTTP editor:
   - Open the editor
   - Click "File" → "Export"
   - Save the `gottp-config.json` file

2. Place the file in this directory:
   - Rename it to `gottp-config.json` (or keep the original name)
   - Place it in `test/comparison/fixtures/issue_13/`

3. Run the test:
   ```bash
   go test ./test/comparison -v -run TestIssue13
   ```

## Test Behavior

The test will:
- Load the `gottp-config.json` file
- Extract the template, input data, variables, and lookups
- Run both Python TTP and GoTTP with the same configuration
- Compare the outputs and report any differences

## Multiple Test Cases

If you have multiple config files to test, you can:
- Place them in subdirectories (e.g., `issue_13/case1/gottp-config.json`)
- Or rename them (e.g., `gottp-config-case1.json`) and update the test to load them

