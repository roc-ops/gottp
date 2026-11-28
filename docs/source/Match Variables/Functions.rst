Functions
=========

GoTTP contains a set of match variable functions that can be applied to match results to transform them or validate and filter match results.

Function call syntax
--------------------

Match variable functions, group attributes and ``functions=``/``chain=``, input functions and output functions are all read the same way, as Python TTP 0.10.1 reads them (``ttp/utils/get_attributes.py``):

* A chain such as ``{{ name | contains("vlan") | upper }}`` is split on every ``|``, **including one inside quotes**. ``re("UP|DOWN")`` therefore becomes the two broken calls ``re("UP`` and ``DOWN")``, as in Python TTP, and gottp adds a compile warning (``GetWarnings()``) naming the variable and the broken-off pieces. To pass an argument that contains ``|``, define it in ``<vars>`` and pass the variable's name: ``re(UP_OR_DOWN)``.
* A call is ``name`` or ``name(arguments)``. The arguments are Python call arguments: positional values, then ``key=value`` keyword arguments. Each value is a Python literal: a string (with Python's escapes, ``r"..."`` raw strings and adjacent-string concatenation), an int, a float, ``True``/``False``/``None``, or a list, tuple or dict of literals. So ``re("\\d+")`` and ``re(r"\d+")`` are both the regex ``\d+``, and an ``=`` inside a quoted string is just a character: ``sformat("v={}")``.
* A bare word is its own text, so ``contains(vlan)`` is ``contains("vlan")``; ``true``, ``false`` and ``none`` in any case are ``True``, ``False`` and ``None``. A bare word is a Python name, so it may use any letters: ``contains(Москва)``. In a match variable, input or output function, a string argument that names a template variable is that variable's value.
* A group attribute is read as the call ``name(value)``: ``contains="ip, description"`` is ``contains(ip, description)``.
* An argument that isn't a literal is passed as its **raw text**, with a compile warning:

  * An unquoted word such as ``set(auto-decision)`` or ``exclude(-VM-)``, which Python TTP evaluates as an expression (``auto - decision``) and fails on. The warning suggests quoting it, ``set("auto-decision")``, which works in both. This is a gottp extension.
  * A Python expression such as ``set(1+2)``, ``x.upper()``, an f-string or ``b"x"``. Python TTP evaluates it; gottp doesn't, and passes the text ``"1+2"``.

* Broken call syntax is a **compile error**: an unterminated quote, unbalanced brackets, an empty argument (``f(a,,b)``), a positional argument after a keyword one, a repeated keyword, and ``macro()`` with a keyword argument or, on an output, without a name.
* In a match variable, a name that isn't a match function runs the value's own Python method of that name (see `Python type methods`_), as in Python TTP. A function name that is neither, in any scope, gives a compile warning, with up to three similar names (Python TTP's ``guess``), and is skipped when the template runs. A Python TTP function gottp doesn't implement (group ``lookup`` and ``str_to_unicode``; output ``deepdiff``, ``validate`` and ``validate_yangson``) is warned about as "not supported by gottp" instead, without suggestions; a match variable keeps its value. A function passed only in ``ParseOptions.Functions`` is warned about too, since it isn't known at compile time; one passed in ``CompileOptions.Functions`` isn't.

Calls are read once, when the template compiles.

Action Functions
----------------

Action functions transform match results into desired state.

.. list-table:: Action functions
   :widths: 15 85
   :header-rows: 1

   * - Name
     - Description
   * - ``join`` ✅
     - Join list of strings with delimiter (default: comma)
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
   * - ``startswith_re`` ✅
     - Check if match starts with certain string using regular expression
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

Python type methods
-------------------

When a match variable function isn't a match function (and isn't a custom function from ``CompileOptions`` or ``ParseOptions``), Python TTP 0.10.1 calls the value's own method or attribute of that name (``getattr(data, name)``, ``ttp.py:2464``). gottp does the same, with Go ports of **Python 3.12's** methods of the value's type:

* A **True or False result is a filter**: ``{{ name | isalpha }}`` drops the match when the value isn't all letters. Any other result **replaces the value**: ``{{ v | find("2") }}`` saves an int, ``{{ v | partition(",") }}`` a list (a tuple is a list).
* The arguments are the call's Python values, as TTP passes them (a template variable's name isn't replaced by its value here).
* A name the value's type doesn't have (``{{ v | to_int | upper }}``) is Python's ``AttributeError``, which TTP logs and skips: the **value is kept**.
* **Any other exception fails the parse**, as it fails Python TTP's: ``index("z")`` when there is no ``z`` (``ValueError``), ``center("a")`` (``TypeError``), ``"{}-{}" | format("x")`` (``IndexError``). ``Parse`` returns an error naming the variable, the call and the exception.
* Methods follow **Python 3.12**, Unicode 15.0.0 included: ``upper("straße")`` is ``STRASSE``, ``title`` capitalizes after any uncased character (``they're`` is ``They'Re``), a final ``Σ`` lowercases to ``ς``, the ``is*`` tests and the whitespace that ``split()`` and ``strip()`` remove are Python's character classes.
* A name that is a TTP match function wins, as in TTP: ``count``, ``copy``, ``join``, ``isdigit`` and ``append`` are TTP's functions, not the methods.

The value's type is its Go form: ``str`` is a string, ``int`` is an int (``to_int``), ``float`` a float64 (``to_float``), ``list`` a list (``split``), ``dict`` a map (``ip_info``) and ``bool`` a bool, which has int's methods, as in Python.

.. list-table:: Ported methods and attributes
   :widths: 15 85
   :header-rows: 1

   * - Type
     - Names
   * - ``str``
     - ``capitalize``, ``casefold``, ``center``, ``count``, ``endswith``, ``expandtabs``, ``find``, ``format``, ``format_map``, ``index``, ``isalnum``, ``isalpha``, ``isascii``, ``isdecimal``, ``isdigit``, ``isidentifier``, ``islower``, ``isnumeric``, ``isprintable``, ``isspace``, ``istitle``, ``isupper``, ``join``, ``ljust``, ``lower``, ``lstrip``, ``maketrans``, ``partition``, ``removeprefix``, ``removesuffix``, ``replace``, ``rfind``, ``rindex``, ``rjust``, ``rpartition``, ``rsplit``, ``rstrip``, ``split``, ``splitlines``, ``startswith``, ``strip``, ``swapcase``, ``title``, ``translate``, ``upper``, ``zfill``
   * - ``int``, ``bool``
     - ``as_integer_ratio``, ``bit_count``, ``bit_length``, ``conjugate``, ``is_integer``; attributes ``real``, ``imag``, ``numerator``, ``denominator``
   * - ``float``
     - ``as_integer_ratio``, ``conjugate``, ``hex``, ``is_integer``; attributes ``real``, ``imag``
   * - ``list``
     - ``count``, ``copy``, ``index``
   * - ``dict``
     - ``copy``, ``get``, ``items``, ``keys``, ``values``

``upper``, ``lower``, ``strip``, ``lstrip``, ``rstrip``, ``split``, ``replace``, ``startswith`` and ``endswith`` are not TTP match functions, so they are these methods. gottp used to have its own versions, which differed from Python: ``split`` trimmed each part and split on ``,`` by default (Python splits on whitespace), ``replace`` ignored its count, ``startswith``/``endswith`` took no tuple or start/end, ``strip`` treated ``None`` as the characters ``N``, ``o``, ``n``, ``e``, ``upper``/``lower`` used Go's simple case mappings (``ß`` stayed ``ß``) and all of them turned a number into a string first, where Python keeps the value.

Not ported, with a compile warning (``GetWarnings()``), and the value is kept:

* Methods that change the value in place: list ``clear``, ``extend``, ``insert``, ``pop``, ``remove``, ``reverse``, ``sort``; dict ``clear``, ``pop``, ``popitem``, ``setdefault``, ``update``. They return ``None``, which Python TTP saves as the value; gottp keeps the value instead.
* Methods whose result has no JSON form: ``str.encode`` and ``int.to_bytes`` (bytes), and the class methods ``int.from_bytes``, ``float.fromhex`` and ``dict.fromkeys``.

Differences from Python TTP (see also ``test/comparison/KNOWN_DIFFERENCES.md``):

* ``keys``, ``values`` and ``items`` give lists (``items`` as ``[key, value]`` lists) in key order, since gottp's maps keep no order. Python TTP saves the view object itself, which its JSON and YAML outputs can't write.
* ``get(x)`` in Python TTP is read as ``re(x)``, a regex for the variable (TTP's pattern lookup finds its own ``get`` function), so ``dict.get`` is never reached there. gottp runs ``dict.get``.
* A dict literal's keys are strings in gottp, so ``translate({97: "x"})`` looks up ``"97"``, and ``get(1)`` finds nothing. A list literal is accepted where Python wants a tuple (``startswith(["a", "b"])``).

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

