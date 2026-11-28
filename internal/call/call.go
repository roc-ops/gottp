// Package call reads function calls the way Python TTP 0.10.1's
// get_attributes (ttp/utils/get_attributes.py) does, for every scope that
// takes them: match variables, group attributes and functions=/chain=,
// input functions and output functions.
//
// A chain such as `contains("vlan") | upper | split(".")` is split on every
// "|", quotes or not, as TTP splits it (a "|" inside quotes breaks the call
// in two; see QuotedPipes). Each call is read with TTP's regular expression
// for a call, `^(?P<name>\S+?)\s?(\((?P<options>.*)\))?$`, and its options
// are evaluated as Python call arguments: positional values, then keyword
// arguments, each a Python literal. A bare name is its own text, and true,
// false and none in any case are True, False and None (TTP's _UndefSubst).
//
// Calls are read once, when a template compiles; the runtime runs the
// parsed calls.
package call

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/roc-ops/gottp/internal/variable"
)

// Call is one function call of a chain.
type Call struct {
	// Name is the function's name.
	Name string
	// Text is the call as written, trimmed.
	Text string
	// Args are the positional arguments as Python values: string, int,
	// float64, bool, nil (None), []interface{} (a list or tuple) or
	// map[string]interface{} (a dict).
	Args []interface{}
	// Kwargs are the keyword arguments in the order written.
	Kwargs []Kwarg

	// Strs are Args as strings (Python's str()), the form gottp's
	// functions take their arguments in.
	Strs []string
	// KwargStrs maps each keyword argument to its value as a string; nil
	// when the call has none.
	KwargStrs map[string]interface{}
	// strArg[i] is true when Args[i] is a string, which a template
	// variable of that name replaces (ResolvedStrs).
	strArg []bool
}

// Kwarg is one keyword argument.
type Kwarg struct {
	Key   string
	Value interface{}
}

// Kwarg returns the value of keyword argument key.
func (c *Call) Kwarg(key string) (interface{}, bool) {
	for _, kw := range c.Kwargs {
		if kw.Key == key {
			return kw.Value, true
		}
	}
	return nil, false
}

// ResolvedStrs returns Strs with every string argument that names one of
// vars replaced by that variable's value as a string, as gottp has always
// resolved function arguments. It returns Strs itself when nothing is
// replaced.
func (c *Call) ResolvedStrs(vars map[string]interface{}) []string {
	if len(vars) == 0 {
		return c.Strs
	}
	var out []string
	for i, s := range c.Strs {
		if !c.strArg[i] {
			continue
		}
		v, ok := vars[s]
		if !ok {
			continue
		}
		if out == nil {
			out = append([]string(nil), c.Strs...)
		}
		out[i] = fmt.Sprintf("%v", v)
	}
	if out == nil {
		return c.Strs
	}
	return out
}

// ResolvedArgs is ResolvedStrs followed by each keyword argument as
// "key=value", the form input, output and group functions take keyword
// arguments in.
func (c *Call) ResolvedArgs(vars map[string]interface{}) []string {
	args := c.ResolvedStrs(vars)
	if len(c.Kwargs) == 0 {
		return args
	}
	out := make([]string, 0, len(args)+len(c.Kwargs))
	out = append(out, args...)
	for _, kw := range c.Kwargs {
		out = append(out, kw.Key+"="+Str(kw.Value))
	}
	return out
}

// KwargValues maps each keyword argument to its Python value; nil when the
// call has none.
func (c *Call) KwargValues() map[string]interface{} {
	if len(c.Kwargs) == 0 {
		return nil
	}
	m := make(map[string]interface{}, len(c.Kwargs))
	for _, kw := range c.Kwargs {
		m[kw.Key] = kw.Value
	}
	return m
}

