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

**Data lines**

Data lines keep their trailing spaces and tabs, as in Python TTP, but gottp trims a trailing ``\r`` from each line. With CRLF input, a Python TTP capture whose pattern can match ``\r`` (``{{ x | re(".*") }}``, ``{{ x | _line_ }}``) includes it at the end of a line; gottp's doesn't. ``PHRASE``, ``ORPHRASE`` and ``WORD`` captures behave the same in both.

Output Formatters
-----------------

**Implemented:** ✅

* raw, json, yaml, csv, table, pprint, tabulate, excel, jinja2, n2g

**Not Implemented:** ❌

* All major output formatters are implemented

.. _output-macro-list:

**Output macro lists (gottp extension):** ``<output macro="a, b">`` and ``<output functions="macro('a', 'b')">`` run each macro in order. Python TTP 0.10.1 raises ``TypeError`` on both; it runs several output macros only as ``functions="macro('a') | macro('b')"``, which gottp also accepts.

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

Function calls are read as Python TTP reads them (see :doc:`Match Variables/Functions`, "Function call syntax"). The differences:

* Python TTP evaluates arguments with ``eval``; gottp reads Python literals and bare names only. An expression such as ``1 + 2`` or a name of a Python built-in (``str``, ``int``) isn't evaluated: the first is passed as its text ``"1+2"`` with a compile warning, the second is the text ``"str"``.
* An unquoted word Python TTP can't evaluate, such as ``set(auto-decision)`` (``auto - decision`` in Python, so TTP exits), is the text ``"auto-decision"`` in gottp, with a compile warning that suggests ``set("auto-decision")``.
* A tuple argument is a list, and a dict argument's keys are not kept in the order written.
* Where Python TTP stops with ``SystemExit`` or raises ``TypeError`` for a call it can't read (``macro(name='m')``, ``<output macro=""/>``), gottp returns a compile error.
* Python TTP logs an unknown function name (with its ``guess`` suggestions) when it meets it; gottp adds a ``GetWarnings()`` entry when the template compiles.

**default() without a regex (gottp extension, #64):** a match variable with ``default()`` and no regex of its own matches any text (``.*?``), where Python TTP gives it the ``WORD`` regex (``\S+``). So a trailing ``{{ col | default("") }}`` makes the last column optional, and it may hold text with spaces:

* ``{{ port }} {{ state }} {{ descr | default("") }}`` matches ``Gi2 down`` (``descr`` is ``""``) and ``Gi3 up core link`` (``descr`` is ``"core link"``). Python TTP drops both lines.
* The whitespace between the text and the variable is not captured: ``ip address {{ ip | default("none") }}`` on ``ip address 1.1.1.1/24`` gives ``"1.1.1.1/24"``. When the template puts the text right against the variable (``ip foo{{ x | default("a") }}``), the space in the data is captured (``ip foo bar`` gives ``" bar"``); Python TTP doesn't match that line.
* Two such variables separated only by whitespace don't split the text the way TTP does: the first always captures ``""`` and the second takes the rest (#166).
* A line made only of ``default()`` variables without their own regex doesn't match blank lines, as in TTP. One with an explicit ``re(".*?")`` does.
* Give the variable a regex (``{{ col | WORD | default("") }}``) to get TTP's behaviour.

**{{ _start_ | re(...) }} (gottp extension, #136):** a line's regex ends at a ``{{ _start_ }}`` or ``{{ _end_ }}`` variable, as in Python TTP, except when that variable has its own ``re()``. gottp then matches the ``re()`` in its place: ``interface {{ _start_ | re("Gi\\S+") }}`` matches ``interface Gi1`` and not ``interface Te1``. Python TTP ignores the ``re()``, so that line matches only ``interface``. Templates that use it rely on the regex; drop the ``re()`` to get TTP's behaviour.

**Variables after {{ _start_ }} or {{ _end_ }} (#136):** such a variable captures nothing, and ``GetWarnings()`` names it when the template compiles. Python TTP doesn't capture it from the data either, but on a line with no other capture, such as ``exit {{ _end_ }} {{ w }}``, it saves the whole matched line, newline first, in the variable and in ``_end_`` (``{"_end_": "\nexit", "w": "\nexit"}``). gottp saves neither.

**Blank data lines:** a data line that is empty or holds only spaces and tabs is matched like any other line, as in Python TTP, so ``{{ x | _line_ }}``, ``re(".*")`` and ``re("[ ]+")`` can match it (#162). gottp skips such a line only for a template line whose regex can't match it.

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

