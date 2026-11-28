package gottp

import (
	"encoding/json"
	"strings"
	"testing"
)

// The {{ _headers_ }} cases Python TTP 0.10.1 can't run (#68): it raises on
// one or two headers and on a header that isn't a Python identifier. The
// rest of _headers_ is pinned against TTP by the ttpsuite repros
// (test/ttpsuite/testdata/gottp-repros/issue68_headers_*).
func TestHeadersIndicatorBeyondTTP(t *testing.T) {
	tests := []struct {
		name, template, data, want string
	}{
		{
			// One header: the only column is mandatory and takes the line,
			// as TTP's columns(1) does.
			name:     "one header",
			template: "<group name=\"g\">\nPort {{ _headers_ }}\n</group>",
			data:     "Port\nGi0/1\nx\nGi0/123456 z\n",
			want:     `[[{"g":[{"Port":"Gi0/1"},{"Port":"x"},{"Port":"Gi0/123456 z"}]}]]`,
		},
		{
			// Two headers: columns defaults to 1, as TTP's columns(1).
			name:     "two headers",
			template: "<group name=\"g\">\nPort   Name {{ _headers_ }}\n</group>",
			data:     "Port   Name\nGi0/1  foo\nGi0/2\nx\nGi0/3  foo bar baz\n",
			want:     `[[{"g":[{"Name":"foo","Port":"Gi0/1"},{"Name":"","Port":"Gi0/2"},{"Name":"","Port":"x"},{"Name":"foo bar baz","Port":"Gi0/3"}]}]]`,
		},
		{
			name:     "non-identifier headers",
			template: "<group name=\"g\">\nPort-Id   1st     a\"b\\c {{ _headers_ }}\n</group>",
			data:     "Port-Id   1st     a\"b\\c\nGi0/1     foo     up\n",
			want:     `[[{"g":{"1st":"foo","Port-Id":"Gi0/1","a\"b\\c":"up"}}]]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			if err := p.AddTemplate(tt.template); err != nil {
				t.Fatalf("AddTemplate: %v", err)
			}
			p.AddInput(tt.data, "")
			if err := p.Parse(); err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got, err := json.Marshal(p.Result()) // map keys sorted
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestHeadersIndicatorCompileErrors(t *testing.T) {
	tests := []struct {
		name, line, want string
	}{
		{"other variable", "Port   Name  Status {{ _headers_ }} {{ x }}", "only variable"},
		{"other variable before", "{{ x }} Port   Name  Status {{ _headers_ }}", "only variable"},
		{"duplicate header", "Port   Port   Status {{ _headers_ }}", `duplicate header "Port"`},
		{"no headers", "{{ _headers_ }}", "no headers"},
		{"reserved header", "Port   ignore  Status {{ _headers_ }}", `header "ignore"`},
		{"columns without argument", "Port   Name  Status {{ _headers_ | columns }}", "columns"},
		{"columns not a number", `Port   Name  Status {{ _headers_ | columns("x") }}`, "columns"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			err := p.AddTemplate("<group name=\"g\">\n" + tt.line + "\n</group>")
			if err == nil {
				t.Fatalf("AddTemplate succeeded, want an error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q, want it to contain %q", err, tt.want)
			}
		})
	}
}
