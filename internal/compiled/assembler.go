package compiled

import (
	"fmt"
	"strings"

	"github.com/roc-ops/gottp/internal/compiler"
)

// The record assembler is a port of Python TTP 0.10.1's _results_class
// (ttp.py: make_results, start, startempty, add, join, end) for one group.
// It turns a group's pattern matches, in input order, into records. It does
// no post-processing: group functions, chain, macro, defaults, YANG and
// record= stay with the caller, as TTP's processgrp and the output stage do.
//
// TTP keeps one results dict per top group, keyed by match position. At each
// position it picks one winning match (make_results' tier rules, ported by
// choose) and then applies the winner's ACTION:
//
//   - start: save the current record and start a new one with the values;
//   - startempty: the same, but the new record starts empty (a variable
//     named _start_);
//   - add: add values the record doesn't have yet (no overwrite);
//   - join: joinmatches and _line_ lines; a string grows by the join
//     character, a list is extended (or the value appended), any other value
//     becomes a list;
//   - end: mark the group ended; the line's own values are discarded.
//
// A start line whose condition rejects it (TTP's result False) ends the
// group too. An ended group takes no more matches until its next start.
//
// Children (refactor C2, not implemented yet). TTP runs a top group's child
// regexes over the parent's region and puts every match in the same
// position-keyed dict, so one assembler sees the whole tree. To get there:
//   - asmGroup gets one entry per group of the tree, with TTP's group_id,
//     parent_group_id and PATH (the record's absolute path, kept unresolved
//     so two sibling groups with the same name compare equal);
//   - candidates at a position are ordered by group (run_re's depth-first
//     order) before rank, and choose's "children of the current record's
//     group" and "parent started" branches start to matter;
//   - startRecord's started-groups tracing already follows TTP's while/else;
//     add and join need TTP's other branch: a match for a started group whose
//     PATH differs from the current record's starts a record with
//     merge_with_last, and end needs its PATH/started_groups[-1] check;
//   - matches of a group whose parent ended are skipped (make_results'
//     parent_group_id in ended_groups);
//   - asmRecord carries the group (and so its PATH) so the caller can place
//     the record in the results tree.
//
// The assembler takes matches one at a time and keeps only the matches at
// the current position and the current record, so ParseStream can feed it a
// line at a time with bounded memory.

// asmPattern is what the assembler needs to know about one pattern of a
// group, from the group's indicator profile.
type asmPattern struct {
	action compiler.PatternAction
	isLine bool // TTP's IS_LINE: the lowest tier when patterns tie
	// rank is the pattern's place in the list TTP appends a position's
	// matches in: start_re, then end_re, then re, each in template order.
	rank int
	// joinVars are the line's joinmatches variables and their join
	// characters, in template order (TTP's join takes the last one present).
	joinVars []asmJoinVar
	// none marks a line TTP has no regex for (an unconditional set() line);
	// its matches never take part in choosing the winner.
	none bool
}

type asmJoinVar struct {
	name string
	char string
}

// asmGroup is one group as the assembler sees it.
type asmGroup struct {
	name     string // TTP's group_id[0], for choose's prefix rule
	id       int    // TTP's group_id
	parentID int    // TTP's parent_group_id; -1 for a top group
	patterns []asmPattern
}

// newAsmGroup builds the assembler's view of a group without children. A
// nested group gets parent ID 1: parseGroup runs it on its parent record's
// slice, where in TTP the parent group has started (see newAssembler).
func newAsmGroup(group *compiler.CompiledGroup) *asmGroup {
	prof := group.Profile()
	g := &asmGroup{name: group.Name, id: 0, parentID: -1, patterns: make([]asmPattern, len(group.Patterns))}
	if group.IsNested {
		g.parentID = 1
	}
	n := len(group.Patterns)
	for i, cp := range group.Patterns {
		pp := prof.Pattern(i)
		ap := asmPattern{action: pp.Action, isLine: pp.IsLine}
		switch pp.Role {
		case compiler.RoleStart:
			ap.rank = i
		case compiler.RoleEnd:
			ap.rank = n + i
		case compiler.RoleOrdinary:
			ap.rank = 2*n + i
		default:
			ap.rank = 3*n + i
			ap.none = true
		}
		for _, name := range cp.VariableOrder {
			v := cp.Variables[name]
			if v == nil {
				continue
			}
			for _, f := range v.Functions {
				if functionNameOf(f) == "joinmatches" {
					ap.joinVars = append(ap.joinVars, asmJoinVar{name: name, char: joinMatchesCharOf(f)})
					break
				}
			}
		}
		g.patterns[i] = ap
	}
	return g
}

