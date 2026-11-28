package compiled

import (
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/pattern"
)

// The source map is recorded from the matches parseGroup actually uses
// (refactor A2, #60). parseGroup reports to an smObserver, which lives on the
// per-parse Runtime copy only when ParseOptions.EnableSourceMap is set; with
// source maps off Runtime.smObs is nil and every hook is skipped.
//
// The observer collects three things:
//   - smMatch: one per match a record consumed (line, columns, group, pattern
//     index, variable spans), reported once the merge phase has decided which
//     matches belong to which record;
//   - smRecord: one per record, followed through record=, filters, macros and
//     storing, so it knows the map it ended up as (or that it was dropped);
//   - the results maps, walked once at the end to turn each stored record map
//     into its path ("interfaces[1].ipv4[0]").

// smObserver collects source-map events during one parse. Match positions
// are offsets in the input's (processed) data: a top group and all its
// descendants are matched on the whole input (tree.go).
type smObserver struct {
	input       string // input the current top-level group is parsing
	lineOffsets []int  // byte offset of each line of that input's (processed) data, plus a sentinel
	matches     []*smMatch
	// owners maps each record map stored as itself in the results to its
	// smRecord, so a record merged into it (a "**" path) takes its path.
	owners map[uintptr]*smRecord
}

func newSMObserver() *smObserver {
	return &smObserver{owners: make(map[uintptr]*smRecord)}
}

// smRecord follows one record from the merge phase to the results.
type smRecord struct {
	input  string
	group  *compiler.CompiledGroup
	parent *smRecord // record of the parent group (nil for a top-level group)
	// alias is the record this one was merged into (record=, an unnamed child
	// group merged into its parent, or several records collapsed into one).
	alias   *smRecord
	final   map[string]interface{} // the map stored in the results, matched by identity
	rel     string                 // fallback path: relative to parent, or the full path at top level
	dropped bool                   // not stored (filtered out, void, empty)
}

// smMatch is one match a record consumed.
type smMatch struct {
	input      string
	line       int
	startCol   int
	endCol     int
	group      *compiler.CompiledGroup
	patternIdx int
	vars       map[string]*VarRange
	values     map[string]interface{} // the match's extracted values (for #25 aliasing)
	rec        *smRecord
}

// smMatchInfo carries a match's submatch indices from the matching phase to
// the observer. idx is relative to offset base of the input.
type smMatchInfo struct {
	idx  []int
	base int
}

// submatchStrings is regexp's FindStringSubmatch built from the indices of
// FindStringSubmatchIndex, so parseGroup runs each regex once.
func submatchStrings(s string, loc []int) []string {
	out := make([]string, len(loc)/2)
	for i := range out {
		if loc[2*i] >= 0 {
			out[i] = s[loc[2*i]:loc[2*i+1]]
		}
	}
	return out
}

// beginInput starts a top-level group on one input's (processed) data.
func (o *smObserver) beginInput(input, data string) {
	o.input = input
	o.lineOffsets = o.lineOffsets[:0]
	o.lineOffsets = append(o.lineOffsets, 0)
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			o.lineOffsets = append(o.lineOffsets, i+1)
		}
	}
	o.lineOffsets = append(o.lineOffsets, len(data)+1)
}

func (o *smObserver) newRecord(group *compiler.CompiledGroup) *smRecord {
	return &smRecord{input: o.input, group: group}
}

// position turns an offset of the input into a line and column.
func (o *smObserver) position(abs int) (line, col int) {
	line = sort.SearchInts(o.lineOffsets, abs+1) - 1
	if line < 0 {
		line = 0
	}
	return line, abs - o.lineOffsets[line]
}

// recordTree creates the smRecord of one record of group and an smMatch for
// every match it consumed (used holds the indices into allMatches). A
// default start, which matched no text, has no smMatch.
func (o *smObserver) recordTree(group *compiler.CompiledGroup, allMatches []patternMatch, used []int) *smRecord {
	rec := o.newRecord(group)
	o.addMatches(rec, group, allMatches, used)
	return rec
}

// addMatches adds an smMatch to rec for each match of group at the indices
// used of allMatches.
func (o *smObserver) addMatches(rec *smRecord, group *compiler.CompiledGroup, allMatches []patternMatch, used []int) {
	for _, mi := range used {
		m := &allMatches[mi]
		if m.patternIdx >= len(group.Patterns) {
			continue
		}
		line, startCol := o.position(m.spanStart)
		sm := &smMatch{
			input:      o.input,
			line:       line,
			startCol:   startCol,
			endCol:     startCol + (m.spanEnd - m.spanStart),
			group:      group,
			patternIdx: m.patternIdx,
			values:     m.result,
			rec:        rec,
		}
		act := group.Profile().Pattern(m.patternIdx).Action
		sm.vars = o.varRanges(group.Patterns[m.patternIdx], m.sm, line, act == compiler.ActionAdd || act == compiler.ActionJoin)
		o.matches = append(o.matches, sm)
	}
}

