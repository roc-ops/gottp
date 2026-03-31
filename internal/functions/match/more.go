package match

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// registerMoreFunctions registers additional match functions
func (r *Registry) registerMoreFunctions() {
	r.Register("count", count)
	r.Register("record", record)
	r.Register("set", set)
	r.Register("lookup", lookup)
	r.Register("to", to)
	r.Register("to_list", toList)
	r.Register("to_str", toStr)
	r.Register("to_int", toInt)
	r.Register("to_float", toFloat)
	r.Register("to_ip", toIP)
	r.Register("item", item)
	r.Register("let", let)
	r.Register("void", void)
	r.Register("joinmatches", joinmatches)
	r.Register("sformat", sformat)
	r.Register("resub", resub)
	r.Register("prepend", prepend)
	r.Register("append", appendFunc)
	r.Register("copy", copyFunc)
	r.Register("raise", raise)
	r.Register("default", defaultFunc)
	r.Register("unrange", unrange)
	r.Register("uptimeparse", uptimeparse)
	r.Register("truncate", truncate)
	r.Register("to_cidr", toCIDR)
	r.Register("replaceall", replaceall)
	r.Register("resuball", resuball)
	r.Register("rlookup", rlookup)
	r.Register("to_net", toNet)
	r.Register("print", printFunc)
	r.Register("macro", macroFunc)
	r.Register("gpvlookup", gpvlookup)
	r.Register("dns", dns)
	r.Register("rdns", rdns)
	r.Register("ip_info", ipInfo)
	r.Register("geoip_lookup", geoipLookup)
	r.Register("to_unicode", toUnicode)
	r.Register("chain", chainFunc)
}

// count counts occurrences or returns length
func count(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return len(v), nil
	case []interface{}:
		return len(v), nil
	case map[string]interface{}:
		return len(v), nil
	default:
		return 1, nil
	}
}

// record stores a value (placeholder - actual implementation in group context)
func record(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	source := args[0]
	target := source // Default target is source name

	if len(args) >= 2 {
		target = args[1]
	}

	// Record the value to template variables (kwargs) and global recorded vars
	if kwargs != nil {
		kwargs[target] = value
		// Also store in global recorded vars (for cross-group access, e.g., sformat)
		if recordedVars, ok := kwargs["_recorded_vars"].(map[string]interface{}); ok {
			recordedVars[target] = value
		}
	}

	// Return the original value (don't modify it)
	return value, nil
}

// set sets a value (placeholder - actual implementation in group context)
// set sets a variable value in the match result
// Python TTP behavior: {{ var_name | set('value') }} sets var_name to 'value' in match result
// The value can be a template variable name (resolved from vars) or a literal value
func set(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	// Get the variable name from kwargs (passed from runtime) or from value as fallback
	varName := ""
	if kwargs != nil {
		if name, ok := kwargs["_var_name"].(string); ok {
			varName = name
		}
	}
	if varName == "" {
		// Fallback: use value as variable name (might be empty or matched text)
		varName = fmt.Sprintf("%v", value)
	}

	// Get the value to set (first argument)
	setValue := args[0]

	// Check if setValue is a template variable (in kwargs/vars)
	// If it's a template variable, use its value; otherwise use as literal
	var finalValue interface{} = setValue
	if kwargs != nil {
		// Check if it's a template variable
		if varValue, ok := kwargs[setValue]; ok {
			finalValue = varValue
		} else {
			// Not a template variable - use as literal string (don't convert)
			// Python TTP: set() keeps string literals as strings, doesn't auto-convert to boolean/number
			finalValue = setValue
		}
	}

	// Set the variable in the match result (similar to let)
	if kwargs != nil {
		if matchData, ok := kwargs["_match_data"].(map[string]interface{}); ok {
			matchData[varName] = finalValue
		}
	}

	// Return the original value (don't replace match result)
	// Note: For set(), we return the original value, but the actual value is set via _match_data
	return value, nil
}

// lookup looks up a value in a table
// Example: lookup("table_name", add_field="key") where value="65100" finds row with key="65100"
// If add_field is specified, the lookup result is added to match data instead of replacing the value
func lookup(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	tableName := args[0]

	// Get lookup tables from kwargs (passed from runtime)
	var lookupTables map[string]map[string]interface{}
	if kwargs != nil {
		if tables, ok := kwargs["_lookup_tables"].(map[string]map[string]interface{}); ok {
			lookupTables = tables
		}
	}

	if lookupTables == nil {
		return value, nil // No lookup tables available
	}

	table, exists := lookupTables[tableName]
	if !exists {
		return value, nil // Table not found
	}

	// Convert value to string for lookup
	valueStr := fmt.Sprintf("%v", value)

	// Look up the value in the table
	// Try exact match first
	lookupResult, found := table[valueStr]
	if !found {
		// Try with quotes (for YAML tables with quoted keys like '65100')
		// YAML parser might preserve quotes in keys, so try both
		lookupResult, found = table["'"+valueStr+"'"]
		if !found {
			lookupResult, found = table[`"`+valueStr+`"`]
		}
		// Also try converting value to number and back (YAML might parse '65100' as number)
		// But table keys are strings, so convert int back to string
		if !found {
			// Try as integer key (convert to string)
			if intVal, err := strconv.Atoi(valueStr); err == nil {
				intStr := strconv.Itoa(intVal)
				lookupResult, found = table[intStr]
			}
		}
	}
	if !found {
		return value, nil // No match found, return original value
	}

	// Check if add_field is specified BEFORE processing the result
	// This ensures we handle add_field correctly for both map and non-map results
	var addField string
	var hasAddField bool
		if kwargs != nil {
			// Check if add_field is in kwargs (from keyword arguments)
			if field, ok := kwargs["add_field"].(string); ok && field != "" {
				addField = field
				hasAddField = true
			}
			
			// Also check args for add_field (in case it's passed positionally)
			if !hasAddField && len(args) > 1 {
				for _, arg := range args[1:] {
					if strings.HasPrefix(arg, "add_field=") {
						addField = strings.TrimPrefix(arg, "add_field=")
						addField = strings.Trim(addField, `"'`)
						hasAddField = true
						break
					}
				}
			}
		}

	// Handle lookup result based on type
	if resultMap, ok := lookupResult.(map[string]interface{}); ok {
		// For CSV tables, lookupResult is a map with all columns
		if hasAddField && addField != "" {
			// Add the lookup result map to match data instead of replacing the value
			if matchData, ok := kwargs["_match_data"].(map[string]interface{}); ok {
				// For CSV tables, Python TTP excludes the key column from the result
				// The key column is the one whose value matches the lookup key
				resultCopy := make(map[string]interface{})
				for k, v := range resultMap {
					// Skip the column if its value matches the lookup key (it's the key column)
					if fmt.Sprintf("%v", v) != valueStr {
						resultCopy[k] = v
					}
				}
				matchData[addField] = resultCopy
			}
			// Return original value (don't replace)
			return value, nil
		}
		// No add_field - return the result map (replace value)
		return lookupResult, nil
	}

	// If lookupResult is not a map, handle add_field for simple values
	if hasAddField && addField != "" {
		// Add the lookup result to match data instead of replacing the value
		if matchData, ok := kwargs["_match_data"].(map[string]interface{}); ok {
			matchData[addField] = lookupResult
		}
		// Return original value (don't replace)
		return value, nil
	}

	// No add_field specified - return lookup result (replace value)
	return lookupResult, nil
}

