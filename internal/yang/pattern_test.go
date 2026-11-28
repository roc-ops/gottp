package yang

import "testing"

// TestXSDToRE2MatchesYangson checks the translation against yangson, the
// library Python TTP validates YANG with: each expectation is what yangson
// 1.7.9 (elementpath 5.1.4 translate_pattern, as yangson calls it) gives for
// the same pattern and value, except where marked "differs": elementpath
// passes \s and \w to Python's re unchanged, while RFC 7950 9.4.5 defers
// to XSD (Part 2, F.1.1), and this follows XSD.
func TestXSDToRE2MatchesYangson(t *testing.T) {
	cases := []struct {
		pattern, value string
		match          bool
	}{
		{"[a-z]+", "abc", true},
		{"[a-z]+", "ABC", false},
		{"[a-z]+", "abc1", false},
		{"[a-z]+", "", false},
		{"x.*", "xyz", true},
		{"x.*", "ax", false},
		{"a.b", "a-b", true},
		{"a.b", "a\nb", false},
		{"a.b", "a\rb", false},
		{"a.b", "aéb", true},
		{"^a$", "a", false},
		{"^a$", "^a$", true},
		{"\\d+", "0123", true},
		{"\\d+", "12a", false},
		{"\\d+", "٣٤", true},
		{"\\s", " ", true},
		{"\\s", "\t", true},
		{"\\s", "\f", false}, // differs: XSD \s is [#x20\t\n\r]; elementpath uses Python's \s
		{"\\S+", "ab", true},
		{"\\S+", "a b", false},
		{"\\w+", "abc", true},
		{"\\w+", "a_b", false}, // differs: XSD \w excludes \p{P}, so '_'; elementpath uses Python's \w
		{"\\w+", "héllo", true},
		{"\\w+", "a b", false},
		{"\\w+", "a1", true},
		{"\\W", "_", true}, // differs: see \w above
		{"\\W", "-", true},
		{"\\W", "a", false},
		{"[\\w\\-]+", "héllo-wörld", true},
		{"[\\w\\-]+", "a_b", false},
		{"[\\w\\-]+", "a.b", false},
		{"[\\S]+", "ab", true},
		{"[\\S]+", "a b", false},
		{"[a\\s]+", "a a", true},
		{"[a\\s]+", "ab", false},
		{"[^\\d]+", "abc", true},
		{"[^\\d]+", "a1", false},
		{"[^\\s]+", "ab", true},
		{"[^\\s]+", "a b", false},
		{"\\p{L}+\\p{N}*", "été3", true},
		{"\\p{L}+\\p{N}*", "3", false},
		{"[\\p{N}\\p{L}]+", "a1", true},
		{"[\\p{N}\\p{L}]+", "a-1", false},
		{"\\P{Lu}+", "abc", true},
		{"\\P{Lu}+", "aBc", false},
		{"[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}", "00:11:22:aa:BB:cc", true},
		{"[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}", "00:11:22:33:44", false},
		{"[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}", "00:11:22:33:44:55:66", false},
		{"(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])(%[\\p{N}\\p{L}]+)?", "10.0.0.1", true},
		{"(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])(%[\\p{N}\\p{L}]+)?", "10.0.0.256", false},
		{"(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])(%[\\p{N}\\p{L}]+)?", "10.0.0.1%eth0", true},
		{"(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])(%[\\p{N}\\p{L}]+)?", "1.2.3", false},
		{"a|b", "a", true},
		{"a|b", "b", true},
		{"a|b", "ab", false},
		{"ab|cd", "ab", true},
		{"ab|cd", "cd", true},
		{"ab|cd", "abd", false},
		{"[\\^a]+", "^a", true},
		{"[\\^a]+", "b", false},
		{"[a^]+", "^a", true},
		{"[a^]+", "b", false},
		{"[$]", "$", true},
		{"[$]", "a", false},
		{"\\.\\-\\|", ".-|", true},
		{"\\.\\-\\|", "a-|", false},
		{"[\\[\\]]+", "[]", true},
		{"[\\[\\]]+", "a", false},
		{"a{2,3}", "aa", true},
		{"a{2,3}", "aaaa", false},
		{"\\n", "\n", true},
		{"\\n", "n", false},
		{"[a-c-]+", "a-c", true},
		{"[a-c-]+", "d", false},
	}
	for _, tc := range cases {
		cp := compilePattern(tc.pattern)
		if cp.err != nil {
			t.Errorf("%q: unexpected translation error: %v", tc.pattern, cp.err)
			continue
		}
		if got := cp.re.MatchString(tc.value); got != tc.match {
			t.Errorf("pattern %q value %q: match=%v, yangson says %v (RE2 %s)", tc.pattern, tc.value, got, tc.match, cp.re)
		}
	}
}

// TestXSDToRE2ReportsWhatRE2CannotExpress lists patterns yangson enforces
// (through Python's re) that RE2 can't: they must be reported, not passed.
func TestXSDToRE2ReportsWhatRE2CannotExpress(t *testing.T) {
	for _, p := range []string{
		"\\i\\c*",
		"\\p{IsBasicLatin}+",
		"[a-z-[aeiou]]+",
		"[^a\\w]",
		"a{1001}",
	} {
		if cp := compilePattern(p); cp.err == nil {
			t.Errorf("%q: expected a translation error, got %s", p, cp.re)
		}
	}
}
