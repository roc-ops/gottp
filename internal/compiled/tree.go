package compiled

import (
	"fmt"
	"github.com/roc-ops/gottp/internal/call"
	"sort"
	"strings"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/pattern"
)

// A top group with child groups is parsed as Python TTP 0.10.1 parses it
// (refactor C2, #60): one stream of matches for the whole tree, one
// assembler, and each record saved at its PATH under the last record of its
// parent.
//
//   - Matching ports _parser_class.run_re. A group's start patterns run on
//     its region of the input; the region then narrows to the span from the
//     first start match to the last end match after the last start, and the
//     end and ordinary patterns run there. The children run on the narrowed
//     region. A group whose start patterns match nothing keeps a default
//     start (at the end of the stream) if its start line has defaults, and
//     its children run only if it has no regexes at all or they have
//     defaults of their own.
//   - All the matches are sorted by position, then by group in run_re's
//     depth-first order and by pattern, and fed to one assembler
//     (assembler.go), as TTP feeds one _results_class.
//   - Each finished record runs its group's pipeline (pipeline.go) when it
//     is saved, as TTP's processgrp runs: a parent before its children. It
//     is then saved with a port of save_curelements: the top group's
//     records form the group's
//     result, and a descendant's record goes to its PATH below the top
//     group's, under the last record there (dict_by_path follows the last
//     item of every list on the way). A record of a group whose PATH is the
//     top group's (an anonymous child) is one more top-level record. A group
//     whose PATH is outside the top group's (an absolute name such as
//     "/vlans", or "/", which is "_anonymous_*") saves its records from the
//     input's results, as save_curelements saves into TTP's self.results.
//
// The top group's result then goes through the orchestrator like any other
// group's (storeGroupResults), which resolves its dynamic path.

// treeGroup is one group of a top group's tree.
type treeGroup struct {
	cg       *compiler.CompiledGroup
	asm      *asmGroup
	parent   *treeGroup
	children []*treeGroup
	// path is TTP's group.path: the result path's segments, with their
	// formatters, unresolved. rel is the part below the top group's path,
	// or, for an outside group (abs), the whole path: abs groups save from
	// the input's results instead of the top group's last record.
	path []string
	rel  []string
	abs  bool
	// start, end and ordinary are the pattern indices of TTP's start_re,
	// end_re and re, in template order; ordinary also holds the lines TTP
	// has no regex for (unconditional set() lines), after the others.
	start, end, ordinary []int
	// defaults is the record of a default start: every variable with
	// default(), first pattern first.
	defaults map[string]interface{}
	yang     string
	void     bool
}

// buildTree builds the tree of top; groups[i] is the group with asm id i.
func buildTree(top *compiler.CompiledGroup) (*treeGroup, []*treeGroup) {
	var groups []*treeGroup
	rankBase := 0
	var build func(cg *compiler.CompiledGroup, parent *treeGroup) *treeGroup
	build = func(cg *compiler.CompiledGroup, parent *treeGroup) *treeGroup {
		tg := &treeGroup{cg: cg, parent: parent, yang: cg.Attributes["yang"], void: isVoidGroup(cg)}
		anonymous := cg.Name == ""
		switch {
		case parent == nil && anonymous:
			tg.path = []string{"_anonymous_*"}
		case parent == nil:
			tg.path = strings.Split(strings.TrimPrefix(cg.Name, "/"), ".")
		case anonymous:
			tg.path = parent.path
		case strings.HasPrefix(cg.Name, "/"):
			// TTP's extract_name: under a parent whose path has an
			// "_anonymous_" segment the path goes below "_anonymous_*", and
			// "/" alone is "_anonymous_*".
			abs := strings.Split(strings.TrimPrefix(cg.Name, "/"), ".")
			switch {
			case containsSegment(parent.path, "_anonymous_"):
				tg.path = append([]string{"_anonymous_*"}, abs...)
			case cg.Name == "/":
				tg.path = []string{"_anonymous_*"}
			default:
				tg.path = abs
			}
		default:
			tg.path = append(append([]string(nil), parent.path...), strings.Split(cg.Name, ".")...)
		}
		if parent != nil {
			// dict_by_path ignores formatters, so "/interfaces*" saves at the
			// top group "interfaces" path.
			// A void top group saves nothing, so its descendants save from
			// the input's results, as in TTP, where processgrp drops only
			// the void group's own records.
			topPath := groups[0].path
			if !groups[0].void && len(tg.path) >= len(topPath) && samePathKeys(tg.path[:len(topPath)], topPath) {
				tg.rel = tg.path[len(topPath):]
			} else {
				tg.rel = tg.path
				tg.abs = true
			}
		}
		name := strings.Join(tg.path, ".")
		if anonymous {
			name += "._anonymous_"
		}
		parentID := -1
		if parent != nil {
			parentID = parent.asm.id
		}
		tg.asm = buildAsmGroup(cg, len(groups), parentID, name, strings.Join(tg.path, "\x00"), rankBase)
		rankBase += 4*len(cg.Patterns) + 1
		groups = append(groups, tg)

		prof := cg.Profile()
		var none []int
		for i := range cg.Patterns {
			if !prof.Matches(i) {
				continue
			}
			switch prof.Pattern(i).Role {
			case compiler.RoleStart:
				tg.start = append(tg.start, i)
			case compiler.RoleEnd:
				tg.end = append(tg.end, i)
			case compiler.RoleOrdinary:
				tg.ordinary = append(tg.ordinary, i)
			default:
				none = append(none, i)
			}
		}
		tg.ordinary = append(tg.ordinary, none...)
		tg.defaults = defaultsRecord(cg)

		for _, child := range cg.Groups {
			tg.children = append(tg.children, build(child, tg))
		}
		return tg
	}
	root := build(top, nil)
	return root, groups
}