// to converts value to specified type
func to(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	targetType := strings.ToLower(args[0])
	str := fmt.Sprintf("%v", value)

	switch targetType {
	case "int", "integer":
		if i, err := strconv.Atoi(str); err == nil {
			return i, nil
		}
		return value, nil
	case "float":
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			return f, nil
		}
		return value, nil
	case "str", "string":
		return str, nil
	case "bool", "boolean":
		if b, err := strconv.ParseBool(str); err == nil {
			return b, nil
		}
		return value, nil
	default:
		return value, nil
	}
}

// item gets an item from a list or dict
func item(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	indexStr := args[0]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid index: %s", indexStr)
	}

	switch v := value.(type) {
	case []interface{}:
		// Handle negative indices
		if index < 0 {
			index = len(v) + index
		}
		if index >= 0 && index < len(v) {
			return v[index], nil
		}
		// Python TTP behavior: out of range returns last item if positive, first item if negative
		if index >= len(v) {
			return v[len(v)-1], nil
		}
		return v[0], nil
	case map[string]interface{}:
		if item, ok := v[indexStr]; ok {
			return item, nil
		}
		return nil, fmt.Errorf("key not found: %s", indexStr)
	case string:
		// Python TTP treats strings as iterables (character sequences)
		// Handle negative indices
		if index < 0 {
			index = len(v) + index
		}
		if index >= 0 && index < len(v) {
			return string(v[index]), nil
		}
		// Python TTP behavior: out of range returns last character if positive, first character if negative
		if index >= len(v) {
			return string(v[len(v)-1]), nil
		}
		return string(v[0]), nil
	default:
		return value, nil
	}
}

// let statically assigns a value to a variable
// Python TTP behavior:
// - let(value): single argument - replaces match result with that value
// - let(var_name, value): two arguments - sets a new field var_name to value in match result
func let(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	// If two arguments: set a new field in the match result
	if len(args) >= 2 {
		// Get match result data structure from kwargs
		if kwargs != nil {
			if matchData, ok := kwargs["_match_data"].(map[string]interface{}); ok {
				// Set the new field
				fieldName := args[0]
				fieldValue := args[1]

				// Convert fieldValue to appropriate type
				// Try to parse as boolean
				if fieldValue == "True" || fieldValue == "true" {
					matchData[fieldName] = true
				} else if fieldValue == "False" || fieldValue == "false" {
					matchData[fieldName] = false
				} else {
					// Try to parse as number
					if intVal, err := strconv.Atoi(fieldValue); err == nil {
						matchData[fieldName] = intVal
					} else if floatVal, err := strconv.ParseFloat(fieldValue, 64); err == nil {
						matchData[fieldName] = floatVal
					} else {
						// Use as string
						matchData[fieldName] = fieldValue
					}
				}
			}
		}
		// Return original value (don't replace match result)
		return value, nil
	}

	// Single argument: replace match result with that value
	return args[0], nil
}

// void returns None/nil (used to skip saving)
func void(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	return nil, nil
}

// toList converts a value to a list
func toList(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// If already a list, return as-is
	if list, ok := value.([]interface{}); ok {
		return list, nil
	}
	// Wrap in a list
	return []interface{}{value}, nil
}

// joinmatches collects multiple matches and joins them
// This is a special function that works with the match context
// For now, it just returns the value as-is (the actual joining happens in the runtime)
func joinmatches(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// joinmatches is handled specially in the runtime
	// It collects values from multiple matches of the same pattern
	// For now, return value as-is - the runtime will handle collection
	return value, nil
}

// re applies regex substitution
func re(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	if len(args) < 2 {
		return str, nil
	}

	pattern := args[0]
	replacement := args[1]

	re, err := regexp.Compile(pattern)
	if err != nil {
		return str, fmt.Errorf("invalid regex pattern: %w", err)
	}

	return re.ReplaceAllString(str, replacement), nil
}

// toStr converts value to string
func toStr(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	return fmt.Sprintf("%v", value), nil
}

// toInt converts value to integer
func toInt(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	i, err := strconv.ParseInt(strings.TrimSpace(str), 10, 64)
	if err != nil {
		// Return original value if conversion fails (like Python TTP)
		return value, nil
	}
	// Return int64 to guarantee 64-bit values (Counter64, HC counters)
	// are preserved regardless of platform word size.
	return i, nil
}

// toFloat converts value to float
func toFloat(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Handle numeric types directly for better performance and accuracy
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	}

	// For strings and other types, parse as string
	str := fmt.Sprintf("%v", value)
	f, err := strconv.ParseFloat(strings.TrimSpace(str), 64)
	if err != nil {
		// Return original value if conversion fails (like Python TTP)
		return value, nil
	}
	return f, nil
}

