# Migration Guide: Python TTP to GoTTP

This guide helps you migrate from Python TTP to GoTTP.

## Key Differences

### 1. Stateless Design

**Python TTP** (stateful):
```python
from ttp import ttp

parser = ttp()
parser.add_template(template)
parser.add_input(data)
parser.parse()
result = parser.result()
parser.clear_input()  # Must reset for next parse
```

**GoTTP** (stateless):
```go
import "github.com/roc-ops/gottp"

// Compile once
compiled, _ := gottp.CompileTemplate(template)

// Parse multiple times - no reset needed!
result1, _ := compiled.Parse(gottp.Inputs{"input": data1}, nil, nil)
result2, _ := compiled.Parse(gottp.Inputs{"input": data2}, nil, nil)
```

### 2. API Structure

**Python TTP**:
```python
parser = ttp(data=data, template=template)
parser.parse()
result = parser.result(format="json")
```

**GoTTP**:
```go
compiled, _ := gottp.CompileTemplate(template)
result, _ := compiled.Parse(
    gottp.Inputs{"Default_Input": data},
    nil, // vars
    nil, // options
)
```

### 3. Python-Compatible API

GoTTP also provides a Python-compatible stateful API for easier migration:

```go
parser := gottp.NewParser()
parser.AddTemplate(template)
parser.AddInput(data, "Default_Input")
parser.Parse()
result := parser.Result()
parser.ClearInput()  // Still available for compatibility
```

## Template Compatibility

GoTTP is **fully compatible** with Python TTP templates. Your existing templates will work without modification:

```xml
<!-- This works in both Python TTP and GoTTP -->
<group name="interfaces">
interface {{ interface }}
 ip address {{ ip }}/{{ mask }}
</group>
```

## Function Compatibility

Most Python TTP functions are available in GoTTP:

| Python TTP | GoTTP | Status |
|------------|-------|--------|
| `upper` | `upper` | ✅ |
| `lower` | `lower` | ✅ |
| `split` | `split` | ✅ |
| `join` | `join` | ✅ |
| `IP` | `IP` | ✅ |
| `MAC` | `MAC` | ✅ |
| `count` | `count` | ✅ |
| `record` | `record` | ✅ |
| `set` | `set` | ✅ |

## Macro Compatibility

### Python Macros

**Python TTP**:
```xml
<macro name="process">
def process(data):
    return data.upper()
</macro>
```

**GoTTP** (Starlark - default):
```xml
<macro name="process" language="starlark">
def process(data):
    return data.upper()
</macro>
```

**GoTTP** (Python - optional, requires -tags python):
```xml
<macro name="process" language="python">
def process(data):
    return data.upper()
</macro>
```

Note: Python macro support requires building with `-tags python` and Python development headers. See [PYTHON_MACROS.md](PYTHON_MACROS.md) for setup instructions.

### JavaScript Macros

GoTTP supports JavaScript macros (not available in Python TTP):

```xml
<macro name="process" language="javascript">
function process(data) {
    return data.toUpperCase();
}
</macro>
```

## Migration Steps

### Step 1: Install GoTTP

```bash
go get github.com/roc-ops/gottp
```

### Step 2: Convert Your Code

**Before (Python)**:
```python
from ttp import ttp

parser = ttp(data="config.txt", template="template.txt")
parser.parse()
result = parser.result(format="json")
print(result)
```

**After (Go)**:
```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    
    "github.com/roc-ops/gottp"
)

func main() {
    // Read template
    template, _ := os.ReadFile("template.txt")
    
    // Compile
    compiled, _ := gottp.CompileTemplate(string(template))
    
    // Read data
    data, _ := os.ReadFile("config.txt")
    
    // Parse
    result, _ := compiled.Parse(
        gottp.Inputs{"Default_Input": string(data)},
        nil,
        nil,
    )
    
    // Output
    jsonData, _ := json.MarshalIndent(result, "", "  ")
    fmt.Println(string(jsonData))
}
```

### Step 3: Use Python-Compatible API (Optional)

If you prefer the stateful API:

```go
parser := gottp.NewParser()
parser.AddTemplate(string(template))
parser.AddInput(string(data), "Default_Input")
parser.Parse()
result := parser.Result()

jsonData, _ := json.MarshalIndent(result, "", "  ")
fmt.Println(string(jsonData))
```

### Step 4: Optimize for Go

Take advantage of GoTTP's stateless design:

