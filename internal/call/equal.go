package call

import "reflect"

// Equal is Python's == for the values gottp's functions see: strings
// compare as strings, numbers (bool, int, float) as numbers (True == 1),
// None only to None, and a string never equals a number ("1" != 1,
// "True" != True). Lists and dicts compare item by item.
func Equal(a, b interface{}) bool {
	if fa, ok := pyNumber(a); ok {
		fb, ok := pyNumber(b)
		return ok && fa == fb
	}
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		return ok && x == y
	case nil:
		return b == nil
	case []interface{}:
		y, ok := b.([]interface{})
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !Equal(x[i], y[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		y, ok := b.(map[string]interface{})
		if !ok || len(x) != len(y) {
			return false
		}
		for k, v := range x {
			w, ok := y[k]
			if !ok || !Equal(v, w) {
				return false
			}
		}
		return true
	}
	if _, ok := pyNumber(b); ok {
		return false
	}
	if _, ok := b.(string); ok {
		return false
	}
	return reflect.DeepEqual(a, b)
}

// pyNumber returns a bool, integer or float as a float64.
func pyNumber(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float64:
		return x, true
	case float32:
		return float64(x), true
	}
	return 0, false
}