// toIP converts value to IP address (placeholder - returns string for now)
// Cached compiled regex for toIP function
var toIPRegex = regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}(?:/[0-9]{1,2})?$`)

func toIP(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))
	// Use cached compiled regex for better performance
	if toIPRegex.MatchString(str) {
		return str, nil
	}
	return str, nil
}

// sformat formats string using format string
func sformat(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}
	formatStr := args[0]
	// Replace {} with the value
	result := strings.Replace(formatStr, "{}", fmt.Sprintf("%v", value), -1)
	return result, nil
}

// resub performs regex substitution
// resub performs regex substitution (like Python re.sub)
// By default, replaces only the FIRST occurrence (matching TTP's behavior)
// If count > 1, replaces first N occurrences
// If count = 0 or -1, replaces ALL occurrences
func resub(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)
	if len(args) < 2 {
		return str, nil
	}

	pattern := args[0]
	replacement := args[1]
	count := 1 // Default: replace only first occurrence (TTP behavior)
	if len(args) > 2 {
		if c, err := strconv.Atoi(args[2]); err == nil {
			count = c
		}
	}

	// Convert Python-style regex patterns to Go RE2 patterns
	// Go's regexp (RE2) doesn't support \d, \w, \s, etc. - need to convert
	pattern = convertPythonRegexToRE2(pattern)

	re, err := regexp.Compile(pattern)
	if err != nil {
		// Invalid regex - return original value (handle gracefully)
		return str, nil
	}

	// If count is 0 or negative, replace all (Python re.sub behavior)
	if count <= 0 {
		return re.ReplaceAllString(str, replacement), nil
	}

	// If count is 1 (default), replace only the first occurrence
	// Python's re.sub with count=1 replaces only the first occurrence
	if count == 1 {
		// Find first match and replace it
		loc := re.FindStringIndex(str)
		if loc == nil {
			return str, nil
		}
		// Replace the first match
		return str[:loc[0]] + replacement + str[loc[1]:], nil
	}

	// Replace first N occurrences
	// Find all matches
	matches := re.FindAllStringIndex(str, -1)
	if len(matches) == 0 {
		return str, nil
	}

	// Limit to count matches
	if len(matches) > count {
		matches = matches[:count]
	}

	// Replace matches from end to start to preserve indices
	result := str
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		start, end := match[0], match[1]
		matched := result[start:end]
		replaced := re.ReplaceAllString(matched, replacement)

		// Replace in result string
		result = result[:start] + replaced + result[end:]
	}

	return result, nil
}

// prepend prepends a string to the value
func prepend(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}
	prefix := args[0]

	switch v := value.(type) {
	case string:
		return prefix + v, nil
	case []interface{}:
		// Prepend to list
		result := make([]interface{}, len(v)+1)
		result[0] = prefix
		copy(result[1:], v)
		return result, nil
	default:
		return prefix + fmt.Sprintf("%v", value), nil
	}
}

// appendFunc appends a string to the value
func appendFunc(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}
	suffix := args[0]

	switch v := value.(type) {
	case string:
		return v + suffix, nil
	case []interface{}:
		// Append to list using built-in append
		result := make([]interface{}, len(v)+1)
		copy(result, v)
		result[len(v)] = suffix
		return result, nil
	default:
		return fmt.Sprintf("%v", value) + suffix, nil
	}
}

// copyFunc copies value to another variable (placeholder - actual implementation in group context)
func copyFunc(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Copy function is typically handled in group context
	// For now, just return the value
	return value, nil
}

// raise raises an error (for testing/debugging)
func raise(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	message := "RuntimeError"
	if len(args) > 0 {
		message = args[0]
	}
	return nil, fmt.Errorf("%s", message)
}

// defaultFunc returns default value if current value is empty
func defaultFunc(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	// Check if value is empty
	str := fmt.Sprintf("%v", value)
	if strings.TrimSpace(str) == "" || str == "<nil>" || value == nil {
		return args[0], nil
	}
	return value, nil
}

// unrange expands integer ranges in a string
// Example: "8,10-13,20" with rangechar='-' and joinchar=',' becomes "8,10,11,12,13,20"
func unrange(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)

	// Get rangechar and joinchar from args or kwargs
	rangechar := "-"
	joinchar := ","

	if len(args) >= 1 {
		rangechar = args[0]
	}
	if len(args) >= 2 {
		joinchar = args[1]
	}

	// Check kwargs for rangechar and joinchar
	if kwargs != nil {
		if rc, ok := kwargs["rangechar"].(string); ok {
			rangechar = rc
		} else if rc, ok := kwargs["rangechar"]; ok {
			// Try to convert to string
			rangechar = fmt.Sprintf("%v", rc)
		}
		if jc, ok := kwargs["joinchar"].(string); ok {
			joinchar = jc
		} else if jc, ok := kwargs["joinchar"]; ok {
			// Try to convert to string
			joinchar = fmt.Sprintf("%v", jc)
		}
	}

	// If rangechar not in data, return as-is
	if !strings.Contains(str, rangechar) {
		return str, nil
	}

	var result []string

	// Split by rangechar
	parts := strings.Split(str, rangechar)
	for i, part := range parts {
		// Split by joinchar and filter empty strings
		var itemParts []string
		for _, item := range strings.Split(part, joinchar) {
			item = strings.TrimSpace(item)
			if item != "" {
				itemParts = append(itemParts, item)
			}
		}

		if i == 0 {
			// First part - just add all items
			result = itemParts
		} else {
			// Subsequent parts - expand range
			if len(result) == 0 {
				result = itemParts
				continue
			}

			// Get last number from result
			lastStr := result[len(result)-1]
			startInt, err := strconv.Atoi(lastStr)
			if err != nil {
				// Not a number, can't expand range
				result = append(result, itemParts...)
				continue
			}

			// Get first number from current part
			if len(itemParts) == 0 {
				continue
			}
			endInt, err := strconv.Atoi(itemParts[0])
			if err != nil {
				// Not a number, can't expand range
				result = append(result, itemParts...)
				continue
			}

			// Generate range (inclusive of both start and end)
			// For "166-173", we want 166,167,168,169,170,171,172,173
			// startInt is 166 (last number from previous part), endInt is 173
			// We need to generate [startInt+1, endInt] inclusive
			for num := startInt + 1; num <= endInt; num++ {
				result = append(result, strconv.Itoa(num))
			}

			// Add remaining items from current part (skip the first one as it's already included in the range)
			if len(itemParts) > 1 {
				result = append(result, itemParts[1:]...)
			}
		}
	}

	return strings.Join(result, joinchar), nil
}

// uptimeparse parses uptime strings and converts them to seconds or a dictionary
// Example: "27 weeks, 3 days, 10 hours, 46 minutes" -> seconds or dict
func uptimeparse(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))

	// Get format from args or kwargs (default: "seconds")
	format := "seconds"
	if len(args) >= 1 {
		format = args[0]
	}
	if kwargs != nil {
		if f, ok := kwargs["format"].(string); ok {
			format = f
		}
	}

	// Regex patterns for time units
	yearsRe := regexp.MustCompile(`(?i)(\d+)\s*(?:ys?|yrs?.?|years?)`)
	monthsRe := regexp.MustCompile(`(?i)(\d+)\s*(?:mos?.?|mths?.?|months?)`)
	weeksRe := regexp.MustCompile(`(?i)([\d.]+)\s*(?:w|wks?|weeks?)`)
	daysRe := regexp.MustCompile(`(?i)([\d.]+)\s*(?:d|dys?|days?)`)
	hoursRe := regexp.MustCompile(`(?i)([\d.]+)\s*(?:h|hrs?|hours?)`)
	minsRe := regexp.MustCompile(`(?i)([\d.]+)\s*(?:m|mins?|minutes?)`)
	secsRe := regexp.MustCompile(`(?i)([\d.]+)\s*(?:s|secs?|seconds?)`)

	// Extract time components
	years := 0
	months := 0
	weeks := 0.0
	days := 0.0
	hours := 0.0
	mins := 0.0
	secs := 0.0

	if match := yearsRe.FindStringSubmatch(str); match != nil {
		if y, err := strconv.Atoi(match[1]); err == nil {
			years = y
		}
	}
	if match := monthsRe.FindStringSubmatch(str); match != nil {
		if m, err := strconv.Atoi(match[1]); err == nil {
			months = m
		}
	}
	if match := weeksRe.FindStringSubmatch(str); match != nil {
		if w, err := strconv.ParseFloat(match[1], 64); err == nil {
			weeks = w
		}
	}
	if match := daysRe.FindStringSubmatch(str); match != nil {
		if d, err := strconv.ParseFloat(match[1], 64); err == nil {
			days = d
		}
	}
	if match := hoursRe.FindStringSubmatch(str); match != nil {
		if h, err := strconv.ParseFloat(match[1], 64); err == nil {
			hours = h
		}
	}
	if match := minsRe.FindStringSubmatch(str); match != nil {
		if m, err := strconv.ParseFloat(match[1], 64); err == nil {
			mins = m
		}
	}
	if match := secsRe.FindStringSubmatch(str); match != nil {
		if s, err := strconv.ParseFloat(match[1], 64); err == nil {
			secs = s
		}
	}

	// Check if we found any time components
	if years == 0 && months == 0 && weeks == 0 && days == 0 && hours == 0 && mins == 0 && secs == 0 {
		// No time components found, return original value
		return str, nil
	}

	// Convert to seconds
	// Multipliers (approximate)
	secondsPerYear := 60 * 60 * 24 * 365
	secondsPerMonth := 60 * 60 * 24 * 30
	secondsPerWeek := 60 * 60 * 24 * 7
	secondsPerDay := 60 * 60 * 24
	secondsPerHour := 60 * 60
	secondsPerMinute := 60

	totalSeconds := float64(years*secondsPerYear) +
		float64(months*secondsPerMonth) +
		weeks*float64(secondsPerWeek) +
		days*float64(secondsPerDay) +
		hours*float64(secondsPerHour) +
		mins*float64(secondsPerMinute) +
		secs

	if format == "dict" {
		// Return as dictionary
		result := make(map[string]interface{})
		if years > 0 {
			result["years"] = strconv.Itoa(years)
		}
		if months > 0 {
			result["months"] = strconv.Itoa(months)
		}
		if weeks > 0 {
			result["weeks"] = fmt.Sprintf("%.0f", weeks)
		}
		if days > 0 {
			result["days"] = fmt.Sprintf("%.0f", days)
		}
		if hours > 0 {
			result["hours"] = fmt.Sprintf("%.0f", hours)
		}
		if mins > 0 {
			result["mins"] = fmt.Sprintf("%.0f", mins)
		}
		if secs > 0 {
			result["secs"] = fmt.Sprintf("%.0f", secs)
		}
		return result, nil
	}

	// Return as seconds (integer)
	return int(totalSeconds), nil
}

// truncate truncates a string to the specified length
func truncate(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)

	// Default length is 0 (no truncation)
	length := 0
	if len(args) >= 1 {
		if l, err := strconv.Atoi(args[0]); err == nil {
			length = l
		}
	}

	// Check kwargs for length
	if kwargs != nil {
		if l, ok := kwargs["length"].(int); ok {
			length = l
		} else if l, ok := kwargs["length"].(string); ok {
			if parsed, err := strconv.Atoi(l); err == nil {
				length = parsed
			}
		}
	}

	// If length is 0 or negative, return as-is
	if length <= 0 {
		return str, nil
	}

	// Truncate if longer than length
	if len(str) > length {
		return str[:length], nil
	}

	return str, nil
}

// toCIDR converts a netmask to CIDR notation
// Example: "255.255.255.0" -> "24"
func toCIDR(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))

	// Check if already in CIDR format (contains /)
	if strings.Contains(str, "/") {
		// Extract the CIDR part
		parts := strings.Split(str, "/")
		if len(parts) == 2 {
			// Convert to integer to match Python TTP behavior
			if cidrInt, err := strconv.Atoi(parts[1]); err == nil {
				return cidrInt, nil
			}
			return parts[1], nil
		}
		return str, nil
	}

	// Parse netmask (e.g., "255.255.255.0")
	parts := strings.Split(str, ".")
	if len(parts) != 4 {
		return str, nil // Not a valid netmask format
	}

	// Convert each octet to integer and count bits
	cidr := 0
	for _, part := range parts {
		octet, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || octet < 0 || octet > 255 {
			return str, nil // Invalid octet
		}

		// Count set bits in octet
		for octet > 0 {
			if octet&1 == 1 {
				cidr++
			}
			octet >>= 1
		}
	}

	// Return as integer to match Python TTP behavior
	return cidr, nil
}

// replaceall performs multiple string replacements
// Python TTP syntax: replaceall("new", "old1", "old2", ...)
// If only one argument: replaceall("old") - replaces "old" with empty string
// If multiple arguments: first is the replacement, rest are old values to replace
// Case 2: If value found in variables and variable value is a list, iterate over list
//
//	and for each item run replace where *new* set to first value and *old* equal to each list item
//
// Case 3: If value found in variables and variable value is a dict, iterate over dict items
//
//	and set *new* to item key and *old* to item value
func replaceall(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)

	if len(args) == 0 {
		return str, nil
	}

	result := str
	var newStr string
	var oldValues []string

	if len(args) == 1 {
		// Only one argument: replace it with empty string
		// But check if it's a variable name first
		arg := args[0]
		if kwargs != nil {
			var varValue interface{}
			var found bool

			// First, try to find the variable by name
			if val, ok := kwargs[arg]; ok {
				varValue = val
				found = true
			} else {
				// Arg might be a string representation of a resolved variable
				// Try to find a variable whose string representation matches
				for varName, val := range kwargs {
					// Skip special kwargs
					if varName == "_recorded_vars" || varName == "_match_data" || varName == "_var_name" {
						continue
					}
					if fmt.Sprintf("%v", val) == arg {
						varValue = val
						found = true
						arg = varName
						break
					}
				}
			}

			if found {
				// Variable found - check if it's a list or dict
				if listValue, ok := varValue.([]interface{}); ok {
					// Case 2: List variable - replace each item with empty string
					newStr = ""
					oldValues = make([]string, len(listValue))
					for i, item := range listValue {
						oldValues[i] = fmt.Sprintf("%v", item)
					}
				} else if dictValue, ok := varValue.(map[string]interface{}); ok {
					// Case 3: Dict variable - iterate over dict items
					// For each item, set new to key and old to value
					// If value is a list, iterate over list items
					for newKey, oldVal := range dictValue {
						if oldList, ok := oldVal.([]interface{}); ok {
							// Value is a list - replace each item with key
							for _, item := range oldList {
								oldStr := fmt.Sprintf("%v", item)
								result = strings.ReplaceAll(result, oldStr, newKey)
							}
						} else {
							// Value is a string - replace with key
							oldStr := fmt.Sprintf("%v", oldVal)
							result = strings.ReplaceAll(result, oldStr, newKey)
						}
					}
					return result, nil
				} else {
					// Variable is not a list or dict - use as literal
					newStr = ""
					oldValues = []string{fmt.Sprintf("%v", varValue)}
				}
			} else {
				// Not a variable - use as literal
				newStr = ""
				oldValues = []string{arg}
			}
		} else {
			// No kwargs - use as literal
			newStr = ""
			oldValues = []string{arg}
		}
	} else {
		// Multiple arguments: first is replacement, rest are old values
		// Check if first argument is a variable
		firstArg := args[0]
		if kwargs != nil {
			if varValue, ok := kwargs[firstArg]; ok {
				newStr = fmt.Sprintf("%v", varValue)
			} else {
				newStr = firstArg
			}
		} else {
			newStr = firstArg
		}

		// Process remaining arguments - check if any are variables
		oldValues = make([]string, 0, len(args)-1)
		for _, arg := range args[1:] {
			if kwargs != nil {
				// Check if arg is a variable name in kwargs
				// Note: parseArguments may have resolved the variable to its string representation
				// So we need to check if the arg matches a variable name first
				var varValue interface{}
				var found bool

				// First, try to find the variable by name
				if val, ok := kwargs[arg]; ok {
					varValue = val
					found = true
				} else {
					// Arg might be a string representation of a resolved variable
					// Try to find a variable whose string representation matches
					// This handles the case where parseArguments resolved the variable
					// during argument parsing (it converts lists/dicts to strings)
					for varName, val := range kwargs {
						// Skip special kwargs
						if varName == "_recorded_vars" || varName == "_match_data" || varName == "_var_name" {
							continue
						}
						if fmt.Sprintf("%v", val) == arg {
							varValue = val
							found = true
							arg = varName // Use the variable name for consistency
							break
						}
					}
				}

				if found {
					// Variable found - check if it's a list
					if listValue, ok := varValue.([]interface{}); ok {
						// Case 2: List variable - expand to individual items
						for _, item := range listValue {
							oldValues = append(oldValues, fmt.Sprintf("%v", item))
						}
					} else if dictValue, ok := varValue.(map[string]interface{}); ok {
						// Case 3: Dict variable - iterate over dict items
						// For each item, set new to key and old to value
						// If value is a list, iterate over list items
						for newKey, oldVal := range dictValue {
							if oldList, ok := oldVal.([]interface{}); ok {
								// Value is a list - replace each item with key
								for _, item := range oldList {
									oldStr := fmt.Sprintf("%v", item)
									result = strings.ReplaceAll(result, oldStr, newKey)
								}
							} else {
								// Value is a string - replace with key
								oldStr := fmt.Sprintf("%v", oldVal)
								result = strings.ReplaceAll(result, oldStr, newKey)
							}
						}
						return result, nil
					} else {
						// Variable is not a list or dict - use as literal
						oldValues = append(oldValues, fmt.Sprintf("%v", varValue))
					}
				} else {
					// Not a variable - use as literal
					oldValues = append(oldValues, arg)
				}
			} else {
				// No kwargs - use as literal
				oldValues = append(oldValues, arg)
			}
		}
	}

	// Replace all old values with new value
	for _, oldStr := range oldValues {
		result = strings.ReplaceAll(result, oldStr, newStr)
	}

	return result, nil
}

// resuball performs multiple regex substitutions
// Python TTP syntax: resuball("replacement", "pattern1", "pattern2", ...)
// If only one argument: resuball("pattern") - replaces pattern with empty string
// If multiple arguments: first is the replacement, rest are patterns to replace
// Case 2: If value found in variables and variable value is a list, iterate over list
//
//	and for each item run replace where *new* set to first value and *old* equal to each list item
//
// Case 3: If value found in variables and variable value is a dict, iterate over dict items
//
//	and set *new* to item key and *old* to item value
func resuball(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", value)

	if len(args) == 0 {
		return str, nil
	}

	result := str
	var replacement string
	var patterns []string

	if len(args) == 1 {
		// Only one argument: replace pattern with empty string
		// But check if it's a variable name first
		arg := args[0]
		if kwargs != nil {
			var varValue interface{}
			var found bool

			// First, try to find the variable by name
			if val, ok := kwargs[arg]; ok {
				varValue = val
				found = true
			} else {
				// Arg might be a string representation of a resolved variable
				// Try to find a variable whose string representation matches
				for varName, val := range kwargs {
					// Skip special kwargs
					if varName == "_recorded_vars" || varName == "_match_data" || varName == "_var_name" {
						continue
					}
					if fmt.Sprintf("%v", val) == arg {
						varValue = val
						found = true
						arg = varName
						break
					}
				}
			}

			if found {
				// Variable found - check if it's a list or dict
				if listValue, ok := varValue.([]interface{}); ok {
					// Case 2: List variable - replace each item with empty string
					replacement = ""
					patterns = make([]string, len(listValue))
					for i, item := range listValue {
						patterns[i] = fmt.Sprintf("%v", item)
					}
				} else if dictValue, ok := varValue.(map[string]interface{}); ok {
					// Case 3: Dict variable - iterate over dict items
					// For each item, set new to key and old to value
					// If value is a list, iterate over list items
					for newKey, oldVal := range dictValue {
						if oldList, ok := oldVal.([]interface{}); ok {
							// Value is a list - replace each item with key
							for _, item := range oldList {
								pattern := fmt.Sprintf("%v", item)
								convertedPattern := convertPythonRegexToRE2(pattern)
								re, err := regexp.Compile(convertedPattern)
								if err != nil {
									continue
								}
								result = re.ReplaceAllString(result, newKey)
							}
						} else {
							// Value is a string - replace with key
							pattern := fmt.Sprintf("%v", oldVal)
							convertedPattern := convertPythonRegexToRE2(pattern)
							re, err := regexp.Compile(convertedPattern)
							if err != nil {
								continue
							}
							result = re.ReplaceAllString(result, newKey)
						}
					}
					return result, nil
				} else {
					// Variable is not a list or dict - use as literal
					replacement = ""
					patterns = []string{fmt.Sprintf("%v", varValue)}
				}
			} else {
				// Not a variable - use as literal
				replacement = ""
				patterns = []string{arg}
			}
		} else {
			// No kwargs - use as literal
			replacement = ""
			patterns = []string{arg}
		}
	} else {
		// Multiple arguments: first is replacement, rest are patterns
		// Check if first argument is a variable
		firstArg := args[0]
		if kwargs != nil {
			if varValue, ok := kwargs[firstArg]; ok {
				replacement = fmt.Sprintf("%v", varValue)
			} else {
				replacement = firstArg
			}
		} else {
			replacement = firstArg
		}

		// Process remaining arguments - check if any are variables
		patterns = make([]string, 0, len(args)-1)
		for _, arg := range args[1:] {
			if kwargs != nil {
				var varValue interface{}
				var found bool

				// First, try to find the variable by name
				if val, ok := kwargs[arg]; ok {
					varValue = val
					found = true
				} else {
					// Arg might be a string representation of a resolved variable
					for varName, val := range kwargs {
						// Skip special kwargs
						if varName == "_recorded_vars" || varName == "_match_data" || varName == "_var_name" {
							continue
						}
						if fmt.Sprintf("%v", val) == arg {
							varValue = val
							found = true
							arg = varName
							break
						}
					}
				}

				if found {
					// Variable found - check if it's a list
					if listValue, ok := varValue.([]interface{}); ok {
						// Case 2: List variable - expand to individual items
						for _, item := range listValue {
							patterns = append(patterns, fmt.Sprintf("%v", item))
						}
					} else if dictValue, ok := varValue.(map[string]interface{}); ok {
						// Case 3: Dict variable - iterate over dict items
						for newKey, oldVal := range dictValue {
							if oldList, ok := oldVal.([]interface{}); ok {
								// Value is a list - replace each item with key
								for _, item := range oldList {
									pattern := fmt.Sprintf("%v", item)
									convertedPattern := convertPythonRegexToRE2(pattern)
									re, err := regexp.Compile(convertedPattern)
									if err != nil {
										continue
									}
									result = re.ReplaceAllString(result, newKey)
								}
							} else {
								// Value is a string - replace with key
								pattern := fmt.Sprintf("%v", oldVal)
								convertedPattern := convertPythonRegexToRE2(pattern)
								re, err := regexp.Compile(convertedPattern)
								if err != nil {
									continue
								}
								result = re.ReplaceAllString(result, newKey)
							}
						}
						return result, nil
					} else {
						// Variable is not a list or dict - use as literal
						patterns = append(patterns, fmt.Sprintf("%v", varValue))
					}
				} else {
					// Not a variable - use as literal
					patterns = append(patterns, arg)
				}
			} else {
				// No kwargs - use as literal
				patterns = append(patterns, arg)
			}
		}
	}

	// Process all patterns with the same replacement
	if len(patterns) == 0 {
		// No patterns to process - return original string
		return str, nil
	}

	for _, pattern := range patterns {
		// Convert Python-style regex patterns to Go RE2 patterns
		convertedPattern := convertPythonRegexToRE2(pattern)

		re, err := regexp.Compile(convertedPattern)
		if err != nil {
			// Invalid regex - skip this pattern but continue with others
			continue
		}

		result = re.ReplaceAllString(result, replacement)
	}

	return result, nil
}

// convertPythonRegexToRE2 converts Python-style regex patterns to Go RE2 patterns
// RE2 doesn't support \d, \w, \s, \D, \W, \S - need to convert to character classes
func convertPythonRegexToRE2(pattern string) string {
	// First, handle escaped backslashes - if we have \\d, convert to \d
	// This handles cases where the pattern string has double backslashes
	// But also handle cases where the pattern already has single backslashes
	// We need to normalize both cases to single backslashes first
	pattern = strings.ReplaceAll(pattern, `\\d`, `\d`)
	pattern = strings.ReplaceAll(pattern, `\\D`, `\D`)
	pattern = strings.ReplaceAll(pattern, `\\w`, `\w`)
	pattern = strings.ReplaceAll(pattern, `\\W`, `\W`)
	pattern = strings.ReplaceAll(pattern, `\\s`, `\s`)
	pattern = strings.ReplaceAll(pattern, `\\S`, `\S`)

	// Now replace common Python regex shortcuts with RE2 equivalents
	// \d -> [0-9]
	pattern = strings.ReplaceAll(pattern, `\d`, `[0-9]`)
	// \D -> [^0-9]
	pattern = strings.ReplaceAll(pattern, `\D`, `[^0-9]`)
	// \w -> [0-9A-Za-z_]
	pattern = strings.ReplaceAll(pattern, `\w`, `[0-9A-Za-z_]`)
	// \W -> [^0-9A-Za-z_]
	pattern = strings.ReplaceAll(pattern, `\W`, `[^0-9A-Za-z_]`)
	// \s -> [\t\n\f\r ]
	pattern = strings.ReplaceAll(pattern, `\s`, `[\t\n\f\r ]`)
	// \S -> [^\t\n\f\r ]
	pattern = strings.ReplaceAll(pattern, `\S`, `[^\t\n\f\r ]`)

	return pattern
}

// rlookup performs reverse lookup - finds the key in lookup table that matches the value
// Example: rlookup("table_name", add_field="key") where value="host1" finds key="10.0.0.1"
// This is the reverse of normal lookup which finds value by key
// Supports dot-separated paths like "locations.cities" to access nested lookup tables
func rlookup(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	tablePath := args[0] // Can be "table_name" or "table_name.section.key"

	// Get lookup tables from kwargs (passed from runtime)
	var lookupTables map[string]map[string]interface{}
	if kwargs != nil {
		if tables, ok := kwargs["_lookup_tables"].(map[string]map[string]interface{}); ok {
			lookupTables = tables
		}
	}

	if lookupTables == nil {
		return value, nil // No lookup tables available
	}

	// Handle dot-separated paths (e.g., "locations.cities")
	// Split path to get table name and nested path
	pathParts := strings.Split(tablePath, ".")
	tableName := pathParts[0]
	
	table, exists := lookupTables[tableName]
	if !exists {
		return value, nil // Table not found
	}

	// Navigate nested structure if path has multiple parts
	var targetTable map[string]interface{}
	if len(pathParts) > 1 {
		// Navigate through nested structure
		current := interface{}(table)
		for i := 1; i < len(pathParts); i++ {
			if currentMap, ok := current.(map[string]interface{}); ok {
				if nested, exists := currentMap[pathParts[i]]; exists {
					current = nested
				} else {
					return value, nil // Path not found
				}
			} else {
				return value, nil // Path not found (not a map)
			}
		}
		// Final target should be a map for reverse lookup
		if finalMap, ok := current.(map[string]interface{}); ok {
			targetTable = finalMap
		} else {
			return value, nil // Final target is not a map
		}
	} else {
		// No nested path - use table directly
		targetTable = table
	}

	// Check if add_field is specified
	var addField string
	var hasAddField bool
	if kwargs != nil {
		// Check if add_field is in kwargs (from keyword arguments)
		if field, ok := kwargs["add_field"].(string); ok && field != "" {
			addField = field
			hasAddField = true
		}
		
		// Also check args for add_field (in case it's passed positionally)
		if !hasAddField && len(args) > 1 {
			for _, arg := range args[1:] {
				if strings.HasPrefix(arg, "add_field=") {
					addField = strings.TrimPrefix(arg, "add_field=")
					addField = strings.Trim(addField, `"'`)
					hasAddField = true
					break
				}
			}
		}
	}

	// Reverse lookup: find key pattern that matches the value
	// Example: value="vic-mel-core1", pattern="-mel-" should match (substring match)
	// Python TTP uses substring matching for reverse lookup patterns
	valueStr := fmt.Sprintf("%v", value)
	var matchedKey string
	var matchedValue interface{}
	
	for k, v := range targetTable {
		// Check if the key pattern is contained in the value (substring match)
		// This handles patterns like "-mel-" matching "vic-mel-core1"
		if strings.Contains(valueStr, k) {
			matchedKey = k
			matchedValue = v
			break
		}
		// Also check glob pattern matching (for patterns with wildcards)
		matched, err := filepath.Match(k, valueStr)
		if err == nil && matched {
			matchedKey = k
			matchedValue = v
			break
		}
		// Also check exact match (fallback)
		if k == valueStr {
			matchedKey = k
			matchedValue = v
			break
		}
	}

	if matchedKey == "" {
		// No match found, return original value
		return value, nil
	}

	// Handle add_field parameter
	if hasAddField && addField != "" {
		// Add the matched key to match data instead of replacing the value
		if matchData, ok := kwargs["_match_data"].(map[string]interface{}); ok {
			matchData[addField] = matchedValue
		}
		// Return original value (don't replace)
		return value, nil
	}

	// No add_field - return the matched key (replace value)
	return matchedKey, nil
}

