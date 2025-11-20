# Go TTP - Template Text Parser for Go

A Go implementation of the Template Text Parser (TTP) library, providing semi-structured text parsing using templates with enhanced features.

## Features

- **Full TTP Compatibility**: Compatible with Python TTP templates and syntax
- **Stateless Compiled Templates**: Compile templates once, use many times without state resets ✅
- **Multi-Language Macros**: Support for Starlark (default), JavaScript, and Python (optional, requires build tag)
- **Dual API Design**: Python-compatible API for easy porting, plus Go-idiomatic API (in progress)
- **Thread-Safe**: Compiled templates are immutable and safe for concurrent use ✅
- **High Performance**: Leverages Go's compiled nature for better performance

## Status

**Foundation Complete** ✅
- Template parsing (XML-based)
- Pattern engine (regex generation)
- Compiled template system
- Basic parsing execution
- Match functions (basic set)
- JSON/YAML formatters

**In Progress** 🚧
- Multi-line pattern matching improvements
- Additional match functions
- Group functions
- Macro execution engines
- Input/output systems
- Template extension

**Planned** 📋
- Full function library
- Python-compatible API
- Multi-processing support
- Comprehensive testing
- Documentation

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

## Installation

```bash
go get github.com/roc-ops/gottp
```

## Documentation

- [User Guide](docs/USER_GUIDE.md) - Complete usage guide
- [Migration Guide](docs/MIGRATION_GUIDE.md) - Migrating from Python TTP
- [Python Macros](docs/PYTHON_MACROS.md) - Python macro setup and usage
- [API Documentation](https://pkg.go.dev/github.com/roc-ops/gottp) - Go package documentation

## License

MIT License - see LICENSE file for details.

