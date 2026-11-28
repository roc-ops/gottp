package yang

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/openconfig/goyang/pkg/yang"
)

// ModuleSet manages a collection of YANG modules
type ModuleSet struct {
	mu             sync.RWMutex
	modules        map[string]*yang.Module
	schemas        map[string]*yang.Entry
	mset           *yang.Modules         // Shared Modules instance for dependency resolution
	addedModules   map[string]bool       // Track which modules we've added (for error messages)
	prefixToModule map[string]string     // Map prefix to module name (e.g., "cms" -> "example-cable-modem-state")
	patternUses    map[string]patternUse // pattern string -> whether it is used with invert-match
	orphans        map[string][]string   // module not loaded -> loaded submodules that belong to it
	ambiguous      map[string][]string   // prefix several modules claim -> those modules, sorted
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
func (ms *ModuleSet) AddModule(name, content string) (err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to parse YANG module '%s': goyang panicked: %v", name, r)
		}
	}()

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
//
// goyang can panic on valid modules (1.6.3 does building the error for an
// unknown type in a submodule); a panic is returned as an error naming the
// loaded modules.
func (ms *ModuleSet) ProcessAllModules() (err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to process YANG modules %s: goyang panicked: %v",
				strings.Join(ms.sortedAddedModules(), ", "), r)
		}
	}()

	// Process all modules together
	restore := ms.linkSubmoduleScopes()
	errs := ms.mset.Process()
	restore()
	if len(errs) > 0 {
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
			errorMsgs = append(errorMsgs, fmt.Sprintf("currently loaded modules: %v", ms.sortedAddedModules()))
		}

		// Add suggestion for missing modules
		if len(missingModules) > 0 {
			errorMsgs = append(errorMsgs, fmt.Sprintf("please add the following module(s) to resolve dependencies: %v", missingModules))
		}

		return fmt.Errorf("failed to process YANG modules: %s", strings.Join(errorMsgs, "; "))
	}

	ms.patternUses = make(map[string]patternUse)
	collectPatternModifiers(ms.mset.Modules, ms.patternUses)
	collectPatternModifiers(ms.mset.SubModules, ms.patternUses)

	// Update stored modules and schemas after processing
	modulePrefixes := map[string]map[string]bool{} // prefix -> modules whose own prefix it is
	for name, module := range ms.mset.Modules {
		ms.modules[name] = module
		entry := yang.ToEntry(module)
		if entry != nil {
			ms.schemas[name] = entry
		}
		if module.Prefix != nil && module.Prefix.Name != "" {
			addClaim(modulePrefixes, module.Prefix.Name, module.Name)
		}
	}
	ms.ambiguous = make(map[string][]string)
	ms.assignPrefixes(modulePrefixes)

	return ms.registerSubmodules()
}

// addClaim records that module claims prefix.
func addClaim(claims map[string]map[string]bool, prefix, module string) {
	if claims[prefix] == nil {
		claims[prefix] = map[string]bool{}
	}
	claims[prefix][module] = true
}

// assignPrefixes maps each prefix in claims that is not already mapped to
// the one module claiming it. A prefix several modules claim maps to none
// and is recorded as ambiguous, so a path using it is an error rather than
// resolving to whichever module map iteration reached last.
func (ms *ModuleSet) assignPrefixes(claims map[string]map[string]bool) {
	for prefix, mods := range claims {
		if _, taken := ms.prefixToModule[prefix]; taken {
			continue
		}
		if _, taken := ms.ambiguous[prefix]; taken {
			continue
		}
		if len(mods) == 1 {
			for m := range mods {
				ms.prefixToModule[prefix] = m
			}
			continue
		}
		names := make([]string, 0, len(mods))
		for m := range mods {
			names = append(names, m)
		}
		sort.Strings(names)
		ms.ambiguous[prefix] = names
	}
}