func containsSegment(path []string, seg string) bool {
	for _, p := range path {
		if p == seg {
			return true
		}
	}
	return false
}

// samePathKeys reports whether two paths name the same keys, formatters
// aside.
func samePathKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if strings.TrimRight(a[i], "*") != strings.TrimRight(b[i], "*") {
			return false
		}
	}
	return true
}

// defaultsRecord is TTP's group defaults from default(): every variable of
// group with default(), set to its default value; a name used on several
// lines takes the first line's.
func defaultsRecord(group *compiler.CompiledGroup) map[string]interface{} {
	out := make(map[string]interface{})
	for _, cp := range group.Patterns {
		for _, varName := range cp.VariableOrder {
			variable := cp.Variables[varName]
			if variable == nil || pattern.IsIndicatorVar(varName) {
				continue
			}
			if _, done := out[varName]; done {
				continue
			}
			for _, c := range variable.Calls().All {
				if c.Name == "default" {
					out[varName] = defaultOf(c)
					break
				}
			}
		}
	}
	return out
}

// defaultOf is the value of a default() call, as TTP's extract_default
// takes it: its one argument as Python reads it (a quoted string, a
// literal, or a bare name, true/false/none in any case being True, False
// and None), and "None" for any other number of arguments.
func defaultOf(c *call.Call) interface{} {
	if len(c.Args) == 1 {
		return c.Args[0]
	}
	return "None"
}

// hasStartReDefault is TTP's has_start_re_default: a variable of a start
// pattern has a default (default() or the group's default= string), or the
// group has no start pattern and its default= names a dict.
func (t *treeRun) hasStartReDefault(tg *treeGroup) bool {
	var dictDefault map[string]interface{}
	stringDefault := false
	if attr := tg.cg.Attributes["default"]; attr != "" {
		if m, ok := t.lookupVar(attr).(map[string]interface{}); ok {
			if len(tg.start) == 0 {
				return true
			}
			dictDefault = m
		} else {
			stringDefault = true
		}
	}
	for _, pi := range tg.start {
		for varName := range tg.cg.Patterns[pi].Variables {
			if pattern.IsIndicatorVar(varName) {
				continue
			}
			if stringDefault {
				return true
			}
			if _, ok := tg.defaults[varName]; ok {
				return true
			}
			if _, ok := dictDefault[varName]; ok {
				return true
			}
		}
	}
	return false
}

func (t *treeRun) lookupVar(name string) interface{} {
	if v, ok := t.vars[name]; ok {
		return v
	}
	if v, ok := t.r.compiled.Vars[name]; ok {
		return v
	}
	return name
}

