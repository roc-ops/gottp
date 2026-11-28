package compiled

import (
	"fmt"
	"slices"
	"strings"

	"go.starlark.net/starlark"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/functions/group"
	"github.com/roc-ops/gottp/internal/pattern"
)

// A group's records are processed one at a time, when the record assembler
// saves them, in the order of Python TTP 0.10.1's _results_class.processgrp
// (refactor E1, #149):
//
//  1. Defaults: every variable's default() value, the values of set() on a
//     line without other text, and the group's default= attribute, filled
//     in with setdefault. A merge_with_last record gets none (TTP clears its
//     DEFAULTS).
//  2. merge_with_last: the caller rebuilds the record on a copy of the
//     group's last record before running the steps (tree.go).
//  3. One list of group functions in template attribute order. TTP's
//     _group_class turns every attribute that is not an option (name,
//     input, output, method, default) into a function call, so macro=,
//     void=, record=, contains=, itemize=, custom attribute functions and
//     so on all run in that list, and functions=/chain= expand in place.
//     The first function that rejects the record drops it.
//  4. Path formation, done by whoever saves the record: a path that can't
//     be formed drops it.
//
// Parse (parseFlat, parseTree) and ParseStream (parseGroupStream) run the
// same steps. The steps of a group are built once per parse (pipelineFor).

// stepKind says what a pipeline step runs.
type stepKind uint8

const (
	stepFunc    stepKind = iota // a group function from the registry
	stepMacro                   // macro= or macro(...): run macros by name
	stepVoid                    // void: reject the record
	stepItemize                 // itemize= or itemize(...)
)

// groupStep is one function of a group's function list.
type groupStep struct {
	kind    stepKind
	name    string
	fn      group.Function
	args    []string
	builtin bool // a built-in group function: gets _recorded_vars and _defaults
	custom  bool // a custom attribute function (for its error message)

	// itemize
	key  string
	path []string // nil: save the value at the group's path

	// macro
	macros []string
}

// groupPipeline is a group's per-record processing.
type groupPipeline struct {
	// defaults are the values step 1 fills in; nil when there are none.
	defaults map[string]interface{}
	// defaultString is the group's default= attribute when it is a string,
	// and defaultVars the variables it applies to. A group that matched
	// nothing gets a record of them (finishFlat).
	defaultString string
	hasDefaultStr bool
	defaultVars   []string
	steps         []groupStep
	// pathVars are the {{ variables }} of the group's name, found segment
	// by segment as TTP's form_path finds them (topPathFormable).
	pathVars []string
	// itemizes is true when a step itemizes without a path, which makes the
	// group's result the list of itemized values.
	itemizes bool
	// macro language and whether every macro is Starlark, for the batch
	// fast path.
	macroLang   string
	allStarlark bool
}

// pipelineFor returns group's pipeline for this parse, building it on first
// use. vars are the parse's vars (template <vars> overridden by the passed
// ones), which a chain= naming a variable and default= resolve against.
func (r *Runtime) pipelineFor(group *compiler.CompiledGroup, vars map[string]interface{}) *groupPipeline {
	if pl, ok := r.pipelines[group]; ok {
		return pl
	}
	pl := r.buildPipeline(group, vars)
	if r.pipelines == nil {
		r.pipelines = make(map[*compiler.CompiledGroup]*groupPipeline)
	}
	r.pipelines[group] = pl
	return pl
}

