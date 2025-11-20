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

**_line_**
   Matches a specific line number (0-indexed).

**ignore**
   Ignores a pattern (doesn't extract it). Can take an argument: ``ignore("pattern")`` or ``ignore(pattern_var)``.

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

