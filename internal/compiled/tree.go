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
//     is then saved with a port of save_curelements at its own path in the
//     input's results: its group's whole path (the top group's path, then
//     the child's), formed from the record's values with form_path. On the
//     way dict_by_path follows the last item of every list, so a child's
//     record goes under the last record at its parent's path. A group whose
//     PATH is outside the top group's (an absolute name such as "/vlans",
//     or "/", which is "_anonymous_*") saves there the same way.
//
// parseTree returns the top group's records for YANG validation; they are
// already in the results.

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
		case parent == nil:
			tg.path = topPathSegments(cg)
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

// topPathSegments is a top group's path as TTP's extract_name splits it: the
// name without a leading "/", split on "."; an anonymous group (and "/")
// saves at "_anonymous_*".
func topPathSegments(g *compiler.CompiledGroup) []string {
	if g.Name == "" || g.Name == "/" {
		return []string{"_anonymous_*"}
	}
	return strings.Split(strings.TrimPrefix(g.Name, "/"), ".")
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
	// results is the input's results map, where every record is saved at
	// its own path (TTP's self.results, shared by the template's top
	// groups).
	results map[string]interface{}

	matches []patternMatch
	nextDef int // position of the next default start, past the input

	// top0 holds the records saved at the top group's path, for YANG
	// validation; itemized holds the top group's itemized values.
	top0       []map[string]interface{}
	itemized   []interface{}
	isItemized bool

	count  map[*treeGroup]int       // records saved per group
	lastSM map[*treeGroup]*smRecord // the smRecord of each group's last record

	// ic is where itemize with a path saves (the input's results).
	ic itemizeCtx

	// yangOrder and yangLists collect each nested group's records per
	// parent record, for YANG validation once the tree is complete.
	yangOrder []yangKey
	yangLists map[yangKey][]map[string]interface{}
	// pathVals holds, per saved record of a yang= group (by identity), the
	// path-variable values forming its path removed from it, so YANG
	// validates the record as it was matched.
	pathVals pathValues
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
		results: r.treeResults,
	}
	if t.results == nil {
		t.results = make(map[string]interface{})
	}
	t.ic = itemizeCtx{root: t.results, dyn: r.pathResolver.dynPathCache}

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

	for _, pi := range tg.start {
		ms := t.matchIn(tg, pi, start, end)
		if len(ms) == 0 {
			continue
		}
		rel0 := ms[0].spanStart + 1 - start
		if s > rel0 || !found {
			found = true
			s = rel0
		}
		// TTP compares the relative start of the last match with se, which
		// holds an absolute end; the arithmetic is kept as is.
		last := ms[len(ms)-1]
		if se < last.spanStart+1-start && !tg.asm.patterns[pi].isLine {
			se = last.spanEnd + 1
		}
	}
	start += s

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

	for _, pi := range tg.end {
		ms := t.matchIn(tg, pi, start, end)
		if len(ms) == 0 {
			continue
		}
		lastEnd := ms[len(ms)-1].spanEnd + 1 - start
		if e < lastEnd && lastEnd > se {
			e = lastEnd
		}
	}
	if e != -1 {
		end = start + e
		// Drop this group's matches past the last end match (a _line_
		// start line can match there).
		kept := t.matches[:first]
		for _, m := range t.matches[first:] {
			if m.spanStart+1 <= end {
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
		target = t.lastElement(tg, result)
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
	if !keep {
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

// topLevel reports whether tg's records go to the top group's path: the top
// group's own, and those of an anonymous child (TTP gives it its parent's
// path).
func (t *treeRun) topLevel(tg *treeGroup) bool {
	return len(tg.rel) == 0 && !tg.abs
}

// saveRecord saves one processed record of tg at its own path in the input's
// results, as TTP's save_curelements does: the path is formed from the
// record's values (form_path, which drops the record when a path variable
// has no value), and dict_by_path follows the last item of every list on the
// way, so a child record goes under the last record at its parent's path. A
// merge_with_last record (target set) is merged into target.
func (t *treeRun) saveRecord(tg *treeGroup, m map[string]interface{}, smr *smRecord, target map[string]interface{}) {
	var taken map[string]interface{}
	if tg.yang != "" && t.r.yangValidator != nil && target == nil {
		taken = takenPathValues(tg.path, m)
	}
	parts, ok := t.formPath(tg, m)
	if !ok {
		if smr != nil {
			smr.dropped = true
		}
		return
	}
	if target != nil {
		mergeRecursively(target, m)
		if smr != nil {
			if owner := t.obs.owners[mapID(target)]; owner != nil {
				smr.alias = owner
			} else {
				// A map no record was saved as (one dict_by_path made):
				// the source map finds it in the results.
				smr.final = target
				smr.rel = locatePath(t.results, segmentKeys(parts))
			}
		}
		return
	}
	if !saveAt(t.results, parts, m, smr, t.obs) {
		return
	}
	t.pathVals.note(m, taken)
	if t.topLevel(tg) {
		t.top0 = append(t.top0, m)
	}
	t.noteSaved(tg, m)
}

// saveItem saves an itemized value: itemize changes the record's path to end
// in "*", so the values form a list at the group's path.
func (t *treeRun) saveItem(tg *treeGroup, item interface{}, smr *smRecord) {
	parts, ok := t.formPath(tg, nil)
	if !ok {
		if smr != nil {
			smr.dropped = true
		}
		return
	}
	parts[len(parts)-1].formatter = "*"
	if !saveAt(t.results, parts, item, smr, t.obs) {
		return
	}
	if tg == t.top {
		t.isItemized = true
		t.itemized = append(t.itemized, item)
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

// lastElement is the record a merge_with_last record of tg merges into: the
// last item at tg's path in the input's results, or nil. The path is formed
// as TTP's reconstruct_last_element forms it, from rec first.
func (t *treeRun) lastElement(tg *treeGroup, rec map[string]interface{}) map[string]interface{} {
	parts, ok := t.formPath(tg, rec)
	if !ok {
		return nil
	}
	return lastMapAt(t.results, parts)
}

// lastMapAt is the last item at parts below root, as a map, or nil when
// there is none. It creates nothing.
func lastMapAt(root map[string]interface{}, parts []pathPart) map[string]interface{} {
	var cur interface{} = root
	for _, key := range walkKeys(parts) {
		m, ok := lastMap(cur)
		if !ok {
			return nil
		}
		cur, ok = m[key]
		if !ok {
			return nil
		}
	}
	m, _ := lastMap(cur)
	return m
}

// walkKeys is the keys dict_by_path walks for parts: a "_" segment is
// skipped, and the segment after it taken as a key even when it is "_"
// too; a "_" last segment ends the path.
func walkKeys(parts []pathPart) []string {
	keys := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		if parts[i].key == "_" {
			if i == len(parts)-1 {
				break
			}
			i++
		}
		keys = append(keys, parts[i].key)
	}
	return keys
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

// pathValues maps a saved record (by identity) to the values of its path
// variables that forming its path removed from it (see takenPathValues).
type pathValues map[uintptr]map[string]interface{}

// takenPathValues is the values in m of the variables of the path segs,
// which formPath removes from m, or nil when m has none.
func takenPathValues(segs []string, m map[string]interface{}) map[string]interface{} {
	var taken map[string]interface{}
	for _, seg := range segs {
		if !strings.Contains(seg, "{{") {
			continue
		}
		for _, sub := range dynPathVarRe.FindAllStringSubmatch(seg, -1) {
			if v, ok := m[sub[1]]; ok {
				if taken == nil {
					taken = make(map[string]interface{})
				}
				taken[sub[1]] = v
			}
		}
	}
	return taken
}

// note records taken as the path values removed from the saved record m.
func (pv *pathValues) note(m map[string]interface{}, taken map[string]interface{}) {
	if len(taken) == 0 {
		return
	}
	if *pv == nil {
		*pv = make(pathValues)
	}
	(*pv)[mapID(m)] = taken
}

// restore is m as YANG validates it: a copy with its removed path values
// back, or m itself when forming its path removed none.
func (pv pathValues) restore(m map[string]interface{}) map[string]interface{} {
	taken := pv[mapID(m)]
	if len(taken) == 0 {
		return m
	}
	c := make(map[string]interface{}, len(m)+len(taken))
	for k, v := range m {
		c[k] = v
	}
	for k, v := range taken {
		if _, ok := c[k]; !ok {
			c[k] = v
		}
	}
	return c
}

// restoreAll restores each record of records (see restore).
func (pv pathValues) restoreAll(records []map[string]interface{}) []map[string]interface{} {
	if len(pv) == 0 {
		return records
	}
	out := make([]map[string]interface{}, len(records))
	for i, m := range records {
		out[i] = pv.restore(m)
	}
	return out
}

// pathPart is one resolved path segment and its formatter ("", "*" or
// "**").
type pathPart struct {
	key       string
	formatter string
}

// formPath forms tg's whole path for record m (see Runtime.formPath).
func (t *treeRun) formPath(tg *treeGroup, m map[string]interface{}) ([]pathPart, bool) {
	return t.r.formPath(tg.path, m, t.vars)
}

// formPath ports TTP's form_path for a path's segments (split on "." with
// their formatters, unresolved). A segment's formatter is its trailing "*"s,
// counted before its {{ variables }} are substituted, and the path is split
// before they are, so a value with a "." or a trailing "*" stays one key
// (#76). Each variable takes the record m's value (removed from m and kept
// in the input's dynamic-path cache), else the cache's, else a recorded
// var's or a template variable's. It fails (TTP drops the record) when a
// variable has no value, which is also the case when a group function or
// macro removed it from the record.
func (r *Runtime) formPath(segs []string, m map[string]interface{}, vars map[string]interface{}) ([]pathPart, bool) {
	parts := make([]pathPart, 0, len(segs))
	for _, seg := range segs {
		key := strings.TrimRight(seg, "*")
		formatter := seg[len(key):]
		if strings.Contains(key, "{{") {
			var ok bool
			key, ok = r.resolveSegment(key, m, vars)
			if !ok {
				return nil, false
			}
		}
		parts = append(parts, pathPart{key: key, formatter: formatter})
	}
	return parts, true
}

// resolveSegment substitutes the {{ variables }} of one path segment (see
// formPath). Like TTP, it finds them with {{\s*(\S+)\s*}} in the segment,
// so "{{x}}{{y}}" names one variable "x}}{{y" that has no value.
func (r *Runtime) resolveSegment(seg string, m map[string]interface{}, vars map[string]interface{}) (string, bool) {
	dyn := r.pathResolver.dynPathCache
	for _, sub := range dynPathVarRe.FindAllStringSubmatch(seg, -1) {
		name := sub[1]
		var value interface{}
		if v, ok := m[name]; ok {
			value = v
			dyn[name] = v
			delete(m, name)
		} else if v, ok := dyn[name]; ok {
			value = v
		} else if v, ok := r.recordedVars[name]; ok {
			value = v
		} else if v, ok := vars[name]; ok {
			value = v
		} else {
			return "", false
		}
		seg = strings.ReplaceAll(seg, sub[0], call.Str(value))
	}
	return seg, true
}

// saveAt saves data (a record, or an itemized value) at parts below root
// with saveCurElements and tells smr (nil when source maps are off) where it
// went: a record stored as itself is found by identity, one merged into a
// record's map takes that record's path, and anything else gets the path it
// was stored at. It reports whether data was stored (an empty record or
// value is not added to a list).
func saveAt(root map[string]interface{}, parts []pathPart, data interface{}, smr *smRecord, obs *smObserver) bool {
	// An empty record already at the path is filled by data (TTP updates
	// it; saveCurElements stores data in its place), so its matches belong
	// to data's record.
	var empty map[string]interface{}
	if smr != nil {
		if el := lastMapAt(root, parts); el != nil && len(el) == 0 {
			empty = el
		}
	}
	saved, at := saveCurElementsAt(root, parts, data)
	if saved == saveSkipped {
		if smr != nil {
			smr.dropped = true
		}
		return false
	}
	if smr == nil {
		return true
	}
	if m, ok := data.(map[string]interface{}); ok {
		if saved == saveStored {
			smr.final = m
			obs.owners[mapID(m)] = smr
			if empty != nil && mapID(empty) != mapID(m) {
				if prev := obs.owners[mapID(empty)]; prev != nil && prev != smr {
					prev.alias = smr
				}
				// m took empty's place in the results, so no record owns
				// empty any more.
				delete(obs.owners, mapID(empty))
			}
			return true
		}
		if el := lastMapAt(root, at); el != nil {
			if owner := obs.owners[mapID(el)]; owner != nil && owner != smr {
				smr.alias = owner
				return true
			}
		}
	}
	smr.rel = locatePath(root, segmentKeys(at))
	return true
}

// result is the top group's result, as parseGroup returns it: its records
// (each already saved at its own path in the input's results), or the
// itemized values, or nil when it saved nothing. It also validates the
// tree's YANG paths.
func (t *treeRun) result() (interface{}, []yangJob, error) {
	r := t.r
	var jobs []yangJob
	if r.yangValidator != nil {
		if t.top.yang != "" && len(t.top0) > 0 {
			jobs = append(jobs, yangJob{key: t.top.cg.Name, path: t.top.yang, name: t.top.cg.Name, group: t.top.cg, data: t.pathVals.restoreAll(t.top0)})
		}
		for _, key := range t.yangOrder {
			list := t.pathVals.restoreAll(t.yangLists[key])
			var data interface{} = list
			if len(list) == 1 {
				data = list[0]
			}
			jobs = append(jobs, yangJob{key: fmt.Sprintf("%s[%d]", key.g.cg.Name, key.parent), path: key.g.yang, name: key.g.cg.Name, group: key.g.cg, data: data})
		}
	}
	if t.isItemized {
		return t.itemized, jobs, nil
	}
	if len(t.top0) == 0 {
		return nil, jobs, nil
	}
	return t.top0, jobs, nil
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
	saved, _ := saveCurElementsAt(root, parts, data)
	return saved
}

// saveCurElementsAt is saveCurElements, also returning the path data went
// to: parts, except when value_to_list merged data at a null path, which it
// reads without parts' leading "_" (see below).
func saveCurElementsAt(root map[string]interface{}, parts []pathPart, data interface{}) (saveResult, []pathPart) {
	orig := parts
	parts = append([]pathPart(nil), parts...)
	e := dictByPath(parts, root)
	// dict_by_path pops a leading "_" from the record's own PATH list when
	// more follows, so value_to_list below sees the path without it.
	if len(parts) > 1 && parts[0].key == "_" {
		parts = parts[1:]
	}
	dataMap, isMap := data.(map[string]interface{})
	switch el := e.(type) {
	case *listRef:
		if !truthy(data) {
			return saveSkipped, orig
		}
		*el.list = append(*el.list, data)
		el.store()
		return saveStored, orig
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
			if parts[len(parts)-1].key == "_" {
				// A null path merges data into the element at parts
				// instead (the root for "_._").
				return saveMerged, parts
			}
			return saveStored, orig
		case isMap:
			// TTP updates the empty element; storing the record itself
			// keeps its identity for the source map.
			if replaceEmpty(root, parts, el, dataMap) {
				return saveStored, orig
			}
			for k, v := range dataMap {
				el[k] = v
			}
		}
	}
	return saveMerged, orig
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
	// TTP's list_to_dict_fwd means to skip a "_" here too, but it compares
	// the (key, formatter) tuple with "_", so a "_" is always a key.
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
	keys := walkKeys(parts)
	var cur interface{} = root
	for _, k := range keys[:len(keys)-1] {
		m, ok := lastMap(cur)
		if !ok {
			return false
		}
		cur = m[k]
	}
	m, ok := lastMap(cur)
	if !ok {
		return false
	}
	key := keys[len(keys)-1]
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
