package python

import (
	"fmt"

	"github.com/roc-ops/gottp/internal/compiled"
	"github.com/roc-ops/gottp/internal/compiler"
	"github.com/roc-ops/gottp/internal/parser"
)

// Parser provides a Python-compatible API for TTP
type Parser struct {
	templates []*compiler.CompiledTemplate
	inputs    map[string]string
	vars      map[string]interface{}
	results   []interface{}
}

// NewParser creates a new Python-compatible parser
func NewParser() *Parser {
	return &Parser{
		templates: []*compiler.CompiledTemplate{},
		inputs:    make(map[string]string),
		vars:      make(map[string]interface{}),
		results:   []interface{}{},
	}
}

// AddTemplate adds a template to the parser
func (p *Parser) AddTemplate(templateText string) error {
	// Parse template
	tmpl, err := parser.ParseTemplate(templateText)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Compile template
	comp := compiler.NewCompiler()
	compiled, err := comp.CompileTemplate(tmpl)
	if err != nil {
		return fmt.Errorf("failed to compile template: %w", err)
	}

	p.templates = append(p.templates, compiled)
	return nil
}

// AddInput adds input data to the parser
func (p *Parser) AddInput(data, inputName string) {
	if inputName == "" {
		inputName = "Default_Input"
	}
	p.inputs[inputName] = data
}

// AddVars adds variables to the parser
func (p *Parser) AddVars(vars map[string]interface{}) {
	if p.vars == nil {
		p.vars = make(map[string]interface{})
	}
	for k, v := range vars {
		p.vars[k] = v
	}
}

// Parse parses all inputs with all templates
func (p *Parser) Parse() error {
	p.results = []interface{}{}

	for _, template := range p.templates {
		runtime := compiled.NewRuntime(template)
		
		// Convert inputs
		inputMap := make(map[string]string)
		for k, v := range p.inputs {
			inputMap[k] = v
		}
		
		// Parse
		result, err := runtime.Parse(inputMap, p.vars, nil)
		if err != nil {
			return fmt.Errorf("failed to parse with template: %w", err)
		}
		
		p.results = append(p.results, result)
	}

	return nil
}

// Result returns parsing results
func (p *Parser) Result() []interface{} {
	return p.results
}

// ClearInput clears all input data
func (p *Parser) ClearInput() {
	p.inputs = make(map[string]string)
}

// ClearResult clears all results
func (p *Parser) ClearResult() {
	p.results = []interface{}{}
}

