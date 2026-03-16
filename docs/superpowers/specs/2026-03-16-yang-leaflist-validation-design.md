# YANG Leaf-List Validation Fix — Design Spec

**Date:** 2026-03-16
**Status:** Approved

## Summary

The YANG validator does not handle `leaf-list` values correctly. When a template produces an array (`[]interface{}`) for a `leaf-list` YANG node, the validator passes the entire slice to scalar type checking, causing false validation failures. Two downstream-reported issues stem from this single root cause.

## Issues

### Issue 1: leaf-list type not recognized from Go slices

The validator's `validateFieldTypes()` calls `validateValueType(value, type, entry)` with the entire `[]interface{}` slice as `value`. Since no scalar type case matches `[]interface{}`, validation fails with "invalid type []interface{}, expected string/decimal64/etc."

### Issue 2: decimal64 range validation fails on integer representations

When Starlark produces integer values (e.g., `52` instead of `52.0`) for `decimal64` fields inside leaf-lists, the range validator can't process the slice. This resolves automatically once Issue 1 is fixed, since `validateValueType()` already handles integers for decimal64.

## Fix

**File:** `internal/yang/validator.go`, function `validateFieldTypes()`

Add a leaf-list detection branch between the scalar leaf check (line ~166) and the container/list check (line ~184):

```go
// After scalar leaf validation, before container check:
if childEntry.Type != nil {
    // Check if value is a slice (leaf-list)
    if slice, ok := value.([]interface{}); ok {
        // Validate each element against the leaf-list's type
        for i, elem := range slice {
            if !validateValueType(elem, childEntry.Type, childEntry) {
                // report error for element i
            }
            // Also validate constraints per element
        }
        continue // skip scalar validation
    }
    // existing scalar validation...
}
```

Detection: use Go type assertion `value.([]interface{})` combined with `childEntry.Type != nil` (leaf-lists have Type, containers have Dir). The `goyang` library's `childEntry.IsLeafList()` can be used for additional safety but isn't strictly required since the type assertion on the value handles it.

**No changes needed to:**
- `validateValueType()` — already handles all scalar types correctly including integers for decimal64
- `validateRange()` — already converts integers to float64
- Any other validator functions

## Testing

**New file:** `internal/yang/validator_test.go`

Tests:
1. leaf-list of strings — valid string array passes
2. leaf-list of strings — invalid element type fails
3. leaf-list of decimal64 — valid float array passes
4. leaf-list of decimal64 — integer elements pass (Issue 2 verification)
5. leaf-list of decimal64 — mixed int/float elements pass
6. Regular leaf validation still works (no regression)

## Risk

Very low — additive change that only affects the previously-unhandled leaf-list case. No changes to existing scalar validation logic.
