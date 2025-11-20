# GoTTP Browser Editor

A powerful browser-based TTP (Template Text Parser) template editor that runs entirely client-side using Go WebAssembly. This tool allows you to create, edit, and test TTP templates with advanced features like variables, lookup tables, and multiple output formats.

## Features

* **Client-side Processing**: Runs entirely in the browser using Go WebAssembly
* **Monaco Editor**: Professional code editing with syntax highlighting, auto-completion, and IntelliSense
* **Multiple Output Formats**: JSON, YAML, table, and CSV formats
* **Global Variables**: Define reusable variables for templates
* **Lookup Tables**: Define lookup tables for data enrichment (coming soon)
* **Export/Import**: Save and share complete configurations as `.gottp.export` files
* **Workspace Management**: Save, load, and manage multiple workspaces
* **Real-time Processing**: Auto-process templates as you type
* **Error Marking**: Visual error indicators in template editor
* **Built-in Examples**: Pre-loaded TTP templates for common use cases
* **Professional UI**: Modern dark theme with dropdown menus, modals, and notifications
* **TTP Syntax Highlighting**: Custom syntax highlighting for TTP templates
* **Auto-completion**: Context-aware suggestions for TTP functions

## Getting Started

### Prerequisites

* A modern web browser with WebAssembly support
* Go 1.21 or later (for building the WASM module)
* Make (optional, for using the Makefile)

### Important: Build First!

**You must build the WASM module before running the editor.** The editor requires `gottp.wasm` and `wasm_exec.js` files which are generated during the build process.

### Building the WASM Module

1. Navigate to the editor directory:
   ```bash
   cd editor
   ```

2. Build the WASM module:
   ```bash
   make build
   ```

   This will:
   - Compile the Go code to WebAssembly (`gottp.wasm`)
   - Copy `wasm_exec.js` from your Go installation

   **Note:** If you see errors about missing files when running the editor, make sure you've run `make build` first.

3. Alternatively, build manually:
   ```bash
   cd wasm
   go mod tidy
   GOOS=js GOARCH=wasm go build -o ../gottp.wasm .
   # Try lib/wasm first (newer Go versions), then misc/wasm (older versions)
   cp $(go env GOROOT)/lib/wasm/wasm_exec.js .. 2>/dev/null || \
     cp $(go env GOROOT)/misc/wasm/wasm_exec.js ..
   ```

### Running the Application

1. **Local Development Server** (recommended):
   ```bash
   python3 -m http.server 8080
   ```
   Then open http://localhost:8080 in your browser.

2. **Direct File Access**: You can also open `index.html` directly in your browser, though some features may be limited due to CORS restrictions.

### Usage

1. **Load the Application**: Open the website and wait for the Go WebAssembly runtime to initialize
2. **Input Data**: Paste your raw text data in the left panel
3. **Create Template**: Write or paste your TTP template in the middle panel
4. **Configure Variables** (optional): Click "Config" → "Variables" to define global variables
5. **Process**: Click "Process" or enable auto-processing
6. **View Results**: See parsed results in the right panel
7. **Change Format**: Select output format (JSON, YAML, Table, CSV) from the dropdown
8. **Export**: Click "File" → "Export" to download results or save complete configuration

### Example Templates

The application includes several built-in examples:

* **Cisco Interface Configuration**: Parse interface settings
* **Routing Table**: Extract routing information
* **System Log Parsing**: Parse various log formats
* **Network Device Inventory**: Extract device information
* **BGP Neighbors**: Parse BGP neighbor information
* **Simple Key-Value Pairs**: Parse simple configuration pairs

Click "Actions" → "Load Example" to try these templates.

## Advanced Features

### Global Variables

Define reusable variables that can be used in your templates:

1. Click "Config" → "Variables"
2. Enter variables in JSON format
3. Use variables in templates with `{{ variable_name }}`

**Example Variables:**
```json
{
  "site": "datacenter1",
  "region": "us-east",
  "environment": "production"
}
```

### Export/Import

Save and share complete configurations:

* **Export**: Click "File" → "Export" to download `.gottp.export` file
* **Import**: Click "File" → "Import" to load configuration from file
* **Workspace**: Use "Workspace" → "Save"/"Load" for local workspace management
* **Manage Workspaces**: Click "Workspace" → "Manage" to organize saved workspaces

### User Interface

The application features a modern, organized interface:

* **Main Actions**: Process, Download, Output Format selector
* **Actions Dropdown**: Clear All, Load Example
* **Config Dropdown**: Inputs, Variables, Lookups
* **File Dropdown**: Export, Import
* **Workspace Dropdown**: Save, Load, Manage workspaces
* **Auto-completion**: Context-aware suggestions for TTP functions
* **Syntax Highlighting**: Custom highlighting for TTP templates
* **Professional Modals**: Beautiful dialogs for configuration and management

