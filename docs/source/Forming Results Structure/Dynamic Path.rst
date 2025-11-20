Dynamic Path
============

Dynamic paths allow group names to be formed using match variables, enabling flexible result structures.

Basic Dynamic Path
------------------

Use match variables in group names:

.. code-block:: xml

   <group name="interfaces.{{ interface }}">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

This creates a dictionary where each interface name is a key.

Complex Dynamic Paths
---------------------

Combine multiple variables:

.. code-block:: xml

   <group name="devices.{{ device }}.interfaces.{{ interface }}">
   interface {{ interface }}
   </group>

This creates a nested structure organized by device and interface.

Path Character
--------------

The default path separator is ``.`` (dot). This can be changed using the template ``pathchar`` attribute:

.. code-block:: xml

   <template pathchar="_">
   <group name="interfaces_{{ interface }}">
   interface {{ interface }}
   </group>
   </template>

