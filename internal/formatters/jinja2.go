package formatters

import (
	"bytes"
	"fmt"

	"github.com/flosch/pongo2/v6"
)

// Jinja2Formatter formats results using Jinja2-style templates
type Jinja2Formatter struct{}

// NewJinja2Formatter creates a new Jinja2 formatter
func NewJinja2Formatter() *Jinja2Formatter {
	return &Jinja2Formatter{}
}

// Jinja2Options contains options for Jinja2 formatting
type Jinja2Options struct {
	Template string // Jinja2 template string
}

// Format formats data using Jinja2 template
func (f *Jinja2Formatter) Format(data interface{}, template string) ([]byte, error) {
	if template == "" {
		return nil, fmt.Errorf("template is required for Jinja2 formatter")
	}

	// Compile template
	tpl, err := pongo2.FromString(template)
	if err != nil {
		return nil, fmt.Errorf("failed to compile Jinja2 template: %w", err)
	}

	// Prepare context - pass data as _data_ variable (matching Python TTP behavior)
	ctx := pongo2.Context{
		"_data_": data,
	}

	// Execute template
	var buf bytes.Buffer
	if err := tpl.ExecuteWriter(ctx, &buf); err != nil {
		return nil, fmt.Errorf("failed to execute Jinja2 template: %w", err)
	}

	return buf.Bytes(), nil
}

// FormatString formats data using Jinja2 template and returns as string
func (f *Jinja2Formatter) FormatString(data interface{}, template string) (string, error) {
	bytes, err := f.Format(data, template)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

