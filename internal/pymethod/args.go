package pymethod

import (
	"math"
	"math/big"

	"github.com/roc-ops/gottp/internal/call"
)

// args are one call's arguments, read the way CPython's argument clinic
// reads them for the method, with its TypeError messages.
type args struct {
	typ, name string
	pos       []interface{}
	kw        []call.Kwarg
	// call is the call the arguments come from; nil for a method gottp
	// calls itself.
	call *call.Call
}

// isTuple reports whether positional argument i is a tuple, not a list.
func (a *args) isTuple(i int) bool { return a.call != nil && a.call.IsTuple(i) }

func (a *args) qual() string { return a.typ + "." + a.name }

// none checks that the method got no arguments.
func (a *args) none() error {
	if err := a.noKwargs(); err != nil {
		return err
	}
	if len(a.pos) > 0 {
		return raise("TypeError", "%s() takes no arguments (%d given)", a.qual(), len(a.pos))
	}
	return nil
}

// noKwargs checks that a method whose arguments are positional-only got no
// keyword arguments.
func (a *args) noKwargs() error {
	if len(a.kw) > 0 {
		return raise("TypeError", "%s() takes no keyword arguments", a.qual())
	}
	return nil
}

// positional checks a method of positional-only arguments: min to max of
// them.
func (a *args) positional(min, max int) error {
	if err := a.noKwargs(); err != nil {
		return err
	}
	n := len(a.pos)
	if n >= min && n <= max {
		return nil
	}
	switch {
	case min == max && min == 1:
		return raise("TypeError", "%s() takes exactly one argument (%d given)", a.qual(), n)
	case n < min && max == min:
		return raise("TypeError", "%s expected %d arguments, got %d", a.name, min, n)
	case n < min:
		return raise("TypeError", "%s expected at least %d argument%s, got %d", a.name, min, plural(min), n)
	}
	return raise("TypeError", "%s expected at most %d argument%s, got %d", a.name, max, plural(max), n)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// keywords binds positional and keyword arguments to the parameters names
// (all optional), as for str.split(sep=None, maxsplit=-1). The result has
// one entry per name, nil where not given (absent and None are told apart
// by given).
func (a *args) keywords(names ...string) (vals []interface{}, given []bool, err error) {
	vals = make([]interface{}, len(names))
	given = make([]bool, len(names))
	if len(a.pos) > len(names) {
		return nil, nil, raise("TypeError", "%s() takes at most %d argument%s (%d given)", a.qual(), len(names), plural(len(names)), len(a.pos))
	}
	for i, v := range a.pos {
		vals[i], given[i] = v, true
	}
	for _, kw := range a.kw {
		i := -1
		for j, n := range names {
			if n == kw.Key {
				i = j
			}
		}
		if i < 0 {
			return nil, nil, raise("TypeError", "%s() got an unexpected keyword argument '%s'", a.qual(), kw.Key)
		}
		if given[i] {
			return nil, nil, raise("TypeError", "argument for %s() given by name ('%s') and position (%d)", a.qual(), kw.Key, i+1)
		}
		vals[i], given[i] = kw.Value, true
	}
	return vals, given, nil
}

// typeName is Python's type(v).__name__.
func typeName(v interface{}) string {
	switch v.(type) {
	case string:
		return "str"
	case bool:
		return "bool"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, *big.Int:
		return "int"
	case float64, float32:
		return "float"
	case []interface{}:
		return "list"
	case map[string]interface{}:
		return "dict"
	case nil:
		return "NoneType"
	}
	return "object"
}

// str reads a str argument.
func (a *args) str(v interface{}, what string) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", raise("TypeError", "%s() argument %s must be str, not %s", a.qual(), what, typeName(v))
	}
	return s, nil
}

// index reads an integer argument (an __index__: int or bool) that fits an
// int, as Py_ssize_t arguments are read.
func index(v interface{}) (int, error) {
	n, ok := toInt(v)
	if !ok {
		return 0, raise("TypeError", "'%s' object cannot be interpreted as an integer", typeName(v))
	}
	if !n.IsInt64() || n.Int64() > math.MaxInt || n.Int64() < math.MinInt {
		return 0, raise("OverflowError", "Python int too large to convert to C ssize_t")
	}
	return int(n.Int64()), nil
}

// sliceIndex reads a start or end argument of find, count, startswith and
// the like: an int, or None for the default.
func sliceIndex(v interface{}, given bool, def int) (int, error) {
	if !given || v == nil {
		return def, nil
	}
	n, ok := toInt(v)
	if !ok {
		return 0, raise("TypeError", "slice indices must be integers or None or have an __index__ method")
	}
	// CPython clamps a huge index to Py_ssize_t, which adjustIndices clamps
	// again to the string.
	if !n.IsInt64() {
		if n.Sign() < 0 {
			return math.MinInt, nil
		}
		return math.MaxInt, nil
	}
	return int(n.Int64()), nil
}

// toInt returns an int or bool as a *big.Int.
func toInt(v interface{}) (*big.Int, bool) {
	switch x := v.(type) {
	case bool:
		if x {
			return big.NewInt(1), true
		}
		return big.NewInt(0), true
	case int:
		return big.NewInt(int64(x)), true
	case int8:
		return big.NewInt(int64(x)), true
	case int16:
		return big.NewInt(int64(x)), true
	case int32:
		return big.NewInt(int64(x)), true
	case int64:
		return big.NewInt(x), true
	case uint:
		return new(big.Int).SetUint64(uint64(x)), true
	case uint8:
		return big.NewInt(int64(x)), true
	case uint16:
		return big.NewInt(int64(x)), true
	case uint32:
		return big.NewInt(int64(x)), true
	case uint64:
		return new(big.Int).SetUint64(x), true
	case *big.Int:
		return x, true
	}
	return nil, false
}

// fromBig returns n as an int when it fits, else a uint64, else n itself.
func fromBig(n *big.Int) interface{} {
	if n.IsInt64() && n.Int64() >= math.MinInt && n.Int64() <= math.MaxInt {
		return int(n.Int64())
	}
	if n.IsUint64() {
		return n.Uint64()
	}
	return n
}
