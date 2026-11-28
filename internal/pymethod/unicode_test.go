package pymethod

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"testing"
)

// TestUnicodeMatchesPython checks every code point's case mappings and
// character classes against Python 3.12.
func TestUnicodeMatchesPython(t *testing.T) {
	out, err := exec.Command(pythonBin(t), "-I", "testdata/pyunicode.py").Output()
	if err != nil {
		t.Fatalf("pyunicode.py: %v", err)
	}
	var py struct {
		Maps    map[string][4]string `json:"maps"`
		Classes [][3]int             `json:"classes"`
	}
	if err := json.Unmarshal(out, &py); err != nil {
		t.Fatal(err)
	}
	maps := make(map[rune][4]string, len(py.Maps))
	for k, v := range py.Maps {
		n, _ := strconv.Atoi(k)
		maps[rune(n)] = v
	}
	bad := 0
	for r := rune(0); r < 0x110000; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		s := string(r)
		want, ok := maps[r]
		if !ok {
			want = [4]string{s, s, s, s}
		}
		got := [4]string{pyUpper(s), pyLower(s), pyTitle(s), pyCasefold(s)}
		if got != want && bad < 20 {
			bad++
			t.Errorf("U+%04X: upper, lower, title, casefold: Python %q, gottp %q", r, want, got)
		}
	}
	classes := []func(rune) bool{
		isAlpha, isDecimal, isDigit, isNumeric, isSpace, isPrintable,
		func(r rune) bool { return strBool(t, strIslower, r) },
		func(r rune) bool { return strBool(t, strIsupper, r) },
		isAlnum,
		func(r rune) bool { return strBool(t, strIsidentifier, r) },
	}
	for _, run := range py.Classes {
		for r := rune(run[0]); r <= rune(run[1]); r++ {
			for k, f := range classes {
				if want := run[2]&(1<<k) != 0; f(r) != want && bad < 40 {
					bad++
					t.Errorf("U+%04X: class %d: Python %v, gottp %v", r, k, want, !want)
				}
			}
		}
	}
}

func strBool(t *testing.T, m func(interface{}, *args) (interface{}, error), r rune) bool {
	v, err := m(string(r), &args{typ: "str", name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return v.(bool)
}
