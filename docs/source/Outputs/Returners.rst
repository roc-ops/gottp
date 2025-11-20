Returners
=========

GoTTP has ``self``, ``terminal``, ``file``, and ``syslog`` returners. The purpose of returner is to return or emit or save data to certain destination.

.. list-table:: Returners
   :widths: 10 90
   :header-rows: 1

   * - Returner
     - Description
   * - ``self`` ✅
     - Return result to calling function (default)
   * - ``terminal`` ✅
     - Print results to terminal screen
   * - ``file`` ✅
     - Save results to file
   * - ``syslog`` ✅
     - Send results over UDP to Syslog server

self
----

Default returner, data processed by output returned back to GoTTP for further processing, that way outputs can be chained to produce required results. Another use case is when GoTTP used as a module, results can be formatted and retrieved.

**Example:**

.. code-block:: xml

   <output format="json" returner="self"/>
   </output>

terminal
--------

Results will be printed to terminal screen (stdout).

**Example:**

.. code-block:: xml

   <output format="json" returner="terminal"/>
   </output>

file
----

Results will be saved to text file on local file system. One file will be produced per template to contain all the results for all the inputs and groups of this template.

**Supported returner attributes:**

* ``url`` - OS path to folder where file should be stored
* ``filename`` - Name of the file, can contain time formatters

**Time formatters for filename:**

* ``%Y`` - Year with century (e.g., 2019)
* ``%m`` - Month as decimal number [01,12]
* ``%d`` - Day of the month as decimal number [01,31]
* ``%H`` - Hour (24-hour clock) as decimal number [00,23]
* ``%M`` - Minute as decimal number [00,59]
* ``%S`` - Second as decimal number [00,61]

**Example:**

.. code-block:: xml

   <output format="json" returner="file" url="/path/to/output" filename="results_%Y-%m-%d_%H-%M-%S.json"/>
   </output>

syslog
------

Results will be sent over UDP to Syslog server.

**Supported returner attributes:**

* ``host`` - Syslog server hostname or IP address
* ``port`` - Syslog server port (default: 514)
* ``facility`` - Syslog facility code
* ``severity`` - Syslog severity level

**Example:**

.. code-block:: xml

   <output format="json" returner="syslog" host="syslog.example.com" port="514"/>
   </output>

