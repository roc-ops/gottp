package compiled

import (
	"fmt"
	"regexp"
)

// PathResolver resolves dynamic paths by replacing variables with actual values
type PathResolver struct {
	dynPathCache map[string]interface{} // Cache for dynamic path variables
}

// NewPathResolver creates a new path resolver
func NewPathResolver() *PathResolver {
	return &PathResolver{
		dynPathCache: make(map[string]interface{}),
	}
}

// ResolvePath resolves a dynamic path template by replacing {{ variable }} patterns
// with actual values from the match results
func (pr *PathResolver) ResolvePath(pathTemplate string, matchResult map[string]interface{}, vars map[string]interface{}) (string, error) {
	if pathTemplate == "" {
		return "", nil
	}

	result := pathTemplate

	// Find all {{ variable }} patterns
	re := regexp.MustCompile(`\{\{\s*(\S+)\s*\}\}`)
	matches := re.FindAllStringSubmatch(pathTemplate, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		varName := match[1]
		pattern := match[0]

		var value interface{}
		var found bool

		// First, try to get from match result
		if matchResult != nil {
			if v, ok := matchResult[varName]; ok {
				value = v
				found = true
				// Cache it for future use
				pr.dynPathCache[varName] = v
			}
		}

		// If not found, try cache
		if !found {
			if v, ok := pr.dynPathCache[varName]; ok {
				value = v
				found = true
			}
		}

		// If still not found, try variables
		if !found && vars != nil {
			if v, ok := vars[varName]; ok {
				value = v
				found = true
			}
		}

		if !found {
			// If variable not found, leave the template as-is for now
			// This allows partial resolution
			continue
		}

		// Convert value to string
		valueStr := fmt.Sprintf("%v", value)

		// Replace in template - handle special characters
		// Try regex replacement first
		escapedPattern := regexp.QuoteMeta(pattern)
		replacer := regexp.MustCompile(escapedPattern)
		result = replacer.ReplaceAllString(result, valueStr)
	}

	return result, nil
}

// UpdateCache updates the dynamic path cache with new values
func (pr *PathResolver) UpdateCache(vars map[string]interface{}) {
	for k, v := range vars {
		pr.dynPathCache[k] = v
	}
}

// ClearCache clears the dynamic path cache
func (pr *PathResolver) ClearCache() {
	pr.dynPathCache = make(map[string]interface{})
}

// ExtractVariablesFromPath extracts variable names used in a dynamic path template
func (pr *PathResolver) ExtractVariablesFromPath(pathTemplate string) []string {
	if pathTemplate == "" {
		return nil
	}

	var varNames []string
	re := regexp.MustCompile(`\{\{\s*(\S+)\s*\}\}`)
	matches := re.FindAllStringSubmatch(pathTemplate, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			varNames = append(varNames, match[1])
		}
	}

	return varNames
}

