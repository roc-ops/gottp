package compiler

import (
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/parser"
)

func compileText(t *testing.T, text string) *CompiledTemplate {
	t.Helper()
	tmpl, err := parser.ParseTemplate(text)
	if err != nil {
		t.Fatal(err)
	}
	ct, err := NewCompiler().CompileTemplate(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

// The compiler reads every call once: the runtime gets the parsed calls and
// never parses a call string while it matches.
func TestCallsAreReadAtCompileTime(t *testing.T) {
	ct := compileText(t, `<input load="text" functions="test">
x 1
</input>
<group name="g">
x {{ v | upper | replace("a", "b") }}
</group>
<output functions="traverse('g')"/>
`)
	if ct.Inputs[0].calls.Load() == nil || ct.Outputs[0].calls.Load() == nil {
		t.Fatal("input/output calls not read at compile time")
	}
	v := ct.Groups[0].Patterns[0].Variables["v"]
	first := v.Calls()
	if len(first.All) != 2 || first.All[1].Name != "replace" || first.All[1].Strs[1] != "b" {
		t.Fatalf("calls %+v", first.All)
	}
	if v.Calls() != first {
		t.Error("Calls() is not cached")
	}
}

// A child template's call warnings reach the root template once.
func TestChildTemplateCallWarnings(t *testing.T) {
	ct := compileText(t, `<template>
<template>
<group name="a">
x {{ v | nosuchfn }}
</group>
</template>
<template>
<group name="b">
y {{ w | upper }}
</group>
</template>
</template>
`)
	n := 0
	for _, w := range ct.Warnings {
		if strings.Contains(w, "nosuchfn") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want one nosuchfn warning, got %q", ct.Warnings)
	}
	if len(ct.UnknownCalls()) != 1 || ct.UnknownCalls()[0].Scope != ScopeMatch {
		t.Errorf("UnknownCalls = %+v", ct.UnknownCalls())
	}
}

// Names a scope handles itself (re, set, pattern names, indicators) are
// not unknown.
func TestKnownNamesDontWarn(t *testing.T) {
	ct := compileText(t, `<group name="g" functions="macro('m') | void | itemize(key='v')">
x {{ v | re("\\d+") | DIGIT | set("a") | default("b") | joinmatches(",") | _start_ | to_int }}
y {{ w | ORPHRASE | _line_ | _exact_ }}
</group>
`)
	for _, w := range ct.Warnings {
		if strings.Contains(w, "unknown") {
			t.Errorf("unexpected warning %q", w)
		}
	}
}

// A capture variable after {{ _start_ }} or {{ _end_ }} gets a compile
// warning: the line's regex ends at the indicator, so it captures nothing
// (#136).
func TestCutVariableWarning(t *testing.T) {
	ct := compileText(t, `<group name="g">
interface {{ _start_ }} {{ name }}
 v {{ v }}
exit {{ _end_ }}
</group>`)
	var got []string
	for _, w := range ct.Warnings {
		if strings.Contains(w, "captures nothing") {
			got = append(got, w)
		}
	}
	if len(got) != 1 || !strings.Contains(got[0], `variable "name" comes after {{ _start_ }}`) {
		t.Errorf("want one warning about name, got %q", got)
	}
}
