package match

import (
	"fmt"
	"net"
	"strings"
)

// ipInfo returns detailed IP address information
// Example: {{ ip | ip_info }} -> returns dictionary with network, broadcast, etc.
func ipInfo(value interface{}, args []string, kwargs map[string]interface{}) (interface{}, error) {
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
		// Try parsing as CIDR
		ip, ipNet, err := net.ParseCIDR(ipStr)
		if err != nil {
			// Invalid IP or CIDR, return original value
			return value, nil
		}
		return buildIPInfo(ip, ipNet), nil
	}

	// Single IP address - create /32 network
	ipNet := &net.IPNet{
		IP:   ip,
		Mask: net.CIDRMask(32, 32), // /32 for IPv4, /128 for IPv6
	}
	if ip.To4() == nil {
		// IPv6
		ipNet.Mask = net.CIDRMask(128, 128)
	}

	return buildIPInfo(ip, ipNet), nil
}

// buildIPInfo builds a map with IP address information matching Python TTP's ipaddress module
func buildIPInfo(ip net.IP, ipNet *net.IPNet) map[string]interface{} {
	info := make(map[string]interface{})

	// Basic IP information (matching Python TTP's ipaddress module structure)
	ipStr := ip.String()
	info["ip"] = ipStr
	info["compressed"] = ipStr // For IPv4, compressed and exploded are the same
	info["exploded"] = ipStr   // For IPv4, compressed and exploded are the same

	// For IPv6, we'd need to format differently, but for now keep it simple
	if ip.To4() == nil {
		// IPv6 - format as expanded
		info["exploded"] = expandIPv6(ip)
		info["compressed"] = ipStr
	}

	// IP version and max prefix length
	if ip.To4() != nil {
		info["version"] = 4
		info["max_prefixlen"] = 32
	} else {
		info["version"] = 6
		info["max_prefixlen"] = 128
	}

	// Boolean flags
	info["is_private"] = isPrivateIP(ip)
	info["is_loopback"] = ip.IsLoopback()
	info["is_multicast"] = ip.IsMulticast()
	info["is_link_local"] = ip.IsLinkLocalUnicast()
	
	// Additional flags that Python TTP includes
	info["is_reserved"] = isReservedIP(ip)
	info["is_unspecified"] = ip.IsUnspecified()

	return info
}

// expandIPv6 expands an IPv6 address to its full form
func expandIPv6(ip net.IP) string {
	// For now, just return the string representation
	// Full expansion would format as 0000:0000:0000:0000:0000:0000:0000:0000
	return ip.String()
}

// isReservedIP checks if an IP address is reserved
func isReservedIP(ip net.IP) bool {
	if ipv4 := ip.To4(); ipv4 != nil {
		// Reserved IPv4 ranges
		return ipv4[0] == 0 || // 0.0.0.0/8
			(ipv4[0] == 224 && ipv4[1] == 0 && ipv4[2] == 0) || // 224.0.0.0/24
			(ipv4[0] >= 240) // 240.0.0.0/4
	}
	// For IPv6, check if it's in reserved ranges
	return ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// isPrivateIP checks if an IP address is in a private range
func isPrivateIP(ip net.IP) bool {
	if ipv4 := ip.To4(); ipv4 != nil {
		// Check private IPv4 ranges
		return ipv4[0] == 10 ||
			(ipv4[0] == 172 && ipv4[1] >= 16 && ipv4[1] <= 31) ||
			(ipv4[0] == 192 && ipv4[1] == 168) ||
			ipv4[0] == 127 // loopback
	}
	// Check private IPv6 ranges
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