// treeRun is one parse of a top group's tree on one input.
type treeRun struct {
	r      *Runtime
	top    *treeGroup
	groups []*treeGroup
	in     *inputLines
	vars   map[string]interface{}
	obs    *smObserver
	// results is the input's results map, where abs groups save their
	// records (TTP's self.results, shared by the template's top groups).
	results map[string]interface{}

	matches []patternMatch
	nextDef int // position of the next default start, past the input

	// The top group's result, as TTP builds the value at its path: top[i]
	// is its i-th record, topRecs[i] that record's smRecord. topList is
	// true once the value is a list (a second record, or a path ending in *).
	top0    []map[string]interface{}
	topRecs []*smRecord
	topList bool
	topStar bool
	// itemized holds the top group's itemized values (itemize on the top
	// group makes its result the list of values).
	itemized   []interface{}
	isItemized bool

	count   map[*treeGroup]int       // records saved per group
	dyn     map[string]interface{}   // TTP's dyn_path_cache
	smByMap map[uintptr]*smRecord    // the smRecord of each saved record map
	lastSM  map[*treeGroup]*smRecord // the smRecord of each group's last record

	// ic is where itemize with a path saves (the input's results).
	ic itemizeCtx

	// yangOrder and yangLists collect each nested group's records per
	// parent record, for YANG validation once the tree is complete.
	yangOrder []yangKey
	yangLists map[yangKey][]map[string]interface{}
}

type yangKey struct {
	g      *treeGroup
	parent int
}

// parseTree parses a top group with child groups (see the top of this file).
func (r *Runtime) parseTree(top *compiler.CompiledGroup, inputData string, vars map[string]interface{}) (interface{}, []yangJob, error) {
	root, groups := buildTree(top)
	t := &treeRun{
		r:       r,
		top:     root,
		groups:  groups,
		in:      newInputLines(inputData),
		vars:    vars,
		obs:     r.smObs,
		nextDef: len(inputData) + 10000000,
		count:   make(map[*treeGroup]int),
		dyn:     make(map[string]interface{}),
		results: r.treeResults,
	}
	if t.results == nil {
		t.results = make(map[string]interface{})
	}
	t.ic = itemizeCtx{root: t.results, dyn: t.dyn}
	if t.obs != nil {
		t.obs.lastNodes = nil
		t.smByMap = make(map[uintptr]*smRecord)
	}
	if last := root.path[len(root.path)-1]; strings.HasSuffix(last, "*") && !strings.HasSuffix(last, "**") {
		t.topStar = true
	}

	// TTP's DATATEXT is "\n" + data + "\n\n", and run_re's top region is
	// DATATEXT[0:-1]. Regions are kept in DATATEXT offsets (input offset
	// + 1) so run_re's arithmetic carries over unchanged.
	t.runRe(root, 0, len(inputData)+2)

	sort.Slice(t.matches, func(i, j int) bool {
		a, b := &t.matches[i], &t.matches[j]
		if a.spanStart != b.spanStart {
			return a.spanStart < b.spanStart
		}
		return t.rank(a) < t.rank(b)
	})

	asmGroups := make([]*asmGroup, len(groups))
	for i, tg := range groups {
		asmGroups[i] = tg.asm
	}
	asm := newTreeAssembler(asmGroups, t.obs != nil, t.save)
	if t.obs != nil {
		t.lastSM = make(map[*treeGroup]*smRecord)
		asm.credit = func(g *asmGroup, idx int) {
			tg := t.groups[g.id]
			if rec := t.lastSM[tg]; rec != nil {
				t.obs.addMatches(rec, tg.cg, t.matches, []int{idx})
			}
		}
	}
	for i := range t.matches {
		m := &t.matches[i]
		if err := asm.add(asmMatch{idx: i, group: m.group, pattern: m.patternIdx, start: m.spanStart, end: m.spanEnd, result: m.result}); err != nil {
			return nil, nil, err
		}
	}
	if err := asm.finish(len(inputData)); err != nil {
		return nil, nil, err
	}
	return t.result()
}

func (t *treeRun) rank(m *patternMatch) int {
	return t.groups[m.group].asm.patterns[m.patternIdx].rank
}

// matchIn appends the matches of pattern pi of tg in the region [startT,
// endT) (DATATEXT offsets) and returns them.
func (t *treeRun) matchIn(tg *treeGroup, pi, startT, endT int) []patternMatch {
	n := len(t.matches)
	t.matches = t.r.matchPattern(tg.cg, pi, tg.asm.id, t.in, startT-1, endT-1, t.vars, t.obs != nil, nil, t.matches)
	return t.matches[n:]
}

