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

// ExtractModuleName extracts the name of the module or submodule in YANG
// content: NAME in "module NAME {" or "submodule NAME {".
func ExtractModuleName(content string) (string, error) {
	_, name, err := ExtractModuleHeader(content)
	return name, err
}

// ExtractModuleHeader returns the keyword ("module" or "submodule") and the
// name of the module or submodule in YANG content. A submodule is loaded
// like a module: it is registered under the module it belongs to, whose
// include statements then resolve (RFC 7950 5.1).
func ExtractModuleHeader(content string) (kind, name string, err error) {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 || (fields[0] != "module" && fields[0] != "submodule") {
			// "module{" or "submodule{" with no space is not a header.
			continue
		}
		if len(fields) < 2 {
			continue
		}
		name = strings.TrimSuffix(fields[1], "{")
		name = strings.Trim(strings.TrimSpace(name), `"'`)
		if name != "" {
			return fields[0], name, nil
		}
	}
	return "", "", fmt.Errorf("module or submodule name not found in YANG content")
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
