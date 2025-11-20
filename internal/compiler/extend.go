package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/roc-ops/gottp/internal/parser"
)

// ExtendResolver resolves and processes extend tags
type ExtendResolver struct {
	basePath string
	loader   TemplateLoader
}

// TemplateLoader loads template content from a path
type TemplateLoader interface {
	LoadTemplate(path string) (string, error)
}

// FileTemplateLoader loads templates from the filesystem
type FileTemplateLoader struct {
	basePath string
}

// NewFileTemplateLoader creates a new file-based template loader
func NewFileTemplateLoader(basePath string) *FileTemplateLoader {
	return &FileTemplateLoader{basePath: basePath}
}

// LoadTemplate loads a template from the filesystem
func (l *FileTemplateLoader) LoadTemplate(path string) (string, error) {
	// Handle TTP template repository paths (ttp://...)
	if strings.HasPrefix(path, "ttp://") {
		// For now, treat as relative path after removing prefix
		// In production, this would resolve from TTP_TEMPLATES_DIR
		path = strings.TrimPrefix(path, "ttp://")
	}

	// Resolve path relative to basePath
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		if l.basePath != "" {
			fullPath = filepath.Join(l.basePath, path)
		} else {
			fullPath = path
		}
	}

	// Check TTP_TEMPLATES_DIR environment variable
	if !filepath.IsAbs(path) {
		if ttpDir := os.Getenv("TTP_TEMPLATES_DIR"); ttpDir != "" {
			ttpPath := filepath.Join(ttpDir, path)
			if _, err := os.Stat(ttpPath); err == nil {
				fullPath = ttpPath
			}
		}
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to load template from %s: %w", fullPath, err)
	}

	return string(content), nil
}

// NewExtendResolver creates a new extend resolver
func NewExtendResolver(basePath string, loader TemplateLoader) *ExtendResolver {
	if loader == nil {
		loader = NewFileTemplateLoader(basePath)
	}
	return &ExtendResolver{
		basePath: basePath,
		loader:   loader,
	}
}

// ResolveExtends resolves all extend tags in a template
func (r *ExtendResolver) ResolveExtends(tmpl *parser.Template) (*parser.Template, error) {
	return r.resolveExtendsRecursive(tmpl, true)
}

// resolveExtendsRecursive recursively resolves extend tags
func (r *ExtendResolver) resolveExtendsRecursive(tmpl *parser.Template, top bool) (*parser.Template, error) {
	// Process extends at the top level
	if top {
		newGroups := []*parser.Group{}
		for _, group := range tmpl.Groups {
			resolved, err := r.resolveGroupExtends(group)
			if err != nil {
				return nil, err
			}
			newGroups = append(newGroups, resolved...)
		}

		// Process top-level extends
		for _, ext := range tmpl.Extends {
			extended, err := r.loadAndProcessExtend(ext, top)
			if err != nil {
				return nil, err
			}

			// Merge extended template content
			if extended != nil {
				// Merge groups
				newGroups = append(newGroups, extended.Groups...)
				// Merge inputs
				tmpl.Inputs = append(tmpl.Inputs, extended.Inputs...)
				// Merge outputs
				tmpl.Outputs = append(tmpl.Outputs, extended.Outputs...)
				// Merge lookups
				tmpl.Lookups = append(tmpl.Lookups, extended.Lookups...)
				// Merge vars
				if extended.Vars != nil {
					if tmpl.Vars == nil {
						tmpl.Vars = make(map[string]interface{})
					}
					for k, v := range extended.Vars {
						tmpl.Vars[k] = v
					}
				}
				// Merge macros
				tmpl.Macros = append(tmpl.Macros, extended.Macros...)
			}
		}

		tmpl.Groups = newGroups
	}

	return tmpl, nil
}

// resolveGroupExtends resolves extend tags within a group
// Returns the group with nested groups preserved (not flattened)
func (r *ExtendResolver) resolveGroupExtends(group *parser.Group) ([]*parser.Group, error) {
	// Groups don't have Extends field directly - extends are processed at template level
	// Recursively process nested groups, but keep them nested (don't flatten)
	// Process nested groups and update the parent's Groups field
	resolvedNestedGroups := []*parser.Group{}
	for _, nestedGroup := range group.Groups {
		resolved, err := r.resolveGroupExtends(nestedGroup)
		if err != nil {
			return nil, err
		}
		// Add resolved nested groups to the parent's Groups field (preserve nesting)
		resolvedNestedGroups = append(resolvedNestedGroups, resolved...)
	}
	// Update the group's nested groups
	group.Groups = resolvedNestedGroups
	
	// Return only the parent group (not flattened)
	return []*parser.Group{group}, nil
}