// asmMatch is one pattern match fed to the assembler.
type asmMatch struct {
	idx     int // the caller's index for the match, reported in asmRecord.used
	pattern int // pattern index in the group
	start   int // span start: matches with the same start tie
	end     int // span end
	// result is the match's values; nil for a start line rejected by a
	// condition (TTP's result False).
	result map[string]interface{}
}

// asmRecord is one finished record.
type asmRecord struct {
	result map[string]interface{}
	// used holds the idx of every match the record took (start, add, join
	// and end), in input order; nil unless the assembler tracks them.
	used []int
	// srcRange is [start of the starting match, end]: the end of the match
	// that ended the record, else the start of the match that started the
	// next one, else the end passed to finish.
	srcRange [2]int
}

// asmCurrent is TTP's self.record: the record being assembled.
type asmCurrent struct {
	group    *asmGroup
	result   map[string]interface{}
	used     []int
	start    int
	closedAt int // end of the match that ended the group, or -1
}

// assembler assembles the records of one group without children.
type assembler struct {
	group     *asmGroup
	trackUsed bool
	emit      func(asmRecord) error

	pending []asmMatch // matches at the current position, not yet resolved

	rec     *asmCurrent // nil until the first start (TTP's initial record is never saved)
	started []int       // TTP's started_groups
	ended   map[int]bool
}

// newAssembler starts an assembler for g. For a nested group the parent is
// already in started_groups, as it is in TTP when the child's matches come
// after the parent's start line; that decides choose's tie-breaks for the
// child's first record.
func newAssembler(g *asmGroup, trackUsed bool, emit func(asmRecord) error) *assembler {
	a := &assembler{group: g, trackUsed: trackUsed, emit: emit, ended: make(map[int]bool)}
	if g.parentID >= 0 {
		a.started = append(a.started, g.parentID)
	}
	return a
}

// add feeds one match. Matches must come in order of start; matches with the
// same start are resolved together when a later one arrives or at finish.
func (a *assembler) add(m asmMatch) error {
	if len(a.pending) > 0 && a.pending[0].start != m.start {
		if err := a.resolve(); err != nil {
			return err
		}
	}
	a.pending = append(a.pending, m)
	return nil
}

// finish resolves the last position and emits the last record, whose
// srcRange ends at end unless an end pattern closed it.
func (a *assembler) finish(end int) error {
	if err := a.resolve(); err != nil {
		return err
	}
	if a.rec != nil {
		rec := a.rec
		a.rec = nil
		return a.emitRecord(rec, end)
	}
	return nil
}

// resolve picks the winner among the pending matches and applies it.
func (a *assembler) resolve() error {
	if len(a.pending) == 0 {
		return nil
	}
	cands := a.pending
	// Lines TTP has no regex for don't compete; their values are added to
	// the open record without overwriting, after the winner.
	var extra []asmMatch
	n := 0
	for _, c := range cands {
		if a.group.patterns[c.pattern].none {
			extra = append(extra, c)
			continue
		}
		cands[n] = c
		n++
	}
	cands = cands[:n]
	var err error
	if len(cands) > 0 {
		err = a.apply(a.choose(cands))
	}
	if err == nil {
		for _, c := range extra {
			if c.result != nil && a.openFor(a.group) {
				a.addValues(c)
			}
		}
	}
	for i := range a.pending {
		a.pending[i] = asmMatch{}
	}
	a.pending = a.pending[:0]
	return err
}

// pat returns the pattern of m.
func (a *assembler) pat(m asmMatch) asmPattern { return a.group.patterns[m.pattern] }

