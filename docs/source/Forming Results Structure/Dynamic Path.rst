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

How a Record's Path Is Formed
-----------------------------

As in Python TTP, each record forms its own path when it is saved:

* The group name is split on ``.`` first, and each segment's ``*`` or ``**`` formatter is read before its variables are substituted. A value with a dot or a trailing ``*`` (``Bundle-Ether801.101``, ``lag-4:1629.*``) stays one key.
* A variable takes the record's own value, which is then removed from the record. Without one, it takes the value the same variable had in the last record saved with it in its path, then a template or recorded variable.
* A record whose path variable has no value is dropped, for example when ``del=``, a macro or a condition removed it.
* Variables are found per segment, as in TTP, with or without spaces inside the braces (``{{x}}`` or ``{{ x }}``). Two variables written without spaces in one segment (``{{x}}{{y}}``) read as one unknown variable, so the record is dropped; write ``{{ x }}{{ y }}``.

Path Character
--------------

The default path separator is ``.`` (dot). This can be changed using the template ``pathchar`` attribute:

.. code-block:: xml

   <template pathchar="_">
   <group name="interfaces_{{ interface }}">
   interface {{ interface }}
   </group>
   </template>

