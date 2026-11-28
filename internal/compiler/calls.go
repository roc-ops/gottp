package compiler

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/roc-ops/gottp/internal/call"
	"github.com/roc-ops/gottp/internal/functions/group"
	"github.com/roc-ops/gottp/internal/functions/input"
	"github.com/roc-ops/gottp/internal/functions/match"
	"github.com/roc-ops/gottp/internal/functions/output"
	"github.com/roc-ops/gottp/internal/pattern"
	"github.com/roc-ops/gottp/internal/pymethod"
)

// Function calls are read at compile time with package call, the one call
// reader for every scope (refactor F1, #36). checkCalls reads each call
// the runtime will run and fails on the first one Python TTP can't read
// either; it also warns about a "|" inside quotes (#23) and about function
// names that aren't known in their scope, with TTP's suggestions.

// BuiltinAttrGroupFunctions are the group functions a group runs when they
// are given as group attributes. A group whose attribute order is unknown
// runs them in this order.
var BuiltinAttrGroupFunctions = []string{"record", "containsall", "contains", "del", "delete", "exclude", "excludeall", "equal", "to_int", "contains_val", "exclude_val", "sformat", "itemize", "set", "expand", "items2dict", "to_ip"}

// Scope names a kind of function, as the warnings and the compile-time
// function overrides name it.
type Scope string

const (
	ScopeMatch  Scope = "match"
	ScopeGroup  Scope = "group"
	ScopeInput  Scope = "input"
	ScopeOutput Scope = "output"
)

// UnknownCall is a function name that isn't known in its scope (or is a
// Python TTP function gottp doesn't support), and the warning given for it.
type UnknownCall struct {
	Scope   Scope
	Name    string
	Warning string
}

// UnknownCalls returns the unknown function names found when the template
// compiled (nil for a template loaded or built by hand). Their warnings are
// in Warnings; a caller that provides a function of that name itself
// (compile-time functions) can drop them.
func (t *CompiledTemplate) UnknownCalls() []UnknownCall {
	return t.unknownCalls
}

// knownNames are the function names each scope knows besides its built-in
// functions, and the candidates TTP's guess suggests from.
type knownNames struct {
	known      map[string]bool
	candidates []string
	// unsupported are Python TTP's functions of the scope that gottp
	// doesn't implement (ttpFunctionNames).
	unsupported map[string]bool
}

var scopeNames = sync.OnceValue(func() map[Scope]knownNames {
	build := func(s Scope, registry []string, extra ...string) knownNames {
		k := knownNames{known: make(map[string]bool), candidates: registry, unsupported: make(map[string]bool)}
		for _, n := range registry {
			k.known[n] = true
		}
		for _, n := range extra {
			k.known[n] = true
		}
		for _, n := range ttpFunctionNames[s] {
			if !k.known[n] {
				k.unsupported[n] = true
			}
		}
		return k
	}
	matchExtra := []string{
		// handled when the template compiles (TTP's extract_funcs)
		"re", "chain", "set", "default", "joinmatches", "ignore", "columns",
		pattern.IndicatorStart, pattern.IndicatorEnd, pattern.IndicatorLine,
		pattern.IndicatorExact, pattern.IndicatorExactSpace, pattern.IndicatorHeaders,
	}
	matchExtra = append(matchExtra, pattern.NewEngine().PatternNames()...)
	return map[Scope]knownNames{
		ScopeMatch:  build(ScopeMatch, match.NewRegistry().Names(), matchExtra...),
		ScopeGroup:  build(ScopeGroup, group.NewRegistry().Names(), "macro", "void", "itemize"),
		ScopeInput:  build(ScopeInput, input.NewRegistry().Names(), "macro"),
		ScopeOutput: build(ScopeOutput, output.NewRegistry().Names(), "macro"),
	}
})

// matchTypeMethodName reports whether name is a method or attribute of a
// match value's type that gottp ports, which Python TTP falls back to for a
// name that isn't a match function (getattr(data, name), ttp.py:2464).
// Such a name isn't unknown.
func matchTypeMethodName(name string) bool {
	return pymethod.IsName(name)
}

// callChecker collects what checkCalls finds.
type callChecker struct {
	vars     map[string]interface{}
	unknown  []UnknownCall
	warnings []string
}

