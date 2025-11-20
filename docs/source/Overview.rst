Overview
========

GoTTP is a Go module that allows relatively fast performance parsing of semi-structured text data using templates. GoTTP was developed as a Go implementation of the Python TTP library, enabling programmatic access to data produced by CLI of networking devices, but, it can be used to parse any semi-structured text that contains distinctive repetition patterns.

In the simplest case GoTTP takes template text and data that needs to be parsed, returning results structure with extracted information.

Same data can be parsed by several templates producing results accordingly, templates are easy to create and users encouraged to write their own GoTTP templates.

Motivation
----------

While networking devices continue to develop API capabilities, there is a big footprint of legacy and not-so devices in the field, these devices are lacking of any well developed API to retrieve structured data, the closest they can get is SNMP and CLI text output. Moreover, even if some devices have API capable of representing their configuration or state data in the format that can be consumed programmatically, in certain cases, the amount of work that needs to be done to make use of these capabilities outweighs the benefits or value of produced results.

There are a number of tools available to parse text data, but, the author of TTP believes that parsing data is only part of the work flow, where the ultimate goal is to make use of the actual data.

Say we have configuration files and we want to create a report of all IP addresses configured on devices together with VRFs and interface descriptions, report should have csv format. To do that we have (1) collect data from various inputs and maybe sort and prepare it, (2) parse that data, (3) format it in certain way and (4) save it somewhere or pass to other program(s). GoTTP has built-in capabilities to address all of these steps to produce desired outcome.

Core Functionality
------------------

GoTTP has a number of systems built into it:

* **Groups system** - help to define results hierarchy and data processing functions with filtering ✅
* **Parsing system** - uses regular expressions derived out of templates to parse and process data ✅
* **Input system** - used to define various input data sources, help to retrieve data, prepare it and map to the groups for parsing ✅
* **Output system** - allows to format parsing results and return them to certain destinations ✅
* **Macro** - inline code (Starlark, JavaScript, or Python) that can be used to process results and extend GoTTP functionality, having access to ``_ttp_`` dictionary containing all groups, match, inputs, outputs functions ✅
* **Lookup tables** - helps to enrich results with additional information or reference results across different templates or groups to combine them ✅
* **Template variables** - variables store, accessible during template execution for caching or retrieving values ✅
* **Template tags** - to define several independent templates within single file together with results forming mode ✅
* **Extend tags** - helps to extend template with other templates to facilitate re-use of templates ✅
* **Stateless Compiled Templates** - Compile templates once, use many times without state resets ✅ (GoTTP Enhancement)
* **Thread-Safe Architecture** - Compiled templates are immutable and safe for concurrent use ✅ (GoTTP Enhancement)
* **Multi-Language Macro Support** - Support for Starlark (default), JavaScript, and Python (optional) ✅ (GoTTP Enhancement)
* **Template Serialization** - Save/load compiled templates in gob, JSON, or YAML formats ✅ (GoTTP Enhancement)

Key Differences from Python TTP
--------------------------------

GoTTP maintains high compatibility with Python TTP templates while providing several enhancements:

**Enhanced Features:**

1. **Stateless Compiled Templates**: Unlike Python TTP which maintains state between parses, GoTTP templates are compiled once and can be used repeatedly without reset. This enables:
   - Better performance for repeated parsing
   - Thread-safe concurrent parsing
   - No need to reset state between parses

2. **Dual API Design**: 
   - **Go-Idiomatic API**: Stateless, compiled template approach
   - **Python-Compatible API**: Stateful API for easy porting from Python TTP

3. **Multi-Language Macro Support**:
   - **Starlark** (default): Python-like syntax, safe and fast
   - **JavaScript**: Using goja runtime
   - **Python**: Optional, requires CGO and build tag

4. **Template Serialization**: Compiled templates can be saved and loaded, enabling:
   - Pre-compilation of templates
   - Distribution of compiled templates
   - Faster startup times

**Compatibility Notes:**

1. **Template Syntax**: 100% compatible with Python TTP templates
2. **Match Functions**: Most match functions are implemented, see :doc:`Match Variables/Functions` for details
3. **Group Functions**: Core group functions are implemented, see :doc:`Groups/Functions` for details
4. **Input Loaders**: Text, YAML, JSON, CSV, File, Directory loaders are implemented
5. **Output Formatters**: JSON, YAML, Raw, CSV, Table formatters are implemented
6. **Returners**: Terminal, File, Syslog returners are implemented
7. **Macros**: Starlark macros are fully supported, JavaScript and Python are available but may have minor differences

**Not Yet Implemented:**

1. **CLI Tool**: Command-line tool for running templates (planned)
2. **Multiprocessing**: Python-style multiprocessing (Go uses goroutines instead)
3. **Some Advanced Formatters**: Excel, Tabulate, Jinja2, N2G, PPrint formatters (planned)
4. **Lazy Loader System**: All functions are available (not needed in Go's compiled environment)
5. **Some Advanced Functions**: A few specialized match/group functions may not be implemented yet

For detailed differences, see :doc:`Differences from Python TTP`.

