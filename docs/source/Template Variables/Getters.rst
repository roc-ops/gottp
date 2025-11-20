Getters
=======

Template variables can be accessed in macros using the ``_ttp_`` dictionary.

Accessing Variables
-------------------

Variables are available in ``_ttp_["vars"]`` dictionary within macros.

**Example:**

.. code-block:: xml

   <vars>
   hostname = "router1"
   </vars>

   <macro language="starlark">
   def process_data(data):
       hostname = _ttp_["vars"]["hostname"]
       data["device"] = hostname
       return data
   </macro>

Recording Variables
-------------------

Variables can be recorded during parsing using the ``record`` match function or ``record`` group function.

**Example:**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface | record("last_interface") }}
    ip address {{ ip }}
   </group>

The recorded variable ``last_interface`` can then be accessed in other groups or macros.

Setting Variables
-----------------

Variables can be set using the ``set`` group function.

**Example:**

.. code-block:: xml

   <vars>
   default_vrf = "default"
   </vars>

   <group name="interfaces" functions="set('vrf', 'default_vrf')">
   interface {{ interface }}
    ip address {{ ip }}
   </group>

