Indicators
==========

Indicators are special markers used in match variables to control pattern matching behavior.

Special Indicators
------------------

**_start_**
   Marks the beginning of a multi-line pattern. All lines until ``_end_`` are included in the match.
   
   **Start Pattern Behavior:**
   
   When ``_start_`` is present in a group:
   
   * Patterns with ``_start_`` are always start patterns (can initiate new matches)
   * Patterns **before** the first ``_start_`` pattern can also be start patterns (if they have non-special variables)
   * Patterns **after** the first ``_start_`` pattern are **not** start patterns and only merge into matches started by ``_start_`` patterns
   
   This ensures that non-start patterns (like ``route-policy``) only match within blocks that were started by a ``_start_`` pattern.

**_end_**
   Marks the end of a multi-line pattern. Matches until this indicator is found.
   
   **End Pattern Behavior:**
   
   When both ``_start_`` and ``_end_`` are present:
   
   * Only patterns with ``_start_`` are start patterns
   * All other patterns merge into the match until ``_end_`` is encountered
   * After ``_end_`` finalizes a match, only ``_start_`` patterns can start new matches

**The line ends at a {{ _start_ }} or {{ _end_ }} variable**
   As in Python TTP, a line's regex stops at a variable named ``_start_`` or ``_end_``: the text and variables after it are dropped, and so is the whitespace just before it.

   * ``interface {{ _start_ }}`` matches ``interface`` (with trailing spaces or not), but not ``interface Gi1``.
   * ``interface {{ name }} {{ _start_ }}`` matches ``interface Gi1`` but not ``interface Gi1 extra``.
   * ``exit {{ _end_ }} now`` matches ``exit``, not ``exit now``.
   * In ``interface {{ _start_ }} {{ name }}``, ``name`` isn't captured from the data and gets no value. On an ``_end_`` line with no capture left, such as ``exit {{ _end_ }} {{ w }}``, every variable of the line, ``_end_`` too, gets the whole matched line with a newline first (``"\n exit"`` for `` exit``), as in Python TTP, and the line no longer ends the group. ``GetWarnings()`` names such a variable when the template compiles. Put the indicator last, or use it as a function: ``interface {{ name | _start_ }}``.
   * With ``_exact_space_`` the space before the indicator is kept: ``interface {{ _start_ | _exact_space_ }}`` matches ``interface `` but not ``interface``.
   * ``_start_`` or ``_end_`` used as a function (``{{ name | _start_ }}``) doesn't cut the line.
   * **gottp extension:** ``{{ _start_ | re("...") }}`` (or ``_end_``) is not cut: it matches its regex, so ``interface {{ _start_ | re("Gi\\S+") }}`` matches ``interface Gi1`` only. Python TTP ignores the ``re()`` and cuts the line there.

**_line_**
   Matches a specific line number (0-indexed).

**ignore**
   Ignores a pattern (doesn't extract it). Can take an argument: ``ignore("pattern")``, or the name of a template variable, ``ignore("pattern_var")``, whose value is the regex. The variable form suits regexes with ``|``, which would split the match variable. The argument is a Python string, so ``ignore("\\d+")`` is the regex ``\d+``.

Usage Examples
--------------

**Multi-line Pattern with _start_ and _end_**

.. code-block:: xml

   <group name="config">
   {{ _start_ }}
   interface {{ interface }}
    ip address {{ ip }}
    description {{ description }}
   {{ _end_ }}
   </group>

**Line-specific Matching**

.. code-block:: xml

   <group name="header">
   {{ _line_0 }}: {{ header_text }}
   </group>

**Ignoring Patterns**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    {{ ignore("!") }}
    ip address {{ ip }}
   </group>

Or with a variable:

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    {{ comment | ignore }}
    ip address {{ ip }}
   </group>

Multi-line Pattern Merging
--------------------------

When using ``_start_`` and ``_end_`` indicators, GoTTP merges all lines between them into a single pattern for matching. Whitespace is normalized during this process, making the template more flexible.

