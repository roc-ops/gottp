package pymethod

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/roc-ops/gottp/internal/call"
)

// str.format and format_map: Python's format string syntax and format
// specification mini-language (PEP 3101) for the values a call's arguments
// can hold: str, int, float, bool, None, list and dict.

func strFormat(self interface{}, a *args) (interface{}, error) {
	kw := make(map[string]interface{}, len(a.kw))
	for _, k := range a.kw {
		kw[k.Key] = k.Value
	}
	f := &formatter{pos: a.pos, kw: kw}
	return f.format(self.(string), 2)
}

func strFormatMap(self interface{}, a *args) (interface{}, error) {
	if err := a.positional(1, 1); err != nil {
		return nil, err
	}
	f := &formatter{mapMode: true}
	if m, ok := a.pos[0].(map[string]interface{}); ok {
		f.kw = m
	} else {
		// Any argument is taken; one that isn't a mapping fails on the
		// first named field.
		f.mapArg = a.pos[0]
	}
	return f.format(self.(string), 2)
}

type formatter struct {
	pos     []interface{}
	kw      map[string]interface{}
	mapMode bool        // format_map: every name is a key of kw
	mapArg  interface{} // format_map's argument when it isn't a dict
	auto    int         // next automatic field number
	manual  bool        // a numbered field was used
	autoUse bool        // an automatic field was used
}

// format renders s; depth is the nesting allowed for fields inside a
// format spec (Python allows one level).
func (f *formatter) format(s string, depth int) (string, error) {
	if depth == 0 {
		return "", raise("ValueError", "Max string recursion exceeded")
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '{' && i+1 < len(s) && s[i+1] == '{':
			b.WriteByte('{')
			i += 2
		case c == '}' && i+1 < len(s) && s[i+1] == '}':
			b.WriteByte('}')
			i += 2
		case c == '}':
			return "", raise("ValueError", "Single '}' encountered in format string")
		case c == '{':
			end, err := fieldEnd(s, i+1)
			if err != nil {
				return "", err
			}
			out, err := f.field(s[i+1:end], depth)
			if err != nil {
				return "", err
			}
			b.WriteString(out)
			i = end + 1
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String(), nil
}

// fieldEnd returns the index of the "}" closing the field that starts at
// i, counting nested braces.
func fieldEnd(s string, i int) (int, error) {
	level := 1
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '[':
			// A key in brackets may hold braces; it ends at "]".
			for j < len(s) && s[j] != ']' {
				j++
			}
			if j >= len(s) {
				return 0, raise("ValueError", "expected '}' before end of string")
			}
		case '{':
			level++
		case '}':
			level--
			if level == 0 {
				return j, nil
			}
		}
	}
	if level > 1 || i < len(s) {
		return 0, raise("ValueError", "expected '}' before end of string")
	}
	return 0, raise("ValueError", "Single '{' encountered in format string")
}

func (f *formatter) field(text string, depth int) (string, error) {
	// field_name ["!" conversion] [":" format_spec]; the name ends at the
	// first "!" or ":" outside brackets.
	name, conv, spec := text, "", ""
	hasConv := false
	for j := 0; j < len(text); j++ {
		if text[j] == '[' {
			for j < len(text) && text[j] != ']' {
				j++
			}
			continue
		}
		if text[j] == '!' || text[j] == ':' {
			name = text[:j]
			rest := text[j:]
			if rest[0] == '!' {
				hasConv = true
				if len(rest) < 2 {
					return "", raise("ValueError", "end of string while looking for conversion specifier")
				}
				conv = rest[1:2]
				rest = rest[2:]
				if rest != "" && rest[0] != ':' {
					return "", raise("ValueError", "expected ':' after conversion specifier")
				}
			}
			if rest != "" {
				spec = rest[1:]
			}
			break
		}
	}
	v, err := f.lookup(name)
	if err != nil {
		return "", err
	}
	if hasConv {
		switch conv {
		case "s":
			v = pyStr(v)
		case "r":
			v = pyRepr(v)
		case "a":
			v = pyASCII(pyRepr(v))
		default:
			return "", raise("ValueError", "Unknown conversion specifier %s", conv)
		}
	}
	if strings.ContainsAny(spec, "{}") {
		if spec, err = f.format(spec, depth-1); err != nil {
			return "", err
		}
	}
	return formatValue(v, spec)
}

