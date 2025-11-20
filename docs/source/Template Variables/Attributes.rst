Attributes
==========

Template variable attributes control how variables are loaded and stored.

load
----

``load="loader_name"``

Loader name to use for loading variables. Supported loaders:

* ``python`` - Python-style variable definitions (default)
* ``yaml`` - YAML structured data
* ``json`` - JSON structured data
* ``csv`` - CSV formatted data
* ``ini`` - INI structured data

**Example:**

.. code-block:: xml

   <vars load="yaml">
   hostname: router1
   </vars>

include
-------

``include="path"``

Absolute OS path to text file with variables data.

**Example:**

.. code-block:: xml

   <vars load="yaml" include="/path/to/variables.yaml">
   </vars>

name
----

``name="variables_tag_name"``

Dot-separated string that specifies path in results structure where variables should be saved. By default it is empty, meaning variables will not be saved in results. Path string follows all the same rules as for group name attribute, for instance ``{{ var_name }}`` can be used to dynamically form path or ``*`` and ``**`` can indicate what type of structure to use for child - list or dictionary.

**Example:**

.. code-block:: xml

   <vars name="vars.info**.{{ hostname }}">
   hostname='switch-1'
   serial='AS4FCVG456'
   model='WS-3560-PS'
   </vars>

   <vars name="vars.ip*">
   IP="Undefined"
   MASK="255.255.255.255"
   </vars>

