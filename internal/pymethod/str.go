package pymethod

import (
	"strings"
	"unicode/utf8"
)

// strMethods are Python 3.12's str methods, except encode (bytes have no
// JSON form). Strings are sequences of code points, as in Python: indexes
// and widths count runes.
var strMethods = table{
	"capitalize":   {fn: noArgStr(pyCapitalize)},
	"casefold":     {fn: noArgStr(pyCasefold)},
	"lower":        {fn: noArgStr(pyLower)},
	"upper":        {fn: noArgStr(pyUpper)},
	"swapcase":     {fn: noArgStr(pySwapcase)},
	"title":        {fn: noArgStr(pyTitle)},
	"isalnum":      {fn: allRunes(isAlnum)},
	"isalpha":      {fn: allRunes(isAlpha)},
	"isdecimal":    {fn: allRunes(isDecimal)},
	"isdigit":      {fn: allRunes(isDigit)},
	"isnumeric":    {fn: allRunes(isNumeric)},
	"isspace":      {fn: allRunes(isSpace)},
	"isascii":      {fn: strIsascii},
	"isprintable":  {fn: strIsprintable},
	"isidentifier": {fn: strIsidentifier},
	"islower":      {fn: strIslower},
	"isupper":      {fn: strIsupper},
	"istitle":      {fn: strIstitle},
	"center":       {fn: strPad('c')},
	"ljust":        {fn: strPad('l')},
	"rjust":        {fn: strPad('r')},
	"zfill":        {fn: strZfill},
	"count":        {fn: strCount},
	"find":         {fn: strFind(false, false)},
	"rfind":        {fn: strFind(true, false)},
	"index":        {fn: strFind(false, true)},
	"rindex":       {fn: strFind(true, true)},
	"startswith":   {fn: strTailmatch(false)},
	"endswith":     {fn: strTailmatch(true)},
	"strip":        {fn: strStrip(true, true)},
	"lstrip":       {fn: strStrip(true, false)},
	"rstrip":       {fn: strStrip(false, true)},
	"removeprefix": {fn: strRemove(false)},
	"removesuffix": {fn: strRemove(true)},
	"replace":      {fn: strReplace},
	"split":        {fn: strSplit(false)},
	"rsplit":       {fn: strSplit(true)},
	"splitlines":   {fn: strSplitlines},
	"partition":    {fn: strPartition(false)},
	"rpartition":   {fn: strPartition(true)},
	"expandtabs":   {fn: strExpandtabs},
	"join":         {fn: strJoin},
	"format":       {fn: strFormat},
	"format_map":   {fn: strFormatMap},
	"maketrans":    {fn: strMaketrans},
	"translate":    {fn: strTranslate},
}

func noArgStr(f func(string) string) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		return f(self.(string)), nil
	}
}

// allRunes is an is* test: true if s isn't empty and every character
// passes.
func allRunes(f func(rune) bool) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		if err := a.none(); err != nil {
			return nil, err
		}
		s := self.(string)
		if s == "" {
			return false, nil
		}
		for _, r := range s {
			if !f(r) {
				return false, nil
			}
		}
		return true, nil
	}
}

func strIsascii(self interface{}, a *args) (interface{}, error) {
	if err := a.none(); err != nil {
		return nil, err
	}
	for _, r := range self.(string) {
		if r >= 0x80 {
			return false, nil
		}
	}
	return true, nil
}

func strIsprintable(self interface{}, a *args) (interface{}, error) {
	if err := a.none(); err != nil {
		return nil, err
	}
	for _, r := range self.(string) {
		if !isPrintable(r) {
			return false, nil
		}
	}
	return true, nil
}

func strIsidentifier(self interface{}, a *args) (interface{}, error) {
	if err := a.none(); err != nil {
		return nil, err
	}
	s := self.(string)
	if s == "" {
		return false, nil
	}
	for i, r := range []rune(s) {
		if i == 0 {
			if r != '_' && !xidStartTable.has(r) {
				return false, nil
			}
		} else if !xidContinueTable.has(r) {
			return false, nil
		}
	}
	return true, nil
}

