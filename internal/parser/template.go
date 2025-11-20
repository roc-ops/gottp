package parser

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Template represents a parsed TTP template
type Template struct {
	Name          string
	BasePath      string
	ResultsMethod string // "per_input" or "per_template"
	PathChar      string // character to separate path items, default "."
	Doc           string
	Vars          map[string]interface{}
	VarsWithName  []*VarsWithName // vars tags with name attribute
	Groups        []*Group
	Inputs        []*Input
	Outputs       []*Output
	Lookups       []*Lookup
	Macros        []*Macro
	Templates     []*Template // child templates
	Extends       []*Extend
}

// VarsWithName represents a vars tag with a name attribute
type VarsWithName struct {
	Name       string                 // name attribute (path in result structure)
	Vars       map[string]interface{} // parsed vars
	Load       string                 // load attribute (python, yaml, json, etc.)
	Attributes map[string]string      // all attributes
}

// Group represents a parsing group
type Group struct {
	Name       string
	Input      string
	Output     string
	Method     string
	Functions  string
	Chain      string
	Macro      string
	Pattern    string
	Groups     []*Group // nested groups
	Attributes map[string]string
}

// Input represents an input tag
type Input struct {
	Name            string
	Groups          []string
	Load            string
	URL             string
	Extensions      []string
	Filters         []string
	Functions       string   // pipe-separated list of input functions
	Macro           string   // comma-separated list of macro function names
	ExtractCommands []string // comma-separated list of commands to extract
	Content         string
	Attributes      map[string]string
}

// Output represents an output tag
type Output struct {
	Name       string
	Format     string
	Functions  string
	Path       string
	Returner   string
	Headers    string
	Content    string
	Attributes map[string]string
}

// Lookup represents a lookup table
type Lookup struct {
	Name       string
	Load       string
	Include    string
	Key        string
	Content    string
	Attributes map[string]string
}

// Macro represents a macro tag
type Macro struct {
	Language   string // "starlark", "javascript", "python" (default: "starlark")
	Content    string
	Attributes map[string]string
}

// Extend represents an extend tag
type Extend struct {
	Template   string
	Macro      string
	Groups     []string
	Inputs     []string
	Vars       []string
	Lookups    []string
	Outputs    []string
	Attributes map[string]string
}

// ParseTemplate parses a TTP template from XML text
func ParseTemplate(templateText string) (*Template, error) {
	// Wrap in <template> tag if not present
	trimmed := strings.TrimSpace(templateText)
	if !strings.HasPrefix(trimmed, "<template") && !strings.HasPrefix(trimmed, "<group") {
		templateText = "<template>\n" + templateText + "\n</template>"
	}

	// Parse XML
	decoder := xml.NewDecoder(strings.NewReader(templateText))
	decoder.Strict = false

	var root xmlElement
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Try wrapping in template tag
			templateText = "<template>\n" + templateText + "\n</template>"
			decoder = xml.NewDecoder(strings.NewReader(templateText))
			tok, err = decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("failed to parse template XML: %w", err)
			}
		}

		switch t := tok.(type) {
		case xml.StartElement:
			root = parseXMLElement(t, decoder)
		}
	}

	// Ensure root is template tag
	if root.XMLName.Local != "template" {
		// Wrap content in template tag
		root = xmlElement{
			XMLName:  xml.Name{Local: "template"},
			Children: []xmlElement{root},
		}
	}

	tmpl := &Template{
		Name:          "_root_template_",
		ResultsMethod: "per_input",
		PathChar:      ".",
		Vars:          make(map[string]interface{}),
		Groups:        []*Group{},
		Inputs:        []*Input{},
		Outputs:       []*Output{},
		Lookups:       []*Lookup{},
		Macros:        []*Macro{},
		Templates:     []*Template{},
		Extends:       []*Extend{},
	}

	// Extract template attributes
	for key, value := range root.Attrs {
		switch key {
		case "name":
			tmpl.Name = value
		case "base_path":
			tmpl.BasePath = value
		case "results":
			tmpl.ResultsMethod = value
		case "pathchar":
			tmpl.PathChar = value
		}
	}

	// Parse child elements
	if err := tmpl.parseElements(root.Children); err != nil {
		return nil, err
	}

	// If no groups were parsed but there is content in root.Content, create an implicit anonymous group
	// This handles templates like "description {{ description | join('.') }}" without explicit <group> tags
	if len(tmpl.Groups) == 0 && strings.TrimSpace(root.Content) != "" {
		content := strings.TrimSpace(root.Content)
		// Create an implicit anonymous group
		implicitGroup := &Group{
			Name:       "", // Empty name = anonymous group
			Pattern:    content,
			Groups:     []*Group{},
			Attributes: make(map[string]string),
		}
		tmpl.Groups = append(tmpl.Groups, implicitGroup)
	}

	return tmpl, nil
}