// runRe ports run_re for tg on the region [start, end) of DATATEXT.
func (t *treeRun) runRe(tg *treeGroup, start, end int) {
	s := 0  // the lowest start match position, relative to start
	se := 0 // TTP's "start end": the end of the last start match
	e := -1 // the end of the last end match, relative to the new start
	found := false
	first := len(t.matches)

	// TTP's region start: start is a line's first character in DATATEXT
	// (one past the newline TTP's line regexes begin with), or 0 for the
	// whole input. A match of a line at input offset L starts at L in TTP.
	ttpOld := start - 1
	if ttpOld < 0 {
		ttpOld = 0
	}
	for _, pi := range tg.start {
		ms := t.matchIn(tg, pi, start, end)
		if len(ms) == 0 {
			continue
		}
		rel0 := ms[0].spanStart - ttpOld
		if s > rel0 || !found {
			found = true
			s = rel0
		}
		// TTP compares the relative start of the last match with se, which
		// holds an absolute end; the arithmetic is kept as is.
		last := ms[len(ms)-1]
		if se < last.spanStart-ttpOld && !tg.asm.patterns[pi].isLine {
			se = last.spanEnd + 1
		}
	}
	if found {
		start = ttpOld + s + 1
	}

	if !found {
		if t.hasStartReDefault(tg) {
			// TTP saves an empty record that processgrp fills with the
			// group's defaults. The record starts with the default()
			// values so the assembler keeps it (the pipeline adds the
			// rest), and the virtual start pattern (index len(Patterns))
			// keeps them even when the start line is a startempty one.
			result := make(map[string]interface{}, len(tg.defaults))
			for k, v := range tg.defaults {
				result[k] = t.r.resolveDefault(v, t.vars)
			}
			t.matches = append(t.matches, patternMatch{group: tg.asm.id, patternIdx: len(tg.cg.Patterns), spanStart: t.nextDef, spanEnd: t.nextDef, result: result})
			t.nextDef++
		}
		for _, child := range tg.children {
			if len(tg.start) == 0 || t.hasStartReDefault(child) {
				t.runRe(child, start, end)
			}
		}
		return
	}

	// start is now the first start match's line start in DATATEXT, and
	// TTP's region start the newline before it.
	ttpStart := start - 1
	for _, pi := range tg.end {
		ms := t.matchIn(tg, pi, start, end)
		if len(ms) == 0 {
			continue
		}
		lastEnd := ms[len(ms)-1].spanEnd + 1 - ttpStart
		if e < lastEnd && lastEnd > se {
			e = lastEnd
		}
	}
	if e != -1 {
		end = ttpStart + e
		// Drop this group's matches that start past the last end match (a
		// _line_ start line can match there). The line right after the
		// end line starts at its newline, which is end, so it stays, as in
		// TTP's "span_start > end" (#139).
		kept := t.matches[:first]
		for _, m := range t.matches[first:] {
			if m.spanStart <= end {
				kept = append(kept, m)
			}
		}
		t.matches = kept
	}

	for _, pi := range tg.ordinary {
		t.matchIn(tg, pi, start, end)
	}
	for _, child := range tg.children {
		t.runRe(child, start, end)
	}
}

// save is the assembler's emit: TTP's processgrp and save_curelements for
// one finished record.
func (t *treeRun) save(rec asmRecord) error {
	r := t.r
	tg := t.groups[rec.group.id]
	r.pathResolver.UpdateCache(rec.result)
	r.matchCollector.Clear()

	var smr *smRecord
	if t.obs != nil {
		smr = t.obs.recordTree(tg.cg, t.matches, rec.used)
		t.lastSM[tg] = smr
	}

	result := rec.result
	// A merge_with_last record is rebuilt on a copy of the group's last
	// saved record (TTP's reconstruct_last_element), processed, and merged
	// back into it.
	var target map[string]interface{}
	if rec.mergeWithLast {
		target = t.lastElement(tg)
		if target != nil {
			rebuilt := make(map[string]interface{}, len(target)+len(result))
			for k, v := range target {
				rebuilt[k] = v
			}
			for k, v := range result {
				rebuilt[k] = v
			}
			result = rebuilt
		}
	}

	pl := r.pipelineFor(tg.cg, t.vars)
	value, keep, itemized, err := r.runPipeline(pl, result, t.vars, rec.mergeWithLast, &t.ic)
	if err != nil {
		return err
	}
	if !keep || (tg == t.top && !r.topPathFormable(pl, value, t.vars)) {
		if smr != nil {
			smr.dropped = true
		}
		return nil
	}
	if itemized {
		t.saveItem(tg, value, smr)
		return nil
	}
	if m, ok := value.(map[string]interface{}); ok {
		t.saveRecord(tg, m, smr, target)
	}
	return nil
}