// Split splits a chain on every "|", as TTP's get_attributes does
// (`line.split("|")`, quotes or not), and returns the trimmed, non-empty
// parts.
func Split(chain string) []string {
	var parts []string
	for _, part := range strings.Split(chain, "|") {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

// ParseChain splits chain (Split) and parses each call.
func ParseChain(chain string) ([]*Call, error) {
	parts := Split(chain)
	calls := make([]*Call, 0, len(parts))
	for _, part := range parts {
		c, err := Parse(part)
		if err != nil {
			return nil, err
		}
		calls = append(calls, c)
	}
	return calls, nil
}

// Name returns the name TTP reads from one call, without evaluating its
// arguments; "" if text is not a call.
func Name(text string) string {
	name, _, _, ok := splitCall(strings.TrimSpace(text))
	if !ok {
		return ""
	}
	return name
}

// Parse parses one call (a part of a chain, see Split). It fails where
// Python TTP fails to read the call: text that doesn't fit TTP's call
// syntax, and arguments that aren't valid Python call arguments (an
// unterminated quote, unbalanced brackets, an expression that isn't a
// literal).
func Parse(text string) (*Call, error) {
	text = strings.TrimSpace(text)
	name, options, hasOptions, ok := splitCall(text)
	if !ok {
		return nil, fmt.Errorf("cannot read function call %q", text)
	}
	c := &Call{Name: name, Text: text}
	if hasOptions && options != "" {
		args, kwargs, err := parseArgs(options)
		if err != nil {
			return nil, fmt.Errorf("cannot read the arguments of %q: %v", text, err)
		}
		c.Args, c.Kwargs = args, kwargs
	}
	if len(c.Args) > 0 {
		c.Strs = make([]string, len(c.Args))
		c.strArg = make([]bool, len(c.Args))
		for i, a := range c.Args {
			c.Strs[i] = Str(a)
			_, c.strArg[i] = a.(string)
		}
	}
	if len(c.Kwargs) > 0 {
		c.KwargStrs = make(map[string]interface{}, len(c.Kwargs))
		for _, kw := range c.Kwargs {
			c.KwargStrs[kw.Key] = Str(kw.Value)
		}
	}
	return c, nil
}

// splitCall matches text against TTP's call expression
// `^(?P<name>\S+?)\s?(\((?P<options>.*)\))?$` (Python re: "." doesn't
// match a newline). The name is the shortest run of non-space characters
// after which the rest is empty or, after at most one space, "(...)".
func splitCall(text string) (name, options string, hasOptions, ok bool) {
	n := len(text)
	if n == 0 {
		return "", "", false, false
	}
	// lastNL is the last newline before the final character: the options
	// may not span one.
	lastNL := -1
	if n > 1 {
		lastNL = strings.LastIndexByte(text[:n-1], '\n')
	}
	closes := text[n-1] == ')'
	for k := 1; k <= n; k++ {
		if isSpace(text[k-1]) {
			return "", "", false, false
		}
		if k == n {
			return text, "", false, true
		}
		if !closes {
			continue
		}
		for _, open := range [2]int{k + 1, k} { // \s? is greedy: one space first
			if open == k+1 && !isSpace(text[k]) {
				continue
			}
			if open < n-1 && text[open] == '(' && lastNL < open+1 {
				return text[:k], text[open+1 : n-1], true, true
			}
		}
	}
	return "", "", false, false
}

func isSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	}
	return false
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// parseArgs evaluates options as Python call arguments, as TTP does with
// eval("_get_args_kwargs(" + options + ")").
func parseArgs(s string) ([]interface{}, []Kwarg, error) {
	var args []interface{}
	var kwargs []Kwarg
	i := skipSpace(s, 0)
	if i == len(s) {
		return nil, nil, nil
	}
	// Every pass reads one argument and its comma, so the loop runs at
	// most len(s) times; the bound guards against a pass that reads nothing.
	for pass := 0; i < len(s); pass++ {
		if pass > len(s) {
			return nil, nil, fmt.Errorf("argument list doesn't advance at offset %d", i)
		}
		if s[i] == '*' {
			return nil, nil, fmt.Errorf("*args and **kwargs are not supported")
		}
		key := ""
		if isIdentStart(s[i]) {
			j := i
			for j < len(s) && isIdentChar(s[j]) {
				j++
			}
			k := skipSpace(s, j)
			if k < len(s) && s[k] == '=' && (k+1 >= len(s) || s[k+1] != '=') {
				key = s[i:j]
				i = skipSpace(s, k+1)
			}
		}
		start := i
		v, next, err := variable.PythonValueAt(s, i, true)
		if err != nil {
			return nil, nil, fmt.Errorf("%v at offset %d", err, start)
		}
		if next <= start {
			return nil, nil, fmt.Errorf("unexpected %q at offset %d", s[start], start)
		}
		i = skipSpace(s, next)
		if key != "" {
			for _, kw := range kwargs {
				if kw.Key == key {
					return nil, nil, fmt.Errorf("keyword argument repeated: %s", key)
				}
			}
			kwargs = append(kwargs, Kwarg{Key: key, Value: v})
		} else {
			if len(kwargs) > 0 {
				return nil, nil, fmt.Errorf("positional argument follows keyword argument")
			}
			args = append(args, v)
		}
		if i == len(s) {
			break
		}
		if s[i] != ',' {
			return nil, nil, fmt.Errorf("expected ',' at offset %d, found %q", i, s[i])
		}
		i = skipSpace(s, i+1)
	}
	return args, kwargs, nil
}

func skipSpace(s string, i int) int {
	for i < len(s) && isSpace(s[i]) {
		i++
	}
	return i
}

// Str is Python's str() of a value read from a call.
func Str(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case float64:
		return floatRepr(x)
	}
	return Repr(v)
}