// checkCalls reads every function call of t's own groups, inputs and outputs
// (child templates check their own) and returns the first error. It fills
// t.unknownCalls and appends its warnings to t.Warnings.
func checkCalls(t *CompiledTemplate) error {
	c := &callChecker{vars: t.Vars}
	for _, g := range t.Groups {
		if err := c.group(g); err != nil {
			return err
		}
	}
	for _, in := range t.Inputs {
		scope := fmt.Sprintf("input %q", in.Name)
		calls, err := c.chain(scope, ScopeInput, "input", in.Functions)
		if err != nil {
			return err
		}
		for _, cl := range calls {
			if cl.Name == "macro" {
				if err := checkMacroKwargs(scope, cl); err != nil {
					return err
				}
			}
		}
		in.calls.Store(&calls)
	}
	for i, out := range t.Outputs {
		scope := fmt.Sprintf("output %q", out.Name)
		if out.Name == "" {
			scope = fmt.Sprintf("output %d", i+1)
		}
		calls, err := c.chain(scope, ScopeOutput, "output", out.Functions)
		if err != nil {
			return err
		}
		for _, cl := range calls {
			if cl.Name == "macro" {
				if err := checkOutputMacro(scope, cl); err != nil {
					return err
				}
			}
		}
		out.calls.Store(&calls)
		if v, ok := out.Attributes["macro"]; ok && strings.TrimSpace(v) == "" {
			// TTP calls macro(data) without a name: TypeError.
			return fmt.Errorf("%s: macro attribute is empty; Python TTP fails with \"macro() missing 1 required positional argument: 'macro_name'\"", scope)
		}
	}
	t.unknownCalls = append(t.unknownCalls, c.unknown...)
	t.callWarnings = append(t.callWarnings, c.warnings...)
	t.Warnings = append(t.Warnings, c.warnings...)
	return nil
}

func (c *callChecker) group(g *CompiledGroup) error {
	scope := fmt.Sprintf("group %q", g.Name)
	for _, cp := range g.Patterns {
		for _, name := range variableNames(cp) {
			if err := c.variable(scope, cp.Variables[name]); err != nil {
				return err
			}
		}
		indicator, cut := pattern.CutVariables(cp)
		for _, name := range cut {
			c.warnings = append(c.warnings, cutWarning(scope, cp, indicator, name))
		}
	}
	// Function attributes are read as name(value), as TTP's
	// extract_function reads them.
	for name, value := range g.Attributes {
		if !slices.Contains(BuiltinAttrGroupFunctions, name) || (value == "" && name != "expand") {
			continue
		}
		cl, err := call.Parse(name + "(" + value + ")")
		if err != nil {
			return fmt.Errorf("%s: attribute %s=%q: %v", scope, name, value, err)
		}
		c.notes(fmt.Sprintf("%s, attribute %s", scope, name), cl)
	}
	if value, ok := g.Attributes["macro"]; ok && value != "" {
		mc, err := call.Parse("macro(" + value + ")")
		if err != nil {
			return fmt.Errorf("%s: attribute macro=%q: %v", scope, value, err)
		}
		c.notes(scope+", attribute macro", mc)
		if err := checkMacroKwargs(scope, mc); err != nil {
			return err
		}
	}
	for _, item := range ResolveGroupChain(g, c.vars) {
		calls, err := c.chain(scope, ScopeGroup, "group", item)
		if err != nil {
			return err
		}
		for _, cl := range calls {
			if cl.Name == "macro" {
				if err := checkMacroKwargs(scope, cl); err != nil {
					return err
				}
			}
		}
	}
	for _, child := range g.Groups {
		if err := c.group(child); err != nil {
			return err
		}
	}
	return nil
}

// variableNames returns the pattern's variable names in order.
func variableNames(cp *pattern.CompiledPattern) []string {
	if len(cp.VariableOrder) == len(cp.Variables) {
		return cp.VariableOrder
	}
	names := append([]string(nil), cp.VariableOrder...)
	for name := range cp.Variables {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names[len(cp.VariableOrder):])
	return names
}

