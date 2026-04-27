package gottp

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTemplateNotStreamable is returned (wrapped) by ParseStream when the
// template's top-level groups don't all pass the streamability check.
// Use errors.Is(err, ErrTemplateNotStreamable) to match.
var ErrTemplateNotStreamable = errors.New("template is not streamable")

// TemplateNotStreamableError carries the per-group reasons explaining why
// a template failed the streamability check. errors.Is matches against
// ErrTemplateNotStreamable.
type TemplateNotStreamableError struct {
	Reasons []string
}

func (e *TemplateNotStreamableError) Error() string {
	if len(e.Reasons) == 0 {
		return "template is not streamable"
	}
	return fmt.Sprintf("template is not streamable: %s", strings.Join(e.Reasons, "; "))
}

func (e *TemplateNotStreamableError) Is(target error) bool {
	return target == ErrTemplateNotStreamable
}

func (e *TemplateNotStreamableError) Unwrap() error {
	return ErrTemplateNotStreamable
}
