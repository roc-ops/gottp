package comparison

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roc-ops/gottp"
)

const topLevelTextData = `interface Gi1
 description uplink
interface Gi2
 description downlink
`

// Python TTP makes text outside any <group> an implicit _anonymous_ group
// only when the template has no XML tags at all (ttp/ttp.py construct_etree,
// "if not list(template_ET)"). With any tag present the text is ignored (#31).
func TestTopLevelTextComparison(t *testing.T) {
	requirePythonTTP(t)

	text := "interface {{ intf }}\n description {{ desc }}"
	dir := t.TempDir()
	textOnly := filepath.Join(dir, "text_only.txt")
	if err := os.WriteFile(textOnly, []byte(text+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	withGroup := filepath.Join(dir, "with_group.txt")
	if err := os.WriteFile(withGroup, []byte("<group name=\"g\">\ninterface {{ intf }}\n</group>\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct{ name, template string }{
		{"no tags", text},
		{"vars", "<vars>\na = \"b\"\n</vars>\n" + text},
		{"vars after the text", text + "\n<vars>\na = \"b\"\n</vars>"},
		{"macro", "<macro>\ndef f(data):\n    return data\n</macro>\n" + text},
		{"doc", "<doc>\nInterfaces.\n</doc>\n" + text},
		{"lookup", "<lookup name=\"l\" load=\"python\">\nx = 1\n</lookup>\n" + text},
		{"group beside the text", "<group name=\"g\">\ninterface {{ intf }}\n</group>\n description {{ desc }}"},
		{"template without tags", "<template>\n" + text + "\n</template>"},
		{"template with vars", "<template>\n<vars>\na = \"b\"\n</vars>\n" + text + "\n</template>"},
		{"template with doc", "<template>\n<doc>\nInterfaces.\n</doc>\n" + text + "\n</template>"},
		{"extend of a text-only template", "<extend template=\"" + textOnly + "\"/>\n"},
		{"text and extend of a text-only template", "interface {{ intf }}\n<extend template=\"" + textOnly + "\"/>\n"},
		{"vars and extend of a text-only template", "<vars>\na = \"b\"\n</vars>\n<extend template=\"" + textOnly + "\"/>\n"},
		{"text and extend of a template with a group", " description {{ desc }}\n<extend template=\"" + withGroup + "\"/>\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			RunComparison(t, "toplevel_text_"+tc.name, tc.template, topLevelTextData, nil, nil)
		})
	}
}

func TestTopLevelTextIgnoredWarning(t *testing.T) {
	want := `top-level text outside any <group> is ignored because the template has XML tags (as in Python TTP): "interface {{ intf }}"`
	nested := `text outside any <group> in a nested <template> is ignored (as in Python TTP): "interface {{ intf }}"`
	cases := []struct {
		name, template string
		want           []string
	}{
		{"no tags", "interface {{ intf }}\n", nil},
		{"vars", "<vars>\na = \"b\"\n</vars>\n\ninterface {{ intf }}\n description {{ desc }}\n", []string{want}},
		{"macro", "<macro>\ndef f(data):\n    return data\n</macro>\ninterface {{ intf }}\n", []string{want}},
		{"doc", "<doc>\nInterfaces.\n</doc>\ninterface {{ intf }}\n", []string{want}},
		{"text after a group", "<group name=\"g\">\nx {{ x }}\n</group>\ninterface {{ intf }}\n", []string{want}},
		{"template with vars", "<template>\n<vars>\na = \"b\"\n</vars>\ninterface {{ intf }}\n</template>\n", []string{want}},
		{"template without tags", "<template>\ninterface {{ intf }}\n</template>\n", nil},
		{"nested template", "<template>\n<template>\ninterface {{ intf }}\n</template>\n</template>\n", []string{nested}},
		{"whitespace only", "<vars>\na = \"b\"\n</vars>\n\n   \n<group name=\"g\">\nx {{ x }}\n</group>\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := gottp.CompileTemplate(tc.template)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			var got []string
			for _, w := range c.GetWarnings() {
				if strings.Contains(w, "outside any <group>") {
					got = append(got, w)
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("warnings = %q, want %q", got, tc.want)
			}
		})
	}
}

// ignore("NAME") and re("NAME") use the <vars> regex NAME, which Python TTP
// evaluates as a Python string: an unknown escape such as \s stays, and \\
// becomes \ (#31, #148).
func TestVarsRegexComparison(t *testing.T) {
	requirePythonTTP(t)

	data := "Model:  X   Power Supply: AC 2400W\n"
	regexes := []string{
		`\s+\S.*`,
		`\s{2,}\S.*`,
		`(?:\s+\S.*)?`,
		`\s+P\S+ \S+: (AC|DC) \d+W`,
		`\s+\w+\s+\w+:\s+\S+\s+\d{2,}W`,
		`[\s\w:]+W`,
		`\s+\d+`, // matches nothing
	}
	for _, re := range regexes {
		doubled := strings.ReplaceAll(re, `\`, `\\`)
		forms := map[string]string{
			"single":           `"` + re + `"`,
			"single quotes":    `'` + re + `'`,
			"double":           `"` + doubled + `"`,
			"raw":              `r"` + re + `"`,
			"quadruple (\\\\)": `"` + strings.ReplaceAll(doubled, `\\`, `\\\\`) + `"`,
		}
		for form, literal := range forms {
			for fn, line := range map[string]string{
				"ignore": `Model: {{ model }}{{ ignore("NC") }}`,
				"re":     `Model: {{ model }}{{ rest | re("NC") }}`,
			} {
				name := fmt.Sprintf("%s %s %s", fn, form, re)
				t.Run(name, func(t *testing.T) {
					template := "<vars>\nNC = " + literal + "\n</vars>\n<group name=\"g\">\n" + line + "\n</group>"
					RunComparison(t, "vars_regex_"+name, template, data, nil, nil)
				})
			}
		}
	}
}

// The repro of #31: ignore("<vars name>") inside a group.
func TestIgnoreVarsNameInGroupComparison(t *testing.T) {
	template := `<vars>
st = "UP|DOWN"
</vars>
<group name="g">
{{ n }} {{ ignore("st") }} {{ w }}
</group>`
	RunComparison(t, "ignore_vars_name_in_group", template, "a UP 1\nb DOWN 2\nc X 3\n", nil, nil)
}

// An inline ignore() argument is a Python string too.
func TestInlineIgnoreRegexComparison(t *testing.T) {
	requirePythonTTP(t)

	for _, arg := range []string{`"\d+"`, `"\\d+"`, `"[\d]+"`, `"[\s\d]+"`, `"[^\s]+"`, `r"\d{2,}"`, `"\\\\d+"`} {
		t.Run(arg, func(t *testing.T) {
			template := "<group name=\"g\">\na {{ ignore(" + arg + ") }} {{ w }}\n</group>"
			RunComparison(t, "inline_ignore_"+arg, template, "a 12 b\n", nil, nil)
		})
	}
}
