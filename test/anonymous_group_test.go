package test

import (
	"encoding/json"
	"testing"

	"github.com/roc-ops/gottp"
)

// TestAnonymousGroup tests anonymous group functionality
func TestAnonymousGroup(t *testing.T) {
	// Anonymous group - no name attribute
	template := `
<group>
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
`

	data := `
interface Lo0
 ip address 1.1.1.1/32
!
interface Lo1
 ip address 2.2.2.2/32
!
`

	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		t.Fatalf("Failed to compile template: %v", err)
	}

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result: %s", string(jsonData))

	// Anonymous groups should still produce results
	if result == nil {
		t.Error("Result is nil for anonymous group")
	}
}


// TestAnonymousGroupResultShape pins the result shape of top-level anonymous
// groups to Python TTP 0.10.1 (#62). TTP's template.form_results
// (ttp/ttp.py:989-1003) makes the _anonymous_ value one input's result,
// appends the rest of that input's results to it, and keeps the other inputs.
// Every expected value is Python TTP 0.10.1's parser.result()[0] for the same
// template and inputs.
func TestAnonymousGroupResultShape(t *testing.T) {
	const (
		implicit   = "interface {{ name }}"
		unnamed    = "<group>\ninterface {{ name }}\n</group>"
		hostGroup  = "\n<group name=\"h\">\nhost {{ host }}\n</group>"
		perTplOpen = "<template results=\"per_template\">\n"
	)
	// Templates with several top-level groups are wrapped in <template>:
	// without it gottp keeps only the first group (#27).
	wrap := func(body string) string { return "<template>\n" + body + "\n</template>" }
	cases := []struct {
		name     string
		template string
		inputs   gottp.Inputs
		want     string
		// skipSourceMap names the issue that keeps the source-map path from
		// matching TTP on this case for a reason unrelated to #62.
		skipSourceMap string
	}{
		{name: "implicit two matches", template: implicit,
			inputs: gottp.Inputs{"Default_Input": "interface A\ninterface B"},
			want:   `[[{"name":"A"},{"name":"B"}]]`},
		{name: "implicit one match is a dict", template: implicit,
			inputs: gottp.Inputs{"Default_Input": "interface A"},
			want:   `[{"name":"A"}]`},
		{name: "implicit no match", template: implicit,
			inputs: gottp.Inputs{"Default_Input": "nothing"},
			want:   `[{}]`},
		{name: "unnamed two matches", template: unnamed,
			inputs: gottp.Inputs{"Default_Input": "interface A\ninterface B"},
			want:   `[[{"name":"A"},{"name":"B"}]]`},
		{name: "unnamed one match is a list", template: unnamed,
			inputs: gottp.Inputs{"Default_Input": "interface A"},
			want:   `[[{"name":"A"}]]`},
		{name: "unnamed no match", template: unnamed,
			inputs: gottp.Inputs{"Default_Input": "nothing"},
			want:   `[{}]`},
		{name: "explicit _anonymous_ name", template: "<group name=\"_anonymous_\">\ninterface {{ name }}\n</group>",
			inputs: gottp.Inputs{"Default_Input": "interface A\ninterface B"},
			want:   `[[{"name":"A"},{"name":"B"}]]`},
		{name: "two inputs keep both", template: implicit,
			inputs:        gottp.Inputs{"Default_Input": "interface A\ninterface B", "in2": "interface C"},
			want:          `[[{"name":"A"},{"name":"B"}],{"name":"C"}]`,
			skipSourceMap: "#105"},
		{name: "two inputs one empty", template: implicit,
			inputs:        gottp.Inputs{"Default_Input": "interface A\ninterface B", "in2": "nothing"},
			want:          `[[{"name":"A"},{"name":"B"}],{}]`,
			skipSourceMap: "#105"},
		{name: "unnamed two inputs keep both", template: unnamed,
			inputs:        gottp.Inputs{"Default_Input": "interface A\ninterface B", "in2": "interface C"},
			want:          `[[{"name":"A"},{"name":"B"}],[{"name":"C"}]]`,
			skipSourceMap: "#105"},
		{name: "anonymous plus named group", template: wrap(unnamed + hostGroup),
			inputs: gottp.Inputs{"Default_Input": "interface A\ninterface B\nhost X"},
			want:   `[[{"name":"A"},{"name":"B"},{"h":{"host":"X"}}]]`},
		{name: "anonymous one match plus named group", template: wrap(unnamed + hostGroup),
			inputs: gottp.Inputs{"Default_Input": "interface A\nhost X"},
			want:   `[[{"name":"A"},{"h":{"host":"X"}}]]`},
		{name: "anonymous no match plus named group", template: wrap(unnamed + hostGroup),
			inputs: gottp.Inputs{"Default_Input": "host X"},
			want:   `[{"h":{"host":"X"}}]`},
		{name: "two unnamed groups", template: wrap(unnamed + "\n<group>\nhost {{ host }}\n</group>"),
			inputs: gottp.Inputs{"Default_Input": "interface A\nhost X"},
			want:   `[[{"name":"A"},{"host":"X"}]]`},
		{name: "anonymous with nested group", template: "<group>\ninterface {{ name }}\n<group name=\"ips\">\n ip {{ ip }}\n</group>\n</group>",
			inputs: gottp.Inputs{"Default_Input": "interface A\n ip 1\ninterface B"},
			want:   `[[{"ips":{"ip":"1"},"name":"A"},{"name":"B"}]]`},
		{name: "per_template", template: perTplOpen + unnamed + "\n</template>",
			inputs:        gottp.Inputs{"Default_Input": "interface A\ninterface B", "in2": "interface C"},
			want:          `[{"name":"A"},{"name":"B"},{"name":"C"}]`,
			skipSourceMap: "#105"},
		{name: "per_template one match", template: perTplOpen + unnamed + "\n</template>",
			inputs: gottp.Inputs{"Default_Input": "interface A"},
			want:   `[{"name":"A"}]`},
		{name: "per_template plus named group", template: perTplOpen + unnamed + hostGroup + "\n</template>",
			inputs: gottp.Inputs{"Default_Input": "interface A\ninterface C\nhost X"},
			want:   `[{"name":"A"},{"name":"C"},{"h":{"host":"X"}}]`},
	}
	for _, tc := range cases {
		for _, sourceMap := range []bool{false, true} {
			name := tc.name
			if sourceMap {
				name += "/sourcemap"
			}
			t.Run(name, func(t *testing.T) {
				if sourceMap && tc.skipSourceMap != "" {
					t.Skipf("source-map path differs for another reason: %s", tc.skipSourceMap)
				}
				compiled, err := gottp.CompileTemplate(tc.template)
				if err != nil {
					t.Fatalf("compile: %v", err)
				}
				got, err := compiled.Parse(tc.inputs, nil, &gottp.ParseOptions{EnableSourceMap: sourceMap})
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				gotJSON, _ := json.Marshal(got)
				var want interface{}
				if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
					t.Fatalf("bad want: %v", err)
				}
				wantJSON, _ := json.Marshal(want)
				if string(gotJSON) != string(wantJSON) {
					t.Errorf("result mismatch\n got: %s\nwant: %s", gotJSON, wantJSON)
				}
			})
		}
	}
}
