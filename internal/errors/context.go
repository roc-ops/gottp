package errors

import (
	"fmt"
	"strings"
)

// ContextError provides enhanced error context
type ContextError struct {
	Message    string
	Location   string
	LineNumber int
	Column     int
	Context    string
	Err        error
}

// Error implements the error interface
func (e *ContextError) Error() string {
	var parts []string

	if e.Location != "" {
		parts = append(parts, fmt.Sprintf("at %s", e.Location))
	}

	if e.LineNumber > 0 {
		if e.Column > 0 {
			parts = append(parts, fmt.Sprintf("line %d, column %d", e.LineNumber, e.Column))
		} else {
			parts = append(parts, fmt.Sprintf("line %d", e.LineNumber))
		}
	}

	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	if e.Context != "" {
		parts = append(parts, fmt.Sprintf("context: %s", e.Context))
	}

	if e.Err != nil {
		parts = append(parts, fmt.Sprintf("(%v)", e.Err))
	}

	return strings.Join(parts, ": ")
}

// Unwrap returns the underlying error
func (e *ContextError) Unwrap() error {
	return e.Err
}

// NewContextError creates a new context error
func NewContextError(message, location string, lineNumber int, err error) *ContextError {
	return &ContextError{
		Message:    message,
		Location:   location,
		LineNumber: lineNumber,
		Err:        err,
	}
}

// WithColumn adds column information to the error
func (e *ContextError) WithColumn(column int) *ContextError {
	e.Column = column
	return e
}

// WithContext adds context information to the error
func (e *ContextError) WithContext(context string) *ContextError {
	e.Context = context
	return e
}

// TemplateError represents a template-specific error
type TemplateError struct {
	TemplateName string
	TagName      string
	Attribute    string
	Value        string
	Message      string
	LineNumber   int
	Err          error
}

// Error implements the error interface
func (e *TemplateError) Error() string {
	var parts []string

	if e.TemplateName != "" {
		parts = append(parts, fmt.Sprintf("template '%s'", e.TemplateName))
	}

	if e.TagName != "" {
		parts = append(parts, fmt.Sprintf("tag <%s>", e.TagName))
	}

	if e.Attribute != "" {
		parts = append(parts, fmt.Sprintf("attribute '%s'", e.Attribute))
		if e.Value != "" {
			parts = append(parts, fmt.Sprintf("='%s'", e.Value))
		}
	}

	if e.LineNumber > 0 {
		parts = append(parts, fmt.Sprintf("line %d", e.LineNumber))
	}

	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	if e.Err != nil {
		parts = append(parts, fmt.Sprintf("(%v)", e.Err))
	}

	return strings.Join(parts, ": ")
}

// Unwrap returns the underlying error
func (e *TemplateError) Unwrap() error {
	return e.Err
}

// NewTemplateError creates a new template error
func NewTemplateError(templateName, tagName, attribute, value, message string, lineNumber int, err error) *TemplateError {
	return &TemplateError{
		TemplateName: templateName,
		TagName:      tagName,
		Attribute:    attribute,
		Value:        value,
		Message:      message,
		LineNumber:   lineNumber,
		Err:          err,
	}
}

// FunctionError represents a function-specific error
type FunctionError struct {
	FunctionName string
	Value        interface{}
	Args         []string
	Message      string
	Err          error
}

// Error implements the error interface
func (e *FunctionError) Error() string {
	var parts []string

	if e.FunctionName != "" {
		parts = append(parts, fmt.Sprintf("function '%s'", e.FunctionName))
	}

	if e.Message != "" {
		parts = append(parts, e.Message)
	}

	if e.Err != nil {
		parts = append(parts, fmt.Sprintf("(%v)", e.Err))
	}

	return strings.Join(parts, ": ")
}

// Unwrap returns the underlying error
func (e *FunctionError) Unwrap() error {
	return e.Err
}

// NewFunctionError creates a new function error
func NewFunctionError(functionName, message string, err error) *FunctionError {
	return &FunctionError{
		FunctionName: functionName,
		Message:      message,
		Err:          err,
	}
}

