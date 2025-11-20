package formatters

import (
	"fmt"
	"sort"
)

// TableFormatter formats results as a table (list of lists)
type TableFormatter struct{}

// NewTableFormatter creates a new table formatter
func NewTableFormatter() *TableFormatter {
	return &TableFormatter{}
}

// Format formats data as a table (list of lists)
// First row is headers, subsequent rows are data
func (f *TableFormatter) Format(data interface{}, options *TableOptions) ([][]string, error) {
	if options == nil {
		options = &TableOptions{}
	}

	// Extract data based on path if specified
	extractedData := data
	if options.Path != "" {
		// TODO: Implement path traversal
		// For now, use data as-is
		extractedData = data
	}

	// Convert to list of maps
	var dataList []map[string]interface{}
	switch v := extractedData.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				dataList = append(dataList, m)
			}
		}
	case []map[string]interface{}:
		dataList = v
	case map[string]interface{}:
		// Single map - check if key is specified
		if options.Key != "" {
			// Transform dictionary using key
			if keyData, ok := v[options.Key]; ok {
				if keyList, ok := keyData.([]interface{}); ok {
					for _, item := range keyList {
						if m, ok := item.(map[string]interface{}); ok {
							dataList = append(dataList, m)
						}
					}
				}
			}
		} else {
			// Single map - wrap in list
			dataList = []map[string]interface{}{v}
		}
	}

	if len(dataList) == 0 {
		return [][]string{}, nil
	}

	// Determine headers
	headers := options.Headers
	if len(headers) == 0 {
		// Collect all keys from all maps
		headerSet := make(map[string]bool)
		for _, item := range dataList {
			for k := range item {
				headerSet[k] = true
			}
		}
		// Sort headers
		for k := range headerSet {
			headers = append(headers, k)
		}
		sort.Strings(headers)
	}

	// Build table
	table := [][]string{headers}

	// Add data rows
	for _, item := range dataList {
		row := make([]string, len(headers))
		for i, header := range headers {
			if val, ok := item[header]; ok {
				row[i] = fmt.Sprintf("%v", val)
			} else {
				row[i] = options.Missing
			}
		}
		table = append(table, row)
	}

	return table, nil
}

// TableOptions contains options for table formatting
type TableOptions struct {
	Path    string   // Dot-separated path to data
	Headers []string // Custom headers
	Missing string   // Value for missing cells
	Key     string   // Key name to transform dictionary to list
}