// choose ports make_results' winner selection for a position matched by
// several patterns: startempty beats start, start beats ordinary and end
// patterns, and _line_ patterns lose to everything. Within a tier the
// candidates are tried in TTP's list order, preferring the current record's
// group, then (start tiers) its children or a group whose parent started,
// then (ordinary tier) a started group; if none qualifies the last
// candidate tried wins. Before the first record of a group, two start
// patterns on one line therefore give the second one, as in TTP.
func (a *assembler) choose(cands []asmMatch) asmMatch {
	if len(cands) == 1 {
		return cands[0]
	}
	// Order by TTP's list order (insertion sort: a position has few matches).
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0 && a.pat(cands[j]).rank < a.pat(cands[j-1]).rank; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
	var startEmpty, start, normal, line []int
	for i, c := range cands {
		p := a.pat(c)
		switch {
		case p.isLine:
			line = append(line, i)
		case p.action == compiler.ActionStart:
			start = append(start, i)
		case p.action == compiler.ActionStartEmpty:
			startEmpty = append(startEmpty, i)
		default:
			normal = append(normal, i)
		}
	}
	var recGroup *asmGroup
	if a.rec != nil {
		recGroup = a.rec.group
	}
	switch {
	case len(startEmpty) > 0:
		return a.chooseStart(cands, startEmpty, recGroup)
	case len(start) > 0:
		return a.chooseStart(cands, start, recGroup)
	case len(normal) > 0:
		var w asmMatch
		for _, i := range normal {
			if w = cands[i]; a.group == recGroup {
				return w
			}
		}
		for _, i := range normal {
			if w = cands[i]; a.isStarted(a.group.id) {
				return w
			}
		}
		return w
	default:
		var w asmMatch
		for _, i := range line {
			if w = cands[i]; a.group == recGroup {
				return w
			}
		}
		return w
	}
}

// chooseStart is choose for the start and startempty tiers, which skip
// rejected matches while looking for a preferred one.
func (a *assembler) chooseStart(cands []asmMatch, idxs []int, recGroup *asmGroup) asmMatch {
	g := a.group
	var w asmMatch
	for _, i := range idxs {
		w = cands[i]
		if w.result == nil {
			continue
		}
		if g == recGroup {
			return w
		}
	}
	for _, i := range idxs {
		w = cands[i]
		if w.result == nil {
			continue
		}
		if recGroup != nil && strings.HasPrefix(g.name, recGroup.name) {
			return w
		}
		if a.isStarted(g.parentID) {
			return w
		}
	}
	return w
}

func (a *assembler) isStarted(id int) bool {
	for _, s := range a.started {
		if s == id {
			return true
		}
	}
	return false
}

// openFor reports whether g's record is open: started and not ended.
func (a *assembler) openFor(g *asmGroup) bool {
	return a.rec != nil && a.rec.group == g && !a.ended[g.id]
}

// apply is make_results' handling of the winning match.
func (a *assembler) apply(m asmMatch) error {
	g := a.group
	p := g.patterns[m.pattern]
	if m.result == nil {
		// A rejected start line ends the group.
		a.ended[g.id] = true
		if a.rec != nil && a.rec.group == g && a.rec.closedAt < 0 {
			a.rec.closedAt = m.start
		}
		return nil
	}
	isStart := p.action == compiler.ActionStart || p.action == compiler.ActionStartEmpty
	if a.ended[g.id] {
		if !isStart {
			return nil
		}
		delete(a.ended, g.id)
	}
	switch p.action {
	case compiler.ActionStart:
		return a.startRecord(g, m, m.result)
	case compiler.ActionStartEmpty:
		return a.startRecord(g, m, nil)
	case compiler.ActionAdd:
		if a.rec != nil && a.rec.group == g {
			a.addValues(m)
		}
	case compiler.ActionJoin:
		if a.rec != nil && a.rec.group == g {
			a.join(m)
		}
	case compiler.ActionEnd:
		if a.rec != nil && a.rec.group == g {
			a.ended[g.id] = true
			a.use(m)
			a.rec.closedAt = m.end
		}
	}
	return nil
}

