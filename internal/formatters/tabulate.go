package formatters

import (
	"fmt"
	"strings"
)

// TabulateFormatter formats results as a text table using tabulate-style formatting
// Similar to Python's tabulate module
type TabulateFormatter struct{}

// NewTabulateFormatter creates a new tabulate formatter
func NewTabulateFormatter() *TabulateFormatter {
	return &TabulateFormatter{}
}

// Format formats data as a text table
func (f *TabulateFormatter) Format(data interface{}, options *TableOptions) ([]byte, error) {
	result, err := f.FormatString(data, options)
	if err != nil {
		return nil, err
	}
	return []byte(result), nil
}

// FormatString formats data as a text table string
func (f *TabulateFormatter) FormatString(data interface{}, options *TableOptions) (string, error) {
	if options == nil {
		options = &TableOptions{}
	}

	// Use table formatter to get table structure
	tableFormatter := &TableFormatter{}
	table, err := tableFormatter.Format(data, options)
	if err != nil {
		return "", fmt.Errorf("failed to create table structure: %w", err)
	}

	if len(table) == 0 {
		return "", nil
	}

	// Calculate column widths
	colWidths := make([]int, len(table[0]))
	for _, row := range table {
		for i, cell := range row {
			cellLen := len(cell)
			if cellLen > colWidths[i] {
				colWidths[i] = cellLen
			}
		}
	}

	// Build table string
	var result strings.Builder
	
	// Format each row
	for rowIdx, row := range table {
		// Format cells with padding
		cells := make([]string, len(row))
		for i, cell := range row {
			cells[i] = f.padCell(cell, colWidths[i])
		}
		
		// Join cells with separator (default: 2 spaces)
		separator := "  "
		rowStr := strings.Join(cells, separator)
		result.WriteString(rowStr)
		
		// Add newline
		result.WriteString("\n")
		
		// Add separator line after header (default: show separator)
		if rowIdx == 0 {
			separatorLine := make([]string, len(colWidths))
			for i, width := range colWidths {
				separatorLine[i] = strings.Repeat("-", width)
			}
			result.WriteString(strings.Join(separatorLine, separator))
			result.WriteString("\n")
		}
	}

	return result.String(), nil
}

// padCell pads a cell to the specified width
func (f *TabulateFormatter) padCell(cell string, width int) string {
	if len(cell) >= width {
		return cell
	}
	// Right-align numbers, left-align strings
	// Simple heuristic: if it looks like a number, right-align
	if f.isNumeric(cell) {
		return fmt.Sprintf("%*s", width, cell)
	}
	return fmt.Sprintf("%-*s", width, cell)
}

// isNumeric checks if a string looks like a number
func (f *TabulateFormatter) isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Check if it's a number (integer or float)
	hasDigit := false
	hasDot := false
	for _, r := range s {
		if r >= '0' && r <= '9' {
			hasDigit = true
		} else if r == '.' {
			if hasDot {
				return false // Multiple dots
			}
			hasDot = true
		} else if r == '-' || r == '+' {
			// Allow sign at start
			continue
		} else {
			return false
		}
	}
	return hasDigit
}

