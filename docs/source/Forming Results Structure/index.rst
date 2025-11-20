Forming Results Structure
=========================

GoTTP provides flexible ways to form results structure using group names, dynamic paths, and path formatters.

Group Names
-----------

Group names determine the structure of results. Each group produces results that are organized according to its name.

**Simple Group Name:**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
   </group>

Results:

.. code-block:: json

   {
     "interfaces": [
       {"interface": "Loopback0"},
       {"interface": "Vlan100"}
     ]
   ]

**Nested Group Names:**

.. code-block:: xml

   <group name="devices.interfaces">
   interface {{ interface }}
   </group>

Results:

.. code-block:: json

   {
     "devices": {
       "interfaces": [
         {"interface": "Loopback0"}
       ]
     }
   }

Dynamic Paths
-------------

Group names can contain match variables to form dynamic paths:

.. code-block:: xml

   <group name="interfaces.{{ interface }}">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

Results:

.. code-block:: json

   {
     "interfaces": {
       "Loopback0": {"interface": "Loopback0", "ip": "192.168.0.1"},
       "Vlan100": {"interface": "Vlan100", "ip": "10.0.0.1"}
     }
   }

Path Formatters
---------------

Special characters can be used in group names to control result structure:

* ``*`` - Creates a list structure
* ``**`` - Creates a dictionary structure

**List Structure (*):**

.. code-block:: xml

   <group name="interfaces.*">
   interface {{ interface }}
   </group>

Results:

.. code-block:: json

   {
     "interfaces": [
       {"interface": "Loopback0"},
       {"interface": "Vlan100"}
     ]
   }

**Dictionary Structure (**):**

.. code-block:: xml

   <group name="interfaces.**">
   interface {{ interface }}
   </group>

Results:

.. code-block:: json

   {
     "interfaces": {
       "0": {"interface": "Loopback0"},
       "1": {"interface": "Vlan100"}
     }
   }

Anonymous Groups
----------------

Groups without names (anonymous groups) are merged into parent group results.

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

Null Path Name
--------------

Groups with empty name attribute are merged into parent results without creating a new level.

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

.. toctree::
   :maxdepth: 2

   Group Name Attribute
   Dynamic Path
   Path formatters
   Anonymous group
   Null path name attribute

