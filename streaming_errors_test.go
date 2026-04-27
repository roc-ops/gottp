package gottp

import (
	"errors"
	"strings"
	"testing"
)

func TestTemplateNotStreamableError_Is(t *testing.T) {
	err := &TemplateNotStreamableError{Reasons: []string{"because reasons"}}
	if !errors.Is(err, ErrTemplateNotStreamable) {
		t.Errorf("errors.Is should match ErrTemplateNotStreamable")
	}
}

func TestTemplateNotStreamableError_Message(t *testing.T) {
	err := &TemplateNotStreamableError{Reasons: []string{"a", "b"}}
	got := err.Error()
	if !strings.Contains(got, "a") || !strings.Contains(got, "b") {
		t.Errorf("error message %q should contain both reasons", got)
	}
	if !strings.Contains(got, ";") {
		t.Errorf("error message %q should join reasons with semicolon", got)
	}
}

func TestTemplateNotStreamableError_EmptyReasons(t *testing.T) {
	err := &TemplateNotStreamableError{}
	got := err.Error()
	if got == "" {
		t.Errorf("error message should not be empty even with no reasons")
	}
}

func TestWhyNotStreamable_Streamable(t *testing.T) {
	// Use a template that we know is streamable.
	tmpl := `<group name="entry*">
mac {{ mac | _start_ }}
ip {{ ip }}
</group>`
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	streamable, reasons := WhyNotStreamable(c)
	if !streamable {
		t.Errorf("expected streamable=true, got false; reasons: %v", reasons)
	}
	if len(reasons) != 0 {
		t.Errorf("expected no reasons when streamable, got: %v", reasons)
	}
}

func TestWhyNotStreamable_NotStreamable(t *testing.T) {
	// joinmatches makes it non-streamable.
	tmpl := `<group name="entry*">
desc {{ desc | joinmatches }}
</group>`
	c, err := CompileTemplate(tmpl)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	streamable, reasons := WhyNotStreamable(c)
	if streamable {
		t.Errorf("expected streamable=false")
	}
	if len(reasons) == 0 {
		t.Errorf("expected at least one reason")
	}
	joined := strings.Join(reasons, " ")
	if !strings.Contains(joined, "joinmatches") {
		t.Errorf("expected reason to mention joinmatches; got: %v", reasons)
	}
}