// varRanges returns each captured variable's columns on line, mapping capture
// groups to variables the way extractMatchResult does. keeps is whether the
// line's action keeps its values (add or join): a WholeMatch line maps its
// variables to the whole match only then.
func (o *smObserver) varRanges(cp *pattern.CompiledPattern, info *smMatchInfo, line int, keeps bool) map[string]*VarRange {
	ranges := make(map[string]*VarRange)
	if info == nil {
		return ranges
	}
	lineStart := o.lineOffsets[line]
	varIndex := 1
	for _, varName := range cp.VariableOrder {
		v, ok := cp.Variables[varName]
		if !ok {
			continue
		}
		if cp.WholeMatch {
			// Each variable's value is the whole match (extractMatchTTP),
			// except an ignore variable's (none) and a set() one's.
			if keeps && varName != pattern.VarIgnore && !hasIgnoreFunction(v) && !v.HasSet && len(info.idx) >= 2 && info.idx[0] >= 0 {
				ranges[varName] = &VarRange{StartCol: info.idx[0] + info.base - lineStart, EndCol: info.idx[1] + info.base - lineStart}
			}
			continue
		}
		if pattern.IsNonCapturingVar(varName) {
			varIndex++
			continue
		}
		if varIndex*2+1 < len(info.idx) {
			s, e := info.idx[varIndex*2], info.idx[varIndex*2+1]
			if s >= 0 && e >= 0 {
				s += info.base - lineStart
				e += info.base - lineStart
				if s >= 0 && e >= 0 {
					ranges[varName] = &VarRange{StartCol: s, EndCol: e}
				}
			}
		}
		varIndex++
	}
	return ranges
}

// mapID identifies a map by its header pointer.
func mapID(m map[string]interface{}) uintptr {
	return reflect.ValueOf(m).Pointer()
}

// indexMaps records the path of every map inside v, keyed by identity. The
// first path found wins; keys are visited in sorted order so it's stable.
func indexMaps(v interface{}, path string, out map[uintptr]string) {
	switch t := v.(type) {
	case map[string]interface{}:
		if path != "" {
			if _, seen := out[mapID(t)]; !seen {
				out[mapID(t)] = path
			}
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := escapePathKey(k)
			if path != "" {
				p = path + "." + p
			}
			indexMaps(t[k], p, out)
		}
	case []interface{}:
		for i, e := range t {
			indexMaps(e, path+"["+strconv.Itoa(i)+"]", out)
		}
	case []map[string]interface{}:
		for i, e := range t {
			indexMaps(e, path+"["+strconv.Itoa(i)+"]", out)
		}
	}
}

// build turns the collected events into sourceMap. resultsFor returns the
// results map an input's records were stored in.
func (o *smObserver) build(sourceMap *SourceMap, resultsFor func(input string) map[string]interface{}) {
	tables := make(map[uintptr]map[uintptr]string)
	tableFor := func(input string) map[uintptr]string {
		results := resultsFor(input)
		if results == nil {
			return nil
		}
		id := mapID(results)
		t, ok := tables[id]
		if !ok {
			t = make(map[uintptr]string)
			indexMaps(results, "", t)
			tables[id] = t
		}
		return t
	}

	root := func(rec *smRecord) *smRecord {
		for rec != nil && rec.alias != nil {
			rec = rec.alias
		}
		return rec
	}
	paths := make(map[*smRecord]string)
	var pathOf func(rec *smRecord) string
	pathOf = func(rec *smRecord) string {
		rec = root(rec)
		if rec == nil || rec.dropped {
			return ""
		}
		if p, ok := paths[rec]; ok {
			return p
		}
		var p string
		found := false
		if rec.final != nil {
			if t := tableFor(rec.input); t != nil {
				p, found = t[mapID(rec.final)]
			}
		}
		if !found {
			switch {
			case rec.parent != nil:
				pp := pathOf(rec.parent)
				switch {
				case pp == "":
					p = ""
				case rec.rel == "":
					p = pp
				default:
					p = pp + "." + rec.rel
				}
			default:
				p = rec.rel
			}
		}
		paths[rec] = p
		return p
	}

	o.aliasMacroRenames(root)

	for _, m := range o.matches {
		in := sourceMap.Inputs[m.input]
		if in == nil || m.line < 0 || m.line >= len(in.Lines) {
			continue
		}
		lm := in.Lines[m.line]
		lm.Matched = true
		lm.Matches = append(lm.Matches, &MatchMapping{
			StartCol:     m.startCol,
			EndCol:       m.endCol,
			GroupName:    m.group.Name,
			PatternIndex: m.patternIdx,
			Variables:    m.vars,
			ResultPath:   pathOf(m.rec),
		})
	}
}

