# Python Macro Support

GoTTP supports Python macros via CGO, allowing you to use Python code in your templates for maximum compatibility with Python TTP.

## Requirements

To use Python macros, you need:

1. **Python 3.x** installed on your system
2. **Python development headers** (python3-dev package)
3. **pkg-config** to locate Python libraries
4. Build GoTTP with the `python` build tag

## Installation

### Ubuntu/Debian

```bash
sudo apt-get install python3-dev pkg-config
```

### macOS

```bash
brew install python3 pkg-config
```

### Building with Python Support

```bash
go build -tags python ./...
```

Or when using as a dependency:

```bash
go build -tags python your_app.go
```

## Usage

### Basic Python Macro

```xml
<macro name="process_data" language="python">
def process_data(data):
    return data.upper() + "_processed"
</macro>
```

### Using in Templates

```
{{ value | macro('process_data') }}
```

### Accessing _ttp_ Context

Python macros have access to the `_ttp_` context dictionary:

```xml
<macro name="custom_process" language="python">
def custom_process(data):
    # Access _ttp_ context
    site = _ttp_.get('site', 'unknown')
    return f"{site}:{data}"
</macro>
```

## Example

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/roc-ops/gottp"
)

func main() {
	template := `
<macro name="format_ip" language="python">
def format_ip(ip):
    parts = ip.split('.')
    return f"{parts[0]}.{parts[1]}.X.X"
</macro>

<group name="interfaces">
interface {{ interface }}
 ip address {{ ip | macro('format_ip') }}
</group>
`

	// Build with: go build -tags python
	compiled, err := gottp.CompileTemplate(template)
	if err != nil {
		log.Fatal(err)
	}

	data := `
interface Loopback0
 ip address 192.168.0.1/24
!
`

	result, err := compiled.Parse(
		gottp.Inputs{"Default_Input": data},
		nil,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonData))
}
```

## Limitations

1. **Build Tag Required**: Python support is optional and requires the `python` build tag
2. **CGO Dependency**: Uses CGO, which may affect cross-compilation
3. **Python Runtime**: Requires Python to be installed on the target system
4. **Performance**: Python execution via CGO has overhead compared to native Go or Starlark

## Alternatives

If you don't need Python macros, consider:

- **Starlark** (default): Python-like syntax, no external dependencies
- **JavaScript**: Using goja runtime, no external dependencies

Both are faster and don't require build tags.

## Troubleshooting

### Error: "Python macro support is not enabled"

**Solution**: Build with `-tags python`:
```bash
go build -tags python
```

### Error: "pkg-config: command not found"

**Solution**: Install pkg-config:
```bash
# Ubuntu/Debian
sudo apt-get install pkg-config

# macOS
brew install pkg-config
```

### Error: "Package python3 was not found"

**Solution**: Install Python development headers:
```bash
# Ubuntu/Debian
sudo apt-get install python3-dev

# macOS
brew install python3
```

### Error: "failed to initialize Python interpreter"

**Solution**: Ensure Python 3 is properly installed and accessible:
```bash
python3 --version
pkg-config --modversion python3
```

## Best Practices

1. **Use Starlark when possible**: It's faster and doesn't require build tags
2. **Only use Python for compatibility**: If you need exact Python TTP compatibility
3. **Test without Python**: Ensure your code works with the stub implementation
4. **Document build requirements**: If your project uses Python macros, document the build tag requirement

## Migration from Python TTP

If you're migrating from Python TTP and have existing Python macros, they should work with minimal changes:

**Python TTP**:
```xml
<macro name="process">
def process(data):
    return data.upper()
</macro>
```

**GoTTP**:
```xml
<macro name="process" language="python">
def process(data):
    return data.upper()
</macro>
```

The only change is adding `language="python"` to the macro tag.

