package parser

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
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

	// TopText is the template's text outside any child tag, kept while the
	// template's only child tags are <extend>: Python TTP appends the text
	// of an extended template that is only text to it, and makes it an
	// implicit _anonymous_ group only if no tag is left (see
	// TopTextPending and applyTopText).
	TopText string
	// TopTextPending reports that the template's only child tags are
	// <extend>, so whether TopText becomes an implicit _anonymous_ group is
	// decided when the extends are resolved.
	TopTextPending bool
	// Warnings lists non-fatal issues found while parsing, such as text
	// outside any <group> that is ignored.
	Warnings []string
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
	Name      string
	Input     string
	Output    string
	Method    string
	Functions string
	Chain     string
	Macro     string
	Pattern   string
	// TailStart is the byte offset in Pattern where the first child group
	// begins, or -1 when the group has no child groups. Pattern is the
	// group's own text followed by the text after each child (the XML tails);
	// Python TTP treats every line from TailStart on as a tail line, which is
	// never a start pattern.
	TailStart  int
	Groups     []*Group // nested groups
	Attributes map[string]string
	// AttrOrder lists Attributes' names in template order: TTP runs a
	// group's function attributes in that order.
	AttrOrder []string
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
	Name      string
	Format    string
	Functions string
	Path      string
	Returner  string
	Headers   string
	// Macro is the macro attribute: macro names run on the results like
	// functions="macro('name')" (Python TTP output macro()).
	Macro string
	// MacroBeforeFunctions reports whether the macro attribute comes before
	// the functions attribute in the tag. Python TTP runs output functions
	// in attribute order (ttp/ttp.py _outputter_class.get_attributes).
	MacroBeforeFunctions bool
	Content              string
	Attributes           map[string]string
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

// knownTags lists the XML element names that are valid in TTP templates.
// Any '<' not followed by one of these (or '/', '!--', '?') is bare content
// and must be escaped before XML parsing.
var knownTags = []string{
	"template", "group", "vars", "variables",
	"input", "output", "lookup", "macro",
	"doc", "extend", "_root_",
}

// escapeContentBlocks escapes bare < characters that are not part of known TTP
// XML tags. It scans the template text in a single pass, checking each '<' to
// see if it starts a recognized tag. Bare '<' (from re() patterns, Starlark
// code, doc text, CLI values) are replaced with &lt;.
// The XML decoder automatically unescapes &lt; back to '<' in CharData.
func escapeContentBlocks(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '<' {
			// Skip CDATA sections entirely — content is literal
			if strings.HasPrefix(text[i:], "<![CDATA[") {
				end := strings.Index(text[i:], "]]>")
				if end >= 0 {
					b.WriteString(text[i : i+end+3])
					i += end + 2 // loop increments i
					continue
				}
			}
			if isKnownTagStart(text, i) {
				b.WriteByte('<')
			} else {
				b.WriteString("&lt;")
			}
		} else {
			b.WriteByte(text[i])
		}
	}
	return b.String()
}

// isKnownTagStart checks if the '<' at position i in text starts a known
// TTP XML tag, closing tag, comment, or processing instruction.
func isKnownTagStart(text string, i int) bool {
	rest := text[i+1:] // everything after '<'
	if len(rest) == 0 {
		return false
	}
	// XML comment: <!-- or CDATA: <![CDATA[
	if rest[0] == '!' {
		return true
	}
	// XML processing instruction: <?
	if rest[0] == '?' {
		return true
	}
	// Closing tag: </tagname
	closing := false
	if rest[0] == '/' {
		closing = true
		rest = rest[1:]
		if len(rest) == 0 {
			return false
		}
	}
	_ = closing
	// Check if rest starts with a known tag name followed by whitespace, '>', or '/'
	for _, tag := range knownTags {
		if strings.HasPrefix(rest, tag) {
			// Must be followed by whitespace, '>', '/', or end of string
			afterTag := len(tag)
			if afterTag >= len(rest) {
				return true // tag at end of string
			}
			c := rest[afterTag]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '>' || c == '/' {
				return true
			}
		}
	}
	return false
}

