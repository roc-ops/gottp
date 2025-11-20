Extend Tag
==========

Extend tag helps to extend template with other templates to facilitate re-use of templates. This allows you to include groups, inputs, outputs, variables, lookups, and macros from other templates.

Extend Attributes
-----------------

.. list-table:: Extend attributes
   :widths: 15 85
   :header-rows: 1

   * - Attribute
     - Description
   * - ``template``
     - Path to template file to extend from
   * - ``groups``
     - Comma-separated list of group names to include
   * - ``inputs``
     - Comma-separated list of input names to include
   * - ``outputs``
     - Comma-separated list of output names to include
   * - ``vars``
     - Comma-separated list of variable names to include
   * - ``lookups``
     - Comma-separated list of lookup names to include
   * - ``macro``
     - Comma-separated list of macro names to include

template
--------

``template="/path/to/template.ttp"``

Path to template file to extend from. This is a required attribute.

**Example:**

.. code-block:: xml

   <extend template="/path/to/base_template.ttp">
   </extend>

Selective Extension
-------------------

You can selectively include specific elements from the extended template:

**Include Specific Groups:**

.. code-block:: xml

   <extend template="/path/to/base_template.ttp" groups="interfaces, vlans">
   </extend>

**Include Specific Inputs:**

.. code-block:: xml

   <extend template="/path/to/base_template.ttp" inputs="config1, config2">
   </extend>

**Include Multiple Elements:**

.. code-block:: xml

   <extend template="/path/to/base_template.ttp" 
           groups="interfaces" 
           inputs="config1" 
           vars="defaults" 
           lookups="ip_table">
   </extend>

**Include Macros:**

.. code-block:: xml

   <extend template="/path/to/base_template.ttp" macro="process_data, validate_data">
   </extend>

Full Extension
--------------

If no specific elements are specified, all elements from the extended template are included:

.. code-block:: xml

   <extend template="/path/to/base_template.ttp">
   </extend>

Example
-------

**Base Template (base_template.ttp):**

.. code-block:: xml

   <template name="base">
   <vars>
   default_vrf = "default"
   </vars>

   <lookup name="ip_table" load="yaml">
   '10.0.0.1': 'host1'
   </lookup>

   <group name="interfaces">
   interface {{ interface }}
    ip address {{ ip }}
   </group>
   </template>

**Extended Template:**

.. code-block:: xml

   <template name="extended">
   <extend template="/path/to/base_template.ttp" groups="interfaces" vars="default_vrf">
   </extend>

   <group name="vlans">
   vlan {{ vlan }}
   </group>
   </template>

The extended template will have access to:
* The ``interfaces`` group from the base template
* The ``default_vrf`` variable from the base template
* Its own ``vlans`` group

Note: Extend functionality requires that the base template file is accessible at the specified path.

