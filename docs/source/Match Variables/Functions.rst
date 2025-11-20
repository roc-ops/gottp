Functions
=========

GoTTP contains a set of match variable functions that can be applied to match results to transform them or validate and filter match results.

Action Functions
----------------

Action functions transform match results into desired state.

.. list-table:: Action functions
   :widths: 15 85
   :header-rows: 1

   * - Name
     - Description
   * - ``upper`` ✅
     - Convert string to uppercase
   * - ``lower`` ✅
     - Convert string to lowercase
   * - ``strip`` ✅
     - Remove leading and trailing whitespace
   * - ``split`` ✅
     - Split string by delimiter (default: comma)
   * - ``join`` ✅
     - Join list of strings with delimiter (default: comma)
   * - ``replace`` ✅
     - Replace all occurrences of old string with new string
   * - ``IP`` ✅
     - Validate and normalize IP address
   * - ``mac_eui`` ✅
     - Transform MAC address to EUI format (e.g., "00:07:11:22:7a:73")
   * - ``count`` ✅
     - Count occurrences or return length of string/list/dict
   * - ``record`` ✅
     - Save match result in template variable with given name
   * - ``set`` ✅
     - Set match result to specific value (used in group context)
   * - ``lookup`` ✅
     - Find match value in lookup table and return result
   * - ``to`` ✅
     - Convert value to specified type (int, float, str, bool)
   * - ``to_list`` ✅
     - Convert value to list (wraps single value in list)
   * - ``to_str`` ✅
     - Convert value to string
   * - ``to_int`` ✅
     - Convert value to integer
   * - ``to_float`` ✅
     - Convert value to float
   * - ``to_ip`` ✅
     - Transform result to IP address object
   * - ``item`` ✅
     - Return item at given index of list or dict key
   * - ``let`` ✅
     - Assign provided value to match variable if current value is empty
   * - ``void`` ✅
     - Always returns nil, allowing to skip saving the result
   * - ``joinmatches`` ✅
     - Join matches from multiple pattern matches using provided character
   * - ``sformat`` ✅
     - Format string using format specifiers (similar to Python str.format)
   * - ``resub`` ✅
     - Replace pattern with replacement in match using regex substitution
   * - ``prepend`` ✅
     - Prepend provided string at the beginning of match result
   * - ``append`` ✅
     - Append provided string to the end of match result
   * - ``copy`` ✅
     - Copy match value into another variable
   * - ``raise`` ✅
     - Raises error with message provided
   * - ``default`` ✅
     - Default value to use for match variable if no matches produced
   * - ``unrange`` ✅
     - Expand range notation (e.g., "1-5" -> ["1", "2", "3", "4", "5"])
   * - ``uptimeparse`` ✅
     - Parse uptime string and convert to seconds
   * - ``truncate`` ✅
     - Truncate string to specified length
   * - ``to_cidr`` ✅
     - Transform netmask to CIDR notation (e.g., "255.255.255.0" -> "24")
   * - ``replaceall`` ✅
     - Run string replace for all given value pairs (old1, new1, old2, new2, ...)
   * - ``resuball`` ✅
     - Run regex substitute for all given pattern/replacement pairs
   * - ``rlookup`` ✅
     - Reverse lookup table (find key by value)
   * - ``to_net`` ✅
     - Transform to IP network object (CIDR notation)
   * - ``print`` ✅
     - Print match result to terminal (for debugging)
   * - ``macro`` ✅
     - Run match result against macro function
   * - ``chain`` ✅
     - Add functions from chain variable (expanded during template compilation)

Condition Functions
-------------------

Condition functions perform checks with match results and return True or False.

.. list-table:: Condition functions
   :widths: 15 85
   :header-rows: 1

   * - Name
     - Description
   * - ``equal`` ✅
     - Check if match is equal to provided value
   * - ``notequal`` ✅
     - Check if match is not equal to provided value
   * - ``startswith`` ✅
     - Check if match starts with certain string
   * - ``startswith_re`` ✅
     - Check if match starts with certain string using regular expression
   * - ``endswith`` ✅
     - Check if match ends with certain string
   * - ``endswith_re`` ✅
     - Check if match ends with certain string using regular expression
   * - ``notstartswith`` ✅
     - Check if match does not start with certain string
   * - ``notstartswith_re`` ✅
     - Check if match does not start with certain string using regular expression
   * - ``notendswith`` ✅
     - Check if match does not end with certain string
   * - ``notendswith_re`` ✅
     - Check if match does not end with certain string using regular expression
   * - ``contains`` ✅
     - Check if match contains certain string
   * - ``contains_re`` ✅
     - Check if match contains certain string using regular expression
   * - ``exclude`` ✅
     - Check if match does not contain certain string
   * - ``exclude_re`` ✅
     - Check if match does not contain certain string using regular expression
   * - ``isdigit`` ✅
     - Check if match is digit string (all characters are digits)
   * - ``notdigit`` ✅
     - Check if match is not digit string
   * - ``greaterthan`` ✅
     - Check if match is greater than given value (numeric comparison)
   * - ``lessthan`` ✅
     - Check if match is less than given value (numeric comparison)
   * - ``is_ip`` ✅
     - Check if match is valid IP address (IPv4 or IPv6)
   * - ``cidr_match`` ✅
     - Check if IP overlaps with given CIDR prefix

Not Yet Implemented
-------------------

The following functions from Python TTP are not yet implemented in GoTTP:

* ``dns`` - Perform DNS forward lookup
* ``geoip_lookup`` - GeoIP2 database lookup
* ``gpvlookup`` - Glob Patterns Values lookup
* ``ip_info`` - Dictionary with IP address information
* ``rdns`` - Perform DNS reverse lookup
* ``to_unicode`` - Convert to unicode (Python 2 specific)

Usage Examples
--------------

**Basic String Functions**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface | upper | strip }}
    description {{ description | lower }}
   </group>

**Type Conversion**

.. code-block:: xml

   <group name="stats">
   packets: {{ packets | to_int }}
   rate: {{ rate | to_float }}
   </group>

**Conditional Functions**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface | contains("Ethernet") }}
    ip address {{ ip }}
   </group>

**Function Chaining**

.. code-block:: xml

   <group name="data">
   value: {{ value | strip | split(",") | join(":") }}
   </group>

**Record and Set**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    ip address {{ ip | record("last_ip") }}
    vrf {{ vrf | set("last_vrf") }}
   </group>

**Lookup**

.. code-block:: xml

   <group name="interfaces">
   interface {{ interface }}
    ip address {{ ip | lookup("ip_table", add_field="hostname") }}
   </group>

**Join Matches**

.. code-block:: xml

   <group name="vlans">
   vlan {{ vlan | joinmatches(",") }}
   </group>

**Unrange**

.. code-block:: xml

   <group name="vlans">
   vlan {{ vlan_range | unrange("-", ",") }}
   </group>

Function Arguments
------------------

Functions can accept positional and keyword arguments:

**Positional Arguments**

.. code-block:: xml

   {{ value | split(",") }}
   {{ value | replace("old", "new") }}

**Keyword Arguments** (where supported)

.. code-block:: xml

   {{ value | unrange(rangechar="-", joinchar=",") }}

Note: Keyword argument support varies by function. Check function documentation for details.

