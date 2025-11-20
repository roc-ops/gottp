package yang

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// LoadModuleFromFile loads a YANG module from a file path
func LoadModuleFromFile(path string) (string, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Read file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read YANG file: %w", err)
	}

	return string(content), nil
}

// LoadModuleFromURL loads a YANG module from a URL
func LoadModuleFromURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch YANG module from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch YANG module: HTTP %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read YANG module content: %w", err)
	}

	return string(content), nil
}

// LoadModuleFromString loads a YANG module from a string (content is already provided)
func LoadModuleFromString(name, content string) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("YANG module content is empty")
	}
	return content, nil
}

// ExtractModuleName extracts the module name from YANG content
func ExtractModuleName(content string) (string, error) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			// Extract module name: "module module-name {"
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				moduleName := strings.TrimSuffix(parts[1], "{")
				moduleName = strings.TrimSpace(moduleName)
				return moduleName, nil
			}
		}
	}
	return "", fmt.Errorf("module name not found in YANG content")
}

// ExtractImports extracts all import statements from YANG content
func ExtractImports(content string) []string {
	var imports []string
	lines := strings.Split(content, "\n")
	inImport := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for import statement: "import module-name {"
		if strings.HasPrefix(trimmed, "import ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				moduleName := strings.TrimSuffix(parts[1], "{")
				moduleName = strings.TrimSpace(moduleName)
				imports = append(imports, moduleName)
				inImport = true
			}
		} else if inImport && strings.HasPrefix(trimmed, "}") {
			// End of import block
			inImport = false
		}
	}
	return imports
}

