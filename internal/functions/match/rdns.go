package match

import (
	"fmt"
	"net"
	"strings"
)

// rdns performs DNS reverse lookup (IP address to hostname)
// Example: {{ ip | rdns }} -> returns hostname
func rdns(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	ipStr := fmt.Sprintf("%v", value)
	ipStr = strings.TrimSpace(ipStr)
	// Remove quotes if present
	ipStr = strings.Trim(ipStr, `"'`)

	if ipStr == "" {
		return value, nil
	}

	// Parse IP address
	ip := net.ParseIP(ipStr)
	if ip == nil {
		// Invalid IP, return original value
		return value, nil
	}

	// Perform reverse DNS lookup
	hostnames, err := net.LookupAddr(ipStr)
	if err != nil {
		// Reverse DNS lookup failed, return original value (don't fail)
		return value, nil
	}

	// Return first hostname, removing trailing dot if present
	if len(hostnames) > 0 {
		hostname := hostnames[0]
		hostname = strings.TrimSuffix(hostname, ".")
		return hostname, nil
	}

	// No hostname found, return original value
	return value, nil
}