// saveRecord saves one processed record of tg. A merge_with_last record
// (target set) is merged into target.
func (t *treeRun) saveRecord(tg *treeGroup, m map[string]interface{}, smr *smRecord, target map[string]interface{}) {
	if target != nil {
		if len(tg.rel) > 0 {
			t.formPath(tg, m) // TTP's form_path pops the path variables
		}
		mergeRecursively(target, m)
		if smr != nil {
			if owner := t.smByMap[mapID(target)]; owner != nil {
				smr.alias = owner
			} else {
				// A map no record was saved as (one dict_by_path made):
				// the source map finds it in the results.
				smr.final = target
				if len(t.topRecs) > 0 && !tg.abs {
					smr.parent = t.topRecs[len(t.topRecs)-1]
				}
			}
		}
		return
	}
	if len(tg.rel) == 0 {
		t.saveTop(tg, m, smr)
		return
	}
	root := t.rootFor(tg)
	parts, ok := t.formPath(tg, m)
	if !ok {
		if smr != nil {
			smr.dropped = true
		}
		return
	}
	saved := saveCurElements(root, parts, m)
	t.noteSaved(tg, m)
	if smr != nil {
		if saved == saveSkipped {
			smr.dropped = true
			return
		}
		if !tg.abs {
			smr.parent = t.topRecs[len(t.topRecs)-1]
		}
		if saved == saveStored {
			smr.final = m
			t.smByMap[mapID(m)] = smr
		}
		paths := locateStored(root, segmentKeys(parts), 1)
		if len(paths) > 0 {
			smr.rel = paths[0]
		}
	}
}

// saveTop saves a record at the top group's path. As in save_curelements,
// the first record is the value; a second makes it a list, even when it is
// empty; after that empty records are not added. A first empty record is
// filled by the next one. With a path ending in *, the value is a list from
// the start.
func (t *treeRun) saveTop(tg *treeGroup, m map[string]interface{}, smr *smRecord) {
	n := len(t.top0)
	switch {
	case t.topStar || t.topList:
		if len(m) == 0 {
			if smr != nil {
				smr.dropped = true
			}
			return
		}
	case n == 1 && len(t.top0[0]) == 0:
		for k, v := range m {
			t.top0[0][k] = v
		}
		if smr != nil {
			smr.alias = t.topRecs[0]
		}
		t.noteSaved(tg, t.top0[0])
		return
	case n == 1:
		t.topList = true
	}
	t.top0 = append(t.top0, m)
	if smr == nil && t.obs != nil {
		smr = t.obs.newRecord(t.top.cg)
	}
	t.topRecs = append(t.topRecs, smr)
	if smr != nil {
		t.smByMap[mapID(m)] = smr
	}
	// The values of the top group's path variables name its children's
	// paths (TTP pops them into dyn_path_cache when it saves the record).
	if tg == t.top {
		for _, v := range t.r.pathResolver.ExtractVariablesFromPath(t.top.cg.Name) {
			if val, ok := m[v]; ok {
				t.dyn[v] = val
			}
		}
	}
	t.noteSaved(tg, m)
}

// saveItem saves an itemized value: itemize changes the record's path to end
// in "*", so the values form a list at the group's path.
func (t *treeRun) saveItem(tg *treeGroup, item interface{}, smr *smRecord) {
	if tg == t.top {
		t.isItemized = true
		t.itemized = append(t.itemized, item)
		return
	}
	if len(tg.rel) == 0 {
		if smr != nil {
			smr.dropped = true
		}
		return
	}
	root := t.rootFor(tg)
	parts, ok := t.formPath(tg, nil)
	if !ok {
		return
	}
	last := &parts[len(parts)-1]
	last.formatter = "*"
	saveCurElements(root, parts, item)
	if smr != nil {
		if !tg.abs {
			smr.parent = t.topRecs[len(t.topRecs)-1]
		}
		smr.rel = strings.Join(segmentKeys(parts), ".")
	}
}

