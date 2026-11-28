package yang

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/openconfig/goyang/pkg/yang"
)

// YANG pattern statements (RFC 7950 9.4.5) are XML Schema regular
// expressions (XSD 1.0 Part 2, Appendix F). They differ from Go's RE2:
//
//   - a pattern matches the whole value; there are no anchors, so ^ and $
//     are ordinary characters;
//   - "." excludes only \n and \r;
//   - \d, \s and \w are Unicode-aware (\w is everything except punctuation,
//     separators and "other" characters, so it excludes '_');
//   - XSD has \i, \c, Unicode block escapes (\p{IsBasicLatin}) and character
//     class subtraction ([a-z-[aeiou]]), which RE2 can't express.
//
// xsdToRE2 translates the subset RE2 can express and returns an error
// naming the construct otherwise.

// Replacements for XSD multi-character escapes. "in" is the text to use
// inside a positive character class; complement is set when the escape is
// the complement of a set, which RE2 can't put inside another class.
var xsdClassEscapes = map[byte]struct {
	out        string // replacement outside a character class
	in         string // set members, for use inside a class
	complement bool   // in is the set to exclude, not include
}{
	'd': {`\p{Nd}`, `\p{Nd}`, false},
	'D': {`\P{Nd}`, `\P{Nd}`, false},
	's': {`[ \t\n\r]`, ` \t\n\r`, false},
	'S': {`[^ \t\n\r]`, ` \t\n\r`, true},
	'w': {`[^\p{P}\p{Z}\p{C}]`, `\p{P}\p{Z}\p{C}`, true},
	'W': {`[\p{P}\p{Z}\p{C}]`, `\p{P}\p{Z}\p{C}`, false},
}

// XSD single-character escapes (\n, \r, \t and the metacharacters).
const xsdSingleEscapes = `nrt\|.-^?*+{}()[]`

