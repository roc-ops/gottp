// Package gottp provides a Go implementation of the Template Text Parser (TTP) library.
//
// GoTTP is a semi-structured text parsing library that uses templates to extract
// structured data from text. It is compatible with Python TTP templates and provides
// enhanced features including stateless compiled templates and multi-language macro support.
//
// # Quick Start
//
//	package main
//
//	import (
//		"fmt"
//		"github.com/roc-ops/gottp"
//	)
//
//	func main() {
//		// Compile a template
//		template := `
//	<group name="interfaces">
//	interface {{ interface }}
//	 ip address {{ ip }}/{{ mask }}
//	</group>
//	`
//
//		compiled, err := gottp.CompileTemplate(template)
//		if err != nil {
//			panic(err)
//		}
//
//		// Parse data (can be called repeatedly without reset)
//		data := `
//	interface Loopback0
//	 ip address 192.168.0.1/24
//	!`
//
//		result, err := compiled.Parse(
//			gottp.Inputs{"Default_Input": data},
//			nil, // vars
//			nil, // options
//		)
//		if err != nil {
//			panic(err)
//		}
//
//		fmt.Printf("%+v\n", result)
//	}
//
// # Key Features
//
//   - Stateless Compiled Templates: Compile once, use many times without state resets
//   - Full TTP Compatibility: Works with existing Python TTP templates
//   - Multi-Language Macros: Support for Starlark, JavaScript, and Python macros
//   - Thread-Safe: Compiled templates are immutable and safe for concurrent use
//   - High Performance: Leverages Go's compiled nature for better performance
//
// # Stateless Design
//
// Unlike Python TTP, GoTTP's compiled templates are stateless. The Parse() method
// takes all necessary parameters and has no internal state, allowing for:
//
//   - Repeated calls without reset
//   - Concurrent execution from multiple goroutines
//   - Better performance and memory efficiency
//
// # Template Syntax
//
// GoTTP uses the same template syntax as Python TTP. Templates are XML-like structures
// with special tags:
//
//   - <template>: Root template container
//   - <group>: Pattern matching group
//   - <input>: Input data source configuration
//   - <output>: Output formatting configuration
//   - <vars>: Template variables
//   - <macro>: Macro function definitions
//   - <extend>: Template extension
//
// # Pattern Matching
//
// Patterns use double curly braces to define variables:
//
//	interface {{ interface }}
//	 ip address {{ ip }}/{{ mask }}
//
// Variables can have functions applied:
//
//	{{ ip | IP }}
//	{{ mac | MAC }}
//	{{ value | upper | split(',') }}
//
// # Macros
//
// Macros allow custom processing logic. Supported languages:
//
//   - Starlark (default): Python-like, safe for embedding, no dependencies
//   - JavaScript: Using goja runtime, no external dependencies
//   - Python: Using CGO (optional, requires -tags python build flag and Python runtime)
//
// Example:
//
//	<macro name="process_data" language="starlark">
//	def process_data(data):
//	    return data.upper()
//	</macro>
//
// # Template Extension
//
// Templates can extend other templates using the <extend> tag:
//
//	<extend template="base_template.txt" groups="common,advanced" />
//
// # Code Generation
//
// Use the gottp-gen tool to embed templates at compile time:
//
//	//go:generate gottp-gen -template=template.txt -var=MyTemplate
//
// Then run: go generate
//
// # Examples
//
// See the examples/ directory for complete working examples.
package gottp

