Quick start
===========

This guide will help you get started with GoTTP quickly.

Basic Example
-------------

Here's a simple example that parses interface configuration:

.. code-block:: go

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

Output:

.. code-block:: json

   [
     {
       "interfaces": [
         {
           "interface": "Loopback0",
           "ip": "192.168.0.113",
           "mask": "24",
           "description": "Router-id-loopback"
         },
         {
           "interface": "Vlan778",
           "ip": "2002::fd37",
           "mask": "124",
           "description": "CPE_Acces_Vlan"
         }
       ]
     }
   ]

Using Match Functions
---------------------

Match functions can transform extracted values:

.. code-block:: go

   template := `
   <group name="interfaces">
   interface {{ interface | upper }}
    ip address {{ ip }}
    description {{ description | strip }}
   </group>
   `
   
   // ... compile and parse as above

Using Group Functions
---------------------

Group functions can filter and process group results:

.. code-block:: go

   template := `
   <group name="interfaces" functions="contains('ip')">
   interface {{ interface }}
    ip address {{ ip }}
    description {{ description }}
   </group>
   `
   
   // Only groups with 'ip' field will be included

Using Inputs and Outputs
------------------------

You can define inputs and outputs in templates:

.. code-block:: go

   template := `
   <input name="config" load="text">
   interface Loopback0
    ip address 192.168.0.1/24
   </input>

   <group name="interfaces" input="config">
   interface {{ interface }}
    ip address {{ ip }}/{{ mask }}
   </group>

   <output format="json" returner="terminal"/>
   `

   compiled, err := gottp.CompileTemplate(template)
   if err != nil {
       panic(err)
   }

   result, err := compiled.Parse(nil, nil, nil)
   if err != nil {
       panic(err)
   }

Python-Compatible API
---------------------

For easier migration from Python TTP, you can use the stateful API:

.. code-block:: go

   parser := gottp.NewParser()
   parser.AddTemplate(template)
   parser.AddInput("Default_Input", data)
   parser.Parse()
   result := parser.Result()

   // Clear for next use
   parser.ClearInput()
   parser.ClearResult()

Saving and Loading Compiled Templates
--------------------------------------

Compiled templates can be saved and loaded for faster startup:

.. code-block:: go

   // Compile and save
   compiled, _ := gottp.CompileTemplate(template)
   gottp.SaveCompiledTemplate(compiled, "template.gob", "gob")

   // Load later
   loaded, _ := gottp.LoadCompiledTemplate("template.gob", "gob")
   result, _ := loaded.Parse(inputs, vars, options)

Next Steps
----------

* Read the :doc:`Overview` for more details
* Check out :doc:`Match Variables/index` for available match functions
* See :doc:`Groups/index` for group functions
* Review :doc:`Writing templates/index` for template writing guides