// xsdToRE2 translates an XSD regular expression to an anchored Go regexp.
func xsdToRE2(p string) (string, error) {
	var b strings.Builder
	b.WriteString(`\A(?:`)
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch c {
		case '\\':
			if i+1 >= len(p) {
				return "", fmt.Errorf("trailing backslash")
			}
			esc, n, err := translateEscape(p[i+1:], false)
			if err != nil {
				return "", err
			}
			b.WriteString(esc.out)
			i += n
		case '[':
			cls, n, err := translateClass(p[i:])
			if err != nil {
				return "", err
			}
			b.WriteString(cls)
			i += n - 1
		case '.':
			b.WriteString(`[^\n\r]`)
		case '^', '$':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '(':
			if i+1 < len(p) && p[i+1] == '?' {
				return "", fmt.Errorf("'(?' is not XSD regex syntax")
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteString(`)\z`)
	return b.String(), nil
}

type translatedEscape struct {
	out        string
	in         string
	complement bool
}

// translateEscape translates the escape that follows a backslash. s starts
// after the backslash; n is the number of bytes of s consumed.
func translateEscape(s string, inClass bool) (translatedEscape, int, error) {
	c := s[0]
	if e, ok := xsdClassEscapes[c]; ok {
		return translatedEscape{e.out, e.in, e.complement}, 1, nil
	}
	if strings.IndexByte(xsdSingleEscapes, c) >= 0 {
		esc := `\` + string(c)
		return translatedEscape{esc, esc, false}, 1, nil
	}
	switch c {
	case 'p', 'P':
		end := strings.IndexByte(s, '}')
		if len(s) < 2 || s[1] != '{' || end < 0 {
			return translatedEscape{}, 0, fmt.Errorf("malformed \\%c escape", c)
		}
		name := s[2:end]
		if strings.HasPrefix(name, "Is") {
			return translatedEscape{}, 0, fmt.Errorf("block escape \\%c{%s} (a Unicode block) is not supported by RE2", c, name)
		}
		esc := s[:end+1]
		return translatedEscape{`\` + esc, `\` + esc, false}, end + 1, nil
	case 'i', 'I', 'c', 'C':
		return translatedEscape{}, 0, fmt.Errorf("XML name escape \\%c is not supported by RE2", c)
	}
	return translatedEscape{}, 0, fmt.Errorf("unknown escape \\%c", c)
}

// translateClass translates the character class at the start of s and
// returns the number of bytes consumed.
func translateClass(s string) (string, int, error) {
	i := 1
	negated := false
	if i < len(s) && s[i] == '^' {
		negated = true
		i++
	}
	var members strings.Builder
	var complements []string
	for first := true; ; first = false {
		if i >= len(s) {
			return "", 0, fmt.Errorf("unterminated character class")
		}
		c := s[i]
		switch {
		case c == ']' && !first:
			i++
			return buildClass(negated, members.String(), complements, s[:i])
		case c == '-' && i+1 < len(s) && s[i+1] == '[':
			return "", 0, fmt.Errorf("character class subtraction is not supported by RE2")
		case c == '\\':
			if i+1 >= len(s) {
				return "", 0, fmt.Errorf("trailing backslash")
			}
			esc, n, err := translateEscape(s[i+1:], true)
			if err != nil {
				return "", 0, err
			}
			if esc.complement {
				complements = append(complements, esc.in)
			} else {
				members.WriteString(esc.in)
			}
			i += 1 + n
		case c == '[' || c == '^':
			// Literal in XSD; escaped so RE2 doesn't read [:alpha:].
			members.WriteByte('\\')
			members.WriteByte(c)
			i++
		default:
			members.WriteByte(c)
			i++
		}
	}
}

func buildClass(negated bool, members string, complements []string, src string) (string, int, error) {
	if len(complements) == 0 {
		if negated {
			return "[^" + members + "]", len(src), nil
		}
		return "[" + members + "]", len(src), nil
	}
	if negated {
		// [^a\w] is (not a) and (not \w): an intersection RE2 can't write.
		return "", 0, fmt.Errorf("negated character class %s with \\S or \\w is not supported by RE2", src)
	}
	alts := make([]string, 0, len(complements)+1)
	if members != "" {
		alts = append(alts, "["+members+"]")
	}
	for _, c := range complements {
		alts = append(alts, "[^"+c+"]")
	}
	return "(?:" + strings.Join(alts, "|") + ")", len(src), nil
}

// compiledPattern is a translated, compiled YANG pattern, or the reason it
// can't be checked.
type compiledPattern struct {
	re  *regexp.Regexp
	err error
}

var patternCache sync.Map // XSD pattern -> *compiledPattern

func compilePattern(p string) *compiledPattern {
	if cp, ok := patternCache.Load(p); ok {
		return cp.(*compiledPattern)
	}
	cp := &compiledPattern{}
	if expr, err := xsdToRE2(p); err != nil {
		cp.err = err
	} else if cp.re, err = regexp.Compile(expr); err != nil {
		cp.err = fmt.Errorf("RE2 rejects the translation: %v", err)
	}
	actual, _ := patternCache.LoadOrStore(p, cp)
	return actual.(*compiledPattern)
}

// patternFailure is a pattern a value failed, for an error message.
type patternFailure struct {
	pattern string
	invert  bool
}

// checkPatterns checks s against every pattern of yangType (RFC 7950
// 9.4.5: all must hold). It returns the first pattern that fails. Patterns
// that can't be checked are skipped; uncheckablePatterns reports them.
func (v *Validator) checkPatterns(s string, yangType *yang.YangType) (patternFailure, bool) {
	for _, p := range yangType.Pattern {
		cp := compilePattern(p)
		if cp.err != nil {
			continue
		}
		invert, ambiguous := v.moduleSet.patternModifier(p)
		if ambiguous {
			continue
		}
		if cp.re.MatchString(s) == invert {
			return patternFailure{p, invert}, false
		}
	}
	return patternFailure{}, true
}

// uncheckablePatterns describes the patterns of yangType (and of its union
// members) that can't be checked.
func (v *Validator) uncheckablePatterns(yangType *yang.YangType) []string {
	if yangType == nil {
		return nil
	}
	var out []string
	for _, p := range yangType.Pattern {
		if cp := compilePattern(p); cp.err != nil {
			out = append(out, fmt.Sprintf("pattern '%s' cannot be checked: %v", p, cp.err))
		} else if _, ambiguous := v.moduleSet.patternModifier(p); ambiguous {
			out = append(out, fmt.Sprintf("pattern '%s' cannot be checked: the loaded modules use it both with and without 'modifier invert-match'", p))
		}
	}
	for _, member := range yangType.Type {
		out = append(out, v.uncheckablePatterns(member)...)
	}
	return out
}

// collectPatternModifiers records which pattern strings carry
// "modifier invert-match". goyang flattens a type's patterns to strings and
// drops the modifier, so it is recovered from the parsed statements.
func collectPatternModifiers(mods map[string]*yang.Module, into map[string]patternUse) {
	var walk func(s *yang.Statement)
	walk = func(s *yang.Statement) {
		if s == nil {
			return
		}
		if s.Keyword == "pattern" {
			use := patternPlain
			for _, sub := range s.SubStatements() {
				if sub.Keyword == "modifier" && sub.Argument == "invert-match" {
					use = patternInverted
				}
			}
			into[s.Argument] |= use
		}
		for _, sub := range s.SubStatements() {
			walk(sub)
		}
	}
	for _, m := range mods {
		walk(m.Source)
	}
}

// patternUse records how a pattern string is used across the loaded modules.
type patternUse uint8

const (
	patternPlain    patternUse = 1 << iota // used without invert-match
	patternInverted                        // used with invert-match
)