// noteSaved counts m as one more saved record of tg (and collects it for
// YANG validation).
func (t *treeRun) noteSaved(tg *treeGroup, m map[string]interface{}) {
	t.count[tg]++
	if tg.yang != "" && tg.parent != nil && t.r.yangValidator != nil {
		parentIdx := 0
		for p := tg.parent; p != nil; p = p.parent {
			if n := t.count[p]; n > 0 {
				parentIdx = n - 1
				break
			}
		}
		key := yangKey{g: tg, parent: parentIdx}
		if t.yangLists == nil {
			t.yangLists = make(map[yangKey][]map[string]interface{})
		}
		if _, seen := t.yangLists[key]; !seen {
			t.yangOrder = append(t.yangOrder, key)
		}
		t.yangLists[key] = append(t.yangLists[key], m)
	}
}

// rootFor is the map tg's path starts from: the input's results for an abs
// group, else the top group's last record.
func (t *treeRun) rootFor(tg *treeGroup) map[string]interface{} {
	if tg.abs {
		return t.results
	}
	return t.topElement()
}

// topElement is the last item at the top group's path, created (as
// dict_by_path creates it) when the group has saved nothing yet.
func (t *treeRun) topElement() map[string]interface{} {
	if len(t.top0) == 0 {
		m := make(map[string]interface{})
		t.top0 = append(t.top0, m)
		var smr *smRecord
		if t.obs != nil {
			smr = t.obs.newRecord(t.top.cg)
			t.smByMap[mapID(m)] = smr
		}
		t.topRecs = append(t.topRecs, smr)
	}
	return t.top0[len(t.top0)-1]
}

// lastElement is the record a merge_with_last record of tg merges into: the
// last item at tg's path, or nil.
func (t *treeRun) lastElement(tg *treeGroup) map[string]interface{} {
	var cur interface{}
	switch {
	case tg.abs:
		cur = t.results
	case len(t.top0) == 0:
		return nil
	case len(tg.rel) == 0:
		return t.top0[len(t.top0)-1]
	default:
		cur = t.top0[len(t.top0)-1]
	}
	parts, ok := t.formPath(tg, nil)
	if !ok {
		return nil
	}
	for _, p := range parts {
		if p.key == "_" {
			continue
		}
		m, ok := lastMap(cur)
		if !ok {
			return nil
		}
		cur, ok = m[p.key]
		if !ok {
			return nil
		}
	}
	m, _ := lastMap(cur)
	return m
}

// lastMap is v, or the last item of v if v is a list, as a map.
func lastMap(v interface{}) (map[string]interface{}, bool) {
	if l, ok := v.([]interface{}); ok {
		if len(l) == 0 {
			return nil, false
		}
		v = l[len(l)-1]
	}
	m, ok := v.(map[string]interface{})
	return m, ok
}

// pathPart is one resolved path segment and its formatter ("", "*" or
// "**").
type pathPart struct {
	key       string
	formatter string
}

// formPath ports form_path for tg's path below the top group: each
// {{ variable }} takes the record's value (removed from the record and kept
// in the dynamic-path cache), else the cache's, else a template variable's.
// It fails (TTP drops the record) when a variable has none.
func (t *treeRun) formPath(tg *treeGroup, m map[string]interface{}) ([]pathPart, bool) {
	parts := make([]pathPart, 0, len(tg.rel))
	for _, seg := range tg.rel {
		trimmed := strings.TrimRight(seg, "*")
		formatter := strings.Repeat("*", len(seg)-len(trimmed))
		if strings.Contains(trimmed, "{{") {
			var ok bool
			trimmed, ok = t.resolveSegment(trimmed, m)
			if !ok {
				return nil, false
			}
		}
		parts = append(parts, pathPart{key: trimmed, formatter: formatter})
	}
	return parts, true
}

func (t *treeRun) resolveSegment(seg string, m map[string]interface{}) (string, bool) {
	for _, sub := range dynPathVarRe.FindAllStringSubmatch(seg, -1) {
		name := sub[1]
		var value interface{}
		if v, ok := m[name]; ok && m != nil {
			value = v
			t.dyn[name] = v
			delete(m, name)
		} else if v, ok := t.dyn[name]; ok {
			value = v
		} else if v, ok := t.r.recordedVars[name]; ok {
			value = v
		} else if v, ok := t.vars[name]; ok {
			value = v
		} else {
			return "", false
		}
		seg = strings.ReplaceAll(seg, sub[0], fmt.Sprintf("%v", value))
	}
	return seg, true
}