// gpvlookup performs glob pattern lookup - finds value in lookup table using glob pattern matching
// Example: gpvlookup("table_name", pattern="*.example.com") where value="host.example.com"
// This uses glob patterns instead of exact match like lookup
func gpvlookup(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	tableName := args[0]

	// Get pattern from kwargs or args
	var pattern string
	if kwargs != nil {
		if p, ok := kwargs["pattern"].(string); ok {
			pattern = p
		}
	}

	// If pattern not in kwargs, check if it's in args
	if pattern == "" && len(args) > 1 {
		pattern = args[1]
	}

	// If no pattern specified, use value as pattern
	if pattern == "" {
		pattern = fmt.Sprintf("%v", value)
	}

	// Get lookup table from kwargs (passed from runtime)
	// The runtime should pass lookup tables in kwargs["_lookup_tables"]
	var lookupTables map[string]map[string]interface{}
	if kwargs != nil {
		if tables, ok := kwargs["_lookup_tables"].(map[string]map[string]interface{}); ok {
			lookupTables = tables
		}
	}

	if lookupTables == nil {
		return value, nil // No lookup tables available
	}

	table, exists := lookupTables[tableName]
	if !exists {
		return value, nil // Table not found
	}

	// Convert pattern to glob pattern if needed
	// Remove quotes if present
	pattern = strings.Trim(pattern, `"'`)

	// Use glob pattern matching to find matching key
	// Match pattern against table keys
	for k, v := range table {
		// Match key against pattern using glob
		matched, err := filepath.Match(pattern, k)
		if err != nil {
			// Invalid pattern, skip
			continue
		}

		if matched {
			// Found matching key, return its value
			return v, nil
		}
	}

	// No match found, return original value
	return value, nil
}

