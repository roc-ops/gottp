Null Path Name Attribute
=========================

Groups with an empty ``name`` attribute (``name=""``) are treated similarly to anonymous groups - their results are merged into the parent group without creating a new level.

Usage
-----

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    <group name="">
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

Difference from Anonymous Groups
---------------------------------

Both anonymous groups and groups with ``name=""`` merge results into the parent. The difference is mainly semantic - ``name=""`` explicitly indicates the intent to merge.

