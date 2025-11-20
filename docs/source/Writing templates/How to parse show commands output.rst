How to Parse Show Commands Output
==================================

This guide shows how to parse output from network device show commands.

Basic Example
-------------

**Show Interface Output:**

.. code-block:: text

   GigabitEthernet0/0 is up, line protocol is up
     Internet address is 192.168.1.1/24
     Description: Uplink to core

**Template:**

.. code-block:: xml

   <group name="interfaces">
   {{ interface }} is {{ status }}, line protocol is {{ protocol }}
     Internet address is {{ ip }}/{{ mask }}
     Description: {{ description | PHRASE }}
   </group>

Handling Multiple Interfaces
----------------------------

**Show IP Route Output:**

.. code-block:: text

   C    192.168.1.0/24 is directly connected, GigabitEthernet0/0
   S    10.0.0.0/8 [1/0] via 192.168.1.254
   O    172.16.0.0/16 [110/2] via 192.168.1.254, 00:00:15, GigabitEthernet0/0

**Template:**

.. code-block:: xml

   <group name="routes">
   {{ code }}    {{ prefix }}/{{ mask }} {{ rest | ROW }}
   </group>

Using Indicators
----------------

**Multi-line Show Command:**

.. code-block:: text

   Router#show version
   Cisco IOS Software, Version 15.1(4)M4
   Copyright (c) 1986-2012 by Cisco Systems, Inc.
   Compiled Wed 22-Aug-12 10:18 by prod_rel_team

**Template:**

.. code-block:: xml

   <group name="version">
   {{ _start_ }}
   Cisco IOS Software, Version {{ version }}
   Copyright (c) {{ copyright }}
   Compiled {{ compiled_date }} by {{ compiled_by }}
   {{ _end_ }}
   </group>

Tips
----

1. Use ``ROW`` pattern for remaining line content
2. Use ``_start_`` and ``_end_`` for multi-line patterns
3. Use ``ignore`` to skip unwanted lines
4. Test with actual command output

