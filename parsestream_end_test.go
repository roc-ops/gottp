package gottp

import (
	"encoding/json"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// streamRecords runs ParseStream and returns the records, JSON-encoded and
// sorted, for comparison with Parse's.
func streamRecords(t *testing.T, c *CompiledTemplate, input string) []string {
	t.Helper()
	var got []string
	err := c.ParseStream(Inputs{"Default_Input": input}, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			b, _ := json.Marshal(m)
			got = append(got, string(b))
			return nil
		})
	if err != nil {
		t.Fatalf("ParseStream: %v", err)
	}
	sort.Strings(got)
	return got
}

// parseRecords runs Parse and returns group g's records, JSON-encoded and
// sorted.
func parseRecords(t *testing.T, c *CompiledTemplate, input string) []string {
	t.Helper()
	res, err := c.Parse(Inputs{"Default_Input": input}, nil, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	b, _ := json.Marshal(res)
	var outer []map[string]json.RawMessage
	if err := json.Unmarshal(b, &outer); err != nil || len(outer) != 1 {
		t.Fatalf("unexpected Parse result %s: %v", b, err)
	}
	var got []string
	raw := outer[0]["g"]
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		list = []json.RawMessage{raw}
	}
	for _, r := range list {
		var m map[string]interface{}
		_ = json.Unmarshal(r, &m)
		b, _ := json.Marshal(m)
		got = append(got, string(b))
	}
	sort.Strings(got)
	return got
}

// ParseStream trims a line's \r as Parse does, so on CRLF input it keeps
// and drops the same records around an _end_ (#139, #33).
func TestParseStream_EndCut_CRLFMatchesParse(t *testing.T) {
	tmpl := "<group name=\"g\">\n{{ l | _line_ }}\nend {{ e | _end_ }}\n</group>"
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, input := range []string{
		"aaa\r\nend x\r\nbbb\r\nccc\r\nddd\r\n",
		"aaa\r\nend x\r\nbbb\r\nccc",
		"aaa\r\nend x\r\n",
		"aaa\r\nbbb\r\nend x",
	} {
		want := parseRecords(t, c, input)
		got := streamRecords(t, c, input)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("input %q:\nParse:       %v\nParseStream: %v", input, want, got)
		}
		lf := strings.ReplaceAll(input, "\r\n", "\n")
		if lfGot := streamRecords(t, c, lf); !reflect.DeepEqual(got, lfGot) {
			t.Errorf("input %q: CRLF gives %v, LF gives %v", input, got, lfGot)
		}
	}
}

// streamPeakHeap streams input through tmpl, sampling the live heap every
// 4096 records and after the last one, and returns the peak heap growth
// and the number of records.
func streamPeakHeap(tb testing.TB, tmpl, input string) (peak uint64, n int) {
	tb.Helper()
	c, err := CompileTemplate(tmpl)
	if err != nil {
		tb.Fatalf("compile: %v", err)
	}
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	base := ms.HeapAlloc
	sample := func() {
		runtime.ReadMemStats(&ms)
		if ms.HeapAlloc > base && ms.HeapAlloc-base > peak {
			peak = ms.HeapAlloc - base
		}
	}
	err = c.ParseStream(Inputs{"Default_Input": input}, nil, nil,
		func(m map[string]interface{}, sr [2]int, gp string) error {
			n++
			if n%4096 == 0 {
				sample()
			}
			return nil
		})
	if err != nil {
		tb.Fatalf("ParseStream: %v", err)
	}
	sample()
	return peak, n
}

// endWorstCaseInput is the worst case for a group with an _end_: one end
// line near the start, then many _line_ lines that run_re's cut decides
// about.
func endWorstCaseInput(lines int) string {
	var b strings.Builder
	b.WriteString("a\nend 1\n")
	for i := 0; i < lines; i++ {
		b.WriteString("line\n")
	}
	return b.String()
}

const (
	endTmpl   = "<group name=\"g\">\n{{ l | _line_ }}\nend {{ e | _end_ }}\n</group>"
	noEndTmpl = "<group name=\"g\">\n{{ l | _line_ }}\n</group>"
)

// ParseStream's memory stays bounded in a group with an _end_: it knows
// run_re's cut before it streams, so it holds no matches after an end match
// (#139). Before, the matches after the last _end_ were all held until the
// end of the input.
func TestParseStream_EndCut_MemoryBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("streams 400k lines")
	}
	input := endWorstCaseInput(400_000)
	// The no-_end_ template streams every line; the _end_ one only those
	// up to the line right after the end line, but the matching work is
	// the same.
	ref, refN := streamPeakHeap(t, noEndTmpl, input)
	got, gotN := streamPeakHeap(t, endTmpl, input)
	c, err := CompileTemplate(endTmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if want := len(parseRecords(t, c, input)); gotN != want {
		t.Errorf("_end_ template streamed %d records, Parse gives %d", gotN, want)
	}
	t.Logf("peak heap growth: with _end_ %d KB (%d records), without %d KB (%d records)", got>>10, gotN, ref>>10, refN)
	// Holding 400k matches costs tens of MB; bounded streaming a few.
	if limit := 2*ref + 8<<20; got > limit {
		t.Errorf("peak heap growth with _end_ = %d KB, want at most %d KB (without _end_: %d KB)", got>>10, limit>>10, ref>>10)
	}
}