// parseElements parses child XML elements
func (t *Template) parseElements(elements []xmlElement) error {
	for _, elem := range elements {
		tagName := strings.ToLower(elem.XMLName.Local)

		switch tagName {
		case "template":
			childTmpl, err := parseChildTemplate(elem)
			if err != nil {
				return err
			}
			t.Templates = append(t.Templates, childTmpl)

		case "v", "vars", "variables":
			if err := t.parseVars(elem); err != nil {
				return err
			}

		case "g", "grp", "group":
			group, err := t.parseGroup(elem)
			if err != nil {
				return err
			}
			t.Groups = append(t.Groups, group)

		case "i", "in", "input":
			input, err := t.parseInput(elem)
			if err != nil {
				return err
			}
			t.Inputs = append(t.Inputs, input)

		case "o", "out", "output":
			output, err := t.parseOutput(elem)
			if err != nil {
				return err
			}
			t.Outputs = append(t.Outputs, output)

		case "lookup":
			lookup, err := t.parseLookup(elem)
			if err != nil {
				return err
			}
			t.Lookups = append(t.Lookups, lookup)

		case "macro":
			macro, err := t.parseMacro(elem)
			if err != nil {
				return err
			}
			t.Macros = append(t.Macros, macro)

		case "doc":
			t.Doc += elem.Content + "\n"

		case "extend":
			extend, err := t.parseExtend(elem)
			if err != nil {
				return err
			}
			t.Extends = append(t.Extends, extend)

		default:
			// Unknown tag, log warning but continue
			// TODO: add logging
		}
	}
	return nil
}

// parseGroup parses a group element
func (t *Template) parseGroup(elem xmlElement) (*Group, error) {
	group := &Group{
		Attributes: make(map[string]string),
		Groups:     []*Group{},
	}

	// Extract attributes
	for key, value := range elem.Attrs {
		group.Attributes[key] = value

		switch key {
		case "name":
			group.Name = value
		case "input":
			group.Input = value
		case "output":
			group.Output = value
		case "method":
			group.Method = value
		case "functions":
			group.Functions = value
		case "chain":
			group.Chain = value
		case "macro":
			group.Macro = value
		case "void":
			// void attribute - if present (even empty), group results should be skipped
			// Store in attributes map, will be checked in runtime
		}
	}

	// Set pattern/content
	group.Pattern = elem.Content

	// Parse nested groups
	for _, child := range elem.Children {
		if strings.ToLower(child.XMLName.Local) == "group" ||
			strings.ToLower(child.XMLName.Local) == "g" ||
			strings.ToLower(child.XMLName.Local) == "grp" {
			nestedGroup, err := t.parseGroup(child)
			if err != nil {
				return nil, err
			}
			group.Groups = append(group.Groups, nestedGroup)
		}
	}

	return group, nil
}

// parseInput parses an input element
func (t *Template) parseInput(elem xmlElement) (*Input, error) {
	input := &Input{
		Name:       "Default_Input",
		Groups:     []string{"all"},
		Attributes: make(map[string]string),
	}

	// Extract attributes
	for key, value := range elem.Attrs {
		input.Attributes[key] = value

		switch key {
		case "name":
			input.Name = value
		case "groups":
			input.Groups = strings.Split(value, ",")
			for i := range input.Groups {
				input.Groups[i] = strings.TrimSpace(input.Groups[i])
			}
		case "load":
			input.Load = value
		case "url":
			input.URL = value
		case "extensions":
			input.Extensions = strings.Split(value, ",")
			for i := range input.Extensions {
				input.Extensions[i] = strings.TrimSpace(input.Extensions[i])
			}
		case "filters":
			input.Filters = strings.Split(value, ",")
			for i := range input.Filters {
				input.Filters[i] = strings.TrimSpace(input.Filters[i])
			}
		case "functions":
			input.Functions = value
		case "macro":
			input.Macro = value
		case "extract_commands":
			// extract_commands is a special attribute that maps to a function
			input.ExtractCommands = strings.Split(value, ",")
			for i := range input.ExtractCommands {
				input.ExtractCommands[i] = strings.TrimSpace(input.ExtractCommands[i])
			}
		}
	}

	input.Content = elem.Content
	return input, nil
}

// parseOutput parses an output element
func (t *Template) parseOutput(elem xmlElement) (*Output, error) {
	output := &Output{
		Attributes: make(map[string]string),
	}

	// Extract attributes
	for key, value := range elem.Attrs {
		output.Attributes[key] = value

		switch key {
		case "name":
			output.Name = value
		case "format":
			output.Format = value
		case "functions":
			output.Functions = value
		case "path":
			output.Path = value
		case "returner":
			output.Returner = value
		case "headers":
			output.Headers = value
		}
	}

	output.Content = elem.Content
	return output, nil
}

// parseLookup parses a lookup element
func (t *Template) parseLookup(elem xmlElement) (*Lookup, error) {
	lookup := &Lookup{
		Load:       "python",
		Attributes: make(map[string]string),
	}

	// Extract attributes
	for key, value := range elem.Attrs {
		lookup.Attributes[key] = value

		switch key {
		case "name":
			lookup.Name = value
		case "load":
			lookup.Load = value
		case "include":
			lookup.Include = value
		case "key":
			lookup.Key = value
		}
	}

	lookup.Content = elem.Content
	return lookup, nil
}

