package lookup

import (
	"database/sql"
	"fmt"
	"strings"
)

// DatabaseLoader loads lookup tables from SQL databases
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

// LoadLookupTable loads a lookup table from a database query
// query: SQL query that returns key-value pairs (should return at least 2 columns)
// keyColumn: name or index of the key column (0-based index or column name)
// valueColumn: name or index of the value column (0-based index or column name)
// Returns a map[string]interface{} suitable for use as a lookup table
func (l *DatabaseLoader) LoadLookupTable(query string, keyColumn, valueColumn interface{}) (map[string]interface{}, error) {
	rows, err := l.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) < 2 {
		return nil, fmt.Errorf("query must return at least 2 columns, got %d", len(columns))
	}

	// Determine key and value column indices
	keyIdx := 0
	valueIdx := 1

	// If keyColumn is a string, find its index
	if keyStr, ok := keyColumn.(string); ok {
		found := false
		for i, col := range columns {
			if strings.EqualFold(col, keyStr) {
				keyIdx = i
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("key column '%s' not found in query results", keyStr)
		}
	} else if keyInt, ok := keyColumn.(int); ok {
		if keyInt < 0 || keyInt >= len(columns) {
			return nil, fmt.Errorf("key column index %d out of range (0-%d)", keyInt, len(columns)-1)
		}
		keyIdx = keyInt
	}

	// If valueColumn is a string, find its index
	if valueStr, ok := valueColumn.(string); ok {
		found := false
		for i, col := range columns {
			if strings.EqualFold(col, valueStr) {
				valueIdx = i
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("value column '%s' not found in query results", valueStr)
		}
	} else if valueInt, ok := valueColumn.(int); ok {
		if valueInt < 0 || valueInt >= len(columns) {
			return nil, fmt.Errorf("value column index %d out of range (0-%d)", valueInt, len(columns)-1)
		}
		valueIdx = valueInt
	}

	// Build lookup table
	lookupTable := make(map[string]interface{})

	// Read rows
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Get key and value
		key := fmt.Sprintf("%v", values[keyIdx])
		value := values[valueIdx]

		// If value is nil, skip this row
		if value == nil {
			continue
		}

		// Store in lookup table
		lookupTable[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return lookupTable, nil
}

// LoadLookupTableWithMultipleValues loads a lookup table where each key can have multiple values
// query: SQL query that returns key-value pairs
// keyColumn: name or index of the key column
// valueColumn: name or index of the value column
// Returns a map[string][]interface{} where each key maps to a list of values
func (l *DatabaseLoader) LoadLookupTableWithMultipleValues(query string, keyColumn, valueColumn interface{}) (map[string][]interface{}, error) {
	rows, err := l.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	if len(columns) < 2 {
		return nil, fmt.Errorf("query must return at least 2 columns, got %d", len(columns))
	}

	// Determine key and value column indices (same logic as LoadLookupTable)
	keyIdx := 0
	valueIdx := 1

	if keyStr, ok := keyColumn.(string); ok {
		for i, col := range columns {
			if strings.EqualFold(col, keyStr) {
				keyIdx = i
				break
			}
		}
	} else if keyInt, ok := keyColumn.(int); ok {
		if keyInt >= 0 && keyInt < len(columns) {
			keyIdx = keyInt
		}
	}

	if valueStr, ok := valueColumn.(string); ok {
		for i, col := range columns {
			if strings.EqualFold(col, valueStr) {
				valueIdx = i
				break
			}
		}
	} else if valueInt, ok := valueColumn.(int); ok {
		if valueInt >= 0 && valueInt < len(columns) {
			valueIdx = valueInt
		}
	}

	// Build lookup table with lists
	lookupTable := make(map[string][]interface{})

	// Read rows
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Get key and value
		key := fmt.Sprintf("%v", values[keyIdx])
		value := values[valueIdx]

		// Append value to list for this key
		if _, exists := lookupTable[key]; !exists {
			lookupTable[key] = []interface{}{}
		}
		lookupTable[key] = append(lookupTable[key], value)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return lookupTable, nil
}

// LoadDatabaseLookup is a convenience function that loads a lookup table from a database
// It creates a loader, executes the query, and closes the connection
// driverName: database driver name
// dataSourceName: connection string
// query: SQL query to execute
// keyColumn: key column name or index
// valueColumn: value column name or index
func LoadDatabaseLookup(driverName, dataSourceName, query string, keyColumn, valueColumn interface{}) (map[string]interface{}, error) {
	loader, err := NewDatabaseLoader(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	defer loader.Close()

	return loader.LoadLookupTable(query, keyColumn, valueColumn)
}

