Outputs
=======

Output system allows to format parsing results and return them to certain destinations.

Output tags can be defined within templates to automatically format and return results.

Output Formatters
-----------------

GoTTP supports the following output formatters:

* **raw** - Default formatter, results returned as-is
* **yaml** - Results transformed into YAML structured, multi-line text
* **json** - Results transformed into JSON structured, multi-line text
* **csv** - Results transformed into CSV spreadsheet format
* **table** - Results transformed into a list of lists, each list representing table row
* **pprint** - Results formatted in a pretty-printed, human-readable format
* **tabulate** - Results formatted as a plain-text table with aligned columns
* **excel** - Results formatted as Excel spreadsheet (.xlsx) with multiple tabs support
* **jinja2** - Results rendered using Jinja2-style templates
* **n2g** - Results formatted as network diagram XML (GraphML or draw.io format)

Output Returners
----------------

GoTTP supports the following returners:

* **self** - Return result to calling function (default)
* **terminal** - Print results to terminal screen
* **file** - Save results to file
* **syslog** - Send results over UDP to Syslog server

Examples
--------

**JSON Output to Terminal**

.. code-block:: xml

   <output format="json" returner="terminal"/>
   </output>

**YAML Output to File**

.. code-block:: xml

   <output format="yaml" returner="file" url="/path/to/output" filename="results.yaml"/>
   </output>

**CSV Output**

.. code-block:: xml

   <output format="csv" returner="file" url="/path/to/output" filename="results.csv"/>
   </output>

**Raw Output**

.. code-block:: xml

   <output format="raw" returner="self"/>
   </output>

Output Attributes
-----------------

.. list-table:: Output attributes
   :widths: 15 85
   :header-rows: 1

   * - Attribute
     - Description
   * - ``name``
     - Uniquely identifies output within template
   * - ``format``
     - Formatter name (raw, json, yaml, csv, table, pprint, tabulate, excel, jinja2, n2g)
   * - ``returner``
     - Returner name (self, terminal, file, syslog)
   * - ``path``
     - Path to results data to format
   * - ``functions``
     - Comma-separated list of output functions to apply
   * - ``url``
     - File path for file returner
   * - ``filename``
     - Filename for file returner

.. toctree::
   :maxdepth: 2

   Attributes
   Formatters
   Returners
   Functions

