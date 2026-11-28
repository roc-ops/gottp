package compiler

import (
	"regexp"
	"strings"

	"github.com/roc-ops/gottp/internal/pattern"
)

// PatternRole is the list Python TTP puts a group's pattern in when it splits
// the group's lines into start patterns (start_re), end patterns (end_re) and
// ordinary patterns (re).
type PatternRole uint8

const (
	// RoleOrdinary is TTP's group.re: the pattern only adds to the current record.
	RoleOrdinary PatternRole = iota
	// RoleStart is TTP's group.start_re: a match starts a new record.
	RoleStart
	// RoleEnd is TTP's group.end_re: a match ends the current record.
	RoleEnd
	// RoleNone marks a template line that TTP does not turn into a regex at
	// all: a line without {{ variables }}, or an unconditional set() line
	// (which only adds a group default). gottp still compiles it, but it
	// takes no part in the partition: it is never the "first line" and never
	// a start or end pattern.
	RoleNone
)

// PatternAction is TTP's per-line ACTION, which decides how a match is saved.
type PatternAction uint8

const (
	ActionAdd        PatternAction = iota // "add": ordinary variable line
	ActionStart                           // "start": start a record with this line's values
	ActionStartEmpty                      // "startempty": the line has a variable named _start_
	ActionEnd                             // "end": end the current record
	ActionJoin                            // "join": joinmatches or _line_
)

func (a PatternAction) String() string {
	switch a {
	case ActionStart:
		return "start"
	case ActionStartEmpty:
		return "startempty"
	case ActionEnd:
		return "end"
	case ActionJoin:
		return "join"
	default:
		return "add"
	}
}

func (r PatternRole) String() string {
	switch r {
	case RoleStart:
		return "start"
	case RoleEnd:
		return "end"
	case RoleNone:
		return "none"
	default:
		return "ordinary"
	}
}

// isStartAction mirrors TTP's `"start" in ACTION`.
func (a PatternAction) isStartAction() bool {
	return a == ActionStart || a == ActionStartEmpty
}

// PatternProfile is the indicator profile of one pattern of a group.
type PatternProfile struct {
	Role   PatternRole
	Action PatternAction
	// IsLine is TTP's IS_LINE: a variable on the line is _line_ or uses _line_.
	IsLine bool
	// Indicated is true when the line itself asks to start a record (_start_
	// or _headers_ on a variable, so TTP's ACTION was already start or
	// startempty before the partition), as opposed to a start pattern that
	// is one only because it is the group's first line or the group is a
	// table.
	Indicated bool
	// Tail is true for a line that follows a child group in the template.
	Tail bool
}

// JoinMatchesSpec describes a variable that collects every match (joinmatches).
type JoinMatchesSpec struct {
	Char      string // join character, "\n" by default
	HasToList bool   // to_list runs before joinmatches
}

// IndicatorProfile is a group's start/end/line classification, compiled once
// per group. It ports Python TTP 0.10.1's group-pattern partition
// (_group_class.get_regexes) rule for rule and is gottp's only definition of
// which patterns start a record, end one, or match any line. The runtime and
// the streamability analysis only query it.
//
// The rules, applied to each line in order (skipping RoleNone lines):
//  1. the line's ACTION is end → end pattern;
//  2. the line follows a child group (a tail line) → ordinary;
//  3. the group's first line → start pattern (ACTION becomes start);
//  4. method="table" → start pattern (ACTION becomes start);
//  5. the line's ACTION is start or startempty → start pattern;
//  6. otherwise → ordinary.
//
// A line's ACTION is the save action of its last variable, except that
// variables named ignore never change it and nothing changes it once it is
// start or startempty. A variable's save action is set by its name and then
// each function in order: _start_ → start (startempty when the variable
// itself is named _start_), _headers_ → start, _end_ → end, joinmatches and
// _line_ → join.
type IndicatorProfile struct {
	patterns []PatternProfile

	joinMatches map[string]JoinMatchesSpec

	hasStart, hasEnd, hasLine, hasIndicated, hasStartEmpty bool
}

