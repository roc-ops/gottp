Performance
===========

GoTTP is designed for high performance parsing of semi-structured text data. This document outlines performance characteristics and optimization strategies.

Compiled Templates
------------------

GoTTP uses compiled templates for better performance:

* **Compilation**: Templates are compiled once into an optimized internal representation
* **Reuse**: Compiled templates can be reused multiple times without recompilation
* **Serialization**: Compiled templates can be saved and loaded for faster startup

**Example:**

.. code-block:: go

   // Compile once
   compiled, _ := gottp.CompileTemplate(template)

   // Use many times
   for _, data := range dataList {
       result, _ := compiled.Parse(gottp.Inputs{"Default_Input": data}, nil, nil)
   }

Thread Safety
-------------

Compiled templates are immutable and thread-safe:

* **Concurrent Use**: Same compiled template can be used from multiple goroutines
* **No Locking**: No mutexes needed for read operations
* **Parallel Parsing**: Parse different inputs in parallel using goroutines

**Example:**

.. code-block:: go

   compiled, _ := gottp.CompileTemplate(template)

   var wg sync.WaitGroup
   for _, data := range dataList {
       wg.Add(1)
       go func(d string) {
           defer wg.Done()
           result, _ := compiled.Parse(gottp.Inputs{"Default_Input": d}, nil, nil)
           // Process result
       }(data)
   }
   wg.Wait()

Template Serialization
----------------------

Pre-compile templates and save them for faster startup:

.. code-block:: go

   // Compile and save
   compiled, _ := gottp.CompileTemplate(template)
   gottp.SaveCompiledTemplate(compiled, "template.gob", "gob")

   // Load later (much faster)
   compiled, _ := gottp.LoadCompiledTemplate("template.gob", "gob")

Performance Tips
----------------

1. **Compile Once**: Always compile templates once and reuse them
2. **Use Serialization**: Save compiled templates for faster startup
3. **Parallel Parsing**: Use goroutines for parallel parsing of multiple inputs
4. **Input Filtering**: Use input ``groups`` attribute to parse only relevant data
5. **Efficient Patterns**: Use specific patterns (WORD, IP, etc.) instead of generic regex when possible
6. **Avoid Complex Macros**: Keep macros simple for better performance

Benchmarking
------------

GoTTP's performance characteristics:

* **Compilation**: ~1-10ms for typical templates
* **Parsing**: ~0.1-1ms per input for typical data
* **Memory**: Low memory footprint due to stateless design

Actual performance depends on:
* Template complexity
* Input data size
* Number of groups and patterns
* Macro complexity

For best results, benchmark with your specific templates and data.

Performance Comparison with Python TTP
---------------------------------------

GoTTP has been benchmarked against Python TTP using the comparison test suite. Results show significant performance improvements:

**Compilation Performance:**
* **GoTTP**: ~18-19µs per operation
* **Python TTP**: ~252-255µs per operation
* **Speedup**: ~13.39x faster

**Parsing Performance:**
* **GoTTP**: ~19µs per operation
* **Python TTP**: ~317-318µs per operation
* **Speedup**: ~16.71x faster

**Large Input Performance:**
* **GoTTP**: ~1.77ms per operation (98 KB input)
* **Python TTP**: ~5.17ms per operation
* **Speedup**: ~2.92x faster

**Overall Performance:**
* **Average Speedup**: ~15.05x faster than Python TTP

**Macro Performance:**
* **GoTTP (Starlark)**: ~55.8ms per operation (651 KB input with macros)
* **GoTTP (Native Go)**: ~35.7ms per operation (same input)
* **Python TTP**: ~48.4ms per operation
* **Native Go macros**: 1.36x faster than Python TTP, 1.56x faster than Starlark

These benchmarks were run with a typical template containing multiple groups and patterns, processing network configuration data. Performance improvements are due to:

* Compiled execution vs interpreted execution
* Go's efficient runtime and garbage collector
* Stateless design eliminating overhead
* Optimized regex compilation and matching

To run performance comparisons yourself:

.. code-block:: bash

   go test ./test/comparison -v -run TestPerformanceComparison

Or use the benchmark functions:

.. code-block:: bash

   go test ./test/comparison -bench=BenchmarkGoTTP

