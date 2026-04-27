# Go TTP - Template Text Parser for Go

A Go implementation of the Template Text Parser (TTP) library, providing semi-structured text parsing using templates with enhanced features.

## Features

- **Full TTP Compatibility**: Compatible with Python TTP templates and syntax
- **Stateless Compiled Templates**: Compile templates once, use many times without state resets ✅
- **Multi-Language Macros**: Support for Starlark (default), JavaScript, and Python (optional, requires build tag)
- **Dual API Design**: Python-compatible API for easy porting, plus Go-idiomatic API ✅
- **Thread-Safe**: Compiled templates are immutable and safe for concurrent use ✅
- **High Performance**: Leverages Go's compiled nature for better performance
- **Source Maps**: Optional source map generation to track which input lines/characters matched which template patterns (useful for editor visualization and debugging)

## Status

**Core Features Complete** ✅
- Template parsing (XML-based)
- Pattern engine (regex generation)
- Compiled template system
- Stateless parsing execution
- Multi-line pattern matching (_start_, _end_, _line_ indicators)
- Template extension (<extend> tag)
- All match functions (52/52) ✅
- All group functions (21/21) ✅
- All input loaders (8/8: text, yaml, json, csv, file, directory, url, database) ✅
- All output formatters (10/10: raw, json, yaml, csv, table, pprint, tabulate, excel, jinja2, n2g) ✅
- Macro execution engines (Starlark, JavaScript, Python) ✅
- Python-compatible API ✅
- Comprehensive test suite (102 comparison tests passing) ✅
- Complete documentation (Sphinx + Markdown) ✅

**Not Applicable** (Go-specific)
- Multi-processing support (Go uses goroutines - not needed)
- Lazy loader system (not needed in Go's compiled environment)

**Future Enhancements** 📋
- Enhanced error messages with detailed context
- Additional template validation options

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/roc-ops/gottp"
)

func main() {
    template := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>
`

    data := `
interface Loopback0
 ip address 192.168.0.113/24
 description Router-id-loopback
!
interface Vlan778
 ip address 2002::fd37/124
 description CPE_Acces_Vlan
!
`

    // Compile template once
    compiled, err := gottp.CompileTemplate(template)
    if err != nil {
        panic(err)
    }

    // Use many times with different inputs - no reset needed
    result, err := compiled.Parse(gottp.Inputs{
        "Default_Input": data,
    }, nil, nil)
    if err != nil {
        panic(err)
    }

    fmt.Printf("%+v\n", result)
}
```

## Streaming mode (ParseStream)

For templates with high-cardinality output (thousands of records) and a clear repeating record boundary, `ParseStream` emits records one at a time via callback, bounding peak heap usage:

```go
err := compiled.ParseStream(inputs, nil, nil,
    func(record map[string]interface{}, srcRange [2]int, groupPath string) error {
        return handle(record)
    })
```

Returns `*TemplateNotStreamableError` (matching `errors.Is(err, gottp.ErrTemplateNotStreamable)`) on templates that don't meet the streamability criteria. Use `gottp.WhyNotStreamable(compiled)` to audit a template before calling.

Streamability requires: top-level group, no nested children, no `joinmatches`, a record boundary (`_start_` indicator or fully line-anchored patterns), and per-record group functions only (no `itemize` / `expand`).

Measured on a 24 MB / 256K-record CMTS capture: peak heap drops from 752 MB (`Parse`) to 24 MB (`ParseStream`) — a 31× reduction.

See `docs/superpowers/specs/2026-04-27-streaming-parsegroup-design.md` for full design and rationale.

## Installation

```bash
go get github.com/roc-ops/gottp
```

## Documentation

- [User Guide](docs/USER_GUIDE.md) - Complete usage guide
- [Migration Guide](docs/MIGRATION_GUIDE.md) - Migrating from Python TTP
- [Python Macros](docs/PYTHON_MACROS.md) - Python macro setup and usage
- [API Documentation](https://pkg.go.dev/github.com/roc-ops/gottp) - Go package documentation

## Online Editor

Try GoTTP in your browser! The editor is live at:

**🌐 [https://roc-ops.github.io/gottp/index.html](https://roc-ops.github.io/gottp/index.html)**

The online editor features:
- Real-time template processing
- Monaco Editor with syntax highlighting and auto-completion
- Multiple output formats (JSON, YAML, Table, CSV)
- Global variables and lookup tables
- Source map visualization for debugging
- Export/import configurations
- Built-in examples

## License

MIT License - see LICENSE file for details.