// ParseTemplate parses a TTP template from XML text
func ParseTemplate(templateText string) (*Template, error) {
	// Escape bare < and > inside <macro> and <doc> blocks before XML parsing
	templateText = escapeContentBlocks(templateText)

	// Wrap in <template> tag if not present
	trimmed := strings.TrimSpace(templateText)
	if !strings.HasPrefix(trimmed, "<template") && !strings.HasPrefix(trimmed, "<group") {
		templateText = "<template>\n" + templateText + "\n</template>"
	}

	// Parse XML - collect all top-level elements to handle multiple <template> sections.
	// Multiple sibling <template> elements are not valid XML (no single root), so we
	// wrap them in a synthetic root element before parsing.
	wrappedText := "<_root_>" + templateText + "</_root_>"
	decoder := xml.NewDecoder(strings.NewReader(wrappedText))
	decoder.Strict = false

	var topLevelElements []xmlElement
	var strayText strings.Builder // text beside the top-level elements
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Fallback: try the original unwrapped approach for backward
			// compatibility. A second error is final: wrapping the text
			// again and starting over (as gottp once did) never ends,
			// growing the text each time (#144).
			topLevelElements = nil
			decoder2 := xml.NewDecoder(strings.NewReader(templateText))
			decoder2.Strict = false
			for {
				tok2, err2 := decoder2.Token()
				if err2 == io.EOF {
					break
				}
				if err2 != nil {
					return nil, fmt.Errorf("failed to parse template XML: %w", err2)
				}
				switch t := tok2.(type) {
				case xml.StartElement:
					topLevelElements = append(topLevelElements, parseXMLElement(t, decoder2))
				}
			}
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "_root_" {
				// Parse children of synthetic root
				for {
					innerTok, innerErr := decoder.Token()
					if innerErr == io.EOF {
						break
					}
					if innerErr != nil {
						break
					}
					switch it := innerTok.(type) {
					case xml.StartElement:
						topLevelElements = append(topLevelElements, parseXMLElement(it, decoder))
					case xml.CharData:
						strayText.Write(it)
					case xml.EndElement:
						if it.Name.Local == "_root_" {
							goto doneParsingRoot
						}
					}
				}
			} else {
				topLevelElements = append(topLevelElements, parseXMLElement(t, decoder))
			}
		}
	}
