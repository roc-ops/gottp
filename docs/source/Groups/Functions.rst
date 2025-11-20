Functions
=========

Group functions can be applied to group results to transform them or validate and filter match results.

Condition functions help to evaluate group results and return ``False`` or ``True``. If ``False`` is returned, group results will be discarded.

.. list-table:: Group functions
   :widths: 15 85
   :header-rows: 1

   * - Name
     - Description
   * - ``contains`` ✅
     - Check if group result contains match for at least one of given variables
   * - ``set`` ✅
     - Get value from template variables and assign it to group result key
   * - ``record`` ✅
     - Save (record) variable value in results object and global scope dictionaries
   * - ``delete`` ✅
     - Delete given keys from group results
   * - ``expand`` ✅
     - Expand match variable dot-separated name to dictionary
   * - ``itemize`` ✅
     - Produce list of items extracted out of group match results dictionary
   * - ``lookup`` ✅
     - Lookup match value in lookup table, other group or template results
   * - ``containsall`` ✅
     - Check if group result contains matches for all given variables
   * - ``exclude`` ✅
     - Invalidate group results if any of given keys present
   * - ``excludeall`` ✅
     - Invalidate group results if all given keys present
   * - ``equal`` ✅
     - Verify that key's value is equal to provided value
   * - ``to_int`` ✅
     - Convert given keys to integer
   * - ``contains_val`` ✅
     - Check if certain key contains certain value
   * - ``exclude_val`` ✅
     - Check if certain key contains certain value (inverse)
   * - ``sformat`` ✅
     - Format provided string with match result and/or template variables
   * - ``items2dict`` ✅
     - Combine values of key_name and value_name keys in key-value pair
   * - ``to_ip`` ✅
     - Transform given values in IP address object
   * - ``cerberus`` ✅
     - Filter results using schema-based validation (Cerberus-compatible)
   * - ``validate`` ✅
     - Add validation information to results without filtering

All Major Functions Implemented
--------------------------------

All major group functions from Python TTP are now implemented in GoTTP:

* ``macro`` ✅ - Name of the macro function to run against group result
* ``functions`` or ``chain`` ✅ - String containing list of functions to run group results through
* ``cerberus`` ✅ - Filter results using schema-based validation (Cerberus-compatible)
* ``validate`` ✅ - Add validation information to results without filtering
* ``void`` - Invalidate group results (use ``void`` attribute instead)

contains
--------

``contains="variable1, variable2, variableN"``

Checks if group results contain match for at least one of the specified variables. If no variables are found, the whole group result is discarded.

**Example:**

.. code-block:: xml

   <group name="interfaces" functions="contains('ip')">
   interface {{ interface }}
    description {{ description }}
    ip address {{ ip }}/{{ mask }}
   </group>

set
---

``set="source, target"``

Gets value from template variables dictionary (or uses source as literal if not found) and assigns it to the target key in group results.

**Parameters:**
* ``source`` - Name of source variable to retrieve value from (first argument)
* ``target`` - Name of field to save into (second argument, optional, defaults to source name if "_use_source_")

**Example:**

.. code-block:: xml

   <vars>
   default_vrf = "default"
   </vars>

   <group name="interfaces" functions="set('vrf', 'default_vrf')">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

This will look up the variable ``default_vrf`` (which has value "default") and set it to the key ``vrf`` in the group results. If the source variable is not found, the source string itself is used as the value.

record
------

``record="variable_name"``

Saves (records) variable value in results object and global scope dictionaries. The recorded variable can be referenced in other groups or macros.

**Example:**

.. code-block:: xml

   <group name="interfaces" functions="record('last_interface')">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

delete
------

``delete="key1, key2"``

Deletes given keys from group results.

**Example:**

.. code-block:: xml

   <group name="interfaces" functions="delete('mask')">
   interface {{ interface }}
    ip address {{ ip }}/{{ mask }}
   </group>

expand
------

``expand="variable_name"``

Expands match variable dot-separated name to dictionary structure.

**Example:**

If ``interface.name`` is matched, it will be expanded to ``{"interface": {"name": "value"}}``.

itemize
-------

``itemize="key_name"``

Produces list of items extracted out of group match results dictionary.

**Example:**

.. code-block:: xml

   <group name="vlans" functions="itemize('vlan_list')">
   vlan {{ vlan_list | split(",") }}
   </group>

lookup
------

``lookup="key, name='lookup_table', add_field='new_field'"``

Looks up match value in lookup table, other group or template results.

**Supported parameters:**

* ``key`` - Name of match variable to use for lookup
* ``name`` - Dot-separated path to lookup table data location
* ``template`` - Dot-separated path to template results to use for lookups
* ``group`` - Dot-separated path to group results to use for lookups
* ``add_field`` - String of new field/key name to assign lookup results to
* ``replace`` - Boolean, if True, lookup results will replace looked up value
* ``update`` - Boolean, if lookup result is a dictionary and update set to True, that dictionary will be merged with group results

**Example:**

.. code-block:: xml

   <lookup name="ip_table" load="yaml">
   '10.0.0.1': 'host1'
   '10.0.0.2': 'host2'
   </lookup>

   <group name="interfaces" functions="lookup('ip', name='ip_table', add_field='hostname')">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

Function Chaining
-----------------

Multiple functions can be chained using comma separation:

.. code-block:: xml

   <group name="interfaces" functions="contains('ip'), record('last_interface'), delete('mask')">
   interface {{ interface }}
    ip address {{ ip }}/{{ mask }}
   </group>

Functions are executed in the order specified.

