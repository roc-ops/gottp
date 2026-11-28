Anonymous Group
===============

Anonymous groups are groups without a ``name`` attribute. They are merged into their parent group's results.

Usage
-----

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    <group>
    ip address {{ ip }}
    </group>
   </group>

Results:

.. code-block:: json

   {
     "interfaces": [
       {
         "interface": "Loopback0",
         "ip": "192.168.0.1"
       }
     ]
   }

The anonymous group's results are merged into the parent group's results.

Nested Anonymous Groups
-----------------------

Anonymous groups can be nested:

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    <group>
    ip address {{ ip }}
     <group>
     description {{ description }}
     </group>
    </group>
   </group>

All results are merged into the parent group.


Top-Level Anonymous Group
-------------------------

A top-level group without a ``name``, and the implicit group gottp creates for a template with no group tags at all, give each input's result as a list of matches, as Python TTP does:

.. code-block:: xml

   <group>
   interface {{ interface }}
   </group>

With the input ``interface Lo0`` / ``interface Lo1``, ``NewParser().Result()`` is:

.. code-block:: json

   [[[{"interface": "Lo0"}, {"interface": "Lo1"}]]]

The outer list has one item per template and the next one item per input; the innermost list is the input's matches. ``CompiledTemplate.Parse`` returns the template's result, ``[[{"interface": "Lo0"}, {"interface": "Lo1"}]]``.

- An unnamed ``<group>`` always gives a list, even for one match: ``[[{"interface": "Lo0"}]]``.
- A template with no group tags (for example just ``interface {{ interface }}``) is an implicit group named ``_anonymous_``, as in Python TTP. One match gives a dict, ``[{"interface": "Lo0"}]``, and several give a list.
- Each input keeps its own result: with a second input ``interface Lo2`` the result is ``[[{"interface": "Lo0"}, {"interface": "Lo1"}], [{"interface": "Lo2"}]]``.
- Results of named top-level groups in the same input are added to the list as one more item. With a ``<group name="host">`` for ``hostname {{ name }}`` next to the anonymous group (inside ``<template>``) and the input line ``hostname r1``: ``[[{"interface": "Lo0"}, {"interface": "Lo1"}, {"host": {"name": "r1"}}]]``.
- An input with no anonymous match gives just its other results (``{}`` when there are none).
- With ``<template results="per_template">``, the matches of all inputs form one list: ``[{"interface": "Lo0"}, {"interface": "Lo1"}, {"interface": "Lo2"}]``.