doneParsingRoot:

	// Handle the case where we found multiple top-level <template> elements.
	// Python TTP behavior: multiple <template> tags create independent template sections,
	// each processed separately, with results merged.
	var root xmlElement
	templateElements := []xmlElement{}
	nonTemplateElements := []xmlElement{}

	for _, elem := range topLevelElements {
		if strings.ToLower(elem.XMLName.Local) == "template" {
			templateElements = append(templateElements, elem)
		} else {
			nonTemplateElements = append(nonTemplateElements, elem)
		}
	}

	if len(templateElements) > 1 {
		// Multiple <template> sections: create a synthetic root template that
		// contains them as child templates. This matches Python TTP's behavior
		// where each <template> section is processed independently.
		root = xmlElement{
			XMLName:  xml.Name{Local: "template"},
			Attrs:    make(map[string]string),
			Children: append(nonTemplateElements, templateElements...),
		}
	} else if len(templateElements) == 1 {
		// Single <template> section - use it as the root
		root = templateElements[0]
		// If there are non-template elements, add them as children
		if len(nonTemplateElements) > 0 {
			root.Children = append(nonTemplateElements, root.Children...)
		}
	} else if len(topLevelElements) == 1 {
		// No <template> elements found - use the element as root
		root = topLevelElements[0]
	} else if len(topLevelElements) > 1 {
		// Several top-level elements without <template> (a template that
		// starts with <group>, which isn't wrapped above): they are all the
		// template's, as in Python TTP, which wraps such a template in
		// <template>. Only the first was kept before.
		root = xmlElement{
			XMLName:  xml.Name{Local: "template"},
			Attrs:    make(map[string]string),
			Children: topLevelElements,
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

	tmpl.applyTopText(root.Content, root.Children, false)
	// Text beside the top-level tags (e.g. after a top-level <group> or
	// <template>): Python TTP wraps the template in <template>, where that
	// text sits beside tags, so it is ignored.
	if line := FirstTextLine(strayText.String()); line != "" {
		tmpl.Warnings = append(tmpl.Warnings, TopTextIgnoredWarning(line))
	}

	return tmpl, nil
}

// TopTextIgnoredWarning is the compile warning for text outside any
// <group> that is ignored because the template has XML tags; line is the
// first ignored line.
func TopTextIgnoredWarning(line string) string {
	return fmt.Sprintf("top-level text outside any <group> is ignored because the template has XML tags (as in Python TTP): %q", line)
}

// nestedTopTextIgnoredWarning is the compile warning for text outside any
// <group> in a nested <template>, which Python TTP never parses.
func nestedTopTextIgnoredWarning(line string) string {
	return fmt.Sprintf("text outside any <group> in a nested <template> is ignored (as in Python TTP): %q", line)
}

// FirstTextLine returns the first non-blank line of text, trimmed, or "".
func FirstTextLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

// NewImplicitAnonymousGroup returns the group Python TTP makes of a
// template's text when the template has no child tags.
func NewImplicitAnonymousGroup(text string) *Group {
	// Python TTP wraps such a template as <group name="_anonymous_">
	// (ttp/ttp.py:1396-1401), not as an unnamed <group>: the path has no
	// "*", so one match gives a dict and several give a list (#62).
	return &Group{
		Name:       "_anonymous_",
		Pattern:    strings.TrimRight(strings.TrimLeft(text, "\r\n"), " \t\r\n"),
		TailStart:  -1,
		Groups:     []*Group{},
		Attributes: make(map[string]string),
	}
}

// applyTopText ports Python TTP's rule for a template's text outside any
// child tag (ttp/ttp.py construct_etree, "if not list(template_ET)"): it
// becomes an implicit _anonymous_ group only when the template has no
// child tags at all. With any tag present (<vars>, <macro>, <doc>,
// <input>, <group>, ...) the text is ignored, and a warning says so.
//
// When the only child tags are <extend>, the decision waits for extend
// resolution: TTP appends the text of an extended template that is only
// text to this text and removes the tag, and other extended templates add
// tags.
//
// A nested <template> never gets an implicit group: TTP builds it from
// ET.tostring(element, encoding="UTF-8"), bytes that construct_etree
// formats as "b'...'" when it wraps them, so its text never parses.
func (t *Template) applyTopText(text string, children []xmlElement, nested bool) {
	line := FirstTextLine(text)
	if line == "" {
		return
	}
	switch {
	case nested:
		t.Warnings = append(t.Warnings, nestedTopTextIgnoredWarning(line))
	case len(children) == 0 && unknownTag(text) != "":
		// gottp reads an unknown tag as text, but Python TTP parses it as
		// an (invalid) child element, so the template has a tag and the
		// text is not an implicit group.
		tag := unknownTag(text)
		t.Warnings = append(t.Warnings, fmt.Sprintf("unknown tag <%s> is ignored (Python TTP logs \"invalid tag\")", tag))
		t.Warnings = append(t.Warnings, TopTextIgnoredWarning(line))
	case len(children) == 0:
		t.Groups = append(t.Groups, NewImplicitAnonymousGroup(text))
	case onlyExtendTags(children):
		t.TopText = text
		t.TopTextPending = true
	default:
		t.Warnings = append(t.Warnings, TopTextIgnoredWarning(line))
	}
}

// unknownTagRe finds a well-formed element with an unknown tag name at the
// start of a line: <name ...>...</name> or <name .../>. gottp escapes such
// tags into text (see escapeContentBlocks), while Python TTP's XML parser
// sees them as child elements. Bare '<' in template text (for example
// "for <{{ vr }}>") doesn't match, because a tag name starts with a letter
// or '_' and needs a closing tag or "/>".
var unknownTagRe = regexp.MustCompile(`(?m)^[ \t]*<([A-Za-z_][\w.-]*)(?:\s[^<>]*)?(/>|>[\s\S]*?</([A-Za-z_][\w.-]*)\s*>)`)

// unknownTag returns the name of the first well-formed unknown element in
// text (see unknownTagRe), or "" if there is none.
func unknownTag(text string) string {
	for _, m := range unknownTagRe.FindAllStringSubmatch(text, -1) {
		if m[2] == "/>" || m[3] == m[1] {
			return m[1]
		}
	}
	return ""
}

// onlyExtendTags reports whether every element is an <extend> tag.
func onlyExtendTags(elements []xmlElement) bool {
	for _, elem := range elements {
		if strings.ToLower(elem.XMLName.Local) != "extend" {
			return false
		}
	}
	return true
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
			t.Warnings = append(t.Warnings, childTmpl.Warnings...)

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
		AttrOrder:  elem.AttrOrder,
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
	group.TailStart = -1

	// Parse nested groups
	for childIdx, child := range elem.Children {
		if strings.ToLower(child.XMLName.Local) == "group" ||
			strings.ToLower(child.XMLName.Local) == "g" ||
			strings.ToLower(child.XMLName.Local) == "grp" {
			if group.TailStart < 0 && childIdx < len(elem.ChildOffsets) {
				group.TailStart = elem.ChildOffsets[childIdx]
			}
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
		case "macro":
			output.Macro = value
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

	if output.Macro != "" {
		for _, key := range elem.AttrOrder {
			if key == "macro" || key == "functions" {
				output.MacroBeforeFunctions = key == "macro"
				break
			}
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
	// Instead of re-serializing to XML and re-parsing (which loses child elements),
	// directly build the Template struct from the already-parsed xmlElement.
	childTmpl := &Template{
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
	for key, value := range elem.Attrs {
		switch key {
		case "name":
			childTmpl.Name = value
		case "base_path":
			childTmpl.BasePath = value
		case "results":
			childTmpl.ResultsMethod = value
		case "pathchar":
			childTmpl.PathChar = value
		}
	}

	// Parse child elements (groups, vars, inputs, outputs, etc.)
	if err := childTmpl.parseElements(elem.Children); err != nil {
		return nil, err
	}

	childTmpl.applyTopText(elem.Content, elem.Children, true)

	return childTmpl, nil
}

// xmlElement represents an XML element for parsing
type xmlElement struct {
	XMLName xml.Name
	Attrs   map[string]string
	// AttrOrder lists the attribute names in document order.
	AttrOrder []string
	Content   string
	Children  []xmlElement
	// ChildOffsets[i] is the length of Content when Children[i] started.
	ChildOffsets []int
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
		if _, dup := elem.Attrs[attr.Name.Local]; !dup {
			elem.AttrOrder = append(elem.AttrOrder, attr.Name.Local)
		}
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
			contentLen := 0
			for _, part := range contentParts {
				contentLen += len(part)
			}
			child := parseXMLElement(t, decoder)
			elem.Children = append(elem.Children, child)
			elem.ChildOffsets = append(elem.ChildOffsets, contentLen)
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
