package macro

import (
	"reflect"
	"testing"
)

// Results can hold []map[string]interface{} and []string; they must reach
// a macro as lists, not as their %v string (#142).
func TestGoToStarlark_TypedSlices(t *testing.T) {
	e := NewStarlarkEngine()
	in := map[string]interface{}{
		"maps": []map[string]interface{}{{"a": "1"}},
		"strs": []string{"x", "y"},
	}
	got := e.StarlarkToGo(e.GoToStarlark(in))
	want := map[string]interface{}{
		"maps": []interface{}{map[string]interface{}{"a": "1"}},
		"strs": []interface{}{"x", "y"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}