### Keyboard Shortcuts

* `Ctrl/Cmd + Enter`: Process template
* `Ctrl/Cmd + L`: Load example
* `Ctrl/Cmd + K`: Clear all inputs
* `Escape`: Close modals and dropdowns

## Technical Details

### Architecture

* **Frontend**: HTML5, CSS3, JavaScript (ES6+)
* **Go Runtime**: Go WebAssembly (WASM)
* **Text Processing**: GoTTP library
* **Code Editor**: Monaco Editor with custom TTP language support
* **Styling**: Modern CSS with dark theme, dropdowns, and modals
* **Storage**: LocalStorage for workspace persistence

### File Structure

```
editor/
├── index.html              # Main application page
├── css/
│   └── main.css            # Application styles
├── js/
│   ├── app.js              # Main application logic
│   ├── wasm-bridge.js      # WASM loading and function wrappers
│   ├── examples.js         # Sample templates and data
│   └── monaco-config.js    # Monaco Editor configuration
├── wasm/
│   ├── main.go             # WASM entry point and JS bindings
│   └── go.mod              # Go module for WASM
├── Makefile                # Build script
├── README.md               # This file
└── .gitignore              # Ignore WASM build artifacts
```

### Browser Compatibility

* Chrome/Chromium 57+
* Firefox 52+
* Safari 11+
* Edge 16+

WebAssembly support is required for the editor to function.

## TTP Template Syntax

TTP uses a template-based approach to parse text data. Here are some key concepts:

### Basic Template Structure

```xml
<template name="example">
<group name="items*">
{{ variable1 }} {{ variable2 }}
{{ variable3 | to_int }} {{ variable4 | re("\\d+") }}
</group>
</template>
```

### Common TTP Functions

* `to_int`: Convert to integer
* `to_float`: Convert to float
* `re("pattern")`: Regular expression matching
* `contains("text")`: Check if text contains substring
* `split("delimiter")`: Split text by delimiter
* `upper`: Convert to uppercase
* `lower`: Convert to lowercase
* `strip`: Remove whitespace

### Group Types

* `group*`: Multiple results (list)
* `group`: Single result (dict)
* `group**`: Nested groups

For more detailed TTP documentation, visit the [GoTTP User Guide](../USER_GUIDE.md).

## Development

### Adding New Examples

To add new examples, edit `js/examples.js` and add entries to the `TTP_EXAMPLES` object:

```javascript
'new_example': {
    name: 'Example Name',
    description: 'Example description',
    data: 'Raw text data...',
    template: 'TTP template...'
}
```

### Customizing Styles

The application uses CSS custom properties for easy theming. Main colors and styles are defined in `css/main.css`.

### Extending Functionality

The modular architecture makes it easy to extend:

* `wasmBridge`: Handles WASM and GoTTP operations
* `GottpEditor`: Manages UI, Monaco editors, modals, and user interactions
* `examples.js`: Contains sample data and templates
* **Monaco Editor**: Professional code editing with TTP syntax highlighting and auto-completion
* **Export/Import System**: File-based configuration sharing
* **Modal System**: Reusable modal components for configuration dialogs
* **Dropdown System**: Organized menu system for better UX

## Troubleshooting

### Common Issues

1. **Slow Initial Load**: WASM downloads and initializes on first load. Subsequent loads are cached.
2. **Memory Issues**: Large datasets may cause memory issues. Try processing smaller chunks.
3. **Template Errors**: Check template syntax. The application provides detailed error messages and visual indicators.
4. **Browser Compatibility**: Ensure your browser supports WebAssembly.
5. **Auto-completion Issues**: If suggestions don't appear, ensure you're typing in the template editor and check the context.
6. **Empty Results**: If you get empty results `[{}]` when the template should match:
   - Check for trailing spaces/newlines in your pattern - try removing them
   - Verify variable whitespace matches the input data spacing
   - Try using `_exact_space_` flag if you need exact space matching
   - Group names with wildcards (like `show.cable.modem*`) should work, but verify the pattern matches

### Performance Tips

* Use specific regular expressions in templates
* Avoid overly complex nested groups
* Process data in reasonable chunks
* Clear results between large processing runs
* Take advantage of auto-completion for faster template writing

## License

This project is licensed under the same license as the main GoTTP project.

## Acknowledgments

* Built on the excellent Go WebAssembly support
* Uses the powerful GoTTP library
* Code editing powered by Monaco Editor
* Inspired by the TTP-Editor project