// BuildIndicatorProfile computes the indicator profile of a group from its
// patterns and method.
func BuildIndicatorProfile(method string, patterns []*pattern.CompiledPattern) *IndicatorProfile {
	p := &IndicatorProfile{
		patterns:    make([]PatternProfile, len(patterns)),
		joinMatches: make(map[string]JoinMatchesSpec),
	}

	index := 0 // position among TTP regexes, i.e. excluding RoleNone lines
	for i, cp := range patterns {
		pp := lineProfile(cp)
		if pp.Role == RoleNone {
			p.patterns[i] = pp
			continue
		}
		switch {
		case pp.Action == ActionEnd:
			pp.Role = RoleEnd
		case pp.Tail:
			pp.Role = RoleOrdinary
		case index == 0, method == "table":
			if !pp.Action.isStartAction() {
				pp.Action = ActionStart
			}
			pp.Role = RoleStart
		case pp.Action.isStartAction():
			pp.Role = RoleStart
		default:
			pp.Role = RoleOrdinary
		}
		index++
		p.patterns[i] = pp

		p.hasStart = p.hasStart || pp.Role == RoleStart
		p.hasEnd = p.hasEnd || pp.Role == RoleEnd
		p.hasLine = p.hasLine || pp.IsLine
		p.hasIndicated = p.hasIndicated || pp.Indicated
		p.hasStartEmpty = p.hasStartEmpty || pp.Action == ActionStartEmpty
	}

	// joinmatches variables. A later pattern's settings for the same variable
	// name win, as they did when the runtime computed this per parse.
	for _, cp := range patterns {
		for varName, v := range cp.Variables {
			hasToList := false
			for _, funcStr := range v.Functions {
				if funcStr == "to_list" {
					hasToList = true
				}
				if strings.HasPrefix(funcStr, "joinmatches") {
					p.joinMatches[varName] = JoinMatchesSpec{Char: joinMatchesChar(funcStr), HasToList: hasToList}
					break
				}
			}
		}
	}
	return p
}

// variableSyntax matches one {{ variable }} on a template line.
var variableSyntax = regexp.MustCompile(`\{\{([\S\s]+?)\}\}`)

// joinMatchesChar extracts the join character from "joinmatches" or
// "joinmatches(',')"; the default is a newline.
func joinMatchesChar(funcStr string) string {
	start := strings.Index(funcStr, "(")
	end := strings.Index(funcStr, ")")
	if start >= 0 && end > start {
		if arg := strings.Trim(funcStr[start+1:end], "'\""); arg != "" {
			return arg
		}
	}
	return "\n"
}

// lineProfile computes a line's ACTION, IS_LINE and whether TTP compiles it at
// all, before the partition.
func lineProfile(cp *pattern.CompiledPattern) PatternProfile {
	pp := PatternProfile{Tail: cp.Tail}
	if !variableSyntax.MatchString(cp.Original) || isUnconditionalSet(cp) {
		pp.Role = RoleNone
		return pp
	}
	action := ActionAdd
	for _, name := range cp.VariableOrder {
		v := cp.Variables[name]
		if v == nil {
			continue
		}
		varAction, isLine := variableAction(v)
		pp.IsLine = pp.IsLine || isLine
		if !action.isStartAction() && v.Name != pattern.VarIgnore {
			action = varAction
		}
	}
	pp.Action = action
	pp.Indicated = action.isStartAction()
	return pp
}

