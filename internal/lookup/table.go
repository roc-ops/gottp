package lookup

import (
	"fmt"
	"sync"
)

// Table represents a lookup table
type Table struct {
	name   string
	data   map[string]interface{}
	mu     sync.RWMutex
}

// NewTable creates a new lookup table
func NewTable(name string) *Table {
	return &Table{
		name: name,
		data: make(map[string]interface{}),
	}
}

// Set sets a value in the lookup table
func (t *Table) Set(key string, value interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.data[key] = value
}

// Get retrieves a value from the lookup table
func (t *Table) Get(key string) (interface{}, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	value, ok := t.data[key]
	return value, ok
}

// GetWithDefault retrieves a value with a default if not found
func (t *Table) GetWithDefault(key string, defaultValue interface{}) interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if value, ok := t.data[key]; ok {
		return value
	}
	return defaultValue
}

// Load loads data into the lookup table
func (t *Table) Load(data map[string]interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, v := range data {
		t.data[k] = v
	}
}

// Clear clears all data from the lookup table
func (t *Table) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.data = make(map[string]interface{})
}

// Registry manages multiple lookup tables
type Registry struct {
	tables map[string]*Table
	mu     sync.RWMutex
}

// NewRegistry creates a new lookup table registry
func NewRegistry() *Registry {
	return &Registry{
		tables: make(map[string]*Table),
	}
}

// Register registers a lookup table
func (r *Registry) Register(name string, table *Table) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tables[name] = table
}

// Get retrieves a lookup table by name
func (r *Registry) Get(name string) (*Table, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	table, ok := r.tables[name]
	return table, ok
}

// Lookup performs a lookup in the specified table
func (r *Registry) Lookup(tableName, key string) (interface{}, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	table, ok := r.tables[tableName]
	if !ok {
		return nil, false
	}
	
	return table.Get(key)
}

// LookupWithDefault performs a lookup with a default value
func (r *Registry) LookupWithDefault(tableName, key string, defaultValue interface{}) interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	table, ok := r.tables[tableName]
	if !ok {
		return defaultValue
	}
	
	return table.GetWithDefault(key, defaultValue)
}

// AddTable creates and registers a new lookup table
func (r *Registry) AddTable(name string, data map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.tables[name]; exists {
		return fmt.Errorf("lookup table %s already exists", name)
	}
	
	table := NewTable(name)
	table.Load(data)
	r.tables[name] = table
	
	return nil
}

// RemoveTable removes a lookup table
func (r *Registry) RemoveTable(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tables, name)
}

// ListTables returns a list of all registered table names
func (r *Registry) ListTables() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	names := make([]string, 0, len(r.tables))
	for name := range r.tables {
		names = append(names, name)
	}
	return names
}

