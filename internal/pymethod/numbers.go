package pymethod

import (
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
)

// intMethods are int's (and bool's) ported methods and attributes. self is
// a *big.Int.
var intMethods = table{
	"bit_length": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		return new(big.Int).Abs(self.(*big.Int)).BitLen(), nil
	}},
	"bit_count": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		n := 0
		for _, w := range new(big.Int).Abs(self.(*big.Int)).Bits() {
			n += bits.OnesCount(uint(w))
		}
		return n, nil
	}},
	"conjugate": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		return fromBig(self.(*big.Int)), nil
	}},
	"as_integer_ratio": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		return []interface{}{fromBig(self.(*big.Int)), 1}, nil
	}},
	"is_integer": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		return true, nil
	}},
	"real":        {attr: true, fn: func(self interface{}, _ *args) (interface{}, error) { return fromBig(self.(*big.Int)), nil }},
	"numerator":   {attr: true, fn: func(self interface{}, _ *args) (interface{}, error) { return fromBig(self.(*big.Int)), nil }},
	"imag":        {attr: true, fn: func(interface{}, *args) (interface{}, error) { return 0, nil }},
	"denominator": {attr: true, fn: func(interface{}, *args) (interface{}, error) { return 1, nil }},
}

// floatMethods are float's ported methods and attributes.
var floatMethods = table{
	"is_integer": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		f := self.(float64)
		return !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f), nil
	}},
	"conjugate": {fn: func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		return self.(float64), nil
	}},
	"as_integer_ratio": {fn: floatRatio},
	"hex":              {fn: floatHex},
	"real":             {attr: true, fn: func(self interface{}, _ *args) (interface{}, error) { return self.(float64), nil }},
	"imag":             {attr: true, fn: func(interface{}, *args) (interface{}, error) { return 0.0, nil }},
}

func floatRatio(self interface{}, a *args) (interface{}, error) {
	if err := a.none(); err != nil {
		return nil, err
	}
	f := self.(float64)
	switch {
	case math.IsInf(f, 0):
		return nil, raise("OverflowError", "cannot convert Infinity to integer ratio")
	case math.IsNaN(f):
		return nil, raise("ValueError", "cannot convert NaN to integer ratio")
	}
	r := new(big.Rat).SetFloat64(f) // exact, in lowest terms
	return []interface{}{fromBig(r.Num()), fromBig(r.Denom())}, nil
}

// floatHex is float.hex: "[-]0x1.<13 hex digits>p<exp>", a subnormal as
// "0x0.<...>p-1022", zero as "0x0.0p+0".
func floatHex(self interface{}, a *args) (interface{}, error) {
	if err := a.none(); err != nil {
		return nil, err
	}
	f := self.(float64)
	switch {
	case math.IsInf(f, 1):
		return "inf", nil
	case math.IsInf(f, -1):
		return "-inf", nil
	case math.IsNaN(f):
		return "nan", nil
	}
	sign := ""
	if math.Signbit(f) {
		sign = "-"
		f = -f
	}
	if f == 0 {
		return sign + "0x0.0p+0", nil
	}
	b := math.Float64bits(f)
	exp := int(b>>52) & 0x7ff
	mant := b & (1<<52 - 1)
	lead := "1"
	if exp == 0 {
		lead = "0"
		exp = -1022
	} else {
		exp -= 1023
	}
	m := strconv.FormatUint(mant, 16)
	m = strings.Repeat("0", 13-len(m)) + m
	es := "+"
	if exp < 0 {
		es = "-"
		exp = -exp
	}
	return sign + "0x" + lead + "." + m + "p" + es + strconv.Itoa(exp), nil
}
