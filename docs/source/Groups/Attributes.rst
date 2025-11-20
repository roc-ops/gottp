Attributes
==========

Group attributes control group behavior and result formation.

.. list-table:: Group attributes
   :widths: 15 85
   :header-rows: 1

   * - Attribute
     - Description
   * - ``name``
     - Group name used for result path formation
   * - ``input``
     - Input name(s) to parse with this group
   * - ``output``
     - Output name(s) to use for this group's results
   * - ``method``
     - Result formation method: "table" or "dict" (default)
   * - ``functions``
     - Comma-separated list of group functions to apply
   * - ``chain``
     - Reference to variable containing function chain
   * - ``macro``
     - Comma-separated list of macro function names
   * - ``void``
     - If "True", group results are discarded

name
----

``name="group_name"``

Group name is used to form the result path. It can contain dynamic parts using match variables: ``{{ variable }}``.

**Examples:**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
   </group>

   <group name="interfaces.{{ interface }}">
   interface {{ interface }}
   </group>

input
-----

``input="input_name1, input_name2"``

Specifies which input(s) should be parsed by this group. If not specified, group parses all inputs.

**Example:**

.. code-block:: xml

   <input name="config1" load="text">...</input>
   <input name="config2" load="text">...</input>

   <group name="interfaces" input="config1">
   interface {{ interface }}
   </group>

output
------

``output="output_name1, output_name2"``

Specifies which output(s) should process this group's results.

method
------

``method="table"`` or ``method="group"`` (default)

Controls how group patterns are treated:

* ``group`` (default): Only the first pattern is a start pattern; subsequent patterns merge into the current match
* ``table``: All patterns are treated as start patterns; each pattern match is saved as a separate result entry (not merged)

**With method="table":**
* Each pattern in the group is treated as a start pattern
* Each pattern match creates a separate result entry
* Patterns are not merged together (unlike default "group" method)
* Useful for parsing table-like data where each line is independent

**Example:**

.. code-block:: xml

   <group name="interfaces" method="table">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

With this template, each line (interface and ip address) will produce a separate result entry, rather than merging them into a single entry per interface.

functions
---------

``functions="contains('ip'), set('vrf', 'default')"``

Comma-separated list of group functions to apply to group results.

**Example:**

.. code-block:: xml

   <group name="interfaces" functions="contains('ip'), record('last_interface')">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

chain
-----

``chain="variable_name"``

Reference to a variable containing a function chain string.

void
----

``void="True"``

If set to "True", group results are discarded and not included in final results.

**Example:**

.. code-block:: xml

   <group name="ignored" void="True">
   ! {{ comment }}
   </group>