// toNet converts value to IP network object (CIDR notation)
// Example: "192.168.0.0/24" -> returns network string representation
// For now, returns CIDR string; in future could return network object
func toNet(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	str := strings.TrimSpace(fmt.Sprintf("%v", value))

	// Check if already in CIDR format
	if strings.Contains(str, "/") {
		// Validate it's a valid CIDR
		parts := strings.Split(str, "/")
		if len(parts) == 2 {
			// Validate IP part
			ipStr := parts[0]
			prefixStr := parts[1]

			// Basic IP validation
			ipv4Regex := regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`)
			if ipv4Regex.MatchString(ipStr) {
				// Validate octets
				ipParts := strings.Split(ipStr, ".")
				valid := true
				for _, part := range ipParts {
					octet, err := strconv.Atoi(part)
					if err != nil || octet < 0 || octet > 255 {
						valid = false
						break
					}
				}

				// Validate prefix length
				prefix, err := strconv.Atoi(prefixStr)
				if err == nil && prefix >= 0 && prefix <= 32 && valid {
					return str, nil // Valid CIDR
				}
			}
		}
	}

	// If not in CIDR format, try to convert IP to /32 network
	ipv4Regex := regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`)
	if ipv4Regex.MatchString(str) {
		// Validate octets
		parts := strings.Split(str, ".")
		valid := true
		for _, part := range parts {
			octet, err := strconv.Atoi(part)
			if err != nil || octet < 0 || octet > 255 {
				valid = false
				break
			}
		}
		if valid {
			return str + "/32", nil // Convert single IP to /32 network
		}
	}

	// Not a valid IP or network, return as-is
	return str, nil
}

// printFunc prints the value to stdout (for debugging)
// Returns the value unchanged
func printFunc(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Print to stdout
	fmt.Printf("%v\n", value)
	// Return value unchanged
	return value, nil
}

// toUnicode converts value to unicode (no-op in Go since strings are UTF-8 by default)
// This is provided for Python TTP compatibility
func toUnicode(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Go strings are already UTF-8, so this is a no-op
	return value, nil
}

// chainFunc applies a chain of functions from a template variable
// Example: chain("my_functions") where my_functions = "upper|strip|split(',')"
// The chain function looks up the variable and applies each function in sequence
func chainFunc(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil
	}

	varName := args[0]

	// Get the function chain string from kwargs (template variables)
	var chainStr string
	if kwargs != nil {
		if varValue, ok := kwargs[varName]; ok {
			if strValue, ok := varValue.(string); ok {
				chainStr = strValue
			} else {
				// Not a string, return original value
				return value, nil
			}
		} else {
			// Variable not found, return original value
			return value, nil
		}
	} else {
		// No kwargs, return original value
		return value, nil
	}

	// Parse pipe-separated function calls
	funcStrs := strings.Split(chainStr, "|")
	for i := range funcStrs {
		funcStrs[i] = strings.TrimSpace(funcStrs[i])
	}

	// Get function registry from kwargs if available
	var registry *Registry
	if kwargs != nil {
		if reg, ok := kwargs["_match_registry"].(*Registry); ok {
			registry = reg
		}
	}

	// Apply each function in sequence
	result := value
	for _, funcStr := range funcStrs {
		if funcStr == "" {
			continue
		}

		// Parse function call (e.g., "upper()", "split(',')", "unrange(rangechar='-', joinchar=',')")
		funcName, funcArgs, funcKwargs, err := parseChainFunction(funcStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse chain function %s: %w", funcStr, err)
		}

		// Merge function-specific kwargs with existing kwargs (function kwargs override)
		chainKwargs := make(map[string]interface{})
		if kwargs != nil {
			for k, v := range kwargs {
				chainKwargs[k] = v
			}
		}
		for k, v := range funcKwargs {
			chainKwargs[k] = v
		}

		// Try resolver first (supports custom function overrides)
		if resolver, ok := chainKwargs["_match_func_resolver"].(func(string) (Function, bool)); ok {
			if fn, ok := resolver(funcName); ok {
				newResult, err := fn(result, funcArgs, chainKwargs)
				if err != nil {
					return nil, fmt.Errorf("chain function %s failed: %w", funcName, err)
				}
				result = newResult
				continue
			}
		}

		// Fall back to registry (backward compat)
		if registry != nil {
			if fn, ok := registry.Get(funcName); ok {
				// Use the actual function from registry
				newResult, err := fn(result, funcArgs, chainKwargs)
				if err != nil {
					return nil, fmt.Errorf("chain function %s failed: %w", funcName, err)
				}
				result = newResult
				continue
			}
		}

		// Fallback to inline implementation
		var err2 error
		result, err2 = applyChainFunction(result, funcName, funcArgs, chainKwargs)
		if err2 != nil {
			return nil, fmt.Errorf("chain function %s failed: %w", funcName, err2)
		}
	}

	return result, nil
}

