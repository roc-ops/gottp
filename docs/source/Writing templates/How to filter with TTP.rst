How to Filter with GoTTP
=========================

This guide shows how to filter results using group functions and condition functions.

Group Function Filtering
------------------------

Use ``contains`` to filter groups:

.. code-block:: xml

   <group name="interfaces" functions="contains('ip')">
   interface {{ interface }}
    ip address {{ ip }}
    description {{ description }}
   </group>

Only interfaces with IP addresses will be included.

Condition Function Filtering
----------------------------

Use condition functions in match variables:

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface | contains("Ethernet") }}
    ip address {{ ip }}
   </group>

Only interfaces containing "Ethernet" will match.

Macro Filtering
---------------

Use macros for complex filtering:

.. code-block:: xml

   <macro language="starlark">
   def filter_interface(data):
       if data.get("ip", "").startswith("192.168"):
           return True
       return False
   </macro>

   <group name="interfaces" macro="filter_interface">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

Combining Filters
-----------------

Combine multiple filters:

.. code-block:: xml

   <group name="interfaces" functions="contains('ip'), contains('description')">
   interface {{ interface }}
    ip address {{ ip }}
    description {{ description }}
   </group>

Tips
----

1. Use group functions for simple filtering
2. Use condition functions for pattern matching
3. Use macros for complex logic
4. Combine filters for precise control

