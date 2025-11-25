# Issue #13 Solution Analysis

## Problem
The parent group's pattern `Major rev {{major}}, Minor rev {{minor}}` matches both "Major rev" lines:
- Line 53: "Major rev 6, Minor rev 23" (should be parent's major/minor)
- Line 58: "Major rev 01, Minor rev 02" (should be nested "io" group's major/minor)

The second match overwrites the first, even though the second "Major rev" line should only be matched by the nested "io" group.

## Root Cause
1. GoTTP processes parent groups first, then nested groups
2. The parent group's pattern matches ALL lines, including those that should only be matched by nested groups
3. When the same pattern matches multiple times in the same match block, the second match overwrites the first

## Solution Options

### Option 1: Comprehensive "First Match Wins" Strategy (RECOMMENDED)
**Approach**: Ensure that when merging patterns, we NEVER overwrite existing values, regardless of code path.

**Pros**:
- Matches Python TTP's behavior: "Preserves existing values (new values don't override)"
- Minimal code changes
- Works for all cases, not just this specific issue

**Cons**:
- Need to ensure fix is applied in ALL merge paths
- May need to audit all places where `currentMatch[k] = v` is used

**Implementation**:
- Apply the fix in ALL merge paths (already done in most places)
- Add a helper function to ensure consistent behavior:
  ```go
  func setIfNotExists(match map[string]interface{}, key string, value interface{}) {
      if _, exists := match[key]; !exists {
          match[key] = value
      }
  }
  ```

### Option 2: Filter Matches During Collection
**Approach**: When collecting matches, if the same pattern matches multiple times at the same position or within the same match block, only keep the first match.

**Pros**:
- Prevents the issue at the source
- More efficient (fewer matches to process)

**Cons**:
- Complex to determine which matches belong to the same match block
- May break legitimate cases where the same pattern should match multiple times

### Option 3: Process Nested Groups First, Then Exclude Their Lines
**Approach**: Process nested groups first, track which lines they matched, then skip those lines when processing the parent group.

**Pros**:
- Most accurate to Python TTP's behavior
- Prevents parent group from matching lines that belong to nested groups

**Cons**:
- Requires significant refactoring
- Complex to track line positions across different parent matches
- May break existing behavior

### Option 4: Pattern Priority/Scope Awareness
**Approach**: Give nested group patterns higher priority, or make parent group patterns aware of nested group scope.

**Pros**:
- Conceptually correct
- Prevents the issue at the source

**Cons**:
- Requires significant refactoring
- Complex to implement correctly
- May break existing behavior

## Recommended Solution: Option 1

The fix we've already applied (preventing overwriting when merging) is the right approach, but we need to ensure it's applied consistently. The issue is that there may be code paths where values are still being overwritten.

**Next Steps**:
1. Audit all places where `currentMatch[k] = v` is used
2. Ensure the "first match wins" logic is applied everywhere
3. Add a helper function to ensure consistent behavior
4. Add tests to verify the fix works for all cases

