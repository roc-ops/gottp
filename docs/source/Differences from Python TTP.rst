Differences from Python TTP
============================

This document outlines the key differences between GoTTP and Python TTP, including implemented features, enhancements, and limitations.

Architecture Differences
------------------------

**Stateless Compiled Templates**

GoTTP uses a stateless compiled template approach, unlike Python TTP which maintains state between parses:

* **Python TTP**: Stateful parser object, requires reset between parses
* **GoTTP**: Stateless compiled templates, can be used repeatedly without reset

**Example:**

Python TTP::

    parser = ttp(data="...", template="...")
    parser.parse()
    result = parser.result()
    parser.reset()  # Required before next parse

GoTTP::

    compiled, _ := gottp.CompileTemplate(template)
    result1, _ := compiled.Parse(inputs1, vars, options)
    result2, _ := compiled.Parse(inputs2, vars, options)  # No reset needed

**Thread Safety**

* **Python TTP**: Not thread-safe, requires separate parser instances
* **GoTTP**: Compiled templates are immutable and thread-safe, can be used concurrently

**Template Serialization**

* **Python TTP**: Templates are parsed on each run
* **GoTTP**: Templates can be compiled once and saved/loaded for faster startup

Feature Parity
--------------

✅ **Fully Implemented**

* Template parsing (XML-based)
* Pattern engine (regex generation)
* Match variables and indicators
* Basic and advanced match functions (most)
* Condition functions (all 14)
* Group functions (core set)
* Input system (text, YAML, JSON, CSV, file, directory)
* Output system (JSON, YAML, raw, CSV, table)
* Returners (terminal, file, syslog)
* Lookup tables
* Template variables
* Template tags
* Extend tags
* Doc tags
* Starlark macros (default, with performance optimizations)
* JavaScript macros (using goja)
* Python macros (optional, requires build tag)
* Native Go macros (for maximum performance, programmatic registration)
* Nested groups
* Result path formation
* Dynamic paths

✅ **All Major Features Implemented**

* All match functions (52/52)
* All group functions (21/21)
* All input loaders (8/8)
* All output formatters (10/10)
* CLI tool
* Database loaders (input and lookup)

❌ **Not Yet Implemented**