func strIslower(self interface{}, a *args) (interface{}, error) {
	if err := a.none(); err != nil {
		return nil, err
	}
	cased := false
	for _, r := range self.(string) {
		if isUpper(r) || isTitle(r) {
			return false, nil
		}
		if isLower(r) {
			cased = true
		}
	}
	return cased, nil
}

func strIsupper(self interface{}, a *args) (interface{}, error) {
	if err := a.none(); err != nil {
		return nil, err
	}
	cased := false
	for _, r := range self.(string) {
		if isLower(r) || isTitle(r) {
			return false, nil
		}
		if isUpper(r) {
			cased = true
		}
	}
	return cased, nil
}

func strIstitle(self interface{}, a *args) (interface{}, error) {
	if err := a.none(); err != nil {
		return nil, err
	}
	cased, prevCased := false, false
	for _, r := range self.(string) {
		switch {
		case isUpper(r) || isTitle(r):
			if prevCased {
				return false, nil
			}
			prevCased, cased = true, true
		case isLower(r):
			if !prevCased {
				return false, nil
			}
			prevCased, cased = true, true
		default:
			prevCased = false
		}
	}
	return cased, nil
}

// fillChar reads the fill character argument of center, ljust and rjust.
func fillChar(v interface{}, a *args) (rune, error) {
	s, ok := v.(string)
	if !ok {
		return 0, raise("TypeError", "%s() argument 2 must be a unicode character, not %s", a.name, typeName(v))
	}
	if utf8.RuneCountInString(s) != 1 {
		return 0, raise("TypeError", "The fill character must be exactly one character long")
	}
	r, _ := utf8.DecodeRuneInString(s)
	return r, nil
}

func strPad(how byte) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		if err := a.positional(1, 2); err != nil {
			return nil, err
		}
		s := self.(string)
		width, err := index(a.pos[0])
		if err != nil {
			return nil, err
		}
		fill := ' '
		if len(a.pos) > 1 {
			if fill, err = fillChar(a.pos[1], a); err != nil {
				return nil, err
			}
		}
		n := utf8.RuneCountInString(s)
		if width <= n {
			return s, nil
		}
		marg := width - n
		var left int
		switch how {
		case 'c':
			// Objects/unicodeobject.c: left = marg / 2 + (marg & width & 1)
			left = marg/2 + (marg & width & 1)
		case 'l':
			left = 0
		case 'r':
			left = marg
		}
		f := string(fill)
		return strings.Repeat(f, left) + s + strings.Repeat(f, marg-left), nil
	}
}

func strZfill(self interface{}, a *args) (interface{}, error) {
	if err := a.positional(1, 1); err != nil {
		return nil, err
	}
	width, err := index(a.pos[0])
	if err != nil {
		return nil, err
	}
	s := self.(string)
	n := utf8.RuneCountInString(s)
	if width <= n {
		return s, nil
	}
	pad := strings.Repeat("0", width-n)
	if s != "" && (s[0] == '+' || s[0] == '-') {
		return s[:1] + pad + s[1:], nil
	}
	return pad + s, nil
}

