Functions
=========

Output functions can be used to process results before formatting.

GoTTP supports the following output functions:

* **traverse** - Get data at a specific path in results tree
* **dict_to_list** - Transform dictionary to list of dictionaries
* **is_equal** - Compare results with expected structure
* **functions** - Pipe-separated list of output functions

traverse
--------

``traverse("path")`` or ``traverse(path='dot.separated.path', strict=True)``

Walks the results tree to the given path and returns data at that location.

**Arguments:**

* **path** (string, required): Dot-separated path to traverse (e.g., ``'interfaces'`` or ``'interfaces.Loopback0'``)
* **strict** (bool, optional): If ``True`` (default), returns empty dict if path not found. If ``False``, returns empty dict on failure.

**Argument Order:**

The function accepts arguments in multiple formats:

1. **Positional argument**: ``traverse('interfaces')`` - The first argument is treated as the path
2. **Keyword arguments**: ``traverse(path='interfaces', strict=True)`` - Explicit parameter names
3. **Mixed format**: ``traverse("path='interfaces'", "strict=True")`` - String arguments with parameter names

**Return Value:**

The function returns the data at the specified path. When traversing nested list structures (per_input format), the result is wrapped in a list to preserve the structure:

* Input: ``[{interfaces: [{interface: "Loopback0"}]}]``
* Traverse to ``'interfaces'``: Returns ``[{interface: "Loopback0"}]`` (wrapped in list)

**Examples:**

.. code-block:: xml

   <!-- Simple path traversal -->
   <output functions="traverse('interfaces')"/>

   <!-- Dot-separated path -->
   <output functions="traverse(path='interfaces.Loopback0')"/>

   <!-- With strict mode -->
   <output functions="traverse(path='interfaces', strict=False)"/>

   <!-- In function chain -->
   <output functions="traverse('interfaces') | json"/>

dict_to_list
------------

``dict_to_list="key_name='interface', path='dot.separated.path'"``

Transforms a dictionary to a list of dictionaries, adding the original key as a value.

**Example:**

.. code-block:: xml

   <output functions="dict_to_list(key_name='interface')"/>

is_equal
--------

``is_equal``

Compares results with expected structure loaded from output tag content.

**Example:**

.. code-block:: xml

   <output functions="is_equal">
   [{"interface": "GigabitEthernet0/0", "status": "up"}]
   </output>

functions
---------

``functions="function1('args') | function2('args')"``

Pipe-separated list of output functions to apply in order.