// BenchmarkParseStreamEndWorstCase measures the worst case above at 2M
// lines, reporting the peak heap growth.
func BenchmarkParseStreamEndWorstCase(b *testing.B) {
	input := endWorstCaseInput(2_000_000)
	for _, bc := range []struct{ name, tmpl string }{{"with_end", endTmpl}, {"without_end", noEndTmpl}} {
		b.Run(bc.name, func(b *testing.B) {
			var peak uint64
			for i := 0; i < b.N; i++ {
				p, _ := streamPeakHeap(b, bc.tmpl, input)
				if p > peak {
					peak = p
				}
			}
			b.ReportMetric(float64(peak)/(1<<20), "peakMB")
		})
	}
}

// Parse trims a line's \r (#33), so run_re's cut keeps the line right
// after a CRLF end line as it does on LF input, in a group with or without
// children (#139).
func TestParse_EndCut_CRLFMatchesLF(t *testing.T) {
	for _, tmpl := range []string{
		"<group name=\"g\">\n{{ l | _line_ }}\nend {{ e | _end_ }}\n</group>",
		"<group name=\"g\">\n{{ l | _line_ }}\nend {{ e | _end_ }}\n<group name=\"c\">\nchild {{ x }}\n</group>\n</group>",
		"<group name=\"g\">\nint {{ i | _start_ }}\n{{ l | _line_ }}\nend {{ e | _end_ }}\n<group name=\"c\">\nchild {{ x }}\n</group>\n</group>",
	} {
		c, err := CompileTemplate(tmpl)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		for _, lf := range []string{"aaa\nend x\nbbb\nccc\nddd\n", "int 1\naaa\nend x\nbbb\nccc\n"} {
			crlf := strings.ReplaceAll(lf, "\n", "\r\n")
			r1, err1 := c.Parse(Inputs{"Default_Input": lf}, nil, nil)
			r2, err2 := c.Parse(Inputs{"Default_Input": crlf}, nil, nil)
			if err1 != nil || err2 != nil {
				t.Fatalf("Parse: %v, %v", err1, err2)
			}
			b1, _ := json.Marshal(r1)
			b2, _ := json.Marshal(r2)
			if string(b1) != string(b2) {
				t.Errorf("template %q, input %q:\nLF:   %s\nCRLF: %s", tmpl, lf, b1, b2)
			}
		}
	}
}

// typicalInput is 100k interface blocks, each closed by "!".
func typicalInput(blocks int) string {
	var b strings.Builder
	for i := 0; i < blocks; i++ {
		b.WriteString("interface Gi0/")
		b.WriteString(strings.Repeat("1", 1+i%3))
		b.WriteString("\n description uplink\n ip address 10.0.0.1 255.255.255.0\n shutdown\n!\n")
	}
	return b.String()
}

// BenchmarkParseStreamEndTypical measures the cost of an _end_ line, and
// so of scanCut's first pass, on a typical template: with an end line
// that closes every block, with one that never matches (scanCut's slowest
// case: it scans the whole input backward), and without one.
func BenchmarkParseStreamEndTypical(b *testing.B) {
	const body = "interface {{ name | _start_ }}\n description {{ desc }}\n ip address {{ ip }} {{ mask }}\n"
	input := typicalInput(100_000)
	for _, bc := range []struct{ name, tmpl string }{
		{"with_end", "<group name=\"i\">\n" + body + "!{{ _end_ }}\n</group>"},
		{"end_never_matches", "<group name=\"i\">\n" + body + "exit {{ x | _end_ }}\n</group>"},
		{"without_end", "<group name=\"i\">\n" + body + "</group>"},
	} {
		b.Run(bc.name, func(b *testing.B) {
			c, err := CompileTemplate(bc.tmpl)
			if err != nil {
				b.Fatalf("compile: %v", err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				n := 0
				if err := c.ParseStream(Inputs{"Default_Input": input}, nil, nil,
					func(m map[string]interface{}, sr [2]int, gp string) error { n++; return nil }); err != nil {
					b.Fatal(err)
				}
				if n != 100_000 {
					b.Fatalf("%d records, want 100000", n)
				}
			}
		})
	}
}
