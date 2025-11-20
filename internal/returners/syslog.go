package returners

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// SyslogReturner sends results to syslog servers over UDP
type SyslogReturner struct {
	Servers  []string // List of syslog server addresses
	Port     int      // UDP port (default: 514)
	Facility int      // Syslog facility (default: 77)
	Path     []string // Path to data in results (dot-separated)
	Iterate  bool     // If true, iterate over lists (default: true)
	Interval int      // Milliseconds between messages (default: 1)
}

// NewSyslogReturner creates a new syslog returner
func NewSyslogReturner(servers []string, port int, facility int, path []string, iterate bool, interval int) *SyslogReturner {
	if port == 0 {
		port = 514
	}
	if facility == 0 {
		facility = 77
	}
	if interval == 0 {
		interval = 1
	}
	return &SyslogReturner{
		Servers:  servers,
		Port:     port,
		Facility: facility,
		Path:     path,
		Iterate:  iterate,
		Interval: interval,
	}
}

// Return sends data to syslog servers
func (s *SyslogReturner) Return(data []byte) error {
	// Parse JSON data
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("failed to parse JSON data: %w", err)
	}

	return s.ReturnData(jsonData)
}

// ReturnString sends string data to syslog servers
func (s *SyslogReturner) ReturnString(data string) error {
	// Try to parse as JSON first
	var jsonData interface{}
	if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
		// If not JSON, send as plain text
		return s.sendToSyslog(data)
	}

	return s.ReturnData(jsonData)
}

// ReturnData sends structured data to syslog servers
func (s *SyslogReturner) ReturnData(data interface{}) error {
	if len(s.Servers) == 0 {
		return fmt.Errorf("no syslog servers specified")
	}

	// Normalize to list
	var sourceData []interface{}
	switch v := data.(type) {
	case []interface{}:
		sourceData = v
	default:
		sourceData = []interface{}{v}
	}

	// Process each datum
	for _, datum := range sourceData {
		if datum == nil {
			continue
		}

		// Traverse path if specified
		item := datum
		if len(s.Path) > 0 {
			var err error
			item, err = traversePath(datum, s.Path)
			if err != nil {
				// Skip if path traversal fails
				continue
			}
		}

		if item == nil {
			continue
		}

		// Handle iteration
		if list, ok := item.([]interface{}); ok && s.Iterate {
			for _, i := range list {
				if err := s.sendItem(i); err != nil {
					// Log error but continue
					continue
				}
				// Wait interval between messages
				if s.Interval > 0 {
					time.Sleep(time.Duration(s.Interval) * time.Millisecond)
				}
			}
		} else {
			if err := s.sendItem(item); err != nil {
				// Log error but continue
				continue
			}
			// Wait interval between messages
			if s.Interval > 0 {
				time.Sleep(time.Duration(s.Interval) * time.Millisecond)
			}
		}
	}

	return nil
}

// sendItem sends a single item to all syslog servers
func (s *SyslogReturner) sendItem(item interface{}) error {
	// Convert item to JSON
	jsonBytes, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item to JSON: %w", err)
	}

	jsonStr := string(jsonBytes)

	// Send to all servers
	for _, server := range s.Servers {
		if err := s.sendToSyslogServer(server, jsonStr); err != nil {
			// Log error but continue to next server
			continue
		}
	}

	return nil
}

// sendToSyslogServer sends a message to a specific syslog server
func (s *SyslogReturner) sendToSyslogServer(server, message string) error {
	// Format syslog message (RFC 3164 format)
	// Priority = (Facility * 8) + Severity
	// Severity 6 = Informational
	priority := s.Facility*8 + 6

	// Format: <PRI>timestamp hostname tag: message
	// For simplicity, we'll use a basic format
	hostname := "gottp"
	tag := "gottp"

	// Get current timestamp
	timestamp := time.Now().Format("Jan 2 15:04:05")

	// Format syslog message
	syslogMsg := fmt.Sprintf("<%d>%s %s %s: %s", priority, timestamp, hostname, tag, message)

	// Send UDP packet
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", server, s.Port))
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("failed to dial UDP: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(syslogMsg))
	if err != nil {
		return fmt.Errorf("failed to write UDP packet: %w", err)
	}

	return nil
}

// sendToSyslog sends a plain text message to syslog
func (s *SyslogReturner) sendToSyslog(message string) error {
	for _, server := range s.Servers {
		if err := s.sendToSyslogServer(server, message); err != nil {
			continue
		}
	}
	return nil
}

// traversePath traverses a path in the data structure
func traversePath(data interface{}, path []string) (interface{}, error) {
	// traversePath expects map at root
	if _, ok := data.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("traversePath expects map at root, got %T", data)
	}

	current := data

	for _, key := range path {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[key]; ok {
				current = val
			} else {
				return nil, fmt.Errorf("key not found: %s", key)
			}
		case []interface{}:
			// Try to convert key to index
			index, err := strconv.Atoi(key)
			if err != nil {
				return nil, fmt.Errorf("invalid index: %s", key)
			}
			if index >= 0 && index < len(v) {
				current = v[index]
			} else {
				return nil, fmt.Errorf("index out of range: %d", index)
			}
		default:
			return nil, fmt.Errorf("cannot traverse path at key: %s", key)
		}
	}

	return current, nil
}

// ParseSyslogOptions parses syslog returner options from a map
func ParseSyslogOptions(options map[string]interface{}) (*SyslogReturner, error) {
	// Get servers
	var servers []string
	if serversVal, ok := options["servers"]; ok {
		switch v := serversVal.(type) {
		case string:
			// Split by comma if multiple servers
			servers = strings.Split(v, ",")
			for i := range servers {
				servers[i] = strings.TrimSpace(servers[i])
			}
		case []string:
			servers = v
		case []interface{}:
			for _, s := range v {
				if str, ok := s.(string); ok {
					servers = append(servers, str)
				}
			}
		}
	}

	if len(servers) == 0 {
		return nil, fmt.Errorf("no syslog servers specified")
	}

	// Get port
	port := 514
	if portVal, ok := options["port"]; ok {
		switch v := portVal.(type) {
		case int:
			port = v
		case string:
			if p, err := strconv.Atoi(v); err == nil {
				port = p
			}
		}
	}

	// Get facility
	facility := 77
	if facilityVal, ok := options["facility"]; ok {
		switch v := facilityVal.(type) {
		case int:
			facility = v
		case string:
			if f, err := strconv.Atoi(v); err == nil {
				facility = f
			}
		}
	}

	// Get path
	var path []string
	if pathVal, ok := options["path"]; ok {
		switch v := pathVal.(type) {
		case string:
			if v != "" {
				path = strings.Split(v, ".")
			}
		case []string:
			path = v
		case []interface{}:
			for _, p := range v {
				if str, ok := p.(string); ok {
					path = append(path, str)
				}
			}
		}
	}

	// Get iterate
	iterate := true
	if iterateVal, ok := options["iterate"]; ok {
		switch v := iterateVal.(type) {
		case bool:
			iterate = v
		case string:
			iterate = strings.ToLower(v) == "true" || v == "1"
		}
	}

	// Get interval
	interval := 1
	if intervalVal, ok := options["interval"]; ok {
		switch v := intervalVal.(type) {
		case int:
			interval = v
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				interval = i
			}
		}
	}

	return NewSyslogReturner(servers, port, facility, path, iterate, interval), nil
}
