Functions
=========

Input functions can be used to process input data before parsing.

GoTTP supports the following input functions:

* **extract_commands** - Extract command output from text data
* **test** - Test function for debugging
* **macro** - Run input data through macro functions
* **functions** - Pipe-separated list of input functions

extract_commands
----------------

``extract_commands="command1, command2, ..."``

Extracts command output from text data. Requires hostname in the data.

**Example:**

.. code-block:: xml

   <input load="text" extract_commands="show interfaces">
   router1#show interfaces
   GigabitEthernet0/0 is up, line protocol is up
   router1#show version
   Version 15.1
   </input>

test
----

``test=""``

Test function that prints information about input data (for debugging).

functions
---------

``functions="function1('args') | function2('args')"``

Pipe-separated list of input functions to apply in order.

macro
-----

``macro="macro1, macro2, ..."``

Comma-separated list of macro function names to run input data through.

