package pattern

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/roc-ops/gottp/internal/call"
	"github.com/roc-ops/gottp/internal/pymethod"
)

// HeadersLine builds the regex and variables of a template line that ends
// with {{ _headers_ }}, a port of Python TTP 0.10.1's regex_headers
// (ttp/ttp.py:2239). The text before {{ _headers_ }} is a table's header row;
// each header becomes a variable that captures its column by position:
//
//	Port      Name    Status {{ _headers_ }}
//	^(.{10})(.{1,8})(.*)$       Port, Name, Status
//
// The first columns-1 columns are exactly as wide as their header (the
// header and the spaces after it), column number columns is 1 up to that
// width, the remaining columns are 0 up to their width, and the last column
// takes the rest of the line (at least one character when it is a mandatory
// column). columns defaults to the number of headers minus 2 and is set with
// {{ _headers_ | columns(N) }}. Each header variable runs strip and
// exclude(header), so the header row itself is not a match; as in TTP, any
// row with a header's name inside that header's column is excluded too.
//
// ok is false when the line has no _headers_ variable. It is an error for
// the line to have other variables, no headers, two headers with the same
// name, or a header named like a gottp indicator (ignore, _start_, ...).
// Unlike TTP, which fails on them, one or two headers work (columns is at
// least 1) and a header need not be a valid Python identifier.
func (e *Engine) HeadersLine(line string, variables []*MatchVariable) (regex string, vars []*MatchVariable, ok bool, err error) {
	var headersVar *MatchVariable
	for _, v := range variables {
		if v.Name == IndicatorHeaders {
			headersVar = v
		}
	}
	if headersVar == nil {
		return "", nil, false, nil
	}
	if len(variables) > 1 {
		return "", nil, true, fmt.Errorf("line %q: {{ _headers_ }} must be the only variable on its line", line)
	}

	// TTP: self.regex = self.LINE[:index].rstrip(), the text before the
	// variable; text after it is dropped.
	head := strings.TrimRightFunc(line[:strings.Index(line, "{{")], pymethod.IsSpace)
	headers := splitHeaders(head)
	if len(headers) == 0 {
		return "", nil, true, fmt.Errorf("line %q: {{ _headers_ }} has no headers before it", line)
	}

	columns := max(1, len(headers)-2)
	for _, f := range headersVar.Functions {
		c, perr := call.Parse(f)
		if perr != nil || c.Name != "columns" {
			continue
		}
		if len(c.Args) == 0 {
			return "", nil, true, fmt.Errorf("line %q: columns needs the number of columns", line)
		}
		n, ierr := pyInt(c.Args[0])
		if ierr != nil {
			return "", nil, true, fmt.Errorf("line %q: columns(%v): %w", line, c.Args[0], ierr)
		}
		columns = min(max(1, n), len(headers))
	}

	// The header row's indent is part of the first column. TTP counts it
	// after replacing each tab with four spaces (tabs inside the headers
	// count as one character).
	expanded := strings.ReplaceAll(head, "\t", "    ")
	indent := utf8.RuneCountInString(expanded) - utf8.RuneCountInString(strings.TrimLeftFunc(expanded, pymethod.IsSpace))
	headers[0] = strings.Repeat(" ", indent) + headers[0]

	seen := make(map[string]bool, len(headers))
	parts := make([]string, len(headers))
	vars = make([]*MatchVariable, len(headers))
	for i, h := range headers {
		name := strings.TrimFunc(h, pymethod.IsSpace)
		if seen[name] {
			return "", nil, true, fmt.Errorf("line %q: duplicate header %q", line, name)
		}
		seen[name] = true
		if IsIndicatorVar(name) || IsNonCapturingVar(name) || name == IndicatorHeaders {
			return "", nil, true, fmt.Errorf("line %q: header %q is a reserved variable name", line, name)
		}
		width := utf8.RuneCountInString(h)
		switch {
		case i < columns-1:
			parts[i] = fmt.Sprintf(".{%d}", width)
		case i == columns-1:
			parts[i] = fmt.Sprintf(".{1,%d}", width)
		default:
			parts[i] = fmt.Sprintf(".{0,%d}", width)
		}
		vars[i] = &MatchVariable{
			Name:      name,
			Functions: []string{"strip", "exclude(" + pyStrLiteral(name) + ")"},
		}
	}
	if columns >= len(headers) {
		parts[len(parts)-1] = ".{1,}"
	} else {
		parts[len(parts)-1] = ".*"
	}
	for i := range vars {
		vars[i].Pattern = parts[i]
	}
	return "^(" + strings.Join(parts, ")(") + ")$", vars, true, nil
}

// splitHeaders is TTP's re.findall(r"(\S+\s+|\S+)", line): each header with
// the whitespace after it. Leading whitespace belongs to no header.
func splitHeaders(line string) []string {
	var headers []string
	start := -1
	inSpace := false
	for i, r := range line {
		space := pymethod.IsSpace(r)
		switch {
		case start < 0:
			if !space {
				start = i
			}
		case inSpace && !space:
			headers = append(headers, line[start:i])
			start = i
			inSpace = false
		case space:
			inSpace = true
		}
	}
	if start >= 0 {
		headers = append(headers, line[start:])
	}
	return headers
}

// pyInt is Python's int() of a call argument.
func pyInt(v interface{}) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int64:
		return int(x), nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, fmt.Errorf("cannot convert %v to integer", x)
		}
		return int(x), nil
	case string:
		n, err := strconv.Atoi(strings.ReplaceAll(strings.TrimFunc(x, pymethod.IsSpace), "_", ""))
		if err != nil {
			return 0, fmt.Errorf("invalid literal for int(): %q", x)
		}
		return n, nil
	}
	return 0, fmt.Errorf("int() argument must be a string or a number, not %T", v)
}

// pyStrLiteral quotes s as a Python string literal.
func pyStrLiteral(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
