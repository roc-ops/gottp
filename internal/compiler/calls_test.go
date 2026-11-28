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
// warning: the line's regex ends at the indicator, so the variable isn't
// captured from the data (#136). The warning says what it gets instead, as
// in Python TTP, once per variable.
func TestCutVariableWarning(t *testing.T) {
	cutWarnings := func(t *testing.T, text string) []string {
		t.Helper()
		ct := compileText(t, text)
		var got []string
		for _, w := range ct.Warnings {
			if strings.Contains(w, "where the line's regex ends") {
				got = append(got, w)
			}
		}
		return got
	}
	tests := []struct {
		name, text string
		want, not  []string
	}{
		{
			name: "start",
			text: "<group name=\"g\">\ninterface {{ _start_ }} {{ name }}\n v {{ v }}\nexit {{ _end_ }}\n</group>",
			want: []string{`variable "name" comes after {{ _start_ }}`, "it gets no value from the line", "({{ name | _start_ }})"},
			not:  []string{"default()", "whole matched line"},
		},
		{
			name: "repeated name warns once",
			text: "<group name=\"g\">\ninterface {{ _start_ }} {{ a }} {{ a }}\n v {{ v }}\n</group>",
			want: []string{`variable "a" comes after {{ _start_ }}`},
		},
		{
			name: "_line_",
			text: "<group name=\"g\">\ninterface {{ _start_ }} {{ _line_ }}\n v {{ v }}\n</group>",
			want: []string{`variable "_line_"`, "capture it in a named variable instead: {{ rest | ORPHRASE | _start_ }}"},
			not:  []string{"{{ _line_ | _start_ }}"},
		},
		{
			name: "default",
			text: "<group name=\"g\">\ninterface {{ _start_ }} {{ x | default(\"d\") }}\n v {{ v }}\n</group>",
			want: []string{`variable "x"`, "its default() value still applies"},
		},
		{
			name: "end with no capture left",
			text: "<group name=\"g\">\ninterface {{ n }}\n exit {{ _end_ }} {{ w | default(\"d\") }}\n</group>",
			want: []string{`"w" and _end_ are set to the whole matched line with a newline first`, "its default() value still applies to a record without such a line", "it no longer ends the group"},
		},
		{
			name: "end after a capture",
			text: "<group name=\"g\">\ninterface {{ n }}\n exit {{ x }} {{ _end_ }} {{ w }}\n</group>",
			want: []string{`variable "w" comes after {{ _end_ }}`, "it gets no value from the line", "it no longer ends the group"},
			not:  []string{"whole matched line"},
		},
		{
			name: "end function keeps the end line",
			text: "<group name=\"g\">\ninterface {{ n }}\n exit {{ _end_ }} {{ w | _end_ }}\n</group>",
			want: []string{`"w" and _end_ are set to the whole matched line`},
			not:  []string{"no longer ends the group"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cutWarnings(t, tt.text)
			if len(got) != 1 {
				t.Fatalf("want one warning, got %q", got)
			}
			for _, s := range tt.want {
				if !strings.Contains(got[0], s) {
					t.Errorf("warning %q lacks %q", got[0], s)
				}
			}
			for _, s := range tt.not {
				if strings.Contains(got[0], s) {
					t.Errorf("warning %q has %q", got[0], s)
				}
			}
		})
	}
}