// adjustIndices is CPython's ADJUST_INDICES: slice indexes clamped to a
// sequence of length n. A start past the end stays past it.
func adjustIndices(start, end, n int) (int, int) {
	if end > n {
		end = n
	} else if end < 0 {
		end += n
		if end < 0 {
			end = 0
		}
	}
	if start < 0 {
		start += n
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

// subArgs reads (sub[, start[, end]]) of count, find and the like, and
// returns s and sub as runes with the adjusted slice.
func subArgs(self interface{}, a *args) (s, sub []rune, start, end int, err error) {
	if err = a.positional(1, 3); err != nil {
		return
	}
	subStr, ok := a.pos[0].(string)
	if !ok {
		err = raise("TypeError", "must be str, not %s", typeName(a.pos[0]))
		return
	}
	s, sub = []rune(self.(string)), []rune(subStr)
	if start, err = optIndex(a.pos, 1, 0); err != nil {
		return
	}
	if end, err = optIndex(a.pos, 2, len(s)); err != nil {
		return
	}
	start, end = adjustIndices(start, end, len(s))
	return
}

// optIndex reads the optional slice index pos[i], def if absent or None.
func optIndex(pos []interface{}, i, def int) (int, error) {
	if i >= len(pos) {
		return def, nil
	}
	return sliceIndex(pos[i], true, def)
}

func runesIndex(s, sub []rune, from int) int {
	for i := from; i+len(sub) <= len(s); i++ {
		if runesEqual(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func strCount(self interface{}, a *args) (interface{}, error) {
	s, sub, start, end, err := subArgs(self, a)
	if err != nil {
		return nil, err
	}
	if end-start < len(sub) || start > len(s) {
		return 0, nil
	}
	if len(sub) == 0 {
		return end - start + 1, nil
	}
	n := 0
	w := s[:end]
	for i := start; i+len(sub) <= len(w); {
		if runesEqual(w[i:i+len(sub)], sub) {
			n++
			i += len(sub)
		} else {
			i++
		}
	}
	return n, nil
}

func strFind(reverse, raises bool) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		s, sub, start, end, err := subArgs(self, a)
		if err != nil {
			return nil, err
		}
		pos := -1
		if end-start >= len(sub) {
			if reverse {
				for i := end - len(sub); i >= start; i-- {
					if runesEqual(s[i:i+len(sub)], sub) {
						pos = i
						break
					}
				}
			} else {
				pos = runesIndex(s[:end], sub, start)
			}
		}
		if pos < 0 && raises {
			return nil, raise("ValueError", "substring not found")
		}
		return pos, nil
	}
}

// strTailmatch is startswith and endswith: prefix is a str or a tuple of
// them.
func strTailmatch(end bool) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		if err := a.positional(1, 3); err != nil {
			return nil, err
		}
		s := []rune(self.(string))
		start, err := optIndex(a.pos, 1, 0)
		if err != nil {
			return nil, err
		}
		stop, err := optIndex(a.pos, 2, len(s))
		if err != nil {
			return nil, err
		}
		var subs []string
		switch p := a.pos[0].(type) {
		case string:
			subs = []string{p}
		case []interface{}:
			if !a.isTuple(0) {
				return nil, raise("TypeError", "%s first arg must be str or a tuple of str, not list", a.name)
			}
			for _, item := range p {
				str, ok := item.(string)
				if !ok {
					return nil, raise("TypeError", "tuple for %s must only contain str, not %s", a.name, typeName(item))
				}
				subs = append(subs, str)
			}
		default:
			return nil, raise("TypeError", "%s first arg must be str or a tuple of str, not %s", a.name, typeName(a.pos[0]))
		}
		start, stop = adjustIndices(start, stop, len(s))
		for _, sub := range subs {
			r := []rune(sub)
			e := stop - len(r)
			if e < start {
				continue
			}
			if end {
				if runesEqual(s[e:stop], r) {
					return true, nil
				}
			} else if runesEqual(s[start:start+len(r)], r) {
				return true, nil
			}
		}
		return false, nil
	}
}

// stripChars reads strip's chars argument: nil for whitespace.
func stripChars(a *args) (*string, error) {
	if err := a.positional(0, 1); err != nil {
		return nil, err
	}
	if len(a.pos) == 0 || a.pos[0] == nil {
		return nil, nil
	}
	s, ok := a.pos[0].(string)
	if !ok {
		return nil, raise("TypeError", "%s arg must be None or str", a.name)
	}
	return &s, nil
}

func strStrip(left, right bool) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		chars, err := stripChars(a)
		if err != nil {
			return nil, err
		}
		strip := isSpace
		if chars != nil {
			set := *chars
			strip = func(r rune) bool { return strings.ContainsRune(set, r) }
		}
		s := self.(string)
		if left {
			s = strings.TrimLeftFunc(s, strip)
		}
		if right {
			s = strings.TrimRightFunc(s, strip)
		}
		return s, nil
	}
}

