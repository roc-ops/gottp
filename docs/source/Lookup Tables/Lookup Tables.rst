Lookup Tables
=============

Lookup tables allow to define a lookup dictionary that can be used to enrich results with additional information or reference results across different templates or groups to combine them. Lookup table can be called from match variable using ``lookup`` function or from group using ``lookup`` group function.

Lookup Table Attributes
-----------------------

.. list-table:: Lookup table attributes
   :widths: 10 90
   :header-rows: 1

   * - Name
     - Description
   * - ``name``
     - Name of the lookup table to reference in match variable ``lookup`` function
   * - ``load``
     - Name of the loader to use to load lookup text
   * - ``include``
     - Specifies location of the file to load lookup table from
   * - ``key``
     - If CSV loader used, ``key`` specifies column name to use as a key
   * - ``database``
     - Name of database loader to use to load lookup data (not yet implemented)

name
----

``name="lookup_table_name"``

String to use as a name for lookup table. This is a required attribute; without it lookup data will not be loaded.

**Example:**

.. code-block:: xml

   <lookup name="ip_table">
   ...
   </lookup>

load
----

``load="loader_name"``

Name of the loader to use to render supplied variables data. Default is ``python``.

Supported loaders:

* ``python`` - Uses Python-style dictionary format
* ``yaml`` - Relies on YAML parser to load YAML structured data
* ``json`` - Used to load JSON formatted variables data
* ``ini`` - INI structured file (using configparser)
* ``csv`` - CSV formatted data loaded with CSV parser

If load is CSV, first column by default will be used to create lookup dictionary. It is possible to supply ``key`` with column name that should be used as keys for row data. If any other type of load provided (e.g., python or yaml), that data must have a dictionary structure, where keys will be compared against match result and on success data associated with given key will be included in results.

**Example:**

.. code-block:: xml

   <lookup name="ip_table" load="yaml">
   '10.0.0.1': 'host1'
   '10.0.0.2': 'host2'
   </lookup>

include
-------

``include="path"``

Specifies location of the file to load lookup table from.

**Example:**

.. code-block:: xml

   <lookup name="ip_table" load="yaml" include="/path/to/lookup.yaml">
   </lookup>

key
---

``key="column_name"``

If CSV loader is used, ``key`` specifies column name to use as a key for lookup dictionary.

**Example:**

.. code-block:: xml

   <lookup name="ip_table" load="csv" key="ip_address">
   ip_address,hostname,location
   10.0.0.1,host1,datacenter1
   10.0.0.2,host2,datacenter2
   </lookup>

Using Lookup Tables
-------------------

Lookup tables can be used in match variables and group functions.

**Match Variable Lookup**

.. code-block:: xml

   <lookup name="ip_table" load="yaml">
   '10.0.0.1': 'host1'
   '10.0.0.2': 'host2'
   </lookup>

   <group name="interfaces">
   interface {{ interface }}
    ip address {{ ip | lookup("ip_table", add_field="hostname") }}
   </group>

**Group Function Lookup**

.. code-block:: xml

   <lookup name="ip_table" load="yaml">
   '10.0.0.1': 'host1'
   '10.0.0.2': 'host2'
   </lookup>

   <group name="interfaces" functions="lookup('ip', name='ip_table', add_field='hostname')">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

Lookup Parameters
-----------------

Lookup functions support the following parameters:

* ``key`` - Name of match variable to use for lookup
* ``name`` - Dot-separated path to lookup table data location
* ``template`` - Dot-separated path to template results to use for lookups
* ``group`` - Dot-separated path to group results to use for lookups
* ``add_field`` - String of new field/key name to assign lookup results to
* ``replace`` - Boolean, if True, lookup results will replace looked up value
* ``update`` - Boolean, if lookup result is a dictionary and update set to True, that dictionary will be merged with group results

**Example:**

.. code-block:: xml

   <group name="interfaces" functions="lookup('ip', name='ip_table', add_field='hostname', replace=False)">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