* Multiprocessing (Python-style; Go uses goroutines instead - not needed in Go)
* Lazy loader system (not needed in Go's compiled environment)
* Enhanced error messages with detailed context (planned)
* Template validation before parsing (planned)

Match Functions
---------------

**Implemented:** ✅

* String functions: upper, lower, strip, split, join, replace
* Type conversion: to, to_str, to_int, to_float, to_ip, to_list
* IP/MAC: IP, mac_eui
* Data manipulation: count, item, let, void, joinmatches, sformat, resub, prepend, append, copy, default, unrange, uptimeparse, truncate, to_cidr, replaceall, resuball, rlookup, to_net, print, macro, chain
* Condition functions: All 20 condition functions (contains, contains_re, startswith, endswith, equal, notequal, exclude, exclude_re, isdigit, notdigit, greaterthan, lessthan, is_ip, cidr_match, and their regex variants)
* Network functions: dns, rdns, ip_info, geoip_lookup, gpvlookup
* Unicode: to_unicode (no-op in Go, strings are UTF-8 by default)

**Not Implemented:** ❌

* All match functions are implemented

Group Functions
---------------

**Implemented:** ✅

* contains, set, record, delete, expand, itemize, lookup, containsall, exclude, excludeall, equal, to_int, contains_val, exclude_val, sformat, items2dict, to_ip
* Advanced: macro, functions/chain, cerberus, validate
* Note: void is handled via attribute instead of function

**Not Implemented:** ❌

* All group functions are implemented

Input Loaders
-------------

**Implemented:** ✅

* text, yaml, json, csv, file, directory, url, database

**Not Implemented:** ❌

* All input loaders are implemented

Output Formatters
-----------------

**Implemented:** ✅

* raw, json, yaml, csv, table, pprint, tabulate, excel, jinja2, n2g

**Not Implemented:** ❌

* All major output formatters are implemented

Returners
---------

**Implemented:** ✅

* self (default), terminal, file, syslog

**Not Implemented:** ❌

* All returners are implemented

Macro Languages
---------------

**Starlark** ✅ (Default)

* Python-like syntax
* Safe and fast
* Fully supported

**JavaScript** ✅

* Using goja runtime
* Fully supported
* May have minor differences from Python TTP's JavaScript support

**Python** ✅ (Optional)

* Requires CGO and build tag: ``go build -tags python``
* Requires Python 3.x development headers
* Fully supported but may have minor differences

API Differences
---------------

**Go-Idiomatic API**

GoTTP provides a stateless, compiled template API:

.. code-block:: go

   compiled, err := gottp.CompileTemplate(template)
   result, err := compiled.Parse(inputs, vars, options)

**Python-Compatible API**

For easier migration, GoTTP also provides a stateful API similar to Python TTP:

.. code-block:: go

   parser := gottp.NewParser()
   parser.AddTemplate(template)
   parser.AddInput("Default_Input", data)
   parser.Parse()
   result := parser.Result()

Template Syntax
---------------

✅ **100% Compatible**

GoTTP templates use the exact same syntax as Python TTP. Templates written for Python TTP should work with GoTTP without modification.

Performance
-----------

**Compilation**

* **Python TTP**: Templates parsed on each run (~255-264µs per operation)
* **GoTTP**: Templates compiled once, can be reused and serialized (~18-19µs per operation)
* **Speedup**: ~13-14x faster

**Parsing**

* **Python TTP**: Interpreted execution (~316-317µs per operation)
* **GoTTP**: Compiled execution, generally faster for repeated parsing (~16-17µs per operation)
* **Speedup**: ~18-19x faster

**Concurrency**

* **Python TTP**: Requires separate parser instances
* **GoTTP**: Single compiled template can be used concurrently

**Benchmark Results**

Comprehensive benchmarks show GoTTP is approximately **16x faster** than Python TTP on average. See :doc:`Performance` for detailed benchmark results and methodology.

Error Handling
--------------

**Python TTP**: Python exceptions

**GoTTP**: Go error returns (standard Go error handling)

.. code-block:: go

   compiled, err := gottp.CompileTemplate(template)
   if err != nil {
       // Handle error
   }

Migration Guide
---------------

For detailed migration instructions, see :doc:`../MIGRATION_GUIDE`.

Key Points:

1. Templates work without modification
2. API usage differs (stateless vs stateful)
3. Error handling uses Go's error return pattern
4. Some advanced features may not be available yet

Test Coverage
-------------

GoTTP includes a comprehensive comparison test suite that validates output matches Python TTP:

**Test Results:**
* ✅ 48 tests passing - Core functionality verified (100% pass rate)
* ⏭️ 11 tests skipped - Known differences or external dependencies
* ❌ 0 tests failing - All tests either pass or are skipped for known differences

**Coverage:**
* ✅ All 52 match functions (all passing)
* ✅ 20/21 group functions (1 skipped - Cerberus validation)
* ✅ 3/8 input loaders (5 skipped - external dependencies)
* ✅ 4/10 output formatters (6 skipped - external libraries)
* ✅ All template structure features (nested groups, dynamic paths, indicators, etc.)
* ✅ All lookup features
* ✅ Starlark macros and match function macros

**Recent Fixes:**
* Dynamic path variables correctly removed from match results
* Output formatter functions (traverse) processed before formatting
* Start/end/line indicators correctly recognized
* Nested groups correctly processed
* CSV and PPrint formatters match Python TTP string representation
* `set` group function argument order corrected (source, target)
* `method="table"` now correctly saves each pattern match separately
* `resub` match function now correctly replaces only first occurrence by default

See ``test/comparison/README.md`` for details on running the comparison tests.

Future Work
-----------

Planned enhancements:

* Enhanced error messages with detailed context (line numbers, template locations) - ✅ Implemented
* Template validation before parsing (syntax checking, function name validation) - ✅ Implemented
* Performance optimizations - Ongoing
* Additional edge case handling - Ongoing
* Resolve table formatter nested list structure differences

