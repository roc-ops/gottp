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