// sortedAddedModules returns the names of the modules added, sorted.
func (ms *ModuleSet) sortedAddedModules() []string {
	names := make([]string, 0, len(ms.addedModules))
	for name := range ms.addedModules {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// linkSubmoduleScopes makes goyang resolve typedefs and groupings in a
// submodule as YANG 1.1 does (RFC 7950 5.1): every top-level typedef and
// grouping of the module and of all its submodules is visible in each of
// its submodules, included or not. goyang looks only in the submodule
// itself and the submodules it includes, so a submodule using its module's
// typedef fails (goyang 1.6.3 panics). Before Process, each submodule a
// module includes, directly or through another submodule, gets synthetic
// include statements for the module and for each of the module's other
// submodules. goyang finds typedefs and groupings through them; it does
// not merge them into the schema, because by the time a submodule's entry
// is built the module has already merged it (the module is built first),
// and the submodule-to-module cycle is skipped as a circular include.
//
// The returned function undoes what Process must not see afterwards: the
// module registered as a submodule so goyang's include resolution finds it,
// and the circular-include option.
func (ms *ModuleSet) linkSubmoduleScopes() (restore func()) {
	findSub := func(inc *yang.Include) *yang.Module {
		if inc.RevisionDate != nil {
			if s := ms.mset.SubModules[inc.Name+"@"+inc.RevisionDate.Name]; s != nil {
				return s
			}
		}
		return ms.mset.SubModules[inc.Name]
	}
	var registered []string
	linked := false
	for name, m := range ms.mset.Modules {
		if name != m.Name || len(m.Include) == 0 {
			continue
		}
		// The module's submodules: those it includes, transitively, that
		// belong to it. One included that belongs elsewhere is an error
		// registerSubmodules reports.
		var subs []*yang.Module
		seen := map[*yang.Module]bool{}
		var walk func(incs []*yang.Include)
		walk = func(incs []*yang.Include) {
			for _, inc := range incs {
				s := findSub(inc)
				if s == nil || seen[s] || s.BelongsTo == nil || s.BelongsTo.Name != m.Name {
					continue
				}
				seen[s] = true
				subs = append(subs, s)
				walk(s.Include)
			}
		}
		walk(m.Include)
		if len(subs) == 0 {
			continue
		}
		if _, ok := ms.mset.SubModules[m.Name]; !ok {
			ms.mset.SubModules[m.Name] = m
			registered = append(registered, m.Name)
		}
		for _, s := range subs {
			has := map[string]bool{s.Name: true}
			for _, inc := range s.Include {
				has[inc.Name] = true
			}
			for _, other := range append([]*yang.Module{m}, subs...) {
				if has[other.Name] {
					continue
				}
				has[other.Name] = true
				s.Include = append(s.Include, &yang.Include{Name: other.Name, Source: s.Source, Parent: s})
				linked = true
			}
		}
	}
	ignored := ms.mset.ParseOptions.IgnoreSubmoduleCircularDependencies
	if linked {
		ms.mset.ParseOptions.IgnoreSubmoduleCircularDependencies = true
	}
	return func() {
		for _, name := range registered {
			delete(ms.mset.SubModules, name)
		}
		ms.mset.ParseOptions.IgnoreSubmoduleCircularDependencies = ignored
	}
}

// registerSubmodules checks each loaded submodule against the module it
// belongs to, as yangson and pyang do: a module may include only its own
// submodules, whose content goyang then merges into the module's schema. A
// submodule whose module is not loaded is not an error (yangson ignores a
// submodule no module includes); it is remembered so that a path naming its
// module says why the module is missing. A submodule's belongs-to prefix
// also resolves to its module when no module uses that prefix.
func (ms *ModuleSet) registerSubmodules() error {
	var errs []string
	for name, m := range ms.mset.Modules {
		if name != m.Name {
			continue // the same module under name@revision
		}
		for _, inc := range m.Include {
			sub := inc.Module
			if sub == nil || sub.BelongsTo == nil || sub.BelongsTo.Name == m.Name {
				continue
			}
			errs = append(errs, fmt.Sprintf("module %s includes submodule %s, which belongs to %s (RFC 7950 7.1.6)",
				m.Name, sub.Name, sub.BelongsTo.Name))
		}
	}
	sort.Strings(errs)
	if len(errs) > 0 {
		return fmt.Errorf("failed to process YANG modules: %s", strings.Join(errs, "; "))
	}

	ms.orphans = make(map[string][]string)
	subPrefixes := map[string]map[string]bool{} // belongs-to prefix -> modules it names
	for name, sub := range ms.mset.SubModules {
		if name != sub.Name || sub.BelongsTo == nil {
			continue
		}
		parent := sub.BelongsTo.Name
		if _, ok := ms.mset.Modules[parent]; !ok {
			ms.orphans[parent] = append(ms.orphans[parent], sub.Name)
			continue
		}
		if sub.BelongsTo.Prefix != nil && sub.BelongsTo.Prefix.Name != "" {
			addClaim(subPrefixes, sub.BelongsTo.Prefix.Name, parent)
		}
	}
	ms.assignPrefixes(subPrefixes)
	for _, subs := range ms.orphans {
		sort.Strings(subs)
	}
	return nil
}

// patternModifier reports whether pattern p carries "modifier invert-match"
// in the loaded modules, and whether it is ambiguous because it is used both
// with and without it.
func (ms *ModuleSet) patternModifier(p string) (invert, ambiguous bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	use := ms.patternUses[p]
	return use == patternInverted, use == patternPlain|patternInverted
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
	ms.patternUses = nil
	ms.orphans = nil
	ms.ambiguous = nil
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
			} else if mods := ms.ambiguous[prefixOrModuleName]; len(mods) > 0 {
				return nil, fmt.Errorf("prefix '%s' is ambiguous: modules %s each use it (as their prefix or their submodules' belongs-to prefix); name the module instead",
					prefixOrModuleName, strings.Join(mods, ", "))
			} else {
				// Not found as module name or prefix
				return nil, fmt.Errorf("module or prefix '%s' not found%s (available modules: %v, available prefixes: %v)",
					prefixOrModuleName, ms.orphanHint(prefixOrModuleName), ms.getModuleNames(), ms.getPrefixes())
			}
		}
	} else {
		nodePath = path
		// Try to find in all modules
		var endErr error
		for name, schema := range ms.schemas {
			candidates := []string{nodePath}
			if strings.HasPrefix(nodePath, name+":") {
				candidates = append(candidates, strings.TrimPrefix(nodePath, name+":"))
			}
			for _, p := range candidates {
				entry, err := resolvePath(schema, p)
				if err == nil {
					return entry, nil
				}
				var se *schemaNodeEndError
				if errors.As(err, &se) && endErr == nil {
					endErr = fmt.Errorf("path '%s' in module '%s' %v", path, name, err)
				}
			}
		}
		if endErr != nil {
			return nil, endErr
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

	entry, err := resolvePath(schema, normalizedPath)
	if err != nil {
		var se *schemaNodeEndError
		if errors.As(err, &se) {
			return nil, fmt.Errorf("path '%s' in module '%s' %v", normalizedPath, moduleName, err)
		}
		return nil, fmt.Errorf("path '%s' not found in module '%s': %v", normalizedPath, moduleName, err)
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

// orphanHint explains why module name is missing when loaded submodules
// belong to it: "; submodule S belongs to NAME, which is not loaded".
func (ms *ModuleSet) orphanHint(name string) string {
	subs := ms.orphans[name]
	if len(subs) == 0 {
		return ""
	}
	if len(subs) == 1 {
		return fmt.Sprintf("; submodule %s belongs to %s, which is not loaded", subs[0], name)
	}
	return fmt.Sprintf("; submodules %s belong to %s, which is not loaded", strings.Join(subs, ", "), name)
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

// schemaNodeEndError reports a path that ends on a choice or case node.
type schemaNodeEndError struct {
	entry *yang.Entry
}

func (e *schemaNodeEndError) Error() string {
	kind := "choice"
	if e.entry.IsCase() {
		kind = "case"
	}
	return fmt.Sprintf("names a %s ('%s'), which is not a data node (RFC 7950 7.9); name a node under it instead (available: %v)",
		kind, e.entry.Name, dataNodeNames(e.entry))
}

// resolvePath walks path (slash-separated node names) down from schema.
// At each step a data node of that name is looked up first, descending
// through choice and case nodes, which are not data nodes (RFC 7950 7.9):
// "c/bc" names container bc in "choice ch { case b { container bc } }".
// Failing that, the step may name a choice or case directly under the
// current node, so the schema-node form "c/ch/b/bc" resolves too. A path
// that ends on a choice or case names no data node and is an error.
func resolvePath(schema *yang.Entry, path string) (*yang.Entry, error) {
	if schema == nil {
		return nil, fmt.Errorf("no schema")
	}
	current := schema
	var pathSoFar []string
	for _, part := range strings.Split(strings.Trim(strings.TrimSpace(path), "/"), "/") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		child, ok := lookupChild(current, part)
		if !ok {
			child, ok = current.Dir[part]
			ok = ok && (child.IsChoice() || child.IsCase())
		}
		if !ok {
			return nil, fmt.Errorf("node '%s' not found at path '%s' (available children: %v)",
				part, strings.Join(pathSoFar, "/"), dataNodeNames(current))
		}
		current = child
		pathSoFar = append(pathSoFar, part)
	}
	if current.IsChoice() || current.IsCase() {
		return nil, &schemaNodeEndError{entry: current}
	}
	return current, nil
}

// dataNodeNames returns, sorted, the names of the data nodes that are
// children of entry in the data tree: its own children, and those under
// its choices and cases.
func dataNodeNames(entry *yang.Entry) []string {
	var names []string
	var collect func(e *yang.Entry)
	collect = func(e *yang.Entry) {
		for _, child := range e.Dir {
			if child.IsChoice() || child.IsCase() {
				collect(child)
				continue
			}
			names = append(names, child.Name)
		}
	}
	if entry != nil {
		collect(entry)
	}
	sort.Strings(names)
	return names
}