// lookup resolves a field name: an argument (automatic, numbered or named)
// followed by ".attr" and "[key]" parts.
func (f *formatter) lookup(name string) (interface{}, error) {
	j := strings.IndexAny(name, ".[")
	first, rest := name, ""
	if j >= 0 {
		first, rest = name[:j], name[j:]
	}
	var v interface{}
	switch {
	case first == "":
		if f.mapMode {
			return nil, raise("ValueError", "Format string contains positional fields")
		}
		if f.manual {
			return nil, raise("ValueError", "cannot switch from manual field specification to automatic field numbering")
		}
		f.autoUse = true
		n := f.auto
		f.auto++
		if n >= len(f.pos) {
			return nil, raise("IndexError", "Replacement index %d out of range for positional args tuple", n)
		}
		v = f.pos[n]
	case isDigits(first):
		if f.mapMode {
			return nil, raise("ValueError", "Format string contains positional fields")
		}
		if f.autoUse {
			return nil, raise("ValueError", "cannot switch from automatic field numbering to manual field specification")
		}
		f.manual = true
		n, err := strconv.Atoi(first)
		if err != nil || n >= len(f.pos) {
			return nil, raise("IndexError", "Replacement index %s out of range for positional args tuple", first)
		}
		v = f.pos[n]
	default:
		if f.mapArg != nil {
			switch f.mapArg.(type) {
			case []interface{}:
				return nil, raise("TypeError", "list indices must be integers or slices, not str")
			case string:
				return nil, raise("TypeError", "string indices must be integers, not 'str'")
			}
			return nil, raise("TypeError", "'%s' object is not subscriptable", typeName(f.mapArg))
		}
		x, ok := f.kw[first]
		if !ok {
			return nil, raise("KeyError", "%s", pyRepr(first))
		}
		v = x
	}
	for rest != "" {
		if rest[0] == '.' {
			k := strings.IndexAny(rest[1:], ".[")
			attr := rest[1:]
			if k >= 0 {
				attr, rest = rest[1:1+k], rest[1+k:]
			} else {
				rest = ""
			}
			if attr == "" {
				return nil, raise("ValueError", "Empty attribute in format string")
			}
			t, self := typeOf(v)
			if t == nil || !t.methods[attr].attr {
				return nil, attributeError(v, attr)
			}
			r, err := t.methods[attr].fn(self, &args{typ: t.name, name: attr})
			if err != nil {
				return nil, err
			}
			v = r
			continue
		}
		// "[key]"
		k := strings.IndexByte(rest, ']')
		if k < 0 {
			return nil, raise("ValueError", "Missing ']' in format string")
		}
		key := rest[1:k]
		rest = rest[k+1:]
		if key == "" {
			return nil, raise("ValueError", "Empty attribute in format string")
		}
		if rest != "" && rest[0] != '.' && rest[0] != '[' {
			return nil, raise("ValueError", "Only '.' or '[' may follow ']' in format field specifier")
		}
		var err error
		if v, err = subscript(v, key); err != nil {
			return nil, err
		}
	}
	return v, nil
}

func subscript(v interface{}, key string) (interface{}, error) {
	switch x := v.(type) {
	case []interface{}:
		if !isDigits(key) {
			return nil, raise("TypeError", "list indices must be integers or slices, not str")
		}
		n, err := strconv.Atoi(key)
		if err != nil || n >= len(x) {
			return nil, raise("IndexError", "list index out of range")
		}
		return x[n], nil
	case map[string]interface{}:
		if r, ok := x[key]; ok {
			return r, nil
		}
		if isDigits(key) {
			return nil, raise("KeyError", "%s", key)
		}
		return nil, raise("KeyError", "%s", pyRepr(key))
	case string:
		if !isDigits(key) {
			return nil, raise("TypeError", "string indices must be integers, not 'str'")
		}
		rs := []rune(x)
		n, err := strconv.Atoi(key)
		if err != nil || n >= len(rs) {
			return nil, raise("IndexError", "string index out of range")
		}
		return string(rs[n]), nil
	}
	return nil, raise("TypeError", "'%s' object is not subscriptable", typeName(v))
}

