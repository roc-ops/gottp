package compiled

import "testing"

// TestResolvePathLiteralReplacement covers the placeholder substitution path.
// Substitution used to go through regexp.MustCompile(QuoteMeta(pattern)) +
// ReplaceAllString, which compiled a regex per placeholder per call and let
// regexp expand `$name` / `$1` sequences inside the *replacement* value. The
// value is data, never a template, so it must be inserted verbatim.
func TestResolvePathLiteralReplacement(t *testing.T) {
	tests := []struct {
		name     string
		template string
		match    map[string]interface{}
		want     string
	}{
		{
			name:     "plain value",
			template: "devices.{{ hostname }}.interfaces",
			match:    map[string]interface{}{"hostname": "router1"},
			want:     "devices.router1.interfaces",
		},
		{
			name:     "value containing dollar-digit is inserted verbatim",
			template: "devices.{{ hostname }}",
			match:    map[string]interface{}{"hostname": "r$1x"},
			want:     "devices.r$1x",
		},
		{
			name:     "value containing dollar-name is inserted verbatim",
			template: "devices.{{ hostname }}",
			match:    map[string]interface{}{"hostname": "a${b}c"},
			want:     "devices.a${b}c",
		},
		{
			name:     "repeated placeholder replaced everywhere",
			template: "{{ site }}/x/{{ site }}",
			match:    map[string]interface{}{"site": "hq"},
			want:     "hq/x/hq",
		},
		{
			name:     "multiple distinct placeholders",
			template: "{{ site }}.{{ host }}",
			match:    map[string]interface{}{"site": "hq", "host": "r1"},
			want:     "hq.r1",
		},
		{
			name:     "unresolved placeholder left intact",
			template: "{{ site }}.{{ missing }}",
			match:    map[string]interface{}{"site": "hq"},
			want:     "hq.{{ missing }}",
		},
		{
			name:     "non-string value formatted",
			template: "vlan.{{ id }}",
			match:    map[string]interface{}{"id": 42},
			want:     "vlan.42",
		},
		{
			name:     "empty template",
			template: "",
			match:    map[string]interface{}{"site": "hq"},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := NewPathResolver()
			got, err := pr.ResolvePath(tt.template, tt.match, nil)
			if err != nil {
				t.Fatalf("ResolvePath: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolvePath(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}
}

// TestResolvePathFallbackOrder pins the match -> cache -> vars precedence that
// the shared package-level regex change must not disturb.
func TestResolvePathFallbackOrder(t *testing.T) {
	pr := NewPathResolver()
	pr.UpdateCache(map[string]interface{}{"host": "cached"})

	got, err := pr.ResolvePath("{{ host }}", nil, map[string]interface{}{"host": "fromvars"})
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if got != "cached" {
		t.Errorf("cache should win over vars: got %q", got)
	}

	got, err = pr.ResolvePath("{{ host }}", map[string]interface{}{"host": "frommatch"}, nil)
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if got != "frommatch" {
		t.Errorf("match should win over cache: got %q", got)
	}

	pr.ClearCache()
	got, err = pr.ResolvePath("{{ host }}", nil, map[string]interface{}{"host": "fromvars"})
	if err != nil {
		t.Fatalf("ResolvePath: %v", err)
	}
	if got != "fromvars" {
		t.Errorf("vars should be used after ClearCache: got %q", got)
	}
}

func TestExtractVariablesFromPath(t *testing.T) {
	pr := NewPathResolver()
	if got := pr.ExtractVariablesFromPath(""); got != nil {
		t.Errorf("empty template should yield nil, got %v", got)
	}
	got := pr.ExtractVariablesFromPath("{{ site }}.x.{{host}}")
	want := []string{"site", "host"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
