Inputs
======

Input system is used to define various input data sources, help to retrieve data, prepare it and map to the groups for parsing.

Input tags can be defined within templates or data can be provided programmatically via the API.

Input Loaders
-------------

GoTTP supports the following input loaders:

* **text** - Plain text data (default)
* **yaml** - YAML structured data
* **json** - JSON structured data
* **csv** - CSV formatted data
* **file** - Load data from file
* **directory** - Load data from directory
* **url** - Load data from HTTP/HTTPS URL ✅

Input Attributes
----------------

.. list-table:: Input attributes
   :widths: 15 85
   :header-rows: 1

   * - Attribute
     - Description
   * - ``name``
     - Uniquely identifies input within template (default: "Default_Input")
   * - ``groups``
     - Comma-separated list of group(s) that should be used to parse input data
   * - ``load``
     - Loader name that should be used to load text data from input tag
   * - ``url``
     - Single or list of URLs of data location (file paths)
   * - ``extensions``
     - Single or list of file extensions to load (e.g., "txt", "log", "conf")
   * - ``filters``
     - Single or list of regular expressions to filter file names

Examples
--------

**Text Input**

.. code-block:: xml

   <input load="text">
   interface Loopback0
    ip address 192.168.0.1/24
   </input>

**File Input**

.. code-block:: xml

   <input name="config" load="file" url="/path/to/config.txt">
   </input>

**Directory Input**

.. code-block:: xml

   <input name="configs" load="directory" url="/path/to/configs" extensions="txt,conf">
   </input>

**YAML Input**

.. code-block:: xml

   <input name="data" load="yaml">
   interfaces:
     - name: Loopback0
       ip: 192.168.0.1
   </input>

**JSON Input**

.. code-block:: xml

   <input name="data" load="json">
   {"interfaces": [{"name": "Loopback0", "ip": "192.168.0.1"}]}
   </input>

**CSV Input**

.. code-block:: xml

   <input name="data" load="csv">
   interface,ip,mask
   Loopback0,192.168.0.1,24
   Vlan100,10.0.0.1,24
   </input>

Programmatic Input
------------------

Inputs can also be provided programmatically:

.. code-block:: go

   compiled, _ := gottp.CompileTemplate(template)
   result, _ := compiled.Parse(
       gottp.Inputs{
           "Default_Input": "interface Loopback0\n ip address 192.168.0.1/24",
           "config2": "interface Vlan100\n ip address 10.0.0.1/24",
       },
       nil,
       nil,
   )

Input Mapping to Groups
-----------------------

Groups can specify which inputs to parse using the ``input`` attribute:

.. code-block:: xml

   <input name="config1" load="text">...</input>
   <input name="config2" load="text">...</input>

   <group name="interfaces" input="config1">
   interface {{ interface }}
   </group>

   <group name="vlans" input="config2">
   vlan {{ vlan }}
   </group>

If no ``input`` attribute is specified, the group parses all inputs.

.. toctree::
   :maxdepth: 2

   Attributes
   Functions