func (r *Runtime) buildPipeline(g *compiler.CompiledGroup, vars map[string]interface{}) *groupPipeline {
	pl := &groupPipeline{macroLang: "starlark", allStarlark: true}
	for _, m := range r.compiled.Macros {
		if m.Language != "" {
			pl.macroLang = m.Language
			break
		}
	}
	r.buildDefaults(pl, g, vars)
	if strings.Contains(g.Name, "{{") {
		for _, seg := range strings.Split(g.Name, ".") {
			for _, sub := range dynPathVarRe.FindAllStringSubmatch(seg, -1) {
				pl.pathVars = append(pl.pathVars, sub[1])
			}
		}
	}

	custom := r.customAttrGroupFunctions(g)
	addAttr := func(name string) {
		value, ok := g.Attributes[name]
		if !ok {
			return
		}
		switch name {
		case "chain", "functions":
			// chain wins when both are set, as before.
			if name == "functions" && g.Chain != "" {
				return
			}
			pl.addChain(r, value, vars)
		case "macro":
			pl.addMacro(r, splitMacroNames([]string{value}))
		case "void":
			if isVoidGroup(g) {
				pl.steps = append(pl.steps, groupStep{kind: stepVoid, name: "void"})
			}
		case "itemize":
			if value != "" {
				pl.addItemize(parseArgumentsWithoutResolve(value))
			}
		default:
			builtin := isBuiltinAttrGroupFunc(name)
			if !builtin && !slices.Contains(custom, name) {
				return
			}
			if builtin && value == "" && name != "expand" {
				return
			}
			fn, ok := r.getGroupFunc(name)
			if !ok {
				return
			}
			pl.steps = append(pl.steps, groupStep{
				kind:    stepFunc,
				name:    name,
				fn:      fn,
				args:    attrFunctionArgs(name, value),
				builtin: slices.Contains(builtinAttrGroupFunctions, name),
				custom:  !builtin,
			})
		}
	}

	if g.AttrOrder != nil {
		for _, name := range g.AttrOrder {
			addAttr(name)
		}
	} else {
		// A group without attribute order (built by hand, or loaded from an
		// old serialized template) keeps the order gottp used before: the
		// built-in attribute functions in builtinAttrGroupFunctions order,
		// the custom ones sorted, then chain/functions, the macro and void.
		for _, name := range builtinAttrGroupFunctions {
			addAttr(name)
		}
		for _, name := range custom {
			addAttr(name)
		}
		if g.Chain != "" {
			addAttr("chain")
		} else {
			addAttr("functions")
		}
		addAttr("macro")
		addAttr("void")
	}
	return pl
}

// buildDefaults collects TTP's group defaults (group.defaults): the group's
// default= string for every variable, overridden by default() values and
// unconditional set() values, and a default= dict over all of them.
func (r *Runtime) buildDefaults(pl *groupPipeline, g *compiler.CompiledGroup, vars map[string]interface{}) {
	defaults := make(map[string]interface{})
	var dict map[string]interface{}
	if attr, ok := g.Attributes["default"]; ok && attr != "" {
		var value interface{} = attr
		if v, ok := vars[attr]; ok {
			value = v
		} else if v, ok := r.compiled.Vars[attr]; ok {
			value = v
		}
		switch v := value.(type) {
		case map[string]interface{}:
			dict = v
		case string:
			pl.defaultString = v
			pl.hasDefaultStr = true
			seen := make(map[string]bool)
			for _, cp := range g.Patterns {
				for _, name := range cp.VariableOrder {
					if pattern.IsIndicatorVar(name) || seen[name] {
						continue
					}
					seen[name] = true
					pl.defaultVars = append(pl.defaultVars, name)
					defaults[name] = v
				}
				// Variables missing from VariableOrder (hand-built patterns).
				for name := range cp.Variables {
					if pattern.IsIndicatorVar(name) || seen[name] {
						continue
					}
					seen[name] = true
					pl.defaultVars = append(pl.defaultVars, name)
					defaults[name] = v
				}
			}
		}
	}
	for k, v := range defaultsRecord(g) {
		defaults[k] = v
	}
	for k, v := range g.Defaults {
		defaults[k] = v
	}
	for k, v := range dict {
		defaults[k] = v
	}
	if len(defaults) > 0 {
		pl.defaults = defaults
	}
}

