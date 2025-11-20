package match

// Record stores a value for later use in path formation
func Record(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Record function is handled specially - it doesn't modify the value
	// but stores it in a registry for path formation
	// For now, just return the value
	return value, nil
}

