Match Variables
===============

Match variables are used to extract data from text patterns. They are defined using double curly braces ``{{ variable_name }}`` within group patterns.

.. toctree::
   :maxdepth: 2

   Patterns
   Indicators
   Functions

Match variables can be used with functions to transform extracted values:

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface | upper }}
    ip address {{ ip }}
    description {{ description | strip }}
   </group>

Functions can be chained:

.. code-block:: xml

   {{ value | strip | upper | split(",") }}

