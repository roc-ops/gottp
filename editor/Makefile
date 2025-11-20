.PHONY: build clean wasm-exec

# Go installation path (defaults to system Go)
GO := go
GOROOT := $(shell go env GOROOT)

# Output directory
OUTPUT_DIR := .
WASM_FILE := gottp.wasm
WASM_EXEC := wasm_exec.js

build: wasm-exec
	@echo "Building GoTTP WASM module..."
	@echo "Ensuring dependencies are up to date..."
	cd wasm && $(GO) mod tidy
	cd wasm && GOOS=js GOARCH=wasm $(GO) build -o ../$(WASM_FILE) .
	@echo "Build complete: $(WASM_FILE)"

wasm-exec:
	@echo "Copying wasm_exec.js..."
	@if [ -f "$(GOROOT)/lib/wasm/wasm_exec.js" ]; then \
		cp "$(GOROOT)/lib/wasm/wasm_exec.js" "$(OUTPUT_DIR)/$(WASM_EXEC)"; \
		echo "Copied wasm_exec.js from $(GOROOT)/lib/wasm/"; \
	elif [ -f "$(GOROOT)/misc/wasm/wasm_exec.js" ]; then \
		cp "$(GOROOT)/misc/wasm/wasm_exec.js" "$(OUTPUT_DIR)/$(WASM_EXEC)"; \
		echo "Copied wasm_exec.js from $(GOROOT)/misc/wasm/"; \
	else \
		echo "Error: wasm_exec.js not found in Go installation at $(GOROOT)"; \
		echo "Checked locations:"; \
		echo "  - $(GOROOT)/lib/wasm/wasm_exec.js"; \
		echo "  - $(GOROOT)/misc/wasm/wasm_exec.js"; \
		echo "Please ensure Go is properly installed."; \
		exit 1; \
	fi

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(WASM_FILE) $(WASM_EXEC)
	@echo "Clean complete"

help:
	@echo "GoTTP Editor Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build      - Build WASM module and copy wasm_exec.js"
	@echo "  wasm-exec  - Copy wasm_exec.js from Go installation"
	@echo "  clean      - Remove build artifacts"
	@echo "  help       - Show this help message"