// result is the top group's result, as parseGroup returns it: one record as
// a map, several as a list, or the itemized values. It also validates the
// tree's YANG paths.
func (t *treeRun) result() (interface{}, []yangJob, error) {
	r := t.r
	var jobs []yangJob
	if r.yangValidator != nil {
		if t.top.yang != "" && len(t.top0) > 0 {
			jobs = append(jobs, yangJob{key: t.top.cg.Name, path: t.top.yang, name: t.top.cg.Name, group: t.top.cg, data: t.top0})
		}
		for _, key := range t.yangOrder {
			list := t.yangLists[key]
			var data interface{} = list
			if len(list) == 1 {
				data = list[0]
			}
			jobs = append(jobs, yangJob{key: fmt.Sprintf("%s[%d]", key.g.cg.Name, key.parent), path: key.g.yang, name: key.g.cg.Name, group: key.g.cg, data: data})
		}
	}
	if t.isItemized {
		if t.obs != nil {
			t.obs.lastNodes = []*smRecord{t.obs.smCollapse(t.top.cg, t.topRecs)}
		}
		return t.itemized, jobs, nil
	}
	if t.obs != nil {
		t.obs.lastNodes = t.topRecs
	}
	switch {
	case len(t.top0) == 0:
		return nil, jobs, nil
	case len(t.top0) == 1 && !t.topStar:
		return t.top0[0], jobs, nil
	default:
		return t.top0, jobs, nil
	}
}

// topSaver ports save_curelements for the records a group saves one after
// another at its own path. The first record becomes the value (a first
// empty record is filled by the records that follow it), a second turns the
// value into a list even when it is empty, and after that empty records are
// not added. With a path ending in "*" the value is a list from the start.
// Dynamic and "**" paths are not followed: their records may go to
// different keys (or be merged), so empty records are just dropped.
type topSaver struct {
	star       bool // list from the start; empty records are dropped
	n          int
	list       bool
	firstEmpty bool
}

type topAction uint8

const (
	topAppend topAction = iota // add the record
	topMerge                   // merge it into the first (empty) record
	topSkip                    // drop it
)

func newTopSaver(path string) *topSaver {
	last := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		last = path[i+1:]
	}
	star := strings.HasSuffix(last, "*") || path == "" || strings.Contains(path, "{{")
	return &topSaver{star: star}
}

func (s *topSaver) add(empty bool) topAction {
	switch {
	case s.star || s.list:
		if empty {
			return topSkip
		}
	case s.n == 1 && s.firstEmpty:
		s.firstEmpty = empty
		return topMerge
	case s.n == 1:
		s.list = true
	case empty:
		s.firstEmpty = true
	}
	s.n++
	return topAppend
}

// saveResult says what saveCurElements did with a record.
type saveResult uint8

const (
	saveMerged  saveResult = iota // merged into a map already at the path
	saveStored                    // stored as itself
	saveSkipped                   // not stored: an empty record for a list
)

// saveCurElements ports save_curelements for a record saved at parts below
// root (the top group's last record).
func saveCurElements(root map[string]interface{}, parts []pathPart, data interface{}) saveResult {
	parts = append([]pathPart(nil), parts...)
	e := dictByPath(parts, root)
	dataMap, isMap := data.(map[string]interface{})
	switch el := e.(type) {
	case *listRef:
		if !truthy(data) {
			return saveSkipped
		}
		*el.list = append(*el.list, data)
		el.store()
		return saveStored
	case map[string]interface{}:
		switch {
		case parts[len(parts)-1].formatter == "**" && isMap:
			for k, v := range el {
				dataMap[k] = v
			}
			for k, v := range dataMap {
				el[k] = v
			}
		case len(el) != 0:
			valueToList(root, parts, data)
			return saveStored
		case isMap:
			// TTP updates the empty element; storing the record itself
			// keeps its identity for the source map.
			if replaceEmpty(root, parts, el, dataMap) {
				return saveStored
			}
			for k, v := range dataMap {
				el[k] = v
			}
		}
	}
	return saveMerged
}

// listRef is a list found by dictByPath, with a way to store it back after
// appending (Go slices are values).
type listRef struct {
	list  *[]interface{}
	store func()
}

