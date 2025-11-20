# GoTTP Documentation

This directory contains the Sphinx documentation for GoTTP.

## Building the Documentation

### Prerequisites

Install Sphinx and the ReadTheDocs theme:

```bash
pip install sphinx sphinx-rtd-theme
```

### Building HTML

From the `docs` directory:

```bash
cd docs
make html
```

The HTML output will be in `docs/_build/html/`.

### Building for ReadTheDocs

The documentation is configured to work with ReadTheDocs. Simply connect your repository to ReadTheDocs and it will build automatically.

## Documentation Structure

The documentation is organized to mirror the original Python TTP documentation structure while noting differences and enhancements in GoTTP:

- **Overview** - Introduction and feature comparison
- **Installation** - Installation instructions
- **Quick Start** - Getting started guide
- **Match Variables** - Match variable patterns, indicators, and functions
- **Groups** - Group attributes and functions
- **Inputs** - Input system and loaders
- **Outputs** - Output formatters and returners
- **Template Variables** - Variable system
- **Lookup Tables** - Lookup table system
- **Macro Tag** - Macro system (Starlark, JavaScript, Python)
- **Template Tag** - Template container
- **Extend Tag** - Template extension
- **Doc Tag** - Documentation tags
- **Forming Results Structure** - Result structure formation
- **Writing Templates** - Template writing guides
- **API Reference** - Go API documentation
- **Differences from Python TTP** - Feature parity and differences
- **Performance** - Performance characteristics and optimization

## Contributing

When adding or updating documentation:

1. Follow the existing RST format
2. Include examples where appropriate
3. Note any differences from Python TTP
4. Update the index.rst if adding new sections
5. Test the build locally before committing

