# How to View GoTTP Documentation

There are several ways to view the GoTTP documentation:

## Option 1: View Markdown Files Directly (Quickest)

The documentation includes several Markdown files that can be viewed directly:

### Main Documentation Files:
- **`README.md`** (root) - Project overview and quick start
- **`docs/USER_GUIDE.md`** - Complete user guide
- **`docs/MIGRATION_GUIDE.md`** - Guide for migrating from Python TTP
- **`docs/PYTHON_MACROS.md`** - Python macro setup and usage
- **`CHANGELOG.md`** - Recent changes and fixes

### View in Terminal:
```bash
# View main README
cat README.md

# View user guide
cat docs/USER_GUIDE.md

# Or use a markdown viewer
cat docs/USER_GUIDE.md | less
```

### View in GitHub/GitLab:
Simply navigate to the files in your repository browser - GitHub/GitLab will render Markdown automatically.

### View in VS Code:
VS Code has built-in Markdown preview:
1. Open any `.md` file
2. Press `Cmd+Shift+V` (Mac) or `Ctrl+Shift+V` (Windows/Linux)
3. Or right-click and select "Open Preview"

## Option 2: Build HTML Documentation (Full Documentation)

The complete documentation is written in reStructuredText (RST) format for Sphinx/ReadTheDocs.

### Prerequisites:
```bash
pip install sphinx sphinx-rtd-theme
```

### Build HTML:
```bash
cd docs
make html
```

The HTML output will be in `docs/_build/html/`. Open `docs/_build/html/index.html` in your browser.

### If Makefile doesn't exist:
```bash
cd docs
sphinx-build -b html source _build/html
```

### View Built Documentation:
```bash
# On macOS
open docs/_build/html/index.html

# On Linux
xdg-open docs/_build/html/index.html

# On Windows
start docs/_build/html/index.html
```

## Option 3: View RST Files Directly

The RST source files are in `docs/source/`:

- **`docs/source/index.rst`** - Main documentation index
- **`docs/source/Overview.rst`** - Introduction
- **`docs/source/Quick start.rst`** - Getting started
- **`docs/source/Match Variables/Indicators.rst`** - Match indicators (recently updated)
- **`docs/source/Outputs/Functions.rst`** - Output functions (recently updated)
- And many more...

You can view these files directly, though they're formatted for Sphinx rendering.

## Option 4: Use Online Documentation Tools

### GitHub/GitLab:
- Navigate to the repository
- Click on any `.md` or `.rst` file
- The file will be rendered automatically

### VS Code with Extensions:
- Install "reStructuredText" extension for RST syntax highlighting
- Install "Markdown Preview Enhanced" for better Markdown viewing

## Quick Reference

**For quick viewing:** Use the Markdown files (`README.md`, `docs/USER_GUIDE.md`)

**For complete documentation:** Build HTML with Sphinx (`cd docs && make html`)

**For specific topics:**
- Start with `README.md` for overview
- Check `docs/USER_GUIDE.md` for detailed usage
- See `docs/source/` for complete reference documentation

## Recently Updated Documentation

The following documentation was recently updated:
- `docs/source/Outputs/Functions.rst` - `traverse` function argument order
- `docs/source/Match Variables/Indicators.rst` - `_start_` and `_end_` behavior
- `docs/source/Writing templates/How to parse hierarchical (configuration) data.rst` - Start pattern examples
- `CHANGELOG.md` - Recent fixes and changes

