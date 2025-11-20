API Reference
=============

GoTTP provides two APIs: a Go-idiomatic stateless API and a Python-compatible stateful API.

Go-Idiomatic API
----------------

**CompileTemplate**

Compile a template string into a reusable CompiledTemplate.

.. code-block:: go

   func CompileTemplate(template string) (*CompiledTemplate, error)

**CompiledTemplate.Parse**

Parse input data using the compiled template.

.. code-block:: go

   func (ct *CompiledTemplate) Parse(
       inputs Inputs,
       vars map[string]interface{},
       options *Options,
   ) (interface{}, error)

**SaveCompiledTemplate**

Save a compiled template to a file.

.. code-block:: go

   func SaveCompiledTemplate(
       ct *CompiledTemplate,
       filename string,
       format string,
   ) error

Supported formats: "gob", "json", "yaml"

**LoadCompiledTemplate**

Load a compiled template from a file.

.. code-block:: go

   func LoadCompiledTemplate(
       filename string,
       format string,
   ) (*CompiledTemplate, error)

**Types**

.. code-block:: go

   type Inputs map[string]interface{}

   type Options struct {
       // Options for parsing
   }

Python-Compatible API
---------------------

**NewParser**

Create a new parser instance.

.. code-block:: go

   func NewParser() *Parser

**Parser.AddTemplate**

Add a template to the parser.

.. code-block:: go

   func (p *Parser) AddTemplate(template string) error

**Parser.AddInput**

Add input data to the parser.

.. code-block:: go

   func (p *Parser) AddInput(name string, data interface{}) error

**Parser.AddVars**

Add template variables.

.. code-block:: go

   func (p *Parser) AddVars(vars map[string]interface{}) error

**Parser.Parse**

Parse all added inputs using added templates.

.. code-block:: go

   func (p *Parser) Parse() error

**Parser.Result**

Get parsing results.

.. code-block:: go

   func (p *Parser) Result() interface{}

**Parser.ClearInput**

Clear all input data.

.. code-block:: go

   func (p *Parser) ClearInput()

**Parser.ClearResult**

Clear parsing results.

.. code-block:: go

   func (p *Parser) ClearResult()

Example Usage
-------------

**Go-Idiomatic API:**

.. code-block:: go

   package main

   import (
       "fmt"
       "github.com/roc-ops/gottp"
   )

   func main() {
       template := `<group name="interfaces">interface {{ interface }}</group>`
       
       compiled, err := gottp.CompileTemplate(template)
       if err != nil {
           panic(err)
       }

       result, err := compiled.Parse(
           gottp.Inputs{"Default_Input": "interface Loopback0"},
           nil,
           nil,
       )
       if err != nil {
           panic(err)
       }

       fmt.Printf("%+v\n", result)
   }

**Python-Compatible API:**

.. code-block:: go

   package main

   import (
       "fmt"
       "github.com/roc-ops/gottp"
   )

   func main() {
       template := `<group name="interfaces">interface {{ interface }}</group>`
       
       parser := gottp.NewParser()
       parser.AddTemplate(template)
       parser.AddInput("Default_Input", "interface Loopback0")
       
       if err := parser.Parse(); err != nil {
           panic(err)
       }

       result := parser.Result()
       fmt.Printf("%+v\n", result)
   }

