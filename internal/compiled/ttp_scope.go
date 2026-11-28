package compiled

import (
	"errors"

	"github.com/roc-ops/gottp/internal/compiler"
)

// pyRaise is a Python exception a match variable's type method raised
// (matchTypeMethod). TTP catches only AttributeError there, so it fails
// TTP's parse, but only if TTP evaluates the match at all: see runReScope.
type pyRaise struct{ err error }

func (p *pyRaise) Error() string { return p.err.Error() }
func (p *pyRaise) Unwrap() error { return p.err }

// asPyRaise returns err's Python exception, or nil.
func asPyRaise(err error) error {
	var p *pyRaise
	if errors.As(err, &p) {
		return p.err
	}
	return nil
}

// runReScope decides which of a group's type-method raises fail the parse
// when the group's patterns run on the whole input: parseFlat and
// parseGroupStream, unlike tree.go's runRe, don't narrow the region each
// pattern runs on as Python TTP's run_re does. A match's functions run
// only where run_re runs its regex, so a raise counts only there:
//
//   - start patterns run on the whole input;
//   - nothing else runs if no start pattern matches;
//   - end patterns run from the first start match on;
//   - ordinary patterns run from the first start match up to the line of
//     the last end match (the region narrows to end there, and TTP's
//     regexes need the newline after a line, so that line is outside too).
//
// The region arithmetic is tree.go's runRe's (TTP's s, se and e), for a top
// group. The scope sees every regex match of a start or end pattern, before
// conditions, as run_re does, and every raise. Of the raises that count, it
// reports the one TTP meets first: start patterns, then end patterns, then
// ordinary ones, each in template order, and in input order within one.
type runReScope struct {
	prof *compiler.IndicatorProfile
	// found is set by the first start match, at input offset s.
	found bool
	s     int
	// last is each start and end pattern's last regex match; ok marks a
	// pattern that matched.
	last []scopeSpan
	// raises holds the raises seen (see compact).
	raises []scopeRaise
	// certain is set once a raise is known to count (compact).
	certain bool
}

type scopeSpan struct {
	start, end int
	ok         bool
}

type scopeRaise struct {
	pattern, pos int
	err          error
}

func newRunReScope(group *compiler.CompiledGroup) *runReScope {
	n := len(group.Patterns)
	return &runReScope{prof: group.Profile(), last: make([]scopeSpan, n)}
}

func (sc *runReScope) role(pi int) compiler.PatternRole { return sc.prof.Pattern(pi).Role }

// observe records a regex match of pattern pi at [start, end) (input
// offsets) and the exception its functions raised, if any.
func (sc *runReScope) observe(pi, start, end int, raise error) {
	if role := sc.role(pi); role == compiler.RoleStart || role == compiler.RoleEnd {
		sc.last[pi] = scopeSpan{start, end, true}
		if role == compiler.RoleStart && (!sc.found || start < sc.s) {
			sc.found = true
			sc.s = start
		}
	}
	if raise != nil {
		sc.raises = append(sc.raises, scopeRaise{pi, start, raise})
	}
}

// compact is parseGroupStream's, after each line: it keeps memory bounded
// by keeping, for each pattern, only the first raise that may count, and
// sets certain once one will. Lines come in input order, so once a start
// line is seen the region's start is known.
func (sc *runReScope) compact() {
	if len(sc.raises) == 0 {
		return
	}
	kept := sc.raises[:0]
	for _, ra := range sc.raises {
		role := sc.role(ra.pattern)
		if role != compiler.RoleStart && !sc.found {
			continue // before the first start line: never evaluated
		}
		dup := false
		for _, k := range kept {
			if k.pattern == ra.pattern {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		kept = append(kept, ra)
		// An ordinary pattern's raise counts unless an end match past it
		// narrows the region, which only the rest of the input tells.
		if (role != compiler.RoleOrdinary && role != compiler.RoleNone) || !sc.prof.HasEnd() {
			sc.certain = true
		}
	}
	sc.raises = kept
}

// err returns the raise that fails TTP's parse, or nil.
func (sc *runReScope) err() error {
	if len(sc.raises) == 0 {
		return nil
	}
	n := sc.prof.Len()
	if e := sc.firstRaise(n, func(pi int, _ int) bool { return sc.role(pi) == compiler.RoleStart }); e != nil {
		return e
	}
	if !sc.found {
		return nil
	}
	if e := sc.firstRaise(n, func(pi, pos int) bool { return sc.role(pi) == compiler.RoleEnd && pos >= sc.s }); e != nil {
		return e
	}
	endLine, narrowed := sc.endLine()
	return sc.firstRaise(n, func(pi, pos int) bool {
		role := sc.role(pi)
		return (role == compiler.RoleOrdinary || role == compiler.RoleNone) && pos >= sc.s && (!narrowed || pos < endLine)
	})
}

// firstRaise returns, for the first pattern (in template order) with a
// raise that in accepts, its earliest such raise.
func (sc *runReScope) firstRaise(n int, in func(pi, pos int) bool) error {
	for pi := 0; pi < n; pi++ {
		var best *scopeRaise
		for i := range sc.raises {
			ra := &sc.raises[i]
			if ra.pattern == pi && in(pi, ra.pos) && (best == nil || ra.pos < best.pos) {
				best = ra
			}
		}
		if best != nil {
			return best.err
		}
	}
	return nil
}

// endLine is run_re's narrowing of the region to the last end match: the
// input offset of the line it ends at, and whether it narrows at all.
func (sc *runReScope) endLine() (int, bool) {
	line, _, ok := sc.narrow()
	return line, ok
}

// narrow ports run_re's s, se and e for a top group. TTP's line regexes
// start with the newline before the line, so in DATATEXT ("\n" + data) a
// match of a line starting at input offset L starts at L and ends at its
// input end + 1. se, an absolute offset, is compared with relative ones as
// TTP does. It returns the input offset of the line of the last end match
// that narrows the region, the region's new end (DATATEXT), and whether it
// narrows at all.
func (sc *runReScope) narrow() (line, end int, ok bool) {
	if !sc.found {
		return 0, 0, false
	}
	start := sc.s
	se := 0
	for pi, l := range sc.last {
		if !l.ok || sc.role(pi) != compiler.RoleStart {
			continue
		}
		if se < l.start && !sc.prof.IsLine(pi) {
			se = l.end + 1
		}
	}
	e := -1
	for pi, l := range sc.last {
		if !l.ok || sc.role(pi) != compiler.RoleEnd || l.start < start {
			continue
		}
		if lastEnd := l.end + 1 - start; e < lastEnd && lastEnd > se {
			e, line = lastEnd, l.start
		}
	}
	if e == -1 {
		return 0, 0, false
	}
	return line, start + e, true
}

// cutKeep returns, when run_re narrows the region to the last end match,
// which of the group's matches it keeps; nil when it doesn't narrow. As in
// run_re, a start match is dropped when it starts past the new end (so the
// line right after the end line is kept: its match starts at the end
// line's newline, which is the end), and an ordinary match is kept only
// when it ends before the end, since TTP's regexes need the newline after
// the line. End matches are all kept. pi is the pattern, start and end the
// match's input offsets. parseFlat and parseGroupStream apply it (#139).
func (sc *runReScope) cutKeep() func(pi, start, end int) bool {
	_, cut, ok := sc.narrow()
	if !ok {
		return nil
	}
	return func(pi, start, end int) bool {
		switch sc.role(pi) {
		case compiler.RoleStart:
			return start <= cut
		case compiler.RoleEnd:
			return true
		default:
			return end+1 < cut
		}
	}
}
