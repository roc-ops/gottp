Patterns
========

GoTTP supports built-in patterns for common data types. These patterns can be used in match variables to match specific data formats.

Built-in Patterns
-----------------

**WORD**
   Matches a single word (alphanumeric characters and underscores)

**PHRASE**
   Matches a phrase (words separated by spaces)

**IP**
   Matches IPv4 or IPv6 addresses

**MAC**
   Matches MAC addresses in various formats

**DIGIT**
   Matches digits

**ROW**
   Matches the rest of the line

**ORPHRASE**
   Matches a phrase that may contain special characters

**NON_WHITESPACE**
   Matches non-whitespace characters

**ANY**
   Matches any characters (greedy)

**NON_WHITESPACE_OR_NEWLINE**
   Matches non-whitespace characters excluding newlines

Usage
-----

Patterns are used in match variables:

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface | WORD }}
    ip address {{ ip | IP }}
    description {{ description | PHRASE }}
   </group>

Custom Patterns
---------------

You can also use regular expressions directly in match variables:

.. code-block:: xml

   <group name="data">
   value: {{ value | re("\\d+") }}
   </group>

Pattern Matching Behavior
-------------------------

GoTTP uses flexible whitespace matching by default. Multiple spaces, tabs, and newlines are normalized during pattern matching, making templates more robust to formatting variations.

