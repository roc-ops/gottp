package yang

import "fmt"

// ValidationError represents a YANG validation error
type ValidationError struct {
	GroupName string
	Path      string
	Message   string
	Field     string
	Line      int
	Column    int
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("YANG validation error in group '%s' at path '%s', field '%s': %s", e.GroupName, e.Path, e.Field, e.Message)
	}
	return fmt.Sprintf("YANG validation error in group '%s' at path '%s': %s", e.GroupName, e.Path, e.Message)
}

// ValidationResult contains validation results for a group
type ValidationResult struct {
	Valid    bool
	Errors   []*ValidationError
	Warnings []*ValidationError
}

// HasErrors returns true if there are any errors
func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any warnings
func (r *ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

