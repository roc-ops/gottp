How to Parse Hierarchical (Configuration) Data
===============================================

This guide shows how to parse hierarchical configuration data using nested groups.

Basic Nested Groups
-------------------

**Configuration Data:**

.. code-block:: text

   interface Loopback0
    ip address 192.168.0.1 255.255.255.0
    description Management loopback
   !
   interface GigabitEthernet0/0
    ip address 10.0.0.1 255.255.255.0
    description Uplink
   !

**Template with Nested Groups:**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    <group name="ip_config">
    ip address {{ ip }} {{ mask }}
    </group>
    <group name="description">
    description {{ description | PHRASE }}
    </group>
   </group>

Results:

.. code-block:: json

   {
     "interfaces": [
       {
         "interface": "Loopback0",
         "ip_config": {"ip": "192.168.0.1", "mask": "255.255.255.0"},
         "description": {"description": "Management loopback"}
       }
     ]
   }

Anonymous Nested Groups
-----------------------

Use anonymous groups to merge results:

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    <group>
    ip address {{ ip }} {{ mask }}
    </group>
    <group>
    description {{ description | PHRASE }}
    </group>
   </group>

Results:

.. code-block:: json

   {
     "interfaces": [
       {
         "interface": "Loopback0",
         "ip": "192.168.0.1",
         "mask": "255.255.255.0",
         "description": "Management loopback"
       }
     ]
   }

Deep Nesting
------------

**BGP Configuration:**

.. code-block:: text

   router bgp 65000
    neighbor 10.0.0.1 remote-as 65001
     description Peer to AS65001
    neighbor 10.0.0.2 remote-as 65002
     description Peer to AS65002

**Template:**

.. code-block:: xml

   <group name="bgp">
   router bgp {{ asn }}
    <group name="neighbors">
    neighbor {{ neighbor }} remote-as {{ remote_as }}
     <group>
     description {{ description | PHRASE }}
     </group>
    </group>
   </group>

Using _start_ and _end_ with Nested Groups
-------------------------------------------

When using ``_start_`` indicators in nested groups, patterns after the ``_start_`` pattern will only match within blocks started by the ``_start_`` pattern:

.. code-block:: xml

   <group name="ipv4_afi">
   address-family ipv4 unicast {{ _start_ }}
    route-policy {{ RPL_IN }} in
    route-policy {{ RPL_OUT }} out
   </group>

In this example, ``route-policy`` patterns will only match when ``address-family ipv4 unicast`` matches first. If the data contains ``address-family ipv4 labeled-unicast`` instead, the ``route-policy`` patterns will not match.

Tips
----

1. Use nested groups to match hierarchical structure
2. Use anonymous groups to merge results
3. Match indentation levels in configuration
4. Use ``_start_`` and ``_end_`` for multi-line blocks
5. Patterns before ``_start_`` can start matches independently
6. Patterns after ``_start_`` only merge into matches started by ``_start_`` patterns

