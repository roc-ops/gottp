# Contributing to GoTTP

Thank you for your interest in contributing to GoTTP! This document provides guidelines for setting up your development environment and running tests.

## Prerequisites

- **Go**: Version 1.21 or higher.
- **Python**: Version 3.x (required for running comparison tests against the original TTP).

## Getting Started

1. **Clone the repository**
   ```bash
   git clone https://github.com/roc-ops/gottp.git
   cd gottp
   ```

2. **Install Dependencies**
   ```bash
   go mod download
   ```

## Project Structure

- `cmd/`: Main applications (`gottp` CLI and `gottp-gen`).
- `internal/`: Core logic, including the parser, compiler, and runtime.
- `docs/`: Documentation files.
- `tests/`: Integration and comparison tests.
- `ttp-original/`: (Optional) Clone of the original Python TTP repository for comparison testing.

## Running Tests

### Unit Tests

Run the standard Go test suite:

```bash
go test ./...
```

### Comparison Tests

To run the comparison tests, you need to have the original Python TTP library available.

1. **Install Python TTP** (if not already installed):
   You can install it from PyPI or use the local `ttp-original` folder if you have it.
   ```bash
   pip install ttp
   ```
   *Or, if using the local submodule:*
   ```bash
   cd ttp-original
   pip install -e .
   ```

2. **Run Comparison Tests**:
   ```bash
   go test ./test/comparison/...
   ```

## Code Style

- Follow standard Go conventions (Effective Go).
- Ensure all code is formatted with `gofmt`.
- Run `go vet` before submitting changes.
