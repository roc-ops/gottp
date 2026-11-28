// Package yangcheck validates a YANG module for the browser editor's YANG
// Modules dialog, so a bad module is rejected when it is loaded instead of
// surfacing later as a failure of the whole parse.
package yangcheck

import (
	"fmt"
	"sort"
	"strings"

	"github.com/openconfig/goyang/pkg/yang"
)

const noSuchModule = "no such module:"

// Check parses content as the YANG module name and resolves it together
// with the already-loaded modules (module name -> content), the same way
// internal/yang's ModuleSet does at parse time: every module is parsed into
// one goyang Modules set and then processed together, so imports resolve
// against the other loaded modules regardless of load order.
//
// It returns nil when the module is usable. Otherwise the error says whether
// the module has a syntax error, imports modules that are not loaded, or has
// another problem (an unknown type, for example). Errors caused only by the
// other loaded modules are not blamed on this one. A loaded entry stored
// under name is treated as the module being replaced.
func Check(name, content string, loaded map[string]string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("YANG module '%s' is empty", name)
	}

	probe := yang.NewModules()
	if err := probe.Parse(content, name); err != nil {
		return fmt.Errorf("syntax error in YANG module '%s': %v", name, err)
	}
	if len(probe.Modules)+len(probe.SubModules) == 0 {
		return fmt.Errorf("YANG module '%s' has no module or submodule statement", name)
	}

	baseline := make(map[string]bool)
	for _, err := range parseOthers(loaded, name).Process() {
		baseline[err.Error()] = true
	}

	ms := parseOthers(loaded, name)
	if err := ms.Parse(content, name); err != nil {
		return fmt.Errorf("YANG module '%s' conflicts with a loaded module: %v", name, err)
	}

	var missing, other []string
	seen := make(map[string]bool)
	for _, err := range ms.Process() {
		msg := err.Error()
		if baseline[msg] || seen[msg] {
			continue
		}
		seen[msg] = true
		if i := strings.Index(msg, noSuchModule); i >= 0 {
			missing = append(missing, strings.TrimSpace(msg[i+len(noSuchModule):]))
			continue
		}
		other = append(other, msg)
	}
	sort.Strings(missing)
	sort.Strings(other)

	switch {
	case len(missing) > 0 && len(other) > 0:
		return fmt.Errorf("YANG module '%s' imports module(s) not loaded: %s (load them first); other errors: %s",
			name, strings.Join(missing, ", "), strings.Join(other, "; "))
	case len(missing) > 0:
		return fmt.Errorf("YANG module '%s' imports module(s) not loaded: %s (load them first)",
			name, strings.Join(missing, ", "))
	case len(other) > 0:
		return fmt.Errorf("YANG module '%s' is invalid: %s", name, strings.Join(other, "; "))
	}
	return nil
}

// parseOthers parses every loaded module except skip into a fresh set.
// Modules that no longer parse are left out: they cannot satisfy an import,
// and their own errors belong to them, not to the module being checked.
func parseOthers(loaded map[string]string, skip string) *yang.Modules {
	names := make([]string, 0, len(loaded))
	for n := range loaded {
		if n != skip {
			names = append(names, n)
		}
	}
	sort.Strings(names)

	ms := yang.NewModules()
	for _, n := range names {
		_ = ms.Parse(loaded[n], n)
	}
	return ms
}
