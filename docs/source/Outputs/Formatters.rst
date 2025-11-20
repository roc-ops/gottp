Formatters
==========

GoTTP supports a number of output formatters.

.. list-table:: Formatters
   :widths: 20 80
   :header-rows: 1

   * - Name
     - Description
   * - ``raw`` ✅
     - Default formatter, results returned as-is (native Go structure)
   * - ``yaml`` ✅
     - Results transformed into YAML structured, multi-line text
   * - ``json`` ✅
     - Results transformed into JSON structured, multi-line text
   * - ``csv`` ✅
     - Results transformed into CSV spreadsheet format
   * - ``table`` ✅
     - Results transformed into a list of lists, each list representing table row
   * - ``pprint`` ✅
     - Results formatted in a pretty-printed, human-readable format (similar to Python pprint)
   * - ``tabulate`` ✅
     - Results formatted as a plain-text table with aligned columns
   * - ``excel`` ✅
     - Results formatted as Excel spreadsheet (.xlsx) with multiple tabs support
   * - ``jinja2`` ✅
     - Results rendered using Jinja2-style templates
   * - ``n2g`` ✅
     - Results formatted as network diagram XML (GraphML or draw.io format)

raw
---

If format is raw, no formatting will be applied and native Go structure will be returned, results will not be converted to string.

**Example:**

.. code-block:: xml

   <output format="raw" returner="self"/>
   </output>

yaml
----

This formatter will run results through YAML encoder to produce YAML structured results.

**Example:**

.. code-block:: xml

   <output format="yaml" returner="terminal"/>
   </output>

json
----

This formatter will run results through JSON encoder to produce JSON structured results.

**Example:**

.. code-block:: xml

   <output format="json" returner="terminal"/>
   </output>

csv
---

Uses table formatter results to emit CSV spreadsheet format.

**Example:**

.. code-block:: xml

   <output format="csv" returner="file" url="/path/to/output" filename="results.csv"/>
   </output>

table
-----

Results transformed into a list of lists, each list representing table row. Used by CSV formatter.

**Example:**

.. code-block:: xml

   <output format="table" returner="terminal"/>
   </output>

excel
-----

Results formatted as Excel spreadsheet (.xlsx file). Supports multiple tabs, custom headers, and styling.

**Prerequisites:** Go excelize library (automatically included)

This formatter uses the table formatter to construct table structure, then writes it to an Excel file.

**Supported attributes:**

* ``path`` - Dot-separated path to results data
* ``headers`` - Comma-separated list of table headers
* ``missing`` - Value to use for missing cells (default: empty string)
* ``key`` - Key name to transform dictionary to list
* ``tab_name`` - Name of the Excel tab (default: "Sheet1")
* ``update`` - Whether to update existing file (default: false)

**Example:**

.. code-block:: xml

   <output format="excel" returner="file" url="/path/to/output" filename="results.xlsx"/>

jinja2
------

Results rendered using Jinja2-style templates. Template should be enclosed in output tag content.

**Prerequisites:** Go pongo2 library (automatically included)

Within the template, the whole parsing results are passed in ``_data_`` variable.

**Example:**

.. code-block:: xml

   <output format="jinja2" returner="terminal">
   {% for input_result in _data_ %}
   {% for item in input_result %}
   Interface: {{ item.interface }}, IP: {{ item.ip }}
   {% endfor %}
   {% endfor %}
   </output>

n2g
---

Results formatted as network diagram XML. Supports GraphML (yEd) and draw.io formats.

**Prerequisites:** None (pure Go implementation)

This formatter takes structured data (list of dictionaries with nodes/links) and generates network topology diagrams.

**Supported attributes:**

* ``module`` - Diagram format: ``yed`` (GraphML) or ``drawio`` (draw.io XML)
* ``path`` - Dot-separated path to results data
* ``method`` - Data loading method: ``from_list`` (default), ``from_dict``, ``from_csv``
* ``node_duplicates`` - How to handle duplicate nodes: ``skip`` (default), ``log``, ``update``
* ``link_duplicates`` - How to handle duplicate links: ``skip`` (default), ``log``, ``update``
* ``algo`` - Layout algorithm: ``grid``, ``kk`` (Kamada-Kawai-like)

**Example:**

.. code-block:: xml

   <output format="n2g" returner="file" filename="network.graphml">
   module = "yed"
   path = "cdp"
   method = "from_list"
   algo = "grid"
   </output>

