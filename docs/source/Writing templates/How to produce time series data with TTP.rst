How to Produce Time Series Data with GoTTP
===========================================

This guide shows how to produce time series data using GoTTP templates and macros.

Adding Timestamps
-----------------

Use macros to add timestamps:

.. code-block:: xml

   <macro language="starlark">
   def add_timestamp(data):
       import time
       data["timestamp"] = time.time()
       return data
   </macro>

   <group name="metrics" macro="add_timestamp">
   interface {{ interface }}
    packets: {{ packets | to_int }}
   </group>

Note: Starlark may have limited time functions. Consider using JavaScript or Python macros for time operations.

Using Variables
---------------

Record timestamps in variables:

.. code-block:: xml

   <vars>
   collection_time = "2024-01-01T12:00:00Z"
   </vars>

   <group name="metrics">
   interface {{ interface }}
    packets: {{ packets | to_int }}
    collection_time: {{ collection_time }}
   </group>

Formatting Time
---------------

Use match functions to format time:

.. code-block:: xml

   <group name="metrics">
   interface {{ interface }}
    timestamp: {{ timestamp | sformat("{}") }}
   </group>

Tips
----

1. Use macros for dynamic timestamps
2. Use variables for static timestamps
3. Format timestamps appropriately for your use case
4. Consider timezone handling