// startRecord is TTP's start/startempty: trace the started groups, save the
// current record and start a new one with values (nil for startempty).
func (a *assembler) startRecord(g *asmGroup, m asmMatch, values map[string]interface{}) error {
	traced := false
	for len(a.started) > 0 {
		if g.parentID == a.started[len(a.started)-1] {
			a.started = append(a.started, g.id)
			traced = true
			break
		}
		if g.patterns[m.pattern].isLine {
			a.started = append(a.started, g.id)
			traced = true
			break
		}
		a.started = a.started[:len(a.started)-1]
	}
	if !traced && !a.isStarted(g.id) {
		a.started = append(a.started, g.id)
	}

	if a.rec != nil {
		if err := a.emitRecord(a.rec, m.start); err != nil {
			return err
		}
	}
	result := make(map[string]interface{}, len(values))
	for k, v := range values {
		result[k] = v
	}
	a.rec = &asmCurrent{group: g, result: result, start: m.start, closedAt: -1}
	a.use(m)
	return nil
}

// addValues is TTP's add: values the record lacks are added.
func (a *assembler) addValues(m asmMatch) {
	for k, v := range m.result {
		if _, ok := a.rec.result[k]; !ok {
			a.rec.result[k] = v
		}
	}
	a.use(m)
}

// join is TTP's join for the current record.
func (a *assembler) join(m asmMatch) {
	char := "\n"
	for _, jv := range a.pat(m).joinVars {
		if _, ok := m.result[jv.name]; ok {
			char = jv.char
		}
	}
	rec := a.rec.result
	for k, v := range m.result {
		old, ok := rec[k]
		if !ok {
			rec[k] = v
			continue
		}
		switch o := old.(type) {
		case string:
			js := &asmJoinedString{}
			js.b.WriteString(o)
			js.b.WriteString(char)
			js.b.WriteString(joinString(v))
			rec[k] = js
		case *asmJoinedString:
			o.b.WriteString(char)
			o.b.WriteString(joinString(v))
		case []interface{}:
			jl := &asmJoinedList{items: append([]interface{}(nil), o...)}
			jl.extend(v)
			rec[k] = jl
		case []string:
			jl := &asmJoinedList{items: make([]interface{}, 0, len(o)+1)}
			for _, s := range o {
				jl.items = append(jl.items, s)
			}
			jl.extend(v)
			rec[k] = jl
		case *asmJoinedList:
			o.extend(v)
		default:
			rec[k] = &asmJoinedList{items: []interface{}{old, v}}
		}
	}
	a.use(m)
}

// joinString is the text TTP appends to a joined string. TTP concatenates
// with +, which fails for a value that isn't a string; gottp formats it.
func joinString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// asmJoinedString and asmJoinedList hold a value that join is growing, so
// a long run of joined lines costs linear time. emitRecord turns them back
// into a string and a []interface{}.
type asmJoinedString struct{ b strings.Builder }

type asmJoinedList struct{ items []interface{} }

func (l *asmJoinedList) extend(v interface{}) {
	switch t := v.(type) {
	case []interface{}:
		l.items = append(l.items, t...)
	case []string:
		for _, s := range t {
			l.items = append(l.items, s)
		}
	default:
		l.items = append(l.items, v)
	}
}

func (a *assembler) use(m asmMatch) {
	if a.trackUsed {
		a.rec.used = append(a.rec.used, m.idx)
	}
}

func (a *assembler) emitRecord(rec *asmCurrent, end int) error {
	for k, v := range rec.result {
		switch t := v.(type) {
		case *asmJoinedString:
			rec.result[k] = t.b.String()
		case *asmJoinedList:
			rec.result[k] = t.items
		}
	}
	if rec.closedAt >= 0 {
		end = rec.closedAt
	}
	return a.emit(asmRecord{result: rec.result, used: rec.used, srcRange: [2]int{rec.start, end}})
}

// functionNameOf returns the name of a match function call such as
// `joinmatches(",")`.
func functionNameOf(f string) string {
	f = strings.TrimSpace(f)
	if i := strings.IndexByte(f, '('); i >= 0 {
		f = f[:i]
	}
	return strings.TrimSpace(f)
}

// joinMatchesCharOf is the join character of a joinmatches call: its first
// argument, or a newline.
func joinMatchesCharOf(f string) string {
	start := strings.Index(f, "(")
	end := strings.LastIndex(f, ")")
	if start >= 0 && end > start {
		if arg := strings.Trim(strings.TrimSpace(f[start+1:end]), "'\""); arg != "" {
			return arg
		}
	}
	return "\n"
}
