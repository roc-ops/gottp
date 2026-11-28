// Package pymethod ports the Python 3.12 methods and attributes of str,
// int, float, bool, list and dict that Python TTP 0.10.1 falls back to for
// a match variable function that isn't a TTP function:
//
//	attrib = getattr(data, func_name)          # ttp.py:2464
//	run_result = attrib(*args, **kwargs) if callable(attrib) else attrib
//
// A True or False result is a filter (False drops the match); any other
// result replaces the value. A name the value's type doesn't have is an
// AttributeError, which TTP logs and skips, keeping the value. Any other
// exception (ValueError from str.index, TypeError from a bad argument)
// propagates out of TTP's parse.
//
// Values are the Go forms of Python's: str is string, int is int, int64 or
// uint64, float is float64, bool is bool (an int subclass, so it has int's
// methods), list is []interface{} and dict is map[string]interface{}. A
// tuple result (str.partition, int.as_integer_ratio) is a list.
//
// Methods that change their object in place (list.sort, dict.update, ...)
// and methods whose result has no JSON form (str.encode gives bytes) are not
// ported: NotPorted names them, and the value is kept.
package pymethod

import (
	"fmt"
	"sort"

	"github.com/roc-ops/gottp/internal/call"
)

// Error is a Python exception raised by a method, such as ValueError from
// str.index. Python TTP doesn't catch it, so its parse fails.
type Error struct {
	// Type is the exception class, e.g. "ValueError".
	Type string
	// Msg is the exception's message.
	Msg string
}

func (e *Error) Error() string { return e.Type + ": " + e.Msg }

func raise(typ, format string, a ...interface{}) error {
	return &Error{Type: typ, Msg: fmt.Sprintf(format, a...)}
}

// method is one method or attribute of a type. self is the value, of the
// type's Go form (see the package doc).
type method struct {
	// attr is true for a data attribute (int.real): it isn't called, so the
	// call's arguments are ignored, as TTP ignores them.
	attr bool
	fn   func(self interface{}, a *args) (interface{}, error)
}

type table map[string]method

// Call runs value's method or attribute name with args and kwargs. found
// is false on Python's AttributeError, which TTP catches and logs, keeping
// the value: value's type has no such method, or the method raised it.
// err is a Python exception (*Error). A bool result is a filter, any other
// result the new value. A miss allocates nothing: the value is converted
// only once the method is found.
func Call(value interface{}, c *call.Call) (result interface{}, found bool, err error) {
	t := typeOf(value)
	if t == nil {
		return nil, false, nil
	}
	m, ok := t.methods[c.Name]
	if !ok {
		return nil, false, nil
	}
	a := &args{typ: t.name, name: c.Name, pos: c.Args, kw: c.Kwargs, call: c}
	r, err := m.fn(selfOf(value), a)
	if e, ok := err.(*Error); ok && e.Type == "AttributeError" {
		// TTP catches an AttributeError raised inside the method too.
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	return r, true, nil
}

type pyType struct {
	name    string
	methods table
}

var (
	strType   = &pyType{name: "str"}
	intType   = &pyType{name: "int"}
	boolType  = &pyType{name: "bool"}
	floatType = &pyType{name: "float"}
	listType  = &pyType{name: "list"}
	dictType  = &pyType{name: "dict"}
)

func init() {
	strType.methods = strMethods
	intType.methods = intMethods
	boolType.methods = intMethods
	floatType.methods = floatMethods
	listType.methods = listMethods
	dictType.methods = dictMethods
	namesByType = make(map[string]string)
	for _, t := range []*pyType{strType, intType, floatType, listType, dictType} {
		for n := range t.methods {
			if namesByType[n] != "" {
				namesByType[n] += ", "
			}
			namesByType[n] += t.name
		}
	}
}

// typeOf returns value's Python type; nil for a value of no ported type.
func typeOf(value interface{}) *pyType {
	switch value.(type) {
	case string:
		return strType
	case bool:
		return boolType
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return intType
	case float64, float32:
		return floatType
	case []interface{}:
		return listType
	case map[string]interface{}:
		return dictType
	}
	return nil
}

// selfOf returns value in its type's Go form for the methods: an int or
// bool as a *big.Int, a float32 as a float64, anything else as it is.
func selfOf(value interface{}) interface{} {
	switch v := value.(type) {
	case float32:
		return float64(v)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		n, _ := toInt(v)
		return n
	}
	return value
}

// namesByType maps each ported name to the types that have it, e.g.
// "str, list" for index.
var namesByType map[string]string

// IsName reports whether name is a ported method or attribute of any type.
func IsName(name string) bool {
	return namesByType[name] != ""
}

// Types returns the types that have the ported method or attribute name,
// e.g. "str" or "str, list"; "" if none has.
func Types(name string) string {
	return namesByType[name]
}

// notPorted maps each Python 3.12 method of str, int, float, list and dict
// that isn't ported to why.
var notPorted = map[string]string{
	// list and dict methods that change the object in place return None,
	// which TTP saves as the value.
	"clear":      "list and dict method that changes the value in place",
	"extend":     "list method that changes the value in place",
	"insert":     "list method that changes the value in place",
	"pop":        "list and dict method that changes the value in place",
	"remove":     "list method that changes the value in place",
	"reverse":    "list method that changes the value in place",
	"sort":       "list method that changes the value in place",
	"popitem":    "dict method that changes the value in place",
	"setdefault": "dict method that changes the value in place",
	"update":     "dict method that changes the value in place",
	// results with no JSON form
	"encode":   "str method that returns bytes, which have no JSON form",
	"to_bytes": "int method that returns bytes, which have no JSON form",
	// class methods that build a value from an argument
	"from_bytes": "int class method that reads bytes, which a template can't give",
	"fromhex":    "float class method",
	"fromkeys":   "dict class method",
}

// mutating is true for the notPorted names that change their object in
// place.
func mutating(name string) bool {
	switch name {
	case "clear", "extend", "insert", "pop", "remove", "reverse", "sort", "popitem", "setdefault", "update":
		return true
	}
	return false
}

// NotPorted reports whether name is a Python 3.12 method of str, int,
// float, list or dict that gottp doesn't port, and returns the compile
// warning for it. scope names the match variable.
func NotPorted(scope, name string) (warning string, ok bool) {
	why, ok := notPorted[name]
	if !ok {
		return "", false
	}
	if mutating(name) {
		return fmt.Sprintf("%s: match variable function %q is a Python %s; gottp doesn't port it and keeps the value unchanged (Python TTP sets the value to the method's result, None)", scope, name, why), true
	}
	return fmt.Sprintf("%s: match variable function %q is a Python %s; gottp doesn't port it and keeps the value unchanged", scope, name, why), true
}

// NotPortedNames returns the names NotPorted reports, sorted.
func NotPortedNames() []string {
	out := make([]string, 0, len(notPorted))
	for n := range notPorted {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Names returns the ported names of type typ ("str", "int", "float",
// "list" or "dict"), sorted.
func Names(typ string) []string {
	var t *pyType
	switch typ {
	case "str":
		t = strType
	case "int", "bool":
		t = intType
	case "float":
		t = floatType
	case "list":
		t = listType
	case "dict":
		t = dictType
	default:
		return nil
	}
	out := make([]string, 0, len(t.methods))
	for n := range t.methods {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