func strRemove(suffix bool) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		if err := a.positional(1, 1); err != nil {
			return nil, err
		}
		x, ok := a.pos[0].(string)
		if !ok {
			return nil, raise("TypeError", "%s() argument must be str, not %s", a.name, typeName(a.pos[0]))
		}
		s := self.(string)
		if suffix {
			return strings.TrimSuffix(s, x), nil
		}
		return strings.TrimPrefix(s, x), nil
	}
}

func strReplace(self interface{}, a *args) (interface{}, error) {
	// replace(old, new, count=-1, /): count became a keyword in 3.13.
	if err := a.positional(2, 3); err != nil {
		return nil, err
	}
	old, err := a.str(a.pos[0], "1")
	if err != nil {
		return nil, err
	}
	nw, err := a.str(a.pos[1], "2")
	if err != nil {
		return nil, err
	}
	count := -1
	if len(a.pos) > 2 {
		if count, err = index(a.pos[2]); err != nil {
			return nil, err
		}
	}
	return strings.Replace(self.(string), old, nw, count), nil
}

// splitArgs reads (sep=None, maxsplit=-1) of split and rsplit.
func splitArgs(a *args) (sep *string, maxsplit int, err error) {
	vals, given, err := a.keywords("sep", "maxsplit")
	if err != nil {
		return nil, 0, err
	}
	maxsplit = -1
	if given[0] && vals[0] != nil {
		s, ok := vals[0].(string)
		if !ok {
			return nil, 0, raise("TypeError", "must be str or None, not %s", typeName(vals[0]))
		}
		if s == "" {
			return nil, 0, raise("ValueError", "empty separator")
		}
		sep = &s
	}
	if given[1] {
		if maxsplit, err = index(vals[1]); err != nil {
			return nil, 0, err
		}
	}
	return sep, maxsplit, nil
}

func strSplit(reverse bool) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		sep, maxsplit, err := splitArgs(a)
		if err != nil {
			return nil, err
		}
		s := self.(string)
		var parts []string
		switch {
		case sep != nil && !reverse:
			parts = strings.SplitN(s, *sep, splitN(maxsplit))
		case sep != nil:
			parts = rsplitSep(s, *sep, maxsplit)
		case !reverse:
			parts = splitSpace(s, maxsplit)
		default:
			parts = rsplitSpace(s, maxsplit)
		}
		return strList(parts), nil
	}
}

func splitN(maxsplit int) int {
	if maxsplit < 0 {
		return -1
	}
	return maxsplit + 1
}

func rsplitSep(s, sep string, maxsplit int) []string {
	var rev []string
	for maxsplit != 0 {
		i := strings.LastIndex(s, sep)
		if i < 0 {
			break
		}
		rev = append(rev, s[i+len(sep):])
		s = s[:i]
		if maxsplit > 0 {
			maxsplit--
		}
	}
	rev = append(rev, s)
	out := make([]string, len(rev))
	for i, p := range rev {
		out[len(rev)-1-i] = p
	}
	return out
}

// splitSpace is str.split() with no separator: runs of whitespace separate
// the words, and the result has no empty strings.
func splitSpace(s string, maxsplit int) []string {
	out := []string{}
	for {
		s = strings.TrimLeftFunc(s, isSpace)
		if s == "" {
			return out
		}
		if maxsplit == 0 {
			return append(out, s)
		}
		i := strings.IndexFunc(s, isSpace)
		if i < 0 {
			return append(out, s)
		}
		out = append(out, s[:i])
		s = s[i:]
		if maxsplit > 0 {
			maxsplit--
		}
	}
}

func rsplitSpace(s string, maxsplit int) []string {
	var rev []string
	for {
		s = strings.TrimRightFunc(s, isSpace)
		if s == "" {
			break
		}
		if maxsplit == 0 {
			rev = append(rev, s)
			break
		}
		i := strings.LastIndexFunc(s, isSpace)
		if i < 0 {
			rev = append(rev, s)
			break
		}
		_, w := utf8.DecodeRuneInString(s[i:])
		rev = append(rev, s[i+w:])
		s = s[:i]
		if maxsplit > 0 {
			maxsplit--
		}
	}
	res := make([]string, len(rev))
	for i, p := range rev {
		res[len(rev)-1-i] = p
	}
	return res
}