// attrFunctionArgs splits a function attribute's value into arguments.
func attrFunctionArgs(name, value string) []string {
	switch name {
	case "sformat":
		return sformatArgs(value)
	case "expand":
		if value == "" {
			return []string{}
		}
		args := strings.Split(value, ",")
		for i := range args {
			args[i] = strings.TrimSpace(args[i])
		}
		return args
	}
	if value == "" {
		return nil
	}
	args := strings.Split(value, ",")
	for i := range args {
		args[i] = strings.Trim(strings.TrimSpace(args[i]), `"'`)
	}
	return args
}

// sformatArgs parses sformat's keyword arguments (string='...',
// add_field='...'), splitting on commas outside quotes.
func sformatArgs(value string) []string {
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)
	var parts []string
	for i := 0; i < len(value); i++ {
		char := value[i]
		switch {
		case !inQuotes && (char == '"' || char == '\''):
			inQuotes = true
			quoteChar = char
			current.WriteByte(char)
		case inQuotes && char == quoteChar:
			inQuotes = false
			quoteChar = 0
			current.WriteByte(char)
		case !inQuotes && char == ',':
			if part := strings.TrimSpace(current.String()); part != "" {
				parts = append(parts, part)
			}
			current.Reset()
		default:
			current.WriteByte(char)
		}
	}
	if part := strings.TrimSpace(current.String()); part != "" {
		parts = append(parts, part)
	}
	var args []string
	for _, part := range parts {
		switch {
		case strings.HasPrefix(part, "string="):
			args = append(args, strings.Trim(strings.TrimPrefix(part, "string="), `"'`))
		case strings.HasPrefix(part, "add_field="):
			args = append(args, strings.Trim(strings.TrimPrefix(part, "add_field="), `"'`))
		}
	}
	return args
}

// addChain expands a chain=/functions= attribute in place: the attribute
// names a variable holding a "|"-separated string or a list of them, or is
// itself the "|"-separated chain (TTP's extract_chain).
func (pl *groupPipeline) addChain(r *Runtime, chain string, vars map[string]interface{}) {
	items := []string{chain}
	if v, ok := vars[chain]; ok {
		switch val := v.(type) {
		case string:
			items = []string{val}
		case []interface{}:
			items = items[:0]
			for _, item := range val {
				if s, ok := item.(string); ok {
					items = append(items, s)
				}
			}
		}
	}
	for _, item := range items {
		for _, funcStr := range strings.Split(item, "|") {
			funcStr = strings.TrimSpace(funcStr)
			if funcStr == "" {
				continue
			}
			name, args, _ := parseFunctionWithoutResolve(funcStr)
			switch name {
			case "macro":
				pl.addMacro(r, splitMacroNames(args))
			case "void":
				pl.steps = append(pl.steps, groupStep{kind: stepVoid, name: "void"})
			case "itemize":
				pl.addItemize(args)
			default:
				fn, ok := r.getGroupFunc(name)
				if !ok {
					// Not a group function: skipped, as before (TTP logs it).
					continue
				}
				pl.steps = append(pl.steps, groupStep{
					kind:    stepFunc,
					name:    name,
					fn:      fn,
					args:    args,
					builtin: slices.Contains(builtinAttrGroupFunctions, name),
				})
			}
		}
	}
}

// splitMacroNames splits macro arguments into names: TTP's macro() takes
// several arguments, each of them comma-separated names.
func splitMacroNames(args []string) []string {
	var names []string
	for _, arg := range args {
		for _, name := range strings.Split(arg, ",") {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

func (pl *groupPipeline) addMacro(r *Runtime, names []string) {
	if len(names) == 0 {
		return
	}
	for _, name := range names {
		if r.macroRegistry.HasGoMacro(name) {
			pl.allStarlark = false
		}
	}
	pl.steps = append(pl.steps, groupStep{kind: stepMacro, name: "macro", macros: names})
}

// addItemize adds itemize(key, path=""): args are positional or key=/path=
// keyword arguments.
func (pl *groupPipeline) addItemize(args []string) {
	var key, path string
	pos := 0
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "key="):
			key = strings.Trim(strings.TrimSpace(strings.TrimPrefix(arg, "key=")), `"'`)
		case strings.HasPrefix(arg, "path="):
			path = strings.Trim(strings.TrimSpace(strings.TrimPrefix(arg, "path=")), `"'`)
		default:
			switch pos {
			case 0:
				key = arg
			case 1:
				path = arg
			}
			pos++
		}
	}
	step := groupStep{kind: stepItemize, name: "itemize", key: key}
	if path != "" {
		for _, seg := range strings.Split(path, ".") {
			step.path = append(step.path, strings.TrimSpace(seg))
		}
	} else {
		pl.itemizes = true
	}
	pl.steps = append(pl.steps, step)
}

