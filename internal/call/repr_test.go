package call

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestReprMatchesPython checks Repr of every code point, and of strings
// that mix quotes and escapes, against Python 3.12's repr().
func TestReprMatchesPython(t *testing.T) {
	py := os.Getenv("GOTTP_PYTHON")
	if py == "" {
		t.Skip("GOTTP_PYTHON is not set: the differential tests need Python 3.12")
	}
	strs := []string{
		"", "a", "it's", `say "hi"`, `it's "x"`, `back\slash`, "a\u00a0b", "tab\tnl\nr\r",
		"\x00\x1f\x7f\u0080\u0085\u009f", "\u00ad\u2028\u2029\u200b\ufeff", "\U000e0001\U0010ffff", "Γειά σου ✓ 😀",
	}
	in, _ := json.Marshal(strs)
	cmd := exec.Command(py, "-I", "testdata/pyrepr.py")
	cmd.Stdin = strings.NewReader(string(in))
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pyrepr.py: %v", err)
	}
	var want struct {
		Chars   map[string]string `json:"chars"`
		Strings []string          `json:"strings"`
	}
	if err := json.Unmarshal(out, &want); err != nil {
		t.Fatal(err)
	}
	chars := make(map[rune]string, len(want.Chars))
	for k, v := range want.Chars {
		n, _ := strconv.Atoi(k)
		chars[rune(n)] = v
	}
	bad := 0
	for r := rune(0); r < 0x110000 && bad < 20; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		w, ok := chars[r]
		if !ok {
			w = "'" + string(r) + "'"
		}
		if got := Repr(string(r)); got != w {
			bad++
			t.Errorf("U+%04X: Python %s, gottp %s", r, w, got)
		}
	}
	for i, s := range strs {
		if got := Repr(s); got != want.Strings[i] {
			t.Errorf("%q: Python %s, gottp %s", s, want.Strings[i], got)
		}
	}
}
