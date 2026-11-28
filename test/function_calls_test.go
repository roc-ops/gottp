package test

// Pins for refactor F1 (#36): one call module for match, group, input and
// output functions. Each test asserts what F1 must deliver and is skipped
// until then; F1 removes the skips. The results these templates give are
// pinned against Python TTP 0.10.1 in test/ttpsuite/testdata/gottp-repros
// (issue36_*, issue23_*, issue151_*).

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/roc-ops/gottp"
)

// compileWarnings compiles tmpl, parses one line through it (TTP skips an
// unknown name at run time and keeps going) and returns the compile warnings.
func compileWarnings(t *testing.T, tmpl string) []string {
	t.Helper()
	c, err := gottp.CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, err := c.Parse(gottp.Inputs{"Default_Input": "x 1\n"}, nil, nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return c.GetWarnings()
}

// TestUnknownFunctionNamesWarn: TTP logs an unknown function name in every
// scope and, for a close match, the names its utils.guess suggests
// (difflib, cutoff 0.8). gottp gives no warning in any scope today. The
// suggestions below are the ones TTP 0.10.1 logs for these templates.
func TestUnknownFunctionNamesWarn(t *testing.T) {
	t.Skip("F1: #36 unknown-name warnings")
	cases := []struct {
		name, tmpl string
		want       []string // substrings of one warning
	}{
		{"match", "x {{ v | nosuchfn }}\n", []string{"nosuchfn"}},
		{"match close", "x {{ v | to_itn }}\n", []string{"to_itn", "to_int"}},
		{"group", "<group name=\"g\" functions=\"nosuchgrp()\">\nx {{ v }}\n</group>\n", []string{"nosuchgrp"}},
		{"group close", "<group name=\"g\" functions=\"contians('v')\">\nx {{ v }}\n</group>\n", []string{"contians", "contains"}},
		{"input", "<input load=\"text\" functions=\"nosuchin()\">\nx 1\n</input>\n<group name=\"g\">\nx {{ v }}\n</group>\n", []string{"nosuchin"}},
		{"input close", "<input load=\"text\" functions=\"extract_comands('x')\">\nx 1\n</input>\n<group name=\"g\">\nx {{ v }}\n</group>\n", []string{"extract_comands", "extract_commands"}},
		{"output", "<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"nosuchout()\"/>\n", []string{"nosuchout"}},
		{"output close", "<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"is_eqal('x')\"/>\n", []string{"is_eqal", "is_equal"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warnings := compileWarnings(t, tc.tmpl)
			for _, w := range warnings {
				if containsAll(w, tc.want) {
					return
				}
			}
			t.Errorf("want a warning containing %q, got %q", tc.want, warnings)
		})
	}
}

// TestPipeInQuotedArgumentWarns (#23, decision 2026-11-27): gottp keeps
// splitting the chain on "|" as TTP does, so the result doesn't change
// (gottp-repros/issue23_re_pipe_splits_chain), but it warns. The warning
// names the variable, the broken-off text and the <vars> workaround.
func TestPipeInQuotedArgumentWarns(t *testing.T) {
	t.Skip("F1: #23 pipe-in-quotes warning")
	warnings := compileWarnings(t, "x {{ direction | re(\"UP|DOWN\") }}\n")
	want := []string{"direction", `DOWN")`, "<vars>"}
	for _, w := range warnings {
		if containsAll(w, want) {
			return
		}
	}
	t.Errorf("want a warning containing %q, got %q", want, warnings)
}

// TestMalformedMacroCallIsCompileError (#146 item 2): TTP raises TypeError
// for both templates; the agreed behaviour is a compile error. gottp skips
// the call silently today.
func TestMalformedMacroCallIsCompileError(t *testing.T) {
	t.Skip("F1: #146 item 2")
	cases := map[string]string{
		// TTP: macro() missing 1 required positional argument: 'macro_name'
		"empty macro attribute": "<group name=\"g\">\nx {{ v }}\n</group>\n<output macro=\"\"/>\n",
		// TTP: macro() got an unexpected keyword argument 'name'
		"keyword argument": "<macro>\ndef m(data):\n    return data\n</macro>\n<group name=\"g\">\nx {{ v }}\n</group>\n<output functions=\"macro(name='m')\"/>\n",
	}
	for name, tmpl := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := gottp.CompileTemplate(tmpl); err == nil {
				t.Error("want a compile error, got none")
			}
		})
	}
}

// issue144Template hangs CompileTemplate with unbounded memory today (#144).
const issue144Template = "<group name=\"items\">\nitem {{ name }}\n</group>\n<output functions=\"macro(\\\"m\\\",\\\"n\\\")\"/>\n"

const issue144ChildEnv = "GOTTP_TEST_ISSUE144_CHILD"

// TestBackslashQuotedAttributeCompilesQuickly (#144): the template must
// compile or return an error quickly. The compile runs in a child process
// (this test binary, re-run for this test only) that is killed after 2s:
// a goroutine that never returns would keep allocating inside the shared
// test binary after the test failed.
func TestBackslashQuotedAttributeCompilesQuickly(t *testing.T) {
	t.Skip("F1: #144")
	if os.Getenv(issue144ChildEnv) == "1" {
		_, err := gottp.CompileTemplate(issue144Template)
		t.Logf("compile returned, err=%v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBackslashQuotedAttributeCompilesQuickly$", "-test.count=1")
	cmd.Env = append(os.Environ(), issue144ChildEnv+"=1")
	cmd.WaitDelay = time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("CompileTemplate did not return within 2s")
	}
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
