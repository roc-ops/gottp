package yang

import (
	"fmt"
	"strings"
	"sync"

	"github.com/openconfig/goyang/pkg/yang"
)

// ModuleSet manages a collection of YANG modules
type ModuleSet struct {
	mu             sync.RWMutex
	modules        map[string]*yang.Module
	schemas        map[string]*yang.Entry
	mset           *yang.Modules     // Shared Modules instance for dependency resolution
	addedModules   map[string]bool   // Track which modules we've added (for error messages)
	prefixToModule map[string]string // Map prefix to module name (e.g., "cms" -> "example-cable-modem-state")
}

// NewModuleSet creates a new empty ModuleSet
func NewModuleSet() *ModuleSet {
	return &ModuleSet{
		modules:        make(map[string]*yang.Module),
		schemas:        make(map[string]*yang.Entry),
		mset:           yang.NewModules(),
		addedModules:   make(map[string]bool),
		prefixToModule: make(map[string]string),
	}
}

// AddModule adds a YANG module to the set
// Note: Modules are parsed immediately but processing (dependency resolution)
// is deferred until ProcessAllModules() is called. This allows dependencies
// to be resolved regardless of the order modules are added.
func (ms *ModuleSet) AddModule(name, content string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Parse the YANG module into the shared Modules instance
	// This allows dependencies to be resolved across all loaded modules
	// We don't process here - that happens in ProcessAllModules()
	if err := ms.mset.Parse(content, name); err != nil {
		return fmt.Errorf("failed to parse YANG module '%s': %w", name, err)
	}

	// Track that we've added this module
	ms.addedModules[name] = true

	// Note: We don't process here - processing happens in ProcessAllModules()
	// after all modules are added, so dependencies are available regardless of order

	return nil
}

// ProcessAllModules processes all parsed modules together
// This should be called after all modules are added to ensure dependencies are resolved
func (ms *ModuleSet) ProcessAllModules() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Process all modules together
	if errs := ms.mset.Process(); len(errs) > 0 {
		// Try to provide more helpful error messages
		var errorMsgs []string
		var missingModules []string
		for _, err := range errs {
			errStr := err.Error()
			// Check if error is about missing module
			if strings.Contains(errStr, "no such module:") {
				// Extract module name from error
				parts := strings.Split(errStr, "no such module:")
				if len(parts) > 1 {
					missingModule := strings.TrimSpace(parts[1])
					missingModules = append(missingModules, missingModule)
					errorMsgs = append(errorMsgs, fmt.Sprintf("missing required module: %s", missingModule))
				} else {
					errorMsgs = append(errorMsgs, errStr)
				}
			} else {
				errorMsgs = append(errorMsgs, errStr)
			}
		}

		// Add helpful context about loaded modules
		if len(ms.addedModules) > 0 {
			moduleList := make([]string, 0, len(ms.addedModules))
			for name := range ms.addedModules {
				moduleList = append(moduleList, name)
			}
			errorMsgs = append(errorMsgs, fmt.Sprintf("currently loaded modules: %v", moduleList))
		}

		// Add suggestion for missing modules
		if len(missingModules) > 0 {
			errorMsgs = append(errorMsgs, fmt.Sprintf("please add the following module(s) to resolve dependencies: %v", missingModules))
		}

		return fmt.Errorf("failed to process YANG modules: %s", strings.Join(errorMsgs, "; "))
	}

	// Update stored modules and schemas after processing
	for name, module := range ms.mset.Modules {
		ms.modules[name] = module
		entry := yang.ToEntry(module)
		if entry != nil {
			ms.schemas[name] = entry
		}

		// Extract and store prefix mapping
		// The prefix is stored in module.Prefix
		if module.Prefix != nil && module.Prefix.Name != "" {
			ms.prefixToModule[module.Prefix.Name] = name
		}
	}

	return nil
}

// GetModule retrieves a module by name
func (ms *ModuleSet) GetModule(name string) (*yang.Module, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	module, ok := ms.modules[name]
	return module, ok
}

// GetSchema retrieves a schema entry by module name
func (ms *ModuleSet) GetSchema(name string) (*yang.Entry, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	schema, ok := ms.schemas[name]
	return schema, ok
}

// RemoveModule removes a module from the set
func (ms *ModuleSet) RemoveModule(name string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	delete(ms.modules, name)
	delete(ms.schemas, name)
}

// ListModules returns a list of all module names
func (ms *ModuleSet) ListModules() []string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	names := make([]string, 0, len(ms.modules))
	for name := range ms.modules {
		names = append(names, name)
	}
	return names
}

// Clear removes all modules from the set
func (ms *ModuleSet) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.modules = make(map[string]*yang.Module)
	ms.schemas = make(map[string]*yang.Entry)
	ms.addedModules = make(map[string]bool)
	ms.prefixToModule = make(map[string]string)
	ms.mset = yang.NewModules() // Reset the shared Modules instance
}

