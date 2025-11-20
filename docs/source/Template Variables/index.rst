Template Variables
==================

GoTTP supports definition of arbitrary variables using dedicated XML tags ``<v>``, ``<vars>``, or ``<variables>``. Within these tags variables can be defined in various formats and loaded using one of supported loaders. Variables can also be defined in external text files and loaded using ``include`` attribute.

Various values can be recorded in template variables before, during or after parsing. That additional data can be added to results, used for dynamic path constructions.

Variable Loaders
----------------

GoTTP supports the following variable loaders:

* **python** - Python-style variable definitions (default)
* **yaml** - YAML structured data
* **json** - JSON structured data
* **csv** - CSV formatted data
* **ini** - INI structured data

Examples
--------

**Python Format**

.. code-block:: xml

   <vars>
   hostname = "router1"
   default_vrf = "default"
   </vars>

**YAML Format**

.. code-block:: xml

   <vars load="yaml">
   hostname: router1
   default_vrf: default
   </vars>

**JSON Format**

.. code-block:: xml

   <vars load="json">
   {
     "hostname": "router1",
     "default_vrf": "default"
   }
   </vars>

**CSV Format**

.. code-block:: xml

   <vars load="csv">
   key,value
   hostname,router1
   default_vrf,default
   </vars>

**INI Format**

.. code-block:: xml

   <vars load="ini">
   [defaults]
   hostname=router1
   default_vrf=default
   </vars>

**Include from File**

.. code-block:: xml

   <vars load="yaml" include="/path/to/variables.yaml">
   </vars>

Using Variables
---------------

Variables can be used in:

* Match variable functions (e.g., ``set``, ``record``)
* Group functions
* Dynamic path formation
* Macros (via ``_ttp_["vars"]``)

**Example:**

.. code-block:: xml

   <vars>
   default_vrf = "default"
   </vars>

   <group name="interfaces" functions="set('vrf', 'default_vrf')">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

.. toctree::
   :maxdepth: 2

   Attributes
   Getters

