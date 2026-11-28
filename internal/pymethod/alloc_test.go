package pymethod

import "testing"

// TestMissAllocatesNothing: a name the value's type doesn't have (a pattern
// name such as MAC left in the chain, or a method of another type) costs
// no allocation, so templates that never use the fallback don't pay for it.
func TestMissAllocatesNothing(t *testing.T) {
	var v interface{} = "aa:bb"
	var n interface{} = int64(5)
	allocs := testing.AllocsPerRun(100, func() {
		Call(v, "MAC", nil, nil)
		Call(n, "upper", nil, nil)
	})
	if allocs != 0 {
		t.Errorf("a miss allocates %v times", allocs)
	}
}
