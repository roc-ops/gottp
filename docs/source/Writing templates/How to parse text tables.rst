How to Parse Text Tables
=========================

This guide shows how to parse text tables using GoTTP.

Basic Table
-----------

**Table Data:**

.. code-block:: text

   Interface          IP-Address      OK? Method Status                Protocol
   GigabitEthernet0/0 192.168.1.1     YES NVRAM  up                    up
   GigabitEthernet0/1 10.0.0.1        YES NVRAM  up                    up

**Template:**

.. code-block:: xml

   <group name="interfaces">
   {{ interface }} {{ ip }} {{ ok }} {{ method }} {{ status }} {{ protocol }}
   </group>

Skip Header Row
---------------

Use ``_line_`` indicator or ignore pattern:

.. code-block:: xml

   <group name="interfaces">
   {{ _line_0 | ignore }}
   {{ interface }} {{ ip }} {{ ok }} {{ method }} {{ status }} {{ protocol }}
   </group>

Or use a separate group for header:

.. code-block:: xml

   <group name="header" void="True">
   Interface          IP-Address      OK? Method Status                Protocol
   </group>

   <group name="interfaces">
   {{ interface }} {{ ip }} {{ ok }} {{ method }} {{ status }} {{ protocol }}
   </group>

Variable Width Columns
----------------------

Use flexible whitespace matching:

.. code-block:: xml

   <group name="interfaces">
   {{ interface | PHRASE }} {{ ip | IP }} {{ rest | ROW }}
   </group>

Tips
----

1. Use ``ROW`` pattern for remaining columns
2. Use ``PHRASE`` for text that may contain spaces
3. Use ``ignore`` or ``void`` groups to skip header rows
4. Test with actual table output

