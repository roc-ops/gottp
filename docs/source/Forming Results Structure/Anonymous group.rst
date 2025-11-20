Anonymous Group
===============

Anonymous groups are groups without a ``name`` attribute. They are merged into their parent group's results.

Usage
-----

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    <group>
    ip address {{ ip }}
    </group>
   </group>

Results:

.. code-block:: json

   {
     "interfaces": [
       {
         "interface": "Loopback0",
         "ip": "192.168.0.1"
       }
     ]
   }

The anonymous group's results are merged into the parent group's results.

Nested Anonymous Groups
-----------------------

Anonymous groups can be nested:

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    <group>
    ip address {{ ip }}
     <group>
     description {{ description }}
     </group>
    </group>
   </group>

All results are merged into the parent group.

