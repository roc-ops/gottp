Group Name Attribute
====================

The ``name`` attribute of a group determines how results are structured in the output.

Basic Usage
-----------

The simplest form is a single name:

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
   </group>

This creates a list of results under the "interfaces" key.

Nested Names
------------

Use dot notation to create nested structures:

.. code-block:: xml

   <group name="devices.interfaces">
   interface {{ interface }}
   </group>

This creates a nested structure: ``devices.interfaces``.

Dynamic Names
-------------

Include match variables in the name to create dynamic paths:

.. code-block:: xml

   <group name="interfaces.{{ interface }}">
   interface {{ interface }}
   </group>

This creates a dictionary keyed by interface name.

Special Characters
------------------

* ``*`` - Creates a list structure
* ``**`` - Creates a dictionary structure

See :doc:`Path formatters` for details.

