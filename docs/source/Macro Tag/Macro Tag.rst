Macro Tag
=========

Macro tag allows to define inline code that can be used to process results and extend GoTTP functionality. Macros have access to ``_ttp_`` dictionary containing all groups, match, inputs, outputs functions, variables, and lookup tables.

Macro Languages
---------------

GoTTP supports four macro approaches:

* **Starlark** (default) - Python-like syntax, safe and fast, with performance optimizations
* **JavaScript** - Using goja runtime
* **Python** - Optional, requires CGO and build tag
* **Native Go** - For maximum performance, programmatic registration only (not available in templates)

Specifying Macro Language
-------------------------

Use the ``language`` attribute to specify the macro language:

.. code-block:: xml

   <macro language="starlark">
   def process_data(data):
       return data.upper()
   </macro>

   <macro language="javascript">
   function processData(data) {
       return data.toUpperCase();
   }
   </macro>

   <macro language="python">
   def process_data(data):
       return data.upper()
   </macro>

_ttp_ Dictionary
----------------

The ``_ttp_`` dictionary provides access to:

* ``_ttp_["groups"]`` - All group results
* ``_ttp_["vars"]`` - Template variables
* ``_ttp_["lookups"]`` - Lookup tables
* ``_ttp_["inputs"]`` - Input functions
* ``_ttp_["outputs"]`` - Output functions
* ``_ttp_["match"]`` - Match functions
* ``_ttp_["global_vars"]`` - Global variables
* ``_ttp_["parser_object"]`` - Parser object with additional context

Using Macros in Groups
-----------------------

Macros can be referenced in group ``macro`` attribute:

.. code-block:: xml

   <macro language="starlark">
   def check_interface(data):
       if "Vlan" in data.get("interface", ""):
           data["is_svi"] = True
       else:
           data["is_svi"] = False
       return data
   </macro>

   <group name="interfaces" macro="check_interface">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

Macro Return Values
-------------------

Depending on data returned by macro function, GoTTP will behave differently:

* If macro returns ``True`` or ``False`` - Original data unchanged, macro handled as condition function, invalidating result on ``False`` and keeps processing result on ``True``
* If macro returns ``None``/``nil`` - Data processing continues, no additional logic associated
* If macro returns single item - That item replaces original data supplied to macro and processed further

**Example - Condition Macro:**

.. code-block:: xml

   <macro language="starlark">
   def validate_interface(data):
       # Only keep interfaces with IP addresses
       if "ip" in data:
           return True
       return False
   </macro>

   <group name="interfaces" macro="validate_interface">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

**Example - Transform Macro:**

.. code-block:: xml

   <macro language="starlark">
   def enrich_interface(data):
       data["device_type"] = "router"
       data["parsed_at"] = "2024-01-01"
       return data
   </macro>

   <group name="interfaces" macro="enrich_interface">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

Starlark Macros
---------------

Starlark is the default macro language. It has Python-like syntax but is safer and faster.

**Example:**

.. code-block:: xml

   <macro language="starlark">
   def process_data(data):
       # Access _ttp_ context
       hostname = _ttp_["vars"].get("hostname", "unknown")
       data["hostname"] = hostname
       return data
   </macro>

JavaScript Macros
-----------------

JavaScript macros use the goja runtime.

**Example:**

.. code-block:: xml

   <macro language="javascript">
   function processData(data) {
       // Access _ttp_ context
       var hostname = _ttp_["vars"]["hostname"] || "unknown";
       data["hostname"] = hostname;
       return data;
   }
   </macro>

Python Macros
-------------

Python macros require CGO and the ``python`` build tag:

.. code-block:: bash

   go build -tags python

**Example:**

.. code-block:: xml

   <macro language="python">
   def process_data(data):
       # Access _ttp_ context
       hostname = _ttp_["vars"].get("hostname", "unknown")
       data["hostname"] = hostname
       return data
   </macro>

Note: Python macro support requires Python 3.x development headers and may not be available on all platforms.

Native Go Macros
----------------

Native Go macros provide the best performance by avoiding data conversion overhead. They are registered programmatically and take precedence over language-based macros with the same name.

**Function Signature:**

.. code-block:: go

   type GoMacroFunc func(
       data map[string]interface{},
       args []string,
       kwargs map[string]interface{},
   ) (map[string]interface{}, bool, error)

**Example:**

.. code-block:: go

   import "github.com/roc-ops/gottp"

   // Register a native Go macro
   macroRegistry := gottp.GetMacroRegistry()
   macroRegistry.RegisterGoMacro("ds_bonded", func(
       data map[string]interface{},
       args []string,
       kwargs map[string]interface{},
   ) (map[string]interface{}, bool, error) {
       dsIntf, ok := data["ds-intf"].(string)
       if !ok {
           return data, true, nil
       }
       if len(dsIntf) > 0 {
           lastChar := dsIntf[len(dsIntf)-1]
           if lastChar == '*' {
               data["ds-intf"] = dsIntf[:len(dsIntf)-1]
               data["ds-bonded"] = true
               data["ds-impaired"] = false
           } else if lastChar == '#' {
               data["ds-bonded"] = true
               data["ds-impaired"] = true
           } else {
               data["ds-bonded"] = false
               data["ds-impaired"] = false
           }
       }
       return data, true, nil
   })

   // Compile template that uses the macro
   compiled, _ := gottp.CompileTemplate(`
       <group name="show_cable_modem*" macro="ds_bonded">
       {{mac-address | MAC | mac_eui}} {{ip-address | IP }}
       {{us-intf}} {{ds-intf}} {{status}}
       </group>
   `)

**Performance Benefits:**

* **No data conversion**: Direct Go map operations, no Go ↔ Starlark/Python/JS conversion
* **No compilation overhead**: Pre-compiled Go functions
* **Priority**: Native Go macros take precedence over language-based macros with the same name
* **Performance**: Up to 1.56x faster than Starlark macros for macro-heavy templates

**Note:** Native Go macros are only available through programmatic registration. They cannot be defined in templates using the ``<macro>`` tag.