// variableAction is TTP's _variable_class SAVEACTION and IS_LINE.
func variableAction(v *pattern.MatchVariable) (PatternAction, bool) {
	action := ActionAdd
	isLine := false
	apply := func(name string) {
		switch name {
		case pattern.IndicatorStart:
			if v.Name == pattern.IndicatorStart {
				action = ActionStartEmpty
			} else {
				action = ActionStart
			}
		case pattern.IndicatorHeaders:
			action = ActionStart
		case pattern.IndicatorEnd:
			action = ActionEnd
		case "joinmatches":
			action = ActionJoin
		case pattern.IndicatorLine:
			action = ActionJoin
			isLine = true
		}
	}
	apply(v.Name)
	for _, f := range v.Functions {
		apply(functionName(f))
	}
	return action, isLine
}

// functionName returns the name of a function call string such as
// `joinmatches(",")`, following TTP's get_attributes.
func functionName(f string) string {
	f = strings.TrimSpace(f)
	if i := strings.IndexByte(f, '('); i >= 0 {
		f = f[:i]
	}
	return strings.TrimSpace(f)
}

// isUnconditionalSet reports whether the line is only set() variables with no
// text to match, which TTP turns into group defaults instead of a regex.
func isUnconditionalSet(cp *pattern.CompiledPattern) bool {
	if strings.TrimSpace(variableSyntax.ReplaceAllString(cp.Original, "")) != "" {
		return false
	}
	for _, v := range cp.Variables {
		for _, f := range v.Functions {
			if functionName(f) == "set" {
				return true
			}
		}
	}
	return false
}

// Len is the number of patterns profiled.
func (p *IndicatorProfile) Len() int { return len(p.patterns) }

// Pattern returns the profile of pattern i (the zero PatternProfile, an
// ordinary pattern, when i is out of range).
func (p *IndicatorProfile) Pattern(i int) PatternProfile {
	if i < 0 || i >= len(p.patterns) {
		return PatternProfile{}
	}
	return p.patterns[i]
}

// IsStart reports whether pattern i is a start pattern (TTP's start_re).
func (p *IndicatorProfile) IsStart(i int) bool { return p.Pattern(i).Role == RoleStart }

// IsEnd reports whether pattern i is an end pattern (TTP's end_re).
func (p *IndicatorProfile) IsEnd(i int) bool { return p.Pattern(i).Role == RoleEnd }

// IsStartEmpty reports whether pattern i has a variable named _start_
// (TTP's "startempty" action).
func (p *IndicatorProfile) IsStartEmpty(i int) bool {
	return p.Pattern(i).Action == ActionStartEmpty
}

// IsIndicatedStart reports whether pattern i asks to start a record itself
// (_start_ or _headers_), rather than only by being the first line or a
// table line.
func (p *IndicatorProfile) IsIndicatedStart(i int) bool { return p.Pattern(i).Indicated }

// IsLine reports whether pattern i matches any line (TTP's IS_LINE).
func (p *IndicatorProfile) IsLine(i int) bool { return p.Pattern(i).IsLine }

// HasStart reports whether the group has any start pattern.
func (p *IndicatorProfile) HasStart() bool { return p.hasStart }

// HasEnd reports whether the group has any end pattern.
func (p *IndicatorProfile) HasEnd() bool { return p.hasEnd }

// HasLine reports whether any pattern of the group is a _line_ pattern.
func (p *IndicatorProfile) HasLine() bool { return p.hasLine }

// HasIndicatedStart reports whether any pattern asks to start a record itself.
func (p *IndicatorProfile) HasIndicatedStart() bool { return p.hasIndicated }

// HasStartEmpty reports whether any pattern has a variable named _start_.
func (p *IndicatorProfile) HasStartEmpty() bool { return p.hasStartEmpty }

// JoinMatches returns the joinmatches variables of the group by name. The map
// is shared; callers must not modify it.
func (p *IndicatorProfile) JoinMatches() map[string]JoinMatchesSpec { return p.joinMatches }

// IsJoinMatches reports whether the variable collects every match.
func (p *IndicatorProfile) IsJoinMatches(varName string) bool {
	_, ok := p.joinMatches[varName]
	return ok
}
