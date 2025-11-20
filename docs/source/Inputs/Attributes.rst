Attributes
==========

Input attributes control how input data is loaded and processed.

name
----

``name="string"``

Uniquely identifies input within template. Default value is "Default_Input" and is used internally to store set of data that should be parsed by all groups.

**Example:**

.. code-block:: xml

   <input name="router_config" load="text">
   interface Loopback0
   </input>

groups
------

``groups="group1, group2, ... , groupN"``

Comma-separated string of group names that should be used to parse given input data. Default value is ``all``.

**Example:**

.. code-block:: xml

   <input name="config" groups="interfaces, vlans" load="text">
   ...
   </input>

load
----

``load="loader_name"``

Loader name that should be used to load text data from input tag. Supported loaders:

* ``text`` - Plain text (default)
* ``yaml`` - YAML structured data
* ``json`` - JSON structured data
* ``csv`` - CSV formatted data
* ``file`` - Load from file
* ``directory`` - Load from directory

**Example:**

.. code-block:: xml

   <input load="yaml">
   interfaces:
     - name: Loopback0
   </input>

url
---

``url="path"`` or ``url="path1, path2"``

Single or list of URLs (file paths) of data location. Used with ``file`` or ``directory`` loaders.

**Example:**

.. code-block:: xml

   <input name="config" load="file" url="/path/to/config.txt">
   </input>

   <input name="configs" load="directory" url="/path/to/configs">
   </input>

extensions
----------

``extensions="ext1, ext2"``

Single or list of file extensions to load. Used with ``directory`` loader to filter files by extension.

**Example:**

.. code-block:: xml

   <input name="configs" load="directory" url="/path/to/configs" extensions="txt,conf,log">
   </input>

filters
-------

``filters="pattern1, pattern2"``

Single or list of regular expressions to filter file names. Used with ``directory`` loader.

**Example:**

.. code-block:: xml

   <input name="configs" load="directory" url="/path/to/configs" filters="router.*, switch.*">
   </input>