// parseChainFunction parses a function string like "upper()" or "split(',')" or "unrange(rangechar='-', joinchar=',')"
// Returns function name, positional arguments, keyword arguments, and error
func parseChainFunction(funcStr string) (string, []string, map[string]interface{}, error) {
	funcStr = strings.TrimSpace(funcStr)

	// Find the opening parenthesis
	parenIdx := strings.Index(funcStr, "(")
	if parenIdx == -1 {
		// No arguments, just function name
		return funcStr, nil, nil, nil
	}

	funcName := funcStr[:parenIdx]

	// Find the closing parenthesis
	closeIdx := strings.LastIndex(funcStr, ")")
	if closeIdx == -1 {
		return "", nil, nil, fmt.Errorf("unclosed parenthesis in function: %s", funcStr)
	}

	// Extract arguments
	argsStr := funcStr[parenIdx+1 : closeIdx]
	if argsStr == "" {
		return funcName, nil, nil, nil
	}

	// Parse arguments (comma-separated, respecting quotes)
	// Support both positional args and keyword args (key='value')
	var args []string
	kwargs := make(map[string]interface{})
	currentArg := ""
	inQuotes := false
	quoteChar := byte(0)

	for i := 0; i < len(argsStr); i++ {
		char := argsStr[i]

		if !inQuotes {
			if char == '"' || char == '\'' {
				inQuotes = true
				quoteChar = char
				currentArg += string(char)
			} else if char == ',' {
				if currentArg != "" {
					arg := strings.TrimSpace(currentArg)
					// Check if it's a keyword argument (key='value' or key="value")
					if strings.Contains(arg, "=") {
						parts := strings.SplitN(arg, "=", 2)
						if len(parts) == 2 {
							key := strings.TrimSpace(parts[0])
							value := strings.TrimSpace(parts[1])
							// Remove quotes from value
							value = strings.Trim(value, `"'`)
							kwargs[key] = value
						} else {
							// Not a valid keyword arg, treat as positional
							args = append(args, arg)
						}
					} else {
						// Positional argument
						args = append(args, arg)
					}
					currentArg = ""
				}
			} else {
				currentArg += string(char)
			}
		} else {
			currentArg += string(char)
			if char == quoteChar {
				inQuotes = false
				quoteChar = 0
			}
		}
	}

	// Process last argument
	if currentArg != "" {
		arg := strings.TrimSpace(currentArg)
		// Check if it's a keyword argument
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes from value
				value = strings.Trim(value, `"'`)
				kwargs[key] = value
			} else {
				// Not a valid keyword arg, treat as positional
				args = append(args, arg)
			}
		} else {
			// Positional argument
			args = append(args, arg)
		}
	}

	// Remove quotes from positional arguments
	for i := range args {
		args[i] = strings.Trim(args[i], `"'`)
	}

	return funcName, args, kwargs, nil
}

