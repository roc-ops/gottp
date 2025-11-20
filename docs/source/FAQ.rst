FAQ
===

Frequently Asked Questions about GoTTP.

General Questions
-----------------

**Q: Is GoTTP compatible with Python TTP templates?**

A: Yes! GoTTP is designed to be 100% compatible with Python TTP template syntax. Templates written for Python TTP should work with GoTTP without modification.

**Q: What are the main advantages of GoTTP over Python TTP?**

A: GoTTP provides several advantages:

* **Stateless compiled templates**: Compile once, use many times without reset
* **Thread-safe**: Compiled templates can be used concurrently
* **Better performance**: Leverages Go's compiled nature
* **Template serialization**: Save/load compiled templates
* **Multi-language macros**: Starlark (default), JavaScript, and Python support

**Q: Can I use GoTTP in production?**

A: GoTTP is actively developed and most core features are implemented. However, some advanced features may still be in progress. Check the :doc:`Differences from Python TTP` page for current status.

**Q: How do I migrate from Python TTP to GoTTP?**

A: See the :doc:`../MIGRATION_GUIDE` for detailed migration instructions. In most cases, templates work without modification, but the API usage differs.

Template Questions
------------------

**Q: What template syntax is supported?**

A: GoTTP supports all standard TTP template syntax including:
* Match variables: ``{{ variable }}``
* Match functions: ``{{ variable | function }}``
* Groups, inputs, outputs, lookups, macros, etc.

**Q: Can I use Python code in macros?**

A: Yes, but Python macro support requires:
1. Building with the ``python`` build tag
2. Python 3.x development headers
3. CGO enabled

Alternatively, you can use Starlark (default) which has Python-like syntax, or JavaScript.

**Q: Are all match functions available?**

A: Most match functions are implemented. See :doc:`Match Variables/Functions` for a complete list. Some specialized functions may not be available yet.

**Q: Are all group functions available?**

A: Core group functions are implemented. See :doc:`Groups/Functions` for details.

Performance Questions
---------------------

**Q: Is GoTTP faster than Python TTP?**

A: GoTTP can be faster for repeated parsing due to compiled templates and Go's performance characteristics. However, initial compilation may take slightly longer.

**Q: Can I use GoTTP concurrently?**

A: Yes! Compiled templates are immutable and thread-safe. You can safely use the same compiled template from multiple goroutines.

**Q: How do I improve parsing performance?**

A: 
* Compile templates once and reuse them
* Use template serialization to pre-compile templates
* Use goroutines for parallel parsing (GoTTP templates are thread-safe)

API Questions
-------------

**Q: What's the difference between the Go-idiomatic API and Python-compatible API?**

A: 
* **Go-idiomatic API**: Stateless, compile once, use many times
* **Python-compatible API**: Stateful, similar to Python TTP's API

**Q: Can I save compiled templates?**

A: Yes! Use ``SaveCompiledTemplate()`` to save in gob, JSON, or YAML format. Use ``LoadCompiledTemplate()`` to load them later.

**Q: How do I handle errors?**

A: GoTTP functions return errors in the standard Go way. Always check errors:

.. code-block:: go

   compiled, err := gottp.CompileTemplate(template)
   if err != nil {
       // Handle error
   }

Input/Output Questions
----------------------

**Q: What input loaders are supported?**

A: Text, YAML, JSON, CSV, File, and Directory loaders are supported.

**Q: What output formatters are supported?**

A: JSON, YAML, Raw, CSV, and Table formatters are implemented. Excel, Tabulate, Jinja2, N2G, and PPrint are planned.

**Q: What returners are supported?**

A: Terminal, File, and Syslog returners are implemented.

**Q: Can I load data from files?**

A: Yes, use the file or directory input loaders, or load data manually and pass it to ``Parse()``.

Macro Questions
---------------

**Q: What macro languages are supported?**

A: 
* **Starlark** (default): Python-like syntax, safe and fast
* **JavaScript**: Using goja runtime
* **Python**: Optional, requires build tag

**Q: How do I specify macro language?**

A: Use the ``language`` attribute in the ``<macro>`` tag:

.. code-block:: xml

   <macro language="starlark">
   def process_data(data):
       return data.upper()
   </macro>

**Q: What's available in the ``_ttp_`` context?**

A: The ``_ttp_`` dictionary provides access to:
* Groups
* Match functions
* Input functions
* Output functions
* Variables
* Lookup tables

See :doc:`Macro Tag/Macro Tag` for details.

Troubleshooting
---------------

**Q: Template parsing fails, what should I check?**

A: 
1. Verify template syntax is valid XML
2. Check that all match variables use ``{{ }}`` syntax
3. Ensure group tags are properly closed
4. Check error messages for specific issues

**Q: Macro execution fails, what's wrong?**

A: 
1. Verify macro language is supported
2. Check macro syntax (Starlark vs Python differences)
3. Ensure ``_ttp_`` context is used correctly
4. Check error messages for details

**Q: Results are empty, why?**

A: 
1. Verify input data matches template patterns
2. Check group functions aren't filtering everything out
3. Ensure match variables are correctly named
4. Verify input mapping is correct

**Q: Performance is slow, how to improve?**

A: 
1. Compile templates once and reuse
2. Use template serialization
3. Check for inefficient regex patterns
4. Consider using goroutines for parallel parsing

