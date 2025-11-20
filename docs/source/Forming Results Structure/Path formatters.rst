Path Formatters
===============

Path formatters use special characters to control result structure.

Asterisk (*)
------------

Creates a list structure:

.. code-block:: xml

   <group name="interfaces.*">
   interface {{ interface }}
   </group>

Results in a list:

.. code-block:: json

   {
     "interfaces": [
       {"interface": "Loopback0"},
       {"interface": "Vlan100"}
     ]
   }

Double Asterisk (**)
--------------------

Creates a dictionary structure:

.. code-block:: xml

   <group name="interfaces.**">
   interface {{ interface }}
   </group>

Results in a dictionary with numeric keys:

.. code-block:: json

   {
     "interfaces": {
       "0": {"interface": "Loopback0"},
       "1": {"interface": "Vlan100"}
     }
   }

Combining with Dynamic Paths
----------------------------

Path formatters can be combined with dynamic paths:

.. code-block:: xml

   <group name="devices.{{ device }}.interfaces.*">
   interface {{ interface }}
   </group>