// applyChainFunction applies a single function from the chain
func applyChainFunction(value interface{}, funcName string, args []string, kwargs map[string]interface{}) (interface{}, error) {
	// Remove quotes from arguments (already done in parseChainFunction, but keep for safety)
	for i := range args {
		args[i] = strings.Trim(args[i], `"'`)
	}

	switch funcName {
	case "upper":
		str := fmt.Sprintf("%v", value)
		return strings.ToUpper(str), nil
	case "lower":
		str := fmt.Sprintf("%v", value)
		return strings.ToLower(str), nil
	case "strip":
		str := fmt.Sprintf("%v", value)
		return strings.TrimSpace(str), nil
	case "split":
		str := fmt.Sprintf("%v", value)
		delimiter := ","
		if len(args) > 0 {
			delimiter = args[0]
		}
		parts := strings.Split(str, delimiter)
		result := make([]interface{}, len(parts))
		for i, part := range parts {
			result[i] = strings.TrimSpace(part)
		}
		return result, nil
	case "join":
		delimiter := " "
		if len(args) > 0 {
			delimiter = args[0]
		}
		if list, ok := value.([]interface{}); ok {
			strs := make([]string, len(list))
			for i, item := range list {
				strs[i] = fmt.Sprintf("%v", item)
			}
			return strings.Join(strs, delimiter), nil
		}
		return value, nil
	case "unrange":
		// Call the actual unrange function from the registry
		// This is a fallback if the registry lookup failed
		return unrange(value, args, kwargs)
	default:
		// Unknown function, return value as-is
		return value, nil
	}
}

