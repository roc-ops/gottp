package formatters

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ExcelFormatter formats results as Excel spreadsheet
type ExcelFormatter struct{}

// NewExcelFormatter creates a new Excel formatter
func NewExcelFormatter() *ExcelFormatter {
	return &ExcelFormatter{}
}

// ExcelOptions contains options for Excel formatting
type ExcelOptions struct {
	Table    []ExcelTableConfig // List of table configurations for multiple tabs
	Update   bool               // Whether to update existing file
	Path     string             // Dot-separated path to data (for single table)
	Headers  []string           // Custom headers (for single table)
	Missing  string             // Value for missing cells
	Key      string             // Key name to transform dictionary to list
	TabName  string             // Name of the tab (for single table)
}

// ExcelTableConfig represents configuration for a single Excel table/tab
type ExcelTableConfig struct {
	Path     string   // Dot-separated path to data
	Headers  []string // Custom headers
	Missing  string   // Value for missing cells
	Key      string   // Key name to transform dictionary to list
	TabName  string   // Name of the tab
	Strict   bool     // Strict mode for path traversal
}

// Format formats data as Excel file bytes
func (f *ExcelFormatter) Format(data interface{}, options *ExcelOptions) ([]byte, error) {
	if options == nil {
		options = &ExcelOptions{}
	}

	// Create new Excel file
	file := excelize.NewFile()
	defer func() {
		if err := file.Close(); err != nil {
			// Log error but don't fail
		}
	}()

	// Delete default sheet
	file.DeleteSheet("Sheet1")

	// Determine table configurations
	var tableConfigs []ExcelTableConfig
	if len(options.Table) > 0 {
		// Multiple tables from options
		tableConfigs = options.Table
	} else {
		// Single table from options
		tabName := options.TabName
		if tabName == "" {
			tabName = "Sheet1"
		}
		tableConfigs = []ExcelTableConfig{
			{
				Path:    options.Path,
				Headers: options.Headers,
				Missing: options.Missing,
				Key:     options.Key,
				TabName: tabName,
			},
		}
	}

	// Process each table configuration
	for tabIdx, tableConfig := range tableConfigs {
		tabName := tableConfig.TabName
		if tabName == "" {
			tabName = fmt.Sprintf("Sheet%d", tabIdx+1)
		}

		// Create new sheet
		sheetIndex, err := file.NewSheet(tabName)
		if err != nil {
			return nil, fmt.Errorf("failed to create sheet %s: %w", tabName, err)
		}

		// Use table formatter to get table structure
		tableFormatter := NewTableFormatter()
		tableOptions := &TableOptions{
			Path:    tableConfig.Path,
			Headers: tableConfig.Headers,
			Missing: tableConfig.Missing,
			Key:     tableConfig.Key,
		}

		table, err := tableFormatter.Format(data, tableOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to format table for sheet %s: %w", tabName, err)
		}

		if len(table) == 0 {
			continue
		}

		// Write table to Excel sheet
		for rowIdx, row := range table {
			for colIdx, cell := range row {
				cellName, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
				if err != nil {
					return nil, fmt.Errorf("failed to convert coordinates: %w", err)
				}

				// Set cell value
				if err := file.SetCellValue(tabName, cellName, cell); err != nil {
					return nil, fmt.Errorf("failed to set cell value: %w", err)
				}
			}
		}

		// Format header row (first row) - make it bold
		if len(table) > 0 {
			headerStyle, err := file.NewStyle(&excelize.Style{
				Font: &excelize.Font{
					Bold: true,
				},
			})
			if err == nil {
				// Apply style to header row
				startCell, _ := excelize.CoordinatesToCellName(1, 1)
				endCell, _ := excelize.CoordinatesToCellName(len(table[0]), 1)
				file.SetCellStyle(tabName, startCell, endCell, headerStyle)
			}
		}

		// Set active sheet to first sheet
		if tabIdx == 0 {
			file.SetActiveSheet(sheetIndex)
		}
	}

	// Write to buffer
	buf, err := file.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write Excel file: %w", err)
	}

	return buf.Bytes(), nil
}

// FormatString formats data as Excel file and returns as string (base64 encoded)
// Note: This is less useful for Excel, but provided for interface consistency
func (f *ExcelFormatter) FormatString(data interface{}, options *ExcelOptions) (string, error) {
	bytes, err := f.Format(data, options)
	if err != nil {
		return "", err
	}
	// Return as base64 encoded string
	return fmt.Sprintf("%x", bytes), nil
}

// ParseExcelOptions parses Excel options from a map (e.g., from output tag attributes)
func ParseExcelOptions(attrs map[string]string, content string) (*ExcelOptions, error) {
	options := &ExcelOptions{
		Missing: "",
		Update:  false,
	}

	// Parse update attribute
	if updateStr, ok := attrs["update"]; ok {
		options.Update = strings.ToLower(updateStr) == "true" || updateStr == "1"
	}

	// Parse path attribute
	if path, ok := attrs["path"]; ok {
		options.Path = path
	}

	// Parse headers attribute
	if headersStr, ok := attrs["headers"]; ok {
		headers := strings.Split(headersStr, ",")
		for i := range headers {
			headers[i] = strings.TrimSpace(headers[i])
		}
		options.Headers = headers
	}

	// Parse missing attribute
	if missing, ok := attrs["missing"]; ok {
		options.Missing = missing
	}

	// Parse key attribute
	if key, ok := attrs["key"]; ok {
		options.Key = key
	}

	// Parse tab_name attribute
	if tabName, ok := attrs["tab_name"]; ok {
		options.TabName = tabName
	}

	// Parse table configuration from content (YAML format)
	if content != "" {
		// TODO: Parse YAML content to extract table configurations
		// For now, we'll use the simple options above
		// In a full implementation, we'd parse the YAML content similar to Python TTP
	}

	return options, nil
}

