Installation
============

GoTTP is a Go module and can be installed using standard Go tooling.

Prerequisites
-------------

* Go 1.18 or later
* For Python macro support (optional): Python 3.x development headers and CGO enabled

Installation
------------

Install GoTTP using ``go get``:

.. code-block:: bash

   go get github.com/roc-ops/gottp

Or add to your ``go.mod`` file:

.. code-block:: go

   require github.com/roc-ops/gottp v0.1.0

Dependencies
------------

GoTTP uses the following dependencies:

* `starlark-go <https://github.com/google/starlark-go>`_ - Starlark macro execution
* `goja <https://github.com/dop251/goja>`_ - JavaScript macro execution
* `yaml.v3 <https://gopkg.in/yaml.v3>`_ - YAML parsing
* Standard Go libraries (encoding/json, encoding/csv, etc.)

Optional Dependencies
---------------------

**Python Macro Support** (requires build tag):

To enable Python macro support, you need:

1. Python 3.x development headers installed
2. Build with the ``python`` tag:

.. code-block:: bash

   go build -tags python

Note: Python macro support requires CGO and may not be available on all platforms.

Verification
------------

Verify installation by importing the package:

.. code-block:: go

   package main

   import (
       "fmt"
       "github.com/roc-ops/gottp"
   )

   func main() {
       fmt.Println("GoTTP version:", gottp.Version)
   }