// macroFunc executes a macro function with the match result
// The macro registry should be passed via kwargs["_macro_registry"]
// The compiled template macros should be passed via kwargs["_macros"]
func macroFunc(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	if len(args) < 1 {
		return value, nil // No macro name provided, return as-is
	}

	macroName := args[0]
	// Remove quotes if present
	macroName = strings.Trim(macroName, `"'`)

	// Get macro registry from kwargs
	var macroRegistry interface {
		ExecuteMacro(language, name string, data interface{}, ttpContext map[string]interface{}) (interface{}, error)
	}

	if registry, ok := kwargs["_macro_registry"]; ok {
		if reg, ok := registry.(interface {
			ExecuteMacro(language, name string, data interface{}, ttpContext map[string]interface{}) (interface{}, error)
		}); ok {
			macroRegistry = reg
		}
	}

	if macroRegistry == nil {
		return value, nil // No macro registry available, return as-is
	}

	// Get macros list from kwargs to find the language
	var macros []struct {
		Language string
		Name     string
	}
	if macrosList, ok := kwargs["_macros"]; ok {
		if mList, ok := macrosList.([]struct {
			Language string
			Name     string
		}); ok {
			macros = mList
		}
	}

	// Try to find and execute the macro
	var macroLang string
	for _, m := range macros {
		// Try to match by name (we'll need to extract function name from macro source)
		// For now, try common languages
		if macroLang == "" {
			macroLang = m.Language
		}
	}

	// Default to starlark if no language found
	if macroLang == "" {
		macroLang = "starlark"
	}

	// Execute macro
	macroResult, err := macroRegistry.ExecuteMacro(macroLang, macroName, value, nil)
	if err != nil {
		// If macro not found, return original value (don't fail)
		return value, nil
	}

	// Handle different return types based on Python TTP behavior:
	// - True/False: treat as condition (return ConditionResult)
	// - None/nil: continue processing (return original value)
	// - Single item: replace value
	// - Tuple of two: first is value, second is dict of additional fields (not fully supported yet)

	// Check if result is boolean (condition)
	if boolResult, ok := macroResult.(bool); ok {
		// Return as ConditionResult
		return &ConditionResult{
			Value:     value,
			Condition: boolResult,
		}, nil
	}

	// Check if result is nil/None
	if macroResult == nil {
		return value, nil
	}

	// Otherwise, return the result (replaces value)
	return macroResult, nil
}
