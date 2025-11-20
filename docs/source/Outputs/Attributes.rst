Attributes
==========

Output attributes control how results are formatted and returned.

name
----

``name="string"``

Uniquely identifies output within template.

format
------

``format="formatter_name"``

Formatter name to use. Supported formatters:

* ``raw`` - Default, results returned as-is
* ``json`` - JSON format
* ``yaml`` - YAML format
* ``csv`` - CSV format
* ``table`` - Table format (list of lists)
* ``pprint`` - Pretty-print format
* ``tabulate`` - Plain-text table format
* ``excel`` - Excel spreadsheet format (.xlsx)
* ``jinja2`` - Jinja2 template rendering
* ``n2g`` - Network diagram XML format (GraphML or draw.io)

**Example:**

.. code-block:: xml

   <output format="json" returner="terminal"/>
   </output>

returner
--------

``returner="returner_name"``

Returner name to use. Supported returners:

* ``self`` - Return to calling function (default)
* ``terminal`` - Print to terminal
* ``file`` - Save to file
* ``syslog`` - Send to Syslog server

**Example:**

.. code-block:: xml

   <output format="json" returner="file" url="/path/to/output" filename="results.json"/>
   </output>

path
----

``path="path.to.results"``

Dot-separated path to results data to format. If not specified, all results are formatted.

**Example:**

.. code-block:: xml

   <output format="json" returner="terminal" path="interfaces"/>
   </output>

functions
---------

``functions="function1, function2"``

Comma-separated list of output functions to apply. Currently not implemented in GoTTP.

url
---

``url="path"``

File path for file returner. Directory where file should be stored.

**Example:**

.. code-block:: xml

   <output format="json" returner="file" url="/path/to/output" filename="results.json"/>
   </output>

filename
--------

``filename="name"``

Filename for file returner. Can contain time formatters (similar to Python strftime).

**Example:**

.. code-block:: xml

   <output format="json" returner="file" url="/path/to/output" filename="results_%Y-%m-%d.json"/>
   </output>