func (c *callChecker) variable(groupScope string, v *pattern.MatchVariable) error {
	if v == nil {
		return nil
	}
	scope := fmt.Sprintf("%s, match variable %q", groupScope, v.Name)
	var funcs []string
	for _, f := range v.Functions {
		if !strings.HasPrefix(f, "pattern:") {
			funcs = append(funcs, f)
		}
	}
	// The variable's functions were split from one chain; joined again, a
	// "|" inside quotes shows.
	broken := call.QuotedPipes(strings.Join(funcs, "|"))
	for _, b := range broken {
		c.warnings = append(c.warnings, call.QuotedPipeWarning(scope, b))
	}
	for _, f := range funcs {
		cl, err := call.Parse(f)
		if err != nil {
			return callError(scope, err, broken)
		}
		c.notes(scope, cl)
		if cl.Name == "macro" {
			if err := checkMacroKwargs(scope, cl); err != nil {
				return err
			}
		}
		if scopeNames()[ScopeMatch].known[cl.Name] || matchTypeMethodName(cl.Name) {
			continue
		}
		if w, ok := pymethod.NotPorted(scope, cl.Name); ok {
			// A Python method gottp doesn't port: the value is kept. A
			// custom function of the name drops the warning, as for an
			// unknown name.
			c.unknown = append(c.unknown, UnknownCall{Scope: ScopeMatch, Name: cl.Name, Warning: w})
			c.warnings = append(c.warnings, w)
			continue
		}
		c.checkName(scope, ScopeMatch, "match variable", cl.Name)
	}
	v.Calls() // read once, now
	return nil
}

// chain reads a "|"-separated chain of scope's functions.
func (c *callChecker) chain(scope string, s Scope, kind, chain string) ([]*call.Call, error) {
	if strings.TrimSpace(chain) == "" {
		return nil, nil
	}
	broken := call.QuotedPipes(chain)
	for _, b := range broken {
		c.warnings = append(c.warnings, call.QuotedPipeWarning(scope, b))
	}
	var calls []*call.Call
	for _, part := range call.Split(chain) {
		cl, err := call.Parse(part)
		if err != nil {
			return nil, callError(scope, err, broken)
		}
		c.notes(scope, cl)
		c.checkName(scope, s, kind, cl.Name)
		calls = append(calls, cl)
	}
	return calls, nil
}

// notes warns about the arguments of cl that aren't Python literals.
func (c *callChecker) notes(scope string, cl *call.Call) {
	for _, n := range cl.Notes {
		c.warnings = append(c.warnings, scope+": "+n)
	}
}

func (c *callChecker) checkName(scope string, s Scope, kind, name string) {
	names := scopeNames()[s]
	if names.known[name] {
		return
	}
	var w string
	if names.unsupported[name] {
		w = call.UnsupportedWarning(scope, kind, name)
	} else {
		w = call.UnknownWarning(scope, kind, name, names.candidates)
	}
	c.unknown = append(c.unknown, UnknownCall{Scope: s, Name: name, Warning: w})
	c.warnings = append(c.warnings, w)
}

func callError(scope string, err error, broken []call.Broken) error {
	if len(broken) > 0 {
		return fmt.Errorf("%s: %v (a \"|\" inside quotes splits the chain, as in Python TTP: see the warning for %s)", scope, err, broken[0].Call)
	}
	return fmt.Errorf("%s: %v", scope, err)
}

// checkMacroKwargs rejects keyword arguments to macro(), which takes macro
// names only (TTP: "macro() got an unexpected keyword argument").
func checkMacroKwargs(scope string, cl *call.Call) error {
	for _, kw := range cl.Kwargs {
		return fmt.Errorf("%s: %s: Python TTP fails with \"macro() got an unexpected keyword argument '%s'\"", scope, cl.Text, kw.Key)
	}
	return nil
}

// checkOutputMacro checks an output's macro(): Python TTP's is
// macro(data, macro_name). gottp also runs several names in order.
func checkOutputMacro(scope string, cl *call.Call) error {
	for _, kw := range cl.Kwargs {
		if kw.Key != "macro_name" {
			return fmt.Errorf("%s: %s: Python TTP fails with \"macro() got an unexpected keyword argument '%s'\"", scope, cl.Text, kw.Key)
		}
	}
	if len(cl.Args) == 0 && len(cl.Kwargs) == 0 {
		return fmt.Errorf("%s: %s: Python TTP fails with \"macro() missing 1 required positional argument: 'macro_name'\"", scope, cl.Text)
	}
	return nil
}

// Calls returns the input's functions= read as calls. The compiler reads
// them; for an input loaded or built by hand they are read on first use,
// and a call that can't be read is left out.
func (in *CompiledInput) Calls() []*call.Call {
	return loadCalls(&in.calls, in.Functions)
}

