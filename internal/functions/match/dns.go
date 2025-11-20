package match

import (
	"fmt"
	"net"
	"strings"
)

// dns performs DNS forward lookup (hostname to IP address)
// Example: {{ hostname | dns }} -> returns IP address
func dns(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
	hostname := fmt.Sprintf("%v", value)
	hostname = strings.TrimSpace(hostname)
	// Remove quotes if present
	hostname = strings.Trim(hostname, `"'`)

	if hostname == "" {
		return value, nil
	}

	// Perform DNS lookup
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// DNS lookup failed, return original value (don't fail)
		return value, nil
	}

	// Return first IPv4 address if available, otherwise first IP
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}

	// No IPv4 found, return first IP (IPv6)
	if len(ips) > 0 {
		return ips[0].String(), nil
	}

	// No IPs found, return original value
	return value, nil
}

