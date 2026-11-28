package pymethod

import "github.com/roc-ops/gottp/internal/call"

// listMethods are list's methods that don't change the list.
var listMethods = table{
	"count": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.positional(1, 1); err != nil {
			return nil, err
		}
		n := 0
		for _, item := range self.([]interface{}) {
			if call.Equal(item, a.pos[0]) {
				n++
			}
		}
		return n, nil
	}},
	"index": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.positional(1, 3); err != nil {
			return nil, err
		}
		l := self.([]interface{})
		start, err := optIndex(a.pos, 1, 0)
		if err != nil {
			return nil, err
		}
		end, err := optIndex(a.pos, 2, len(l))
		if err != nil {
			return nil, err
		}
		start, end = adjustIndices(start, end, len(l))
		for i := start; i < end; i++ {
			if call.Equal(l[i], a.pos[0]) {
				return i, nil
			}
		}
		return nil, raise("ValueError", "%s is not in list", pyRepr(a.pos[0]))
	}},
	"copy": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		return append([]interface{}{}, self.([]interface{})...), nil
	}},
}

// dictMethods are dict's methods that don't change the dict. A dict's keys
// are strings, so a key of another type is never found. keys, values and
// items give lists, in key order (a Go map keeps no order); see the docs.
var dictMethods = table{
	"get": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.positional(1, 2); err != nil {
			return nil, err
		}
		var def interface{}
		if len(a.pos) > 1 {
			def = a.pos[1]
		}
		key, ok := a.pos[0].(string)
		if !ok {
			if _, isList := a.pos[0].([]interface{}); isList {
				return nil, raise("TypeError", "unhashable type: 'list'")
			}
			if _, isDict := a.pos[0].(map[string]interface{}); isDict {
				return nil, raise("TypeError", "unhashable type: 'dict'")
			}
			return def, nil
		}
		if v, ok := self.(map[string]interface{})[key]; ok {
			return v, nil
		}
		return def, nil
	}},
	"keys": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		m := self.(map[string]interface{})
		out := make([]interface{}, 0, len(m))
		for _, k := range sortedKeys(m) {
			out = append(out, k)
		}
		return out, nil
	}},
	"values": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		m := self.(map[string]interface{})
		out := make([]interface{}, 0, len(m))
		for _, k := range sortedKeys(m) {
			out = append(out, m[k])
		}
		return out, nil
	}},
	"items": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		m := self.(map[string]interface{})
		out := make([]interface{}, 0, len(m))
		for _, k := range sortedKeys(m) {
			out = append(out, []interface{}{k, m[k]})
		}
		return out, nil
	}},
	"copy": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		m := self.(map[string]interface{})
		out := make(map[string]interface{}, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out, nil
	}},
}