func truthy(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case map[string]interface{}:
		return len(t) > 0
	case []interface{}:
		return len(t) > 0
	case string:
		return t != ""
	case bool:
		return t
	}
	return true
}

// dictByPath ports dict_by_path: it walks parts from element, creating
// missing keys as list_to_dict_fwd does and entering the last item of every
// list, and returns the element at the end (a map, or a *listRef).
func dictByPath(parts []pathPart, element map[string]interface{}) interface{} {
	cur := element
	for i := 0; i < len(parts); i++ {
		name := parts[i].key
		if name == "_" {
			if i == len(parts)-1 {
				return cur
			}
			i++
			name = parts[i].key
		}
		v, ok := cur[name]
		if !ok {
			for k, nv := range listToDictFwd(parts[i:]) {
				cur[k] = nv
			}
			v = cur[name]
		}
		if i == len(parts)-1 {
			if l, ok := v.([]interface{}); ok {
				c, key := cur, name
				ref := &listRef{list: &l}
				ref.store = func() { c[key] = *ref.list }
				return ref
			}
			if m, ok := v.(map[string]interface{}); ok {
				return m
			}
			return nil
		}
		switch t := v.(type) {
		case map[string]interface{}:
			cur = t
		case []interface{}:
			if len(t) == 0 {
				m := listToDictFwd(parts[i+1:])
				t = append(t, m)
				cur[name] = t
			}
			last, ok := t[len(t)-1].(map[string]interface{})
			if !ok {
				return nil
			}
			cur = last
		default:
			return nil
		}
	}
	return cur
}

// listToDictFwd ports list_to_dict_fwd: the nested branch for parts, with a
// list where a segment ends in a single "*".
func listToDictFwd(parts []pathPart) map[string]interface{} {
	if len(parts) == 0 {
		return map[string]interface{}{}
	}
	if parts[0].key == "_" {
		if len(parts) == 1 {
			return map[string]interface{}{}
		}
		parts = parts[1:]
	}
	name := parts[0].key
	if parts[0].formatter == "*" {
		if len(parts) == 1 {
			return map[string]interface{}{name: []interface{}{}}
		}
		return map[string]interface{}{name: []interface{}{listToDictFwd(parts[1:])}}
	}
	return map[string]interface{}{name: listToDictFwd(parts[1:])}
}

// valueToList ports value_to_list: the map at parts below data becomes a
// list holding it and result; with a "_" last segment result is merged into
// it instead.
func valueToList(data map[string]interface{}, parts []pathPart, result interface{}) {
	var cur interface{} = data
	for i := 0; i < len(parts); i++ {
		name := parts[i].key
		if name == "_" {
			if i == len(parts)-1 {
				if rm, ok := result.(map[string]interface{}); ok {
					if m, ok := lastMap(cur); ok {
						for k, v := range rm {
							m[k] = v
						}
					}
				}
				return
			}
			i++
			name = parts[i].key
		}
		m, ok := lastMap(cur)
		if !ok {
			return
		}
		v, ok := m[name]
		if !ok {
			return
		}
		if i == len(parts)-1 {
			m[name] = []interface{}{v, result}
			return
		}
		cur = v
	}
}

// replaceEmpty stores data in place of the empty map el found at parts below
// root, so the stored map is data itself. It reports whether it could.
func replaceEmpty(root map[string]interface{}, parts []pathPart, el, data map[string]interface{}) bool {
	if len(parts) == 0 || parts[len(parts)-1].key == "_" {
		return false
	}
	var cur interface{} = root
	for i := 0; i < len(parts)-1; i++ {
		if parts[i].key == "_" {
			continue
		}
		m, ok := lastMap(cur)
		if !ok {
			return false
		}
		cur = m[parts[i].key]
	}
	m, ok := lastMap(cur)
	if !ok {
		return false
	}
	key := parts[len(parts)-1].key
	if existing, ok := m[key].(map[string]interface{}); ok && mapID(existing) == mapID(el) {
		m[key] = data
		return true
	}
	return false
}

// mergeRecursively ports _merge_recursively: data's values go into element,
// merging into maps element already holds.
func mergeRecursively(element, data map[string]interface{}) {
	for k, v := range data {
		if dm, ok := v.(map[string]interface{}); ok {
			if em, ok := element[k].(map[string]interface{}); ok && len(em) > 0 {
				mergeRecursively(em, dm)
				continue
			}
		}
		element[k] = v
	}
}
