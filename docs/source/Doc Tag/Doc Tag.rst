Doc Tag
=======

Doc tag allows to add documentation or comments to templates. The content of doc tags is stored but does not affect template parsing or execution.

Usage
-----

Doc tags can be used to document templates, groups, or any part of a template:

.. code-block:: xml

   <doc>
   This template parses interface configurations from Cisco IOS devices.
   
   It extracts:
   - Interface names
   - IP addresses and masks
   - Descriptions
   </doc>

   <template name="interface_parser">
   <group name="interfaces">
   interface {{ interface }}
    ip address {{ ip }}/{{ mask }}
    description {{ description }}
   </group>
   </template>

Multiple Doc Tags
-----------------

Multiple doc tags can be used throughout a template:

.. code-block:: xml

   <doc>
   Main template documentation
   </doc>

   <template name="parser">
   <doc>
   This group parses interface configurations
   </doc>
   <group name="interfaces">
   interface {{ interface }}
   </group>

   <doc>
   This group parses VLAN configurations
   </doc>
   <group name="vlans">
   vlan {{ vlan }}
   </group>
   </template>

Accessing Documentation
-----------------------

Documentation from doc tags is stored in the template's ``Doc`` field and can be accessed programmatically:

.. code-block:: go

   compiled, _ := gottp.CompileTemplate(template)
   // Documentation is available in compiled.Doc

Note: Doc tags are primarily for documentation purposes and do not affect template execution or results.

