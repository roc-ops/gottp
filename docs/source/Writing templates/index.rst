Writing Templates
=================

This section provides guides and best practices for writing GoTTP templates.

Guides
------

.. toctree::
   :maxdepth: 2

   How to parse show commands output
   How to parse hierarchical (configuration) data
   How to parse text tables
   How to filter with TTP
   How to produce time series data with TTP

Best Practices
--------------

1. **Use Specific Patterns**: Use built-in patterns (WORD, IP, PHRASE) when possible instead of generic regex
2. **Name Groups Clearly**: Use descriptive group names that reflect the data structure
3. **Use Dynamic Paths**: Leverage dynamic paths for flexible result structures
4. **Keep Patterns Simple**: Simple patterns are easier to maintain and debug
5. **Test Incrementally**: Test templates with small data samples first
6. **Document Templates**: Use doc tags to document template purpose and usage

Common Patterns
---------------

**Interface Configuration:**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    ip address {{ ip }}/{{ mask }}
    description {{ description | PHRASE }}
   </group>

**Table Parsing:**

.. code-block:: xml

   <group name="routes">
   {{ prefix | IP }} via {{ next_hop | IP }} {{ interface }}
   </group>

**Multi-line Patterns:**

.. code-block:: xml

   <group name="config">
   {{ _start_ }}
   interface {{ interface }}
    ip address {{ ip }}
   {{ _end_ }}
   </group>