// aliasMacroRenames gives fields a group macro renamed the source span of the
// field they came from (#25). A field of the final record with no span of its
// own gets the span of a captured variable of the same record whose value is
// equal. It can't see what the macro did, so a renamed and transformed value
// stays without a span, and of two captured variables with the same value the
// first in name order wins.
func (o *smObserver) aliasMacroRenames(root func(*smRecord) *smRecord) {
	byRecord := make(map[*smRecord][]*smMatch)
	var order []*smRecord
	for _, m := range o.matches {
		if m.group.Macro == "" {
			continue
		}
		rec := root(m.rec)
		if rec == nil || rec.dropped || rec.final == nil {
			continue
		}
		if _, seen := byRecord[rec]; !seen {
			order = append(order, rec)
		}
		byRecord[rec] = append(byRecord[rec], m)
	}
	for _, rec := range order {
		matches := byRecord[rec]
		direct := make(map[string]bool)
		preValue := make(map[string]interface{})
		preOwner := make(map[string]*smMatch)
		for _, m := range matches {
			for varName := range m.vars {
				direct[varName] = true
				if v, ok := m.values[varName]; ok {
					preValue[varName] = v
					preOwner[varName] = m
				}
			}
		}
		origKeys := make([]string, 0, len(preValue))
		for k := range preValue {
			origKeys = append(origKeys, k)
		}
		sort.Strings(origKeys)
		newKeys := make([]string, 0, len(rec.final))
		for k := range rec.final {
			newKeys = append(newKeys, k)
		}
		sort.Strings(newKeys)
		for _, newKey := range newKeys {
			if direct[newKey] {
				continue
			}
			newVal := rec.final[newKey]
			for _, origKey := range origKeys {
				if origKey == newKey || !reflect.DeepEqual(preValue[origKey], newVal) {
					continue
				}
				owner := preOwner[origKey]
				if r, ok := owner.vars[origKey]; ok {
					owner.vars[newKey] = r
				}
				break
			}
		}
	}
}

// locateStored returns the paths of the last n values stored at keys (a
// dotted path split into keys, formatters removed) in results, just after
// they were stored there. A list met on the way is entered at its last
// element, which is where storeAtPath put the newest value. It is the
// fallback path of records whose maps were copied while being stored (a
// "**" path merges a record into an existing dict) or later (a Starlark
// macro on the parent), so they can't be found by identity.
func locateStored(results map[string]interface{}, keys []string, n int) []string {
	var b strings.Builder
	var cur interface{} = results
	for i, key := range keys {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(escapePathKey(key))
		cur = m[key]
		if l, ok := cur.([]interface{}); ok && len(l) > 0 {
			if i == len(keys)-1 {
				base := b.String()
				paths := make([]string, n)
				for j := 0; j < n; j++ {
					k := len(l) - n + j
					if k < 0 {
						k = 0
					}
					paths[j] = base + "[" + strconv.Itoa(k) + "]"
				}
				return paths
			}
			b.WriteString("[" + strconv.Itoa(len(l)-1) + "]")
			cur = l[len(l)-1]
		}
	}
	paths := make([]string, n)
	for j := range paths {
		paths[j] = b.String()
	}
	return paths
}

// locatePath is the path of the value just stored at keys in results (see
// locateStored), or "".
func locatePath(results map[string]interface{}, keys []string) string {
	if paths := locateStored(results, keys, 1); len(paths) == 1 {
		return paths[0]
	}
	return ""
}

// escapePathKey escapes a result key for a ResultPath: a "." or "*" in a key
// (a dynamic path value such as "lag-4:1629.*", #76) gets a "\" before it,
// as does a "\", so neither can be read as a separator or a path formatter.
func escapePathKey(k string) string {
	if !strings.ContainsAny(k, ".*\\") {
		return k
	}
	var b strings.Builder
	for i := 0; i < len(k); i++ {
		if c := k[i]; c == '.' || c == '*' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(k[i])
	}
	return b.String()
}

// segmentKeys returns the keys of resolved path segments. A "_" segment
// (TTP's null path) names no key: its records live in their parent.
func segmentKeys(parts []pathPart) []string {
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.key != "" && p.key != "_" {
			keys = append(keys, p.key)
		}
	}
	return keys
}