// groupKwargs builds the keyword arguments of one group function call:
// template vars, then the parse's vars, then the recorded vars, each
// overriding the one before. Built-in functions also get the live recorded
// vars (_recorded_vars, so record() can save into them) and the record's
// defaults (_defaults, which contains and exclude compare against); custom
// functions get neither, so user code can't change gottp's state.
func (r *Runtime) groupKwargs(vars map[string]interface{}, builtin bool, defaults map[string]interface{}) map[string]interface{} {
	kwargs := make(map[string]interface{}, len(r.compiled.Vars)+len(vars)+len(r.recordedVars)+2)
	for k, v := range r.compiled.Vars {
		kwargs[k] = v
	}
	for k, v := range vars {
		kwargs[k] = v
	}
	for k, v := range r.recordedVars {
		kwargs[k] = v
	}
	if builtin {
		kwargs["_recorded_vars"] = r.recordedVars
		if defaults != nil {
			kwargs["_defaults"] = defaults
		}
	}
	return kwargs
}

// itemizeCtx is where itemize with a path saves its values: root is the
// input's results map (TTP's self.results) and dyn the dynamic-path cache.
type itemizeCtx struct {
	root map[string]interface{}
	dyn  map[string]interface{}
}

// runPipeline runs pl's steps 1 and 3 on one record. It returns the record
// (or, after itemize without a path, the itemized value) and whether it is
// kept; itemized reports that the value is an itemized value, which the
// caller saves as an item of a list at the group's path.
func (r *Runtime) runPipeline(pl *groupPipeline, rec map[string]interface{}, vars map[string]interface{}, mergeWithLast bool, ic *itemizeCtx) (value interface{}, keep, itemized bool, err error) {
	if rec == nil {
		rec = make(map[string]interface{})
	}
	defaults := pl.defaults
	if mergeWithLast {
		defaults = nil
	}
	for k, v := range defaults {
		if _, ok := rec[k]; !ok {
			rec[k] = v
		}
	}
	if len(pl.steps) == 0 {
		return rec, true, false, nil
	}

	value = rec
	for i := range pl.steps {
		step := &pl.steps[i]
		data, isMap := value.(map[string]interface{})
		if !isMap {
			// After itemize without a path the record is the itemized value;
			// the functions after it can only reject it.
			data = map[string]interface{}{}
		}
		var out map[string]interface{}
		ok := true
		switch step.kind {
		case stepVoid:
			ok = false
		case stepFunc:
			out, ok, err = step.fn(data, step.args, r.groupKwargs(vars, step.builtin, defaults))
			if err != nil {
				if step.custom {
					return nil, false, false, fmt.Errorf("custom group function %s failed: %w", step.name, err)
				}
				return nil, false, false, fmt.Errorf("group function %s failed: %w", step.name, err)
			}
		case stepMacro:
			out, ok, err = r.runMacros(pl, step.macros, data)
			if err != nil {
				return nil, false, false, err
			}
		case stepItemize:
			v, found := data[step.key]
			if !found || step.key == "" {
				ok = false
				break
			}
			if !isMap {
				break
			}
			if ic != nil {
				for k, dv := range data {
					ic.dyn[k] = dv
				}
			}
			if step.path == nil {
				delete(data, step.key)
				value = v
				itemized = true
				continue
			}
			if ic == nil {
				return nil, false, false, fmt.Errorf("itemize with a path is not supported here")
			}
			r.itemizeToPath(ic, step.path, data, v, vars)
			out = data
		}
		if !ok {
			return nil, false, false, nil
		}
		if isMap && step.kind != stepItemize {
			if out == nil {
				out = map[string]interface{}{}
			}
			value = out
		}
	}
	return value, true, itemized, nil
}