```go
// Compile once at startup
var compiled *gottp.CompiledTemplate

func init() {
    template, _ := os.ReadFile("template.txt")
    compiled, _ = gottp.CompileTemplate(string(template))
}

// Use in request handlers (no state management needed!)
func handleRequest(data string) {
    result, _ := compiled.Parse(
        gottp.Inputs{"Default_Input": data},
        nil,
        nil,
    )
    // process result
}
```

## Common Patterns

### Pattern 1: Batch Processing

**Python TTP**:
```python
parser = ttp(template=template)
for data_file in data_files:
    parser.clear_input()  # Reset required
    parser.add_input(data_file)
    parser.parse()
    result = parser.result()
    process(result)
```

**GoTTP**:
```go
compiled, _ := gottp.CompileTemplate(template)
for _, data := range dataList {
    result, _ := compiled.Parse(
        gottp.Inputs{"Default_Input": data},
        nil,
        nil,
    )
    process(result)  // No reset needed!
}
```

### Pattern 2: Concurrent Processing

**Python TTP** (requires multiprocessing):
```python
from multiprocessing import Pool

def parse_data(data):
    parser = ttp(template=template)
    parser.add_input(data)
    parser.parse()
    return parser.result()

with Pool() as pool:
    results = pool.map(parse_data, data_list)
```

**GoTTP** (native goroutines):
```go
compiled, _ := gottp.CompileTemplate(template)

var wg sync.WaitGroup
results := make([]interface{}, len(dataList))

for i, data := range dataList {
    wg.Add(1)
    go func(idx int, d string) {
        defer wg.Done()
        result, _ := compiled.Parse(
            gottp.Inputs{"Default_Input": d},
            nil,
            nil,
        )
        results[idx] = result
    }(i, data)
}

wg.Wait()
```

### Pattern 3: Template Serialization

**Python TTP**:
```python
# Not directly supported - templates are text files
```

**GoTTP**:
```go
// Compile and save
compiled, _ := gottp.CompileTemplate(template)
data, _ := gottp.SaveCompiledTemplate(compiled, "gob")
os.WriteFile("template.gob", data, 0644)

// Load later
data, _ := os.ReadFile("template.gob")
compiled, _ := gottp.LoadCompiledTemplate(data, "gob")
```

## Performance Considerations

### Compilation

- **Python TTP**: Templates are parsed on every run
- **GoTTP**: Templates are compiled once and reused

### Memory

- **Python TTP**: State accumulates between parses (requires `clear_input()`)
- **GoTTP**: Stateless design means no memory leaks

### Concurrency

- **Python TTP**: Requires multiprocessing (heavyweight)
- **GoTTP**: Native goroutines (lightweight)

## Troubleshooting

### Issue: Template not matching

**Solution**: Check that your template syntax is correct. GoTTP uses the same syntax as Python TTP.

### Issue: Functions not working

**Solution**: Ensure function names match Python TTP conventions. Most functions are available.

### Issue: Macros not executing

**Solution**: 
- Check macro language attribute (default is "starlark")
- Ensure macro function name matches usage
- Verify macro syntax for chosen language

### Issue: Performance concerns

**Solution**:
- Compile templates once and reuse
- Use code generation for production
- Consider parallel parsing for large datasets

## Getting Help

- Check the [User Guide](USER_GUIDE.md)
- Review [examples](examples/)
- See [API documentation](https://pkg.go.dev/github.com/roc-ops/gottp)

## Feature Parity

| Feature | Python TTP | GoTTP | Notes |
|---------|------------|-------|-------|
| Template parsing | ✅ | ✅ | Full compatibility |
| Pattern matching | ✅ | ✅ | Full compatibility |
| Functions | ✅ | ✅ | Most functions available |
| Macros (Python) | ✅ | ✅ | Via Starlark or CGO |
| Macros (JavaScript) | ❌ | ✅ | GoTTP exclusive |
| Template extension | ✅ | ✅ | Full compatibility |
| Stateless parsing | ❌ | ✅ | GoTTP advantage |
| Code generation | ❌ | ✅ | GoTTP advantage |
| Concurrent parsing | ⚠️ | ✅ | Native goroutines |

## Conclusion

GoTTP provides a smooth migration path from Python TTP while offering significant advantages:

- **Stateless design** eliminates state management issues
- **Better performance** through compilation
- **Native concurrency** with goroutines
- **Code generation** for production deployments

Your existing templates will work without modification, making migration straightforward.