// loadAndProcessExtend loads and processes an extend tag
func (r *ExtendResolver) loadAndProcessExtend(ext *parser.Extend, top bool) (*parser.Template, error) {
	// Load template content
	content, err := r.loader.LoadTemplate(ext.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to load extended template %s: %w", ext.Template, err)
	}

	// Apply macro if specified
	if ext.Macro != "" {
		// TODO: Apply macro transformation
		// For now, we'll need to execute the macro during compilation
		// This requires access to the macro registry
		_ = ext.Macro
	}

	// Parse the extended template
	extended, err := parser.ParseTemplate(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse extended template %s: %w", ext.Template, err)
	}

	// Apply filters
	filtered := r.filterExtendedTemplate(extended, ext, top)

	// Recursively resolve extends in the extended template
	return r.resolveExtendsRecursive(filtered, top)
}

// filterExtendedTemplate filters the extended template based on extend tag attributes
func (r *ExtendResolver) filterExtendedTemplate(tmpl *parser.Template, ext *parser.Extend, top bool) *parser.Template {
	filtered := &parser.Template{
		Name:          tmpl.Name,
		BasePath:      tmpl.BasePath,
		ResultsMethod: tmpl.ResultsMethod,
		PathChar:      tmpl.PathChar,
		Doc:           tmpl.Doc,
		Groups:        []*parser.Group{},
		Inputs:        []*parser.Input{},
		Outputs:       []*parser.Output{},
		Lookups:       []*parser.Lookup{},
		Macros:        []*parser.Macro{},
		Vars:          make(map[string]interface{}),
		Extends:       []*parser.Extend{},
	}

	// When nested (not top), only groups and extends are loaded
	if !top {
		// Filter groups
		if len(ext.Groups) > 0 {
			for i, group := range tmpl.Groups {
				// Check by name or index
				if shouldInclude(group.Name, ext.Groups) || shouldInclude(fmt.Sprintf("%d", i), ext.Groups) {
					filtered.Groups = append(filtered.Groups, group)
				}
			}
		} else {
			// No filter, include all groups
			filtered.Groups = tmpl.Groups
		}
		// Always include extends for nested resolution
		filtered.Extends = tmpl.Extends
		return filtered
	}

	// Top-level filtering
	// Filter groups
	if len(ext.Groups) > 0 {
		for i, group := range tmpl.Groups {
			// Check by name or index
			if shouldInclude(group.Name, ext.Groups) || shouldInclude(fmt.Sprintf("%d", i), ext.Groups) {
				filtered.Groups = append(filtered.Groups, group)
			}
		}
	} else {
		filtered.Groups = tmpl.Groups
	}

	// Filter inputs
	if len(ext.Inputs) > 0 {
		for _, input := range tmpl.Inputs {
			if shouldInclude(input.Name, ext.Inputs) {
				filtered.Inputs = append(filtered.Inputs, input)
			}
		}
	} else {
		filtered.Inputs = tmpl.Inputs
	}

	// Filter outputs
	if len(ext.Outputs) > 0 {
		for _, output := range tmpl.Outputs {
			if shouldInclude(output.Name, ext.Outputs) {
				filtered.Outputs = append(filtered.Outputs, output)
			}
		}
	} else {
		filtered.Outputs = tmpl.Outputs
	}

	// Filter lookups
	if len(ext.Lookups) > 0 {
		for _, lookup := range tmpl.Lookups {
			if shouldInclude(lookup.Name, ext.Lookups) {
				filtered.Lookups = append(filtered.Lookups, lookup)
			}
		}
	} else {
		filtered.Lookups = tmpl.Lookups
	}

	// Filter vars
	if len(ext.Vars) > 0 {
		if tmpl.Vars != nil {
			for k, v := range tmpl.Vars {
				if shouldInclude(k, ext.Vars) {
					if filtered.Vars == nil {
						filtered.Vars = make(map[string]interface{})
					}
					filtered.Vars[k] = v
				}
			}
		}
	} else {
		filtered.Vars = tmpl.Vars
	}

	// Always include extends for recursive resolution
	filtered.Extends = tmpl.Extends

	return filtered
}

// shouldInclude checks if a name should be included based on filter
func shouldInclude(name string, filter []string) bool {
	if filter == nil || len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == name {
			return true
		}
	}
	return false
}

