Template Tag
============

Template tag is the root container for all TTP template definitions. Multiple template tags can be defined within a single template file to create independent templates.

Template Attributes
-------------------

.. list-table:: Template attributes
   :widths: 15 85
   :header-rows: 1

   * - Attribute
     - Description
   * - ``name``
     - Template name for identification
   * - ``base_path``
     - Base path for relative file paths in inputs
   * - ``results``
     - Results formation mode: "per_input" or "per_template"
   * - ``pathchar``
     - Character to separate path items (default: ".")

name
----

``name="template_name"``

Template name for identification. Useful when multiple templates are defined in a single file.

**Example:**

.. code-block:: xml

   <template name="interface_parser">
   <group name="interfaces">
   interface {{ interface }}
   </group>
   </template>

base_path
---------

``base_path="/path/to/base"``

Base path for relative file paths in inputs. All relative paths in input ``url`` attributes will be resolved relative to this base path.

**Example:**

.. code-block:: xml

   <template base_path="/my/base/path/">
   <input name="config" load="file" url="Data/config.txt">
   </input>
   </template>

results
-------

``results="per_input"`` or ``results="per_template"``

Controls how results are structured:

* ``per_input`` (default) - Results are organized per input, each input produces separate result entry
* ``per_template`` - All inputs are combined into a single result entry

**Example - per_input:**

.. code-block:: xml

   <template results="per_input">
   <input name="config1" load="text">...</input>
   <input name="config2" load="text">...</input>
   <group name="interfaces">...</group>
   </template>

Results structure:

.. code-block:: json

   [
     {
       "interfaces": [...]
     },
     {
       "interfaces": [...]
     }
   ]

**Example - per_template:**

.. code-block:: xml

   <template results="per_template">
   <input name="config1" load="text">...</input>
   <input name="config2" load="text">...</input>
   <group name="interfaces">...</group>
   </template>

Results structure:

.. code-block:: json

   [
     {
       "interfaces": [...]
     }
   ]

pathchar
--------

``pathchar="."``

Character to separate path items in dynamic path formation. Default is ".".

**Example:**

.. code-block:: xml

   <template pathchar="_">
   <group name="interfaces_{{ interface }}">
   interface {{ interface }}
   </group>
   </template>

Multiple Templates
------------------

Multiple template tags can be defined within a single file:

.. code-block:: xml

   <template name="template1">
   <group name="interfaces">
   interface {{ interface }}
   </group>
   </template>

   <template name="template2">
   <group name="vlans">
   vlan {{ vlan }}
   </group>
   </template>

Each template is processed independently and produces separate results.