// parseMacro parses a macro element
func (t *Template) parseMacro(elem xmlElement) (*Macro, error) {
	macro := &Macro{
		Language:   "starlark", // default
		Attributes: make(map[string]string),
	}

	// Extract attributes
	for key, value := range elem.Attrs {
		macro.Attributes[key] = value

		switch key {
		case "language":
			macro.Language = value
		}
	}

	macro.Content = elem.Content
	return macro, nil
}

// parseExtend parses an extend element
func (t *Template) parseExtend(elem xmlElement) (*Extend, error) {
	extend := &Extend{
		Attributes: make(map[string]string),
	}

	// Extract attributes
	for key, value := range elem.Attrs {
		extend.Attributes[key] = value

		switch key {
		case "template":
			extend.Template = value
		case "macro":
			extend.Macro = value
		case "groups":
			extend.Groups = strings.Split(value, ",")
			for i := range extend.Groups {
				extend.Groups[i] = strings.TrimSpace(extend.Groups[i])
			}
		case "inputs":
			extend.Inputs = strings.Split(value, ",")
			for i := range extend.Inputs {
				extend.Inputs[i] = strings.TrimSpace(extend.Inputs[i])
			}
		case "vars":
			extend.Vars = strings.Split(value, ",")
			for i := range extend.Vars {
				extend.Vars[i] = strings.TrimSpace(extend.Vars[i])
			}
		case "lookups":
			extend.Lookups = strings.Split(value, ",")
			for i := range extend.Lookups {
				extend.Lookups[i] = strings.TrimSpace(extend.Lookups[i])
			}
		case "outputs":
			extend.Outputs = strings.Split(value, ",")
			for i := range extend.Outputs {
				extend.Outputs[i] = strings.TrimSpace(extend.Outputs[i])
			}
		}
	}

	return extend, nil
}

// parseVars parses a vars element
func (t *Template) parseVars(elem xmlElement) error {
	// Vars can be in different formats (python, yaml, json, etc.)
	// For now, we'll store the content and parse it later based on load attribute
	load := "python" // default
	name := ""       // name attribute (path in result structure)
	attrs := make(map[string]string)

	for key, value := range elem.Attrs {
		attrs[key] = value
		if key == "load" {
			load = value
		}
		if key == "name" {
			name = value
		}
		if key == "include" {
			// TODO: load from file
		}
	}

	// If name attribute is present, store vars separately
	if name != "" {
		varsWithName := &VarsWithName{
			Name:       name,
			Load:       load,
			Attributes: attrs,
			Vars:       make(map[string]interface{}),
		}
		// Store raw content for parsing later
		varsWithName.Vars["_raw_content_"] = elem.Content
		varsWithName.Vars["_load_"] = load

		if t.VarsWithName == nil {
			t.VarsWithName = []*VarsWithName{}
		}
		t.VarsWithName = append(t.VarsWithName, varsWithName)
	} else {
		// Store raw content for now, will be parsed later (vars without name attribute)
		if t.Vars == nil {
			t.Vars = make(map[string]interface{})
		}
		t.Vars["_raw_content_"] = elem.Content
		t.Vars["_load_"] = load
	}

	return nil
}

// parseChildTemplate parses a child template element
func parseChildTemplate(elem xmlElement) (*Template, error) {
	// Convert element back to XML string for parsing
	// This is a simplified approach - in production we'd want to be more efficient
	var buf strings.Builder
	buf.WriteString("<template")
	for key, value := range elem.Attrs {
		buf.WriteString(fmt.Sprintf(` %s="%s"`, key, value))
	}
	buf.WriteString(">")
	buf.WriteString(elem.Content)

	// Recursively process children
	for _, child := range elem.Children {
		// TODO: properly serialize child elements
		_ = child
	}

	buf.WriteString("</template>")

	return ParseTemplate(buf.String())
}

// xmlElement represents an XML element for parsing
type xmlElement struct {
	XMLName  xml.Name
	Attrs    map[string]string
	Content  string
	Children []xmlElement
}

// parseXMLElement recursively parses an XML element
func parseXMLElement(start xml.StartElement, decoder *xml.Decoder) xmlElement {
	elem := xmlElement{
		XMLName:  start.Name,
		Attrs:    make(map[string]string),
		Children: []xmlElement{},
	}

	// Extract attributes
	for _, attr := range start.Attr {
		elem.Attrs[attr.Name.Local] = attr.Value
	}

	// Parse children and content
	var contentParts []string
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			child := parseXMLElement(t, decoder)
			elem.Children = append(elem.Children, child)
		case xml.EndElement:
			if t.Name == start.Name {
				elem.Content = strings.Join(contentParts, "")
				return elem
			}
		case xml.CharData:
			contentParts = append(contentParts, string(t))
		}
	}

	elem.Content = strings.Join(contentParts, "")
	return elem
}