// FindSchemaByPath finds a schema entry by YANG path
// Path format: "module-name:path/to/node" or "prefix:path/to/node" or "path/to/node"
func (ms *ModuleSet) FindSchemaByPath(path string) (*yang.Entry, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// Parse the path
	var moduleName, nodePath string
	if idx := strings.Index(path, ":"); idx > 0 {
		prefixOrModuleName := path[:idx]
		nodePath = path[idx+1:]

		// First, try to find by module name directly
		if _, ok := ms.schemas[prefixOrModuleName]; ok {
			moduleName = prefixOrModuleName
		} else {
			// Try to resolve prefix to module name
			if actualModuleName, ok := ms.prefixToModule[prefixOrModuleName]; ok {
				moduleName = actualModuleName
			} else {
				// Not found as module name or prefix
				return nil, fmt.Errorf("module or prefix '%s' not found (available modules: %v, available prefixes: %v)",
					prefixOrModuleName, ms.getModuleNames(), ms.getPrefixes())
			}
		}
	} else {
		nodePath = path
		// Try to find in all modules
		for name, schema := range ms.schemas {
			entry := ms.findNodeInSchema(schema, nodePath)
			if entry != nil {
				return entry, nil
			}
			// Also try with module name prefix
			if strings.HasPrefix(nodePath, name+":") {
				entry := ms.findNodeInSchema(schema, strings.TrimPrefix(nodePath, name+":"))
				if entry != nil {
					return entry, nil
				}
			}
		}
		return nil, fmt.Errorf("path '%s' not found in any module", path)
	}

	// Get the module schema
	schema, ok := ms.schemas[moduleName]
	if !ok {
		return nil, fmt.Errorf("module '%s' not found", moduleName)
	}

	// Normalize path: convert dots to slashes, remove wildcards, and strip any prefixes
	normalizedPath := ms.normalizePath(nodePath, moduleName)

	// Find the node in the schema
	entry := ms.findNodeInSchema(schema, normalizedPath)
	if entry == nil {
		// Provide helpful error message with available paths
		parts := strings.Split(strings.Trim(normalizedPath, "/"), "/")
		var current *yang.Entry = schema
		var pathSoFar []string
		var availableChildren []string

		// Try to find where the path fails
		for i, part := range parts {
			if current == nil || current.Dir == nil {
				break
			}
			pathSoFar = append(pathSoFar, part)
			if child, ok := current.Dir[part]; ok {
				current = child
			} else {
				// This is where it failed - list available children
				for name := range current.Dir {
					availableChildren = append(availableChildren, name)
				}
				return nil, fmt.Errorf("path '%s' not found in module '%s': node '%s' not found at path '%s' (available children: %v)",
					normalizedPath, moduleName, part, strings.Join(pathSoFar[:i], "/"), availableChildren)
			}
		}

		// If we got here, the path was traversed but entry is nil
		if len(availableChildren) == 0 && schema.Dir != nil {
			for name := range schema.Dir {
				availableChildren = append(availableChildren, name)
			}
		}
		return nil, fmt.Errorf("path '%s' not found in module '%s' (root children: %v)",
			normalizedPath, moduleName, availableChildren)
	}

	return entry, nil
}

// normalizePath normalizes YANG paths by converting dots to slashes, removing wildcards, and stripping prefixes
func (ms *ModuleSet) normalizePath(path, moduleName string) string {
	// Trim whitespace first
	path = strings.TrimSpace(path)

	// Remove wildcard suffix (*)
	path = strings.TrimSuffix(path, "*")

	// Convert dots to slashes
	path = strings.ReplaceAll(path, ".", "/")

	// Remove any prefixes from the path (e.g., "cms:modem-entry" -> "modem-entry")
	// Since we've already resolved the module, any prefixes in the path are redundant
	parts := strings.Split(path, "/")
	normalizedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		// Trim whitespace from each part
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Check if part has a prefix (format: "prefix:name")
		if idx := strings.Index(part, ":"); idx > 0 {
			// Remove the prefix, keep only the name
			part = strings.TrimSpace(part[idx+1:])
		}
		normalizedParts = append(normalizedParts, part)
	}

	return strings.Join(normalizedParts, "/")
}

// getModuleNames returns a list of all module names (helper for error messages)
func (ms *ModuleSet) getModuleNames() []string {
	names := make([]string, 0, len(ms.schemas))
	for name := range ms.schemas {
		names = append(names, name)
	}
	return names
}

// getPrefixes returns a list of all prefixes (helper for error messages)
func (ms *ModuleSet) getPrefixes() []string {
	prefixes := make([]string, 0, len(ms.prefixToModule))
	for prefix := range ms.prefixToModule {
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

// findNodeInSchema recursively finds a node in the schema tree
func (ms *ModuleSet) findNodeInSchema(schema *yang.Entry, path string) *yang.Entry {
	if schema == nil {
		return nil
	}

	// Trim whitespace and split path
	path = strings.TrimSpace(path)
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		return schema
	}

	current := schema
	for _, part := range parts {
		// Trim whitespace from each part
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Check direct children
		found := false
		if current.Dir != nil {
			for name, child := range current.Dir {
				if name == part {
					current = child
					found = true
					break
				}
			}
		}
		if !found {
			// Return nil - error message will be provided in FindSchemaByPath
			return nil
		}
	}

	return current
}