// attributeError is the AttributeError TTP catches: it keeps the value.
func attributeError(v interface{}, attr string) error {
	return &Error{Type: "AttributeError", Msg: fmt.Sprintf("'%s' object has no attribute '%s'", typeName(v), attr)}
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// pyStr is Python's str() of a value.
func pyStr(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float32:
		return call.Str(float64(x))
	case *big.Int:
		return x.String()
	}
	if n, ok := toInt(v); ok {
		if _, isBool := v.(bool); !isBool {
			return n.String()
		}
	}
	switch x := v.(type) {
	case []interface{}, map[string]interface{}:
		return pyRepr(x)
	}
	return call.Str(v)
}

// pyRepr is Python's repr() of a value. A dict's keys are sorted (Go maps
// don't keep their order).
func pyRepr(v interface{}) string {
	switch x := v.(type) {
	case string:
		return call.Repr(x)
	case []interface{}:
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = pyRepr(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		keys := sortedKeys(x)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = call.Repr(k) + ": " + pyRepr(x[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return pyStr(v)
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// pyASCII escapes the non-ASCII characters of a repr, as ascii() does.
func pyASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
		}
	}
	return b.String()
}

// spec is a parsed format specification:
// [[fill]align][sign]["z"]["#"]["0"][width][grouping]["." precision][type]
type spec struct {
	fill      rune
	align     byte // 0, '<', '>', '^' or '='
	sign      byte // 0, '+', '-' or ' '
	z, alt    bool
	zero      bool
	width     int
	grouping  byte // 0, ',' or '_'
	precision int  // -1 if absent
	typ       byte // 0 if absent
}

func parseSpec(s, typeName string) (*spec, error) {
	sp := &spec{fill: ' ', precision: -1}
	rs := []rune(s)
	i := 0
	isAlign := func(r rune) bool { return r == '<' || r == '>' || r == '^' || r == '=' }
	if len(rs) >= 2 && isAlign(rs[1]) {
		sp.fill, sp.align = rs[0], byte(rs[1])
		i = 2
	} else if len(rs) >= 1 && isAlign(rs[0]) {
		sp.align = byte(rs[0])
		i = 1
	}
	if i < len(rs) && (rs[i] == '+' || rs[i] == '-' || rs[i] == ' ') {
		sp.sign = byte(rs[i])
		i++
	}
	if i < len(rs) && rs[i] == 'z' {
		sp.z = true
		i++
	}
	if i < len(rs) && rs[i] == '#' {
		sp.alt = true
		i++
	}
	if i < len(rs) && rs[i] == '0' {
		sp.zero = true
		i++
	}
	start := i
	for i < len(rs) && rs[i] >= '0' && rs[i] <= '9' {
		i++
	}
	if i > start {
		w, err := strconv.Atoi(string(rs[start:i]))
		if err != nil {
			return nil, raise("ValueError", "Too many decimal digits in format string")
		}
		sp.width = w
	}
	if i < len(rs) && (rs[i] == ',' || rs[i] == '_') {
		sp.grouping = byte(rs[i])
		i++
		if i < len(rs) && (rs[i] == ',' || rs[i] == '_') {
			return nil, raise("ValueError", "Cannot specify both ',' and '_'.")
		}
	}
	if i < len(rs) && rs[i] == '.' {
		i++
		start = i
		for i < len(rs) && rs[i] >= '0' && rs[i] <= '9' {
			i++
		}
		if i == start {
			return nil, raise("ValueError", "Format specifier missing precision")
		}
		p, err := strconv.Atoi(string(rs[start:i]))
		if err != nil {
			return nil, raise("ValueError", "Too many decimal digits in format string")
		}
		sp.precision = p
	}
	if len(rs)-i > 1 {
		return nil, raise("ValueError", "Invalid format specifier '%s' for object of type '%s'", s, typeName)
	}
	if i < len(rs) {
		if rs[i] >= 0x80 {
			return nil, raise("ValueError", "Unknown format code '\\x%x' for object of type '%s'", rs[i], typeName)
		}
		sp.typ = byte(rs[i])
	}
	if sp.zero && sp.align == 0 {
		sp.fill, sp.align = '0', '='
	}
	return sp, nil
}

func formatValue(v interface{}, specText string) (string, error) {
	switch x := v.(type) {
	case string:
		return formatStr(x, specText)
	case bool:
		if specText == "" {
			return pyStr(x), nil
		}
		n, _ := toInt(x)
		return formatInt(n, specText)
	case float64:
		return formatFloat(x, specText)
	case float32:
		return formatFloat(float64(x), specText)
	case nil:
		if specText == "" {
			return "None", nil
		}
		return "", raise("TypeError", "unsupported format string passed to NoneType.__format__")
	case []interface{}, map[string]interface{}:
		if specText == "" {
			return pyStr(x), nil
		}
		return "", raise("TypeError", "unsupported format string passed to %s.__format__", typeName(x))
	}
	if n, ok := toInt(v); ok {
		return formatInt(n, specText)
	}
	return pyStr(v), nil
}

// pad aligns body (with prefix, the sign and base prefix, before it) in
// sp.width; def is the type's default alignment.
func pad(sp *spec, prefix, body string, def byte) string {
	n := utf8.RuneCountInString(prefix) + utf8.RuneCountInString(body)
	align := sp.align
	if align == 0 {
		align = def
	}
	if sp.width <= n {
		return prefix + body
	}
	fill := strings.Repeat(string(sp.fill), sp.width-n)
	switch align {
	case '<':
		return prefix + body + fill
	case '^':
		left := (sp.width - n) / 2
		return strings.Repeat(string(sp.fill), left) + prefix + body + strings.Repeat(string(sp.fill), sp.width-n-left)
	case '=':
		return prefix + fill + body
	}
	return fill + prefix + body
}

func formatStr(s, specText string) (string, error) {
	if specText == "" {
		return s, nil
	}
	sp, err := parseSpec(specText, "str")
	if err != nil {
		return "", err
	}
	if sp.typ != 0 && sp.typ != 's' {
		return "", raise("ValueError", "Unknown format code '%c' for object of type 'str'", sp.typ)
	}
	switch {
	case sp.sign != 0:
		return "", raise("ValueError", "Sign not allowed in string format specifier")
	case sp.alt:
		return "", raise("ValueError", "Alternate form (#) not allowed in string format specifier")
	case sp.z:
		return "", raise("ValueError", "Negative zero coercion (z) not allowed in format specifier")
	case sp.align == '=':
		return "", raise("ValueError", "'=' alignment not allowed in string format specifier")
	case sp.grouping != 0:
		return "", raise("ValueError", "Cannot specify '%c' with 's'.", sp.grouping)
	}
	if sp.precision >= 0 {
		rs := []rune(s)
		if len(rs) > sp.precision {
			s = string(rs[:sp.precision])
		}
	}
	return pad(sp, "", s, '<'), nil
}

// group inserts sep every n digits of digits from the right, padding with
// zeros to make the grouped result at least minWidth long.
func group(digits string, sep byte, n, minWidth int) string {
	for {
		d := len(digits)
		if d+(d-1)/n >= minWidth {
			break
		}
		digits = "0" + digits
	}
	var b strings.Builder
	first := len(digits) % n
	if first == 0 {
		first = n
	}
	b.WriteString(digits[:first])
	for i := first; i < len(digits); i += n {
		b.WriteByte(sep)
		b.WriteString(digits[i : i+n])
	}
	return b.String()
}

func signPrefix(sp *spec, neg bool) string {
	switch {
	case neg:
		return "-"
	case sp.sign == '+':
		return "+"
	case sp.sign == ' ':
		return " "
	}
	return ""
}

func formatInt(n *big.Int, specText string) (string, error) {
	if specText == "" {
		return n.String(), nil
	}
	sp, err := parseSpec(specText, "int")
	if err != nil {
		return "", err
	}
	switch sp.typ {
	case 'e', 'E', 'f', 'F', 'g', 'G', '%':
		f, _ := new(big.Float).SetInt(n).Float64()
		if math.IsInf(f, 0) {
			return "", raise("OverflowError", "int too large to convert to float")
		}
		return formatFloatSpec(f, sp, "int")
	case 0, 'd', 'n', 'b', 'o', 'x', 'X', 'c':
	default:
		return "", raise("ValueError", "Unknown format code '%c' for object of type 'int'", sp.typ)
	}
	if sp.precision >= 0 {
		return "", raise("ValueError", "Precision not allowed in integer format specifier")
	}
	if sp.z {
		return "", raise("ValueError", "Negative zero coercion (z) not allowed in integer format specifier")
	}
	if sp.typ == 'c' {
		if sp.sign != 0 {
			return "", raise("ValueError", "Sign not allowed with integer format specifier 'c'")
		}
		if sp.alt {
			return "", raise("ValueError", "Alternate form (#) not allowed with integer format specifier 'c'")
		}
		if !n.IsInt64() || n.Int64() < 0 || n.Int64() > 0x10ffff {
			return "", raise("OverflowError", "%%c arg not in range(0x110000)")
		}
		return pad(sp, "", string(rune(n.Int64())), '<'), nil
	}
	if sp.grouping == ',' && sp.typ != 0 && sp.typ != 'd' {
		return "", raise("ValueError", "Cannot specify ',' with '%c'.", sp.typ)
	}
	if sp.typ == 'n' && sp.grouping != 0 {
		return "", raise("ValueError", "Cannot specify '%c' with 'n'.", sp.grouping)
	}
	neg := n.Sign() < 0
	abs := new(big.Int).Abs(n)
	base, prefix, every := 10, "", 3
	switch sp.typ {
	case 'b':
		base, prefix, every = 2, "0b", 4
	case 'o':
		base, prefix, every = 8, "0o", 4
	case 'x':
		base, prefix, every = 16, "0x", 4
	case 'X':
		base, prefix, every = 16, "0X", 4
	}
	digits := abs.Text(base)
	if sp.typ == 'X' {
		digits = strings.ToUpper(digits)
	}
	if !sp.alt {
		prefix = ""
	}
	pre := signPrefix(sp, neg) + prefix
	if sp.grouping != 0 {
		min := 0
		if sp.align == '=' && sp.fill == '0' {
			min = sp.width - len(pre)
		}
		digits = group(digits, sp.grouping, every, min)
		if min > 0 {
			return pre + digits, nil
		}
	}
	return pad(sp, pre, digits, '>'), nil
}

func formatFloat(f float64, specText string) (string, error) {
	if specText == "" {
		return call.Str(f), nil
	}
	sp, err := parseSpec(specText, "float")
	if err != nil {
		return "", err
	}
	return formatFloatSpec(f, sp, "float")
}

func formatFloatSpec(f float64, sp *spec, typeName string) (string, error) {
	switch sp.typ {
	case 0, 'e', 'E', 'f', 'F', 'g', 'G', 'n', '%':
	default:
		return "", raise("ValueError", "Unknown format code '%c' for object of type '%s'", sp.typ, typeName)
	}
	if sp.grouping != 0 && sp.typ == 'n' {
		return "", raise("ValueError", "Cannot specify '%c' with 'n'.", sp.grouping)
	}
	neg := math.Signbit(f) && !math.IsNaN(f)
	if neg && sp.z {
		// "z" coerces a negative zero after rounding to zero.
		r, _ := floatBody(math.Abs(f), sp)
		if strings.Trim(r, "0.%eE+-") == "" {
			neg = false
		}
	}
	body, upper := floatBody(math.Abs(f), sp)
	if upper {
		body = strings.ToUpper(body)
	}
	pre := signPrefix(sp, neg)
	if sp.grouping != 0 && !math.IsInf(f, 0) && !math.IsNaN(f) {
		intPart, rest := body, ""
		if k := strings.IndexAny(body, ".eE%"); k >= 0 {
			intPart, rest = body[:k], body[k:]
		}
		min := 0
		if sp.align == '=' && sp.fill == '0' {
			min = sp.width - len(pre) - len(rest)
		}
		body = group(intPart, sp.grouping, 3, min) + rest
		if min > 0 {
			return pre + body, nil
		}
	}
	return pad(sp, pre, body, '>'), nil
}

// floatBody formats f >= 0 (or nan) without sign; upper reports an upper
// case type (E, F, G).
func floatBody(f float64, sp *spec) (string, bool) {
	p := sp.precision
	switch {
	case math.IsInf(f, 0):
		return "inf" + pctSuffix(sp), sp.typ == 'E' || sp.typ == 'F' || sp.typ == 'G'
	case math.IsNaN(f):
		return "nan" + pctSuffix(sp), sp.typ == 'E' || sp.typ == 'F' || sp.typ == 'G'
	}
	switch sp.typ {
	case 'e', 'E':
		if p < 0 {
			p = 6
		}
		s := strconv.FormatFloat(f, 'e', p, 64) // e+05: Python's form too
		if sp.alt && p == 0 {
			s = strings.Replace(s, "e", ".e", 1)
		}
		return s, sp.typ == 'E'
	case 'f', 'F', '%':
		if p < 0 {
			p = 6
		}
		if sp.typ == '%' {
			f *= 100
		}
		s := strconv.FormatFloat(f, 'f', p, 64)
		if sp.alt && p == 0 {
			s += "."
		}
		return s + pctSuffix(sp), sp.typ == 'F'
	case 'g', 'G', 'n':
		if p < 0 {
			p = 6
		}
		return generalForm(f, p, sp.alt, false), sp.typ == 'G'
	}
	// No type: repr without a precision; with one, like 'g' but always
	// with a digit after the point.
	if p < 0 {
		return call.Str(f), false
	}
	return generalForm(f, p, sp.alt, true), false
}

func pctSuffix(sp *spec) string {
	if sp.typ == '%' {
		return "%"
	}
	return ""
}

// generalForm is the 'g' presentation with precision p (0 is 1): fixed
// notation when -4 <= exp < p, else scientific, without trailing zeros
// unless alt. noType is the presentation with a precision and no type: a
// fixed result keeps a digit after the point, and scientific is used from
// exp >= p-1.
func generalForm(f float64, p int, alt, noType bool) string {
	if p == 0 {
		p = 1
	}
	e := strconv.FormatFloat(f, 'e', p-1, 64)
	mant, expStr, _ := strings.Cut(e, "e")
	exp, _ := strconv.Atoi(expStr)
	sciLimit := p
	if noType {
		sciLimit = p - 1
	}
	var s string
	if exp < -4 || exp >= sciLimit {
		if !alt {
			mant = trimZeros(mant)
		} else if !strings.Contains(mant, ".") {
			mant += "."
		}
		sign := "+"
		if exp < 0 {
			sign, exp = "-", -exp
		}
		s = fmt.Sprintf("%se%s%02d", mant, sign, exp)
	} else {
		s = strconv.FormatFloat(f, 'f', p-1-exp, 64)
		if !alt {
			s = trimZeros(s)
		} else if !strings.Contains(s, ".") {
			s += "."
		}
		if noType && !strings.Contains(s, ".") {
			s += ".0"
		}
	}
	return s
}

func trimZeros(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}