// itemizeToPath saves an itemized value at path in the input's results, as
// TTP's itemize does with a path: the path's last segment ends in "*", and
// its {{ variables }} take the record's values (removed from the record),
// else the dynamic-path cache's, the recorded vars' or the parse's vars'. A
// path that can't be formed saves nothing.
func (r *Runtime) itemizeToPath(ic *itemizeCtx, path []string, rec map[string]interface{}, value interface{}, vars map[string]interface{}) {
	parts := make([]pathPart, 0, len(path))
	for i, seg := range path {
		key := strings.TrimRight(seg, "*")
		formatter := strings.Repeat("*", len(seg)-len(key))
		if i == len(path)-1 && formatter != "*" {
			formatter = "*"
		}
		if strings.Contains(key, "{{") {
			for _, sub := range dynPathVarRe.FindAllStringSubmatch(key, -1) {
				name := sub[1]
				var v interface{}
				if rv, ok := rec[name]; ok {
					v = rv
					ic.dyn[name] = rv
					delete(rec, name)
				} else if dv, ok := ic.dyn[name]; ok {
					v = dv
				} else if gv, ok := r.recordedVars[name]; ok {
					v = gv
				} else if pv, ok := vars[name]; ok {
					v = pv
				} else {
					return
				}
				key = strings.ReplaceAll(key, sub[0], fmt.Sprintf("%v", v))
			}
		}
		parts = append(parts, pathPart{key: key, formatter: formatter})
	}
	if ic.root == nil || len(parts) == 0 {
		return
	}
	saveCurElements(ic.root, parts, value)
}

// runMacros runs the named macros on one record, as TTP's group macro()
// does: a name not in the macro registry is skipped, False drops the
// record, True or None keeps it as it is, and a dict replaces it. Names are
// looked up in the macro registry directly, never resolved as variables
// (#147).
func (r *Runtime) runMacros(pl *groupPipeline, names []string, rec map[string]interface{}) (map[string]interface{}, bool, error) {
	// All Starlark: chain the macros without converting between Go and
	// Starlark in between.
	if pl.allStarlark && pl.macroLang == "starlark" {
		if engine := r.macroRegistry.GetStarlarkEngine(); engine != nil {
			resultVal, err := engine.ExecuteMacroStarlarkBatch(names, engine.GoToStarlark(rec), nil)
			if err == nil {
				// None means "keep the original data" in TTP, not "drop".
				if resultVal != starlark.None {
					if m, ok := engine.StarlarkToGo(resultVal).(map[string]interface{}); ok {
						return m, true, nil
					}
				}
				return rec, true, nil
			}
			if !macroErrorIsNotFound(err) {
				return nil, false, fmt.Errorf("group macro %v failed: %w", names, err)
			}
			// A name not found: run them one by one below, skipping it.
		}
	}

	processed := rec
	for _, name := range names {
		var result interface{}
		var err error
		if r.macroRegistry.HasGoMacro(name) {
			result, err = r.macroRegistry.ExecuteMacro("go", name, processed, nil)
		} else {
			result, err = r.macroRegistry.ExecuteMacro(pl.macroLang, name, processed, nil)
		}
		if err != nil {
			if !macroErrorIsNotFound(err) {
				return nil, false, fmt.Errorf("macro %s failed: %w", name, err)
			}
			continue // not found: skipped, as in TTP
		}
		switch v := result.(type) {
		case bool:
			if !v {
				return nil, false, nil
			}
		case map[string]interface{}:
			processed = v
		}
	}
	return processed, true, nil
}
