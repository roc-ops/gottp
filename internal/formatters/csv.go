package formatters

import (
	"encoding/csv"
	"fmt"
	"strings"
)

// CSVFormatter formats results as CSV
type CSVFormatter struct{}

// NewCSVFormatter creates a new CSV formatter
func NewCSVFormatter() *CSVFormatter {
	return &CSVFormatter{}
}

// CSVOptions contains options for CSV formatting
type CSVOptions struct {
	Sep     string   // Separator character (default: ",")
	Quote   string   // Quote character (default: "\"")
	Path    string   // Dot-separated path to data
	Headers []string // Custom headers
	Missing string   // Value for missing cells
	Key     string   // Key name to transform dictionary to list
}

// Format formats data as CSV
func (f *CSVFormatter) Format(data interface{}, options *CSVOptions) ([]byte, error) {
	str, err := f.FormatString(data, options)
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}

// FormatString formats data as CSV string
func (f *CSVFormatter) FormatString(data interface{}, options *CSVOptions) (string, error) {
	// Check for unsupported types
	switch data.(type) {
	case chan int, chan string, chan interface{}:
		return "", fmt.Errorf("CSV formatter does not support channel types")
	case func():
		return "", fmt.Errorf("CSV formatter does not support function types")
	}

	if options == nil {
		options = &CSVOptions{
			Sep:     ",",
			Quote:   "\"",
			Missing: "",
		}
	}

	// Set defaults
	if options.Sep == "" {
		options.Sep = ","
	}
	if options.Quote == "" {
		options.Quote = "\""
	}

	// Use table formatter to convert data to table format
	tableFormatter := NewTableFormatter()
	tableOptions := &TableOptions{
		Path:    options.Path,
		Headers: options.Headers,
		Missing: options.Missing,
		Key:     options.Key,
	}

	table, err := tableFormatter.Format(data, tableOptions)
	if err != nil {
		return "", fmt.Errorf("failed to format table: %w", err)
	}

	if len(table) == 0 {
		return "", nil
	}

	// Convert table to CSV
	var builder strings.Builder
	writer := csv.NewWriter(&builder)

	// Set custom separator and quote if needed
	if options.Sep != "," || options.Quote != "\"" {
		writer.Comma = rune(options.Sep[0])
		// Note: csv.Writer doesn't support custom quote character directly
		// We'll handle it manually if needed
	}

	// Write all rows
	for _, row := range table {
		// Convert row to []string if needed
		strRow := make([]string, len(row))
		for i, val := range row {
			strRow[i] = fmt.Sprintf("%v", val)
		}
		if err := writer.Write(strRow); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("failed to flush CSV: %w", err)
	}

	result := builder.String()

	// If custom quote is needed and different from default, replace quotes
	if options.Quote != "\"" {
		// Replace double quotes with custom quote
		result = strings.ReplaceAll(result, "\"", options.Quote)
	}

	return result, nil
}