// Calls returns the output's functions= read as calls (see
// CompiledInput.Calls).
func (out *CompiledOutput) Calls() []*call.Call {
	return loadCalls(&out.calls, out.Functions)
}

func loadCalls(p *atomic.Pointer[[]*call.Call], chain string) []*call.Call {
	if c := p.Load(); c != nil {
		return *c
	}
	var calls []*call.Call
	for _, part := range call.Split(chain) {
		if cl, err := call.Parse(part); err == nil {
			calls = append(calls, cl)
		}
	}
	p.CompareAndSwap(nil, &calls)
	return *p.Load()
}

// CheckCustomAttrCalls reads the group attributes that name one of the
// custom group functions isCustom reports (CompileOptions.Functions), as
// the group pipeline will read them (name(value)), and returns the first
// that can't be read. The compiler can't check these itself: it doesn't
// know the custom functions. Its warnings (arguments that aren't Python
// literals) are appended to t.Warnings.
func CheckCustomAttrCalls(t *CompiledTemplate, isCustom func(name string) bool) error {
	var check func(g *CompiledGroup) error
	check = func(g *CompiledGroup) error {
		for name, value := range g.Attributes {
			if slices.Contains(BuiltinAttrGroupFunctions, name) || NonFunctionGroupAttrs[name] || !isCustom(name) {
				continue
			}
			cl, err := call.Parse(name + "(" + value + ")")
			if err != nil {
				return fmt.Errorf("group %q: attribute %s=%q: %v", g.Name, name, value, err)
			}
			for _, n := range cl.Notes {
				t.Warnings = append(t.Warnings, fmt.Sprintf("group %q, attribute %s: %s", g.Name, name, n))
			}
		}
		for _, child := range g.Groups {
			if err := check(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, g := range t.Groups {
		if err := check(g); err != nil {
			return err
		}
	}
	return nil
}

// NonFunctionGroupAttrs are group attributes that are never a custom group
// function.
var NonFunctionGroupAttrs = map[string]bool{
	"name": true, "input": true, "method": true, "output": true, "default": true,
	"chain": true, "functions": true, "macro": true,
}

// cutWarning is the warning for a variable that comes after the {{ _start_ }}
// or {{ _end_ }} its line's regex is cut at (pattern.CutVariables). It says
// what the variable gets, as in Python TTP: nothing on a _start_ line (the
// line starts an empty record); on an _end_ line with no capture left, the
// whole matched line, newline first (TTP's check_matches with an empty
// groupdict); otherwise nothing.
func cutWarning(scope string, cp *pattern.CompiledPattern, indicator, name string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s, line %q: variable %q comes after {{ %s }}, where the line's regex ends (as in Python TTP), so it isn't captured from the data", scope, strings.TrimSpace(cp.Original), name, indicator)
	if indicator == pattern.IndicatorEnd && cp.WholeMatch {
		fmt.Fprintf(&b, ". The line has no capture left, so, as in Python TTP, %q and _end_ are set to the whole matched line with a newline first (\"\\n exit\" for a line \" exit\")", name)
	} else {
		fmt.Fprintf(&b, ": it gets no value from the line")
	}
	v := cp.Variables[name]
	if v != nil && hasFunction(v, "default") {
		if indicator == pattern.IndicatorEnd && cp.WholeMatch {
			b.WriteString("; its default() value still applies to a record without such a line")
		} else {
			b.WriteString("; its default() value still applies")
		}
	}
	if indicator == pattern.IndicatorEnd && lineProfile(cp).Action != ActionEnd {
		b.WriteString(". The variable also makes the line an ordinary line, so it no longer ends the group")
	}
	if name == pattern.IndicatorLine {
		fmt.Fprintf(&b, ". To keep the rest of the line, capture it in a named variable instead: {{ rest | ORPHRASE | %s }}", indicator)
	} else {
		fmt.Fprintf(&b, ". Move {{ %s }} to the end of the line, or make it a function of the variable ({{ %s | %s }})", indicator, name, indicator)
	}
	return b.String()
}

// hasFunction reports whether v has a function named fn.
func hasFunction(v *pattern.MatchVariable, fn string) bool {
	for _, f := range v.Functions {
		if pattern.FunctionName(f) == fn {
			return true
		}
	}
	return false
}
