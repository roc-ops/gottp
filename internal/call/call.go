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

	"github.com/roc-ops/gottp/internal/pyunicode"
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
	// Notes describe the arguments that aren't Python literals, which are
	// passed as their raw text (see Parse).
	Notes []string

	// strArg[i] is true when Args[i] is a string, which a template
	// variable of that name replaces (ResolvedStrs).
	strArg []bool
	// tupleArg[i] is true when Args[i] is a tuple literal; nil when no
	// argument is one.
	tupleArg []bool
}

// IsTuple reports whether positional argument i is a tuple: a list and a
// tuple are both []interface{} in Args.
func (c *Call) IsTuple(i int) bool {
	return i >= 0 && i < len(c.tupleArg) && c.tupleArg[i]
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

// Parse parses one call (a part of a chain, see Split). It fails on text
// that doesn't fit TTP's call syntax and on broken argument syntax (an
// unterminated quote, unbalanced brackets; see parseArgs). An argument
// that is a Python expression rather than a literal, or a bare word such
// as auto-decision that Python TTP fails to evaluate, is passed as its raw
// text and described in Notes.
func Parse(text string) (*Call, error) {
	text = strings.TrimSpace(text)
	name, options, hasOptions, ok := splitCall(text)
	if !ok {
		return nil, fmt.Errorf("cannot read function call %q", text)
	}
	c := &Call{Name: name, Text: text}
	if hasOptions && options != "" {
		args, tuples, kwargs, notes, err := parseArgs(text, len(text)-1-len(options), options)
		if err != nil {
			return nil, fmt.Errorf("cannot read the arguments of %q: %v", text, err)
		}
		c.Args, c.tupleArg, c.Kwargs, c.Notes = args, tuples, kwargs, notes
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

// parseArgs evaluates options as Python call arguments, as TTP does with
// eval("_get_args_kwargs(" + options + ")"). Each argument is a Python
// literal, with bare names read as TTP's _UndefSubst reads them. An
// argument that isn't a literal is passed as its raw text, with a note
// (Notes): Python TTP evaluates it as an expression, or fails on it. Only
// call syntax that no Python call has is an error: an unterminated string,
// unbalanced brackets, an empty argument, a positional argument after a
// keyword argument, a repeated keyword argument and literals nested too
// deeply. text is the whole call and off is the offset of options in it,
// for the notes. tuples[i] is true when args[i] is a tuple; tuples is nil
// when none is.
func parseArgs(text string, off int, s string) (args []interface{}, tuples []bool, kwargs []Kwarg, notes []string, err error) {
	segs, err := splitArgs(s)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, seg := range segs {
		i := seg.start
		key := ""
		if variable.IdentStartAt(s, i) {
			j := variable.IdentEnd(s, i)
			k := skipSpace(s, j)
			if k < seg.end && s[k] == '=' && (k+1 >= seg.end || s[k+1] != '=') {
				key = s[i:j]
				i = skipSpace(s, k+1)
				if i >= seg.end {
					return nil, nil, nil, nil, fmt.Errorf("keyword argument %s has no value", key)
				}
			}
		}
		raw := s[i:seg.end]
		v, next, tuple, err := variable.PythonValueTupleAt(raw, 0, true)
		switch {
		case err == nil && skipSpace(raw, next) == len(raw):
		case err != nil && strings.Contains(err.Error(), "nested more than"):
			return nil, nil, nil, nil, fmt.Errorf("%v at offset %d", err, i)
		default:
			v, tuple = raw, false
			notes = append(notes, rawArgNote(text, off+i, off+seg.end, raw))
		}
		if key != "" {
			for _, kw := range kwargs {
				if kw.Key == key {
					return nil, nil, nil, nil, fmt.Errorf("keyword argument repeated: %s", key)
				}
			}
			kwargs = append(kwargs, Kwarg{Key: key, Value: v})
		} else {
			if len(kwargs) > 0 {
				return nil, nil, nil, nil, fmt.Errorf("positional argument follows keyword argument")
			}
			if tuple {
				if tuples == nil {
					tuples = make([]bool, len(segs))
				}
				tuples[len(args)] = true
			}
			args = append(args, v)
		}
	}
	return args, tuples, kwargs, notes, nil
}

// argSpan is one argument of a call's options: s[start:end], trimmed.
type argSpan struct{ start, end int }

// splitArgs splits options at the commas outside strings and brackets and
// returns the trimmed arguments. A trailing comma is allowed, as in
// Python. It fails on an unterminated string, unbalanced brackets and an
// empty argument.
func splitArgs(s string) ([]argSpan, error) {
	var spans []argSpan
	var stack []byte
	segStart := 0
	add := func(end int, last bool) error {
		a, b := skipSpace(s, segStart), end
		for b > a && isSpace(s[b-1]) {
			b--
		}
		if a == b {
			if last && (len(spans) > 0 || skipSpace(s, 0) == len(s)) {
				return nil // trailing comma, or no arguments
			}
			return fmt.Errorf("empty argument at offset %d", a)
		}
		spans = append(spans, argSpan{a, b})
		return nil
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\'':
			q := s[i : i+1]
			if strings.HasPrefix(s[i:], strings.Repeat(q, 3)) {
				q = strings.Repeat(q, 3)
			}
			j := i + len(q)
			for ; j < len(s); j++ {
				if s[j] == '\\' {
					j++
					continue
				}
				if strings.HasPrefix(s[j:], q) {
					break
				}
			}
			if j >= len(s) {
				return nil, fmt.Errorf("unterminated string starting at offset %d", i)
			}
			i = j + len(q) - 1
		case '(', '[', '{':
			stack = append(stack, c)
		case ')', ']', '}':
			open := map[byte]byte{')': '(', ']': '[', '}': '{'}[c]
			if len(stack) == 0 || stack[len(stack)-1] != open {
				return nil, fmt.Errorf("unbalanced %q at offset %d", c, i)
			}
			stack = stack[:len(stack)-1]
		case ',':
			if len(stack) == 0 {
				if err := add(i, false); err != nil {
					return nil, err
				}
				segStart = i + 1
			}
		}
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("unclosed %q", stack[len(stack)-1])
	}
	if err := add(len(s), true); err != nil {
		return nil, err
	}
	return spans, nil
}

// rawArgNote describes an argument that isn't a Python literal, which
// gottp passes as its raw text. text[start:end] is the argument in the
// call.
func rawArgNote(text string, start, end int, raw string) string {
	if isBareWord(raw) {
		quoted := `"` + strings.ReplaceAll(raw, `\`, `\\`) + `"`
		return fmt.Sprintf(`argument %s of %s is not a Python literal: Python TTP evaluates it as a Python expression and fails; gottp uses its text %q. Quote it, e.g. %s, so that Python TTP reads it too`,
			raw, text, raw, text[:start]+quoted+text[end:])
	}
	return fmt.Sprintf(`argument %s of %s is a Python expression, not a literal: Python TTP evaluates it; gottp passes its text %q unevaluated`,
		raw, text, raw)
}

// isBareWord reports an unquoted word such as auto-decision or 10.0.0.1,
// which Python TTP evaluates as an expression of names and numbers
// (auto - decision) and fails on. Text with quotes, brackets or spaces, or
// with only "+" and "*" between its names and numbers (which Python can
// evaluate), isn't one.
func isBareWord(raw string) bool {
	name, other := false, false
	prevIdent := false
	for _, r := range raw {
		switch {
		case strings.ContainsRune(`"'()[]{}`, r):
			return false
		case variable.IsIdentStartRune(r) && !prevIdent:
			name = true
		case variable.IsIdentRune(r), r == '+', r == '*':
		default:
			other = true
		}
		prevIdent = variable.IsIdentRune(r)
	}
	return other && (name || strings.Count(raw, ".") > 1)
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

// stringRepr is Python's repr() of a str (unicode_repr in
// Objects/unicodeobject.c).
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
		case r < 0x7f || pyunicode.IsPrintable(r):
			b.WriteRune(r)
		// A character str.isprintable rejects is escaped, in the
		// shortest of \xhh, \uhhhh and \Uhhhhhhhh.
		case r <= 0xff:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r <= 0xffff:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			fmt.Fprintf(&b, `\U%08x`, r)
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

// UnsupportedWarning is the compile warning for a Python TTP function that
// gottp doesn't implement, such as the group function lookup. It has no
// suggestions: the name isn't a typo.
func UnsupportedWarning(scope, kind, name string) string {
	return fmt.Sprintf("%s: %s function %q is a Python TTP function that is not supported by gottp; it is skipped", scope, kind, name)
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