// Repr is Python's repr() of a value read from a call. A dict's keys are
// sorted (the order they were written in isn't kept).
func Repr(v interface{}) string {
	switch x := v.(type) {
	case string:
		return stringRepr(x)
	case []interface{}:
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = Repr(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = stringRepr(k) + ": " + Repr(x[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case nil, bool, int, float64:
		return Str(v)
	}
	return fmt.Sprintf("%v", v)
}

// floatRepr is Python's repr() of a float: the shortest digits that read
// back, in fixed notation for exponents -4 to 15 and with ".0" when whole.
func floatRepr(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	case math.IsNaN(f):
		return "nan"
	}
	e := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	mant, expStr, _ := strings.Cut(e, "e")
	exp, _ := strconv.Atoi(expStr)
	if exp < -4 || exp >= 16 {
		sign := "+"
		if exp < 0 {
			sign, exp = "-", -exp
		}
		return fmt.Sprintf("%se%s%02d", mant, sign, exp)
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// stringRepr is Python's repr() of a str.
func stringRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}

// Broken is a call that a "|" inside quotes breaks into Parts when the
// chain is split (Split).
type Broken struct {
	// Call is the call as written, "|" included.
	Call string
	// Parts are the pieces Split makes of it.
	Parts []string
}

// QuotedPipes returns the calls of chain that a "|" inside quotes breaks
// (#23). Python TTP splits on that "|" too, so gottp does the same and
// warns.
func QuotedPipes(chain string) []Broken {
	if !strings.Contains(chain, "|") {
		return nil
	}
	pieces := strings.Split(chain, "|")
	// joined[i] is true when the "|" after pieces[i] is inside quotes.
	joined := make([]bool, len(pieces))
	any := false
	piece := 0
	var quote byte
	for i := 0; i < len(chain); i++ {
		c := chain[i]
		switch {
		case c == '|':
			if quote != 0 {
				joined[piece] = true
				any = true
			}
			piece++
		case quote != 0 && c == '\\':
			if i+1 < len(chain) && chain[i+1] != '|' {
				i++
			}
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '"' || c == '\''):
			quote = c
		}
	}
	if !any {
		return nil
	}
	var out []Broken
	for i := 0; i < len(pieces); i++ {
		if !joined[i] {
			continue
		}
		j := i
		for j < len(pieces)-1 && joined[j] {
			j++
		}
		b := Broken{Call: strings.TrimSpace(strings.Join(pieces[i:j+1], "|"))}
		for _, p := range pieces[i : j+1] {
			if p = strings.TrimSpace(p); p != "" {
				b.Parts = append(b.Parts, p)
			}
		}
		out = append(out, b)
		i = j
	}
	return out
}

// QuotedPipeWarning is the compile warning for a call that a "|" inside
// quotes breaks. scope names where the chain is, such as
// `match variable "direction"`.
func QuotedPipeWarning(scope string, b Broken) string {
	parts := make([]string, len(b.Parts))
	for i, p := range b.Parts {
		parts[i] = strconv.Quote(p)
	}
	return fmt.Sprintf(`%s: the "|" inside quotes in %s splits the function chain, as in Python TTP, so %s are read as separate functions; to pass an argument containing "|", define it in <vars> and pass the variable's name`,
		scope, b.Call, strings.Join(parts, " and "))
}

// UnknownWarning is the compile warning for a function name that isn't
// known in its scope (`match variable "v"`, `group "g"`, ...), with TTP's
// suggestions from candidates (Guess). kind names the function kind, such
// as "match variable".
func UnknownWarning(scope, kind, name string, candidates []string) string {
	msg := fmt.Sprintf("%s: unknown %s function %q is skipped", scope, kind, name)
	if similar := Guess(name, candidates); len(similar) > 0 {
		msg += "; the most similar: " + strings.Join(similar, ", ")
	}
	return msg
}

// Lookup finds a function by name in gottp's one lookup order: the
// per-parse overrides (ParseOptions.Functions), then the compile-time
// overrides (CompileOptions.Functions), then the built-in functions.
func Lookup[F any](name string, perParse, compileTime map[string]F, builtin func(string) (F, bool)) (F, bool) {
	if fn, ok := perParse[name]; ok {
		return fn, true
	}
	if fn, ok := compileTime[name]; ok {
		return fn, true
	}
	return builtin(name)
}
