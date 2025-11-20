package input

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"strings"
)

// DatabaseLoader loads data from SQL databases
// Note: This requires importing the appropriate database driver
// Example: _ "github.com/lib/pq" for PostgreSQL
//          _ "github.com/go-sql-driver/mysql" for MySQL
//          _ "github.com/mattn/go-sqlite3" for SQLite
type DatabaseLoader struct {
	db *sql.DB
}

// NewDatabaseLoader creates a new database loader with a connection
// driverName: database driver name (e.g., "postgres", "mysql", "sqlite3")
// dataSourceName: connection string (e.g., "user=postgres dbname=mydb sslmode=disable")
func NewDatabaseLoader(driverName, dataSourceName string) (*DatabaseLoader, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DatabaseLoader{db: db}, nil
}

// Close closes the database connection
func (l *DatabaseLoader) Close() error {
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

// LoadQuery executes a SQL query and returns results as text
// The results are formatted as CSV-like text for parsing
func (l *DatabaseLoader) LoadQuery(query string) (string, error) {
	rows, err := l.db.Query(query)
	if err != nil {
		return "", fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) == 0 {
		return "", nil
	}

	// Build CSV output
	var result strings.Builder
	writer := csv.NewWriter(&result)

	// Write header
	if err := writer.Write(columns); err != nil {
		return "", fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Read rows
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert values to strings
		row := make([]string, len(columns))
		for i, val := range values {
			if val == nil {
				row[i] = ""
			} else {
				row[i] = fmt.Sprintf("%v", val)
			}
		}

		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating rows: %w", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("failed to flush CSV: %w", err)
	}

	return result.String(), nil
}

// LoadQueryAsJSON executes a SQL query and returns results as JSON string
func (l *DatabaseLoader) LoadQueryAsJSON(query string) (string, error) {
	rows, err := l.db.Query(query)
	if err != nil {
		return "", fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) == 0 {
		return "[]", nil
	}

	// Build JSON output
	var result strings.Builder
	result.WriteString("[")

	// Read rows
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	firstRow := true
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}

		if !firstRow {
			result.WriteString(",")
		}
		firstRow = false

		result.WriteString("{")
		for i, col := range columns {
			if i > 0 {
				result.WriteString(",")
			}
			result.WriteString(fmt.Sprintf(`"%s":`, col))

			val := values[i]
			if val == nil {
				result.WriteString("null")
			} else {
				// Simple JSON encoding (for complex types, use encoding/json)
				valStr := fmt.Sprintf("%v", val)
				// Escape quotes
				valStr = strings.ReplaceAll(valStr, `"`, `\"`)
				// Check if it's a number or boolean
				if isNumeric(valStr) || valStr == "true" || valStr == "false" {
					result.WriteString(valStr)
				} else {
					result.WriteString(fmt.Sprintf(`"%s"`, valStr))
				}
			}
		}
		result.WriteString("}")
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating rows: %w", err)
	}

	result.WriteString("]")
	return result.String(), nil
}

// LoadQueryAsYAML executes a SQL query and returns results as YAML string
func (l *DatabaseLoader) LoadQueryAsYAML(query string) (string, error) {
	rows, err := l.db.Query(query)
	if err != nil {
		return "", fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) == 0 {
		return "[]", nil
	}

	// Build YAML output
	var result strings.Builder

	// Read rows
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	rowIndex := 0
	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return "", fmt.Errorf("failed to scan row: %w", err)
		}

		result.WriteString(fmt.Sprintf("- row_%d:\n", rowIndex))
		for i, col := range columns {
			val := values[i]
			if val == nil {
				result.WriteString(fmt.Sprintf("    %s: null\n", col))
			} else {
				valStr := fmt.Sprintf("%v", val)
				// Escape special YAML characters
				if strings.Contains(valStr, ":") || strings.Contains(valStr, "\n") {
					valStr = fmt.Sprintf(`"%s"`, strings.ReplaceAll(valStr, `"`, `\"`))
				}
				result.WriteString(fmt.Sprintf("    %s: %s\n", col, valStr))
			}
		}
		rowIndex++
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating rows: %w", err)
	}

	return result.String(), nil
}

// isNumeric checks if a string represents a number
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	// Check for integer or float
	hasDot := false
	for i, r := range s {
		if i == 0 && (r == '-' || r == '+') {
			continue
		}
		if r == '.' {
			if hasDot {
				return false
			}
			hasDot = true
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// LoadDatabase is a convenience function that loads data from a database
// It creates a loader, executes the query, and closes the connection
// driverName: database driver name
// dataSourceName: connection string
// query: SQL query to execute
// format: output format ("csv", "json", "yaml")
func LoadDatabase(driverName, dataSourceName, query, format string) (string, error) {
	loader, err := NewDatabaseLoader(driverName, dataSourceName)
	if err != nil {
		return "", err
	}
	defer loader.Close()

	switch strings.ToLower(format) {
	case "json":
		return loader.LoadQueryAsJSON(query)
	case "yaml":
		return loader.LoadQueryAsYAML(query)
	case "csv", "":
		return loader.LoadQuery(query)
	default:
		return "", fmt.Errorf("unsupported format: %s (supported: csv, json, yaml)", format)
	}
}

