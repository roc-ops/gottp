# GoTTP User Guide

## Table of Contents

1. [Introduction](#introduction)
2. [Installation](#installation)
3. [Quick Start](#quick-start)
4. [Template Syntax](#template-syntax)
5. [Pattern Matching](#pattern-matching)
6. [Functions](#functions)
7. [Macros](#macros)
8. [Template Extension](#template-extension)
9. [Stateless Parsing](#stateless-parsing)
10. [Code Generation](#code-generation)
11. [Best Practices](#best-practices)

## Introduction

GoTTP is a Go implementation of the Template Text Parser (TTP) library. It allows you to parse semi-structured text data using templates, extracting structured information into Go data structures.

### Key Advantages

- **Stateless Design**: Compile once, use many times without state resets
- **Thread-Safe**: Compiled templates are immutable and safe for concurrent use
- **High Performance**: Leverages Go's compiled nature
- **TTP Compatible**: Works with existing Python TTP templates
- **Multi-Language Macros**: Support for Starlark, JavaScript, and Python

## Installation

```bash
go get github.com/roc-ops/gottp
```

## Quick Start

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/roc-ops/gottp"
)

func main() {
	// Define a template
	template := `
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
</group>
`

	// Compile the template
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		log.Fatal(err)
	}

	// Define input data
	data := `
interface Loopback0
 ip address 192.168.0.1/24
 description Router-id-loopback
!
interface Vlan100
 ip address 10.0.0.1/24
 description Management-VLAN
!
`

	// Parse the data
	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil, // no variables
		nil, // default options
	)
	if err != nil {
		log.Fatal(err)
	}

	// Print results as JSON
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonData))
}
```

Output:
```json
{
  "interfaces": [
    {
      "interface": "Loopback0",
      "ip": "192.168.0.1",
      "mask": "24",
      "description": "Router-id-loopback"
    },
    {
      "interface": "Vlan100",
      "ip": "10.0.0.1",
      "mask": "24",
      "description": "Management-VLAN"
    }
  ]
}
```

## Template Syntax

Templates are XML-like structures with special tags:

### Basic Template Structure

```xml
<template>
  <group name="my_group">
    <!-- patterns here -->
  </group>
</template>
```

### Template Tags

- **`<template>`**: Root container (optional if only one group)
- **`<group>`**: Pattern matching group
- **`<input>`**: Input data source configuration
- **`<output>`**: Output formatting configuration
- **`<vars>`**: Template variables
- **`<macro>`**: Macro function definitions
- **`<extend>`**: Template extension

## Pattern Matching

### Basic Patterns

Use double curly braces to define variables:

```
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
```

### Pattern Attributes

Groups can have various attributes:

```xml
<group name="interfaces" input="config" output="interfaces.json" method="table">
  <!-- patterns -->
</group>
```

### Multi-line Patterns

Patterns can span multiple lines:

```
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
 description {{ description }}
```

### Nested Groups

Groups can be nested:

```xml
<group name="device">
hostname {{ hostname }}
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}
</group>
</group>
```

## Functions

Functions can be applied to matched variables:

### Built-in Functions

- **`upper`**: Convert to uppercase
- **`lower`**: Convert to lowercase
- **`strip`**: Remove whitespace
- **`split(sep)`**: Split string by separator
- **`join(sep)`**: Join list with separator
- **`IP`**: Validate IP address
- **`MAC`**: Validate MAC address
- **`count`**: Count items
- **`record`**: Record value for path formation
- **`set`**: Set variable value

### Function Pipeline

Multiple functions can be chained:

```
{{ value | upper | split(',') }}
```

## Macros

Macros allow custom processing logic. Supported languages:

### Starlark (Default)

```xml
<macro name="process" language="starlark">
def process(data):
    return data.upper() + "_processed"
</macro>
```

### JavaScript

```xml
<macro name="process" language="javascript">
function process(data) {
    return data.toUpperCase() + "_processed";
}
</macro>
```

### Using Macros in Patterns

```
{{ value | macro('process') }}
```

## Template Extension

Templates can extend other templates:

```xml
<extend template="base_template.txt" groups="common,advanced" />
```

### Extension Filters

- **`groups`**: Filter groups to include
- **`inputs`**: Filter inputs to include
- **`outputs`**: Filter outputs to include
- **`vars`**: Filter variables to include
- **`lookups`**: Filter lookups to include

## Source Maps

Source maps allow you to track which parts of the input text matched which template patterns. This is particularly useful for editor visualization and debugging. Source maps are optional and have zero overhead when disabled.

### Enabling Source Maps

To enable source maps, use `ParseWithValidation` with `EnableSourceMap: true`:

```go
parseResult, err := compiled.ParseWithValidation(
    gottp.Inputs{"Default_Input": data},
    nil, // vars
    &gottp.ParseOptions{
        EnableSourceMap: true,
    },
)
if err != nil {
    log.Fatal(err)
}

// Access parsed data
result := parseResult.Data

// Access source map
if parseResult.SourceMap != nil {
    inputMap := parseResult.SourceMap.Inputs["Default_Input"]
    for _, line := range inputMap.Lines {
        if line.Matched {
            fmt.Printf("Line %d matched\n", line.LineNumber+1)
            for _, match := range line.Matches {
                fmt.Printf("  Match: %s (cols %d-%d)\n", 
                    match.GroupName, match.StartCol, match.EndCol)
                fmt.Printf("  Result path: %s\n", match.ResultPath)
            }
        }
    }
}
```

### Source Map Structure

The source map provides detailed information about matches:

- **`SourceMap.Inputs`**: Map of input name to input source map
- **`InputSourceMap.Lines`**: Array of line mappings, one per input line
- **`LineMapping.Matched`**: Whether the line matched any pattern
- **`LineMapping.Matches`**: Array of matches on this line
- **`MatchMapping.StartCol`/`EndCol`**: Character range of the match (0-indexed)
- **`MatchMapping.GroupName`**: Name of the group that matched
- **`MatchMapping.ResultPath`**: Path in result structure (e.g., "interfaces[0]")
- **`MatchMapping.Variables`**: Map of variable names to their character ranges

### Use Cases

Source maps are useful for:

- **Editor Visualization**: Highlight which input lines matched in a text editor
- **Debugging**: Understand why certain lines matched or didn't match
- **Error Reporting**: Show users exactly which parts of their input were processed
- **Interactive Tools**: Enable click-to-navigate between input and output

### Performance

Source maps have minimal overhead when enabled, but for maximum performance in production, leave them disabled unless needed.

## Stateless Parsing

One of GoTTP's key features is stateless parsing. Unlike Python TTP, you don't need to reset state between parses:

```go
// Compile once
compiled, _ := gottp.CompileTemplate(template)

// Parse multiple times with different data
result1, _ := compiled.Parse(gottp.Inputs{"input": data1}, nil, nil)
result2, _ := compiled.Parse(gottp.Inputs{"input": data2}, nil, nil)
result3, _ := compiled.Parse(gottp.Inputs{"input": data3}, nil, nil)

// No reset needed!
```

### Concurrent Parsing

Compiled templates are thread-safe. For concurrent parsing:

```go
var wg sync.WaitGroup
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(data string) {
        defer wg.Done()
        result, _ := compiled.Parse(
            gottp.Inputs{"input": data},
            nil,
            nil,
        )
        // process result
    }(dataList[i])
}
wg.Wait()
```

## Code Generation

Use `gottp-gen` to embed templates at compile time:

### Installation

```bash
go install github.com/roc-ops/gottp/cmd/gottp-gen@latest
```

### Usage

1. Create a template file (`template.txt`):
```
<group name="test">{{ value }}</group>
```

2. Add a `//go:generate` directive:
```go
//go:generate gottp-gen -template=template.txt -var=MyTemplate -format=gob
```

3. Run `go generate`:
```bash
go generate
```

4. Use the embedded template:
```go
result, err := MyTemplate.Parse(inputs, vars, nil)
```

### Benefits

- Templates compiled at build time
- No runtime compilation overhead
- No need to ship template files
- Type-safe access

## Best Practices

### 1. Compile Once, Use Many Times

```go
// Good: Compile once
compiled, _ := gottp.CompileTemplate(template)
for _, data := range dataList {
    result, _ := compiled.Parse(gottp.Inputs{"input": data}, nil, nil)
}

// Bad: Compiling repeatedly
for _, data := range dataList {
    compiled, _ := gottp.CompileTemplate(template) // Don't do this!
    result, _ := compiled.Parse(gottp.Inputs{"input": data}, nil, nil)
}
```

### 2. Use Code Generation for Production

For production applications, use code generation to embed templates:

```go
//go:generate gottp-gen -template=production.template -var=ProdTemplate
```

### 3. Handle Errors

Always check for errors:

```go
compiled, err := gottp.CompileTemplate(template)
if err != nil {
    log.Fatalf("Failed to compile: %v", err)
}

result, err := compiled.Parse(inputs, vars, nil)
if err != nil {
    log.Fatalf("Failed to parse: %v", err)
}
```

### 4. Use Variables for Reusability

```go
vars := gottp.Vars{
    "site": "datacenter1",
    "region": "us-east",
}

result, _ := compiled.Parse(inputs, vars, nil)
```

### 5. Serialize Templates for Distribution

```go
// Save compiled template
data, _ := gottp.SaveCompiledTemplate(compiled, "gob")
os.WriteFile("template.gob", data, 0644)

// Load later
data, _ := os.ReadFile("template.gob")
compiled, _ := gottp.LoadCompiledTemplate(data, "gob")
```

## Examples

See the `examples/` directory for complete working examples:

- `basic/`: Basic template parsing
- `serialize/`: Template serialization
- `codegen/`: Code generation usage
- `python-api/`: Python-compatible API usage

## Migration from Python TTP

If you're migrating from Python TTP, see [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) for detailed migration instructions.

## API Reference

See [godoc](https://pkg.go.dev/github.com/roc-ops/gottp) for complete API documentation.