func strList(parts []string) []interface{} {
	out := make([]interface{}, len(parts))
	for i, p := range parts {
		out[i] = p
	}
	return out
}

// isLineBreak is Python's line boundary for splitlines.
func isLineBreak(r rune) bool {
	switch r {
	case '\n', '\r', '\v', '\f', 0x1c, 0x1d, 0x1e, 0x85, 0x2028, 0x2029:
		return true
	}
	return false
}

func strSplitlines(self interface{}, a *args) (interface{}, error) {
	vals, given, err := a.keywords("keepends")
	if err != nil {
		return nil, err
	}
	keep := false
	if given[0] {
		// keepends is an int flag: bool or int.
		n, ok := toInt(vals[0])
		if !ok {
			return nil, raise("TypeError", "'%s' object cannot be interpreted as an integer", typeName(vals[0]))
		}
		keep = n.Sign() != 0
	}
	s := self.(string)
	out := []interface{}{}
	for s != "" {
		i := strings.IndexFunc(s, isLineBreak)
		if i < 0 {
			out = append(out, s)
			break
		}
		_, w := utf8.DecodeRuneInString(s[i:])
		if s[i] == '\r' && i+1 < len(s) && s[i+1] == '\n' {
			w = 2
		}
		if keep {
			out = append(out, s[:i+w])
		} else {
			out = append(out, s[:i])
		}
		s = s[i+w:]
	}
	return out, nil
}

func strPartition(reverse bool) func(interface{}, *args) (interface{}, error) {
	return func(self interface{}, a *args) (interface{}, error) {
		if err := a.positional(1, 1); err != nil {
			return nil, err
		}
		sep, ok := a.pos[0].(string)
		if !ok {
			return nil, raise("TypeError", "must be str, not %s", typeName(a.pos[0]))
		}
		if sep == "" {
			return nil, raise("ValueError", "empty separator")
		}
		s := self.(string)
		var i int
		if reverse {
			i = strings.LastIndex(s, sep)
		} else {
			i = strings.Index(s, sep)
		}
		if i < 0 {
			if reverse {
				return []interface{}{"", "", s}, nil
			}
			return []interface{}{s, "", ""}, nil
		}
		return []interface{}{s[:i], sep, s[i+len(sep):]}, nil
	}
}

func strExpandtabs(self interface{}, a *args) (interface{}, error) {
	vals, given, err := a.keywords("tabsize")
	if err != nil {
		return nil, err
	}
	tabsize := 8
	if given[0] {
		if tabsize, err = index(vals[0]); err != nil {
			return nil, err
		}
	}
	var b strings.Builder
	col := 0
	for _, r := range self.(string) {
		switch r {
		case '\t':
			if tabsize > 0 {
				n := tabsize - col%tabsize
				b.WriteString(strings.Repeat(" ", n))
				col += n
			}
		case '\n', '\r':
			b.WriteRune(r)
			col = 0
		default:
			b.WriteRune(r)
			col++
		}
	}
	return b.String(), nil
}

func strJoin(self interface{}, a *args) (interface{}, error) {
	if err := a.positional(1, 1); err != nil {
		return nil, err
	}
	var items []interface{}
	switch v := a.pos[0].(type) {
	case []interface{}:
		items = v
	case string:
		for _, r := range v {
			items = append(items, string(r))
		}
	case map[string]interface{}:
		for _, k := range sortedKeys(v) {
			items = append(items, k)
		}
	default:
		return nil, raise("TypeError", "can only join an iterable")
	}
	parts := make([]string, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, raise("TypeError", "sequence item %d: expected str instance, %s found", i, typeName(item))
		}
		parts[i] = s
	}
	return strings.Join(parts, self.(string)), nil
}
