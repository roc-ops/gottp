package test

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/roc-ops/gottp/internal/returners"
)

// TestSyslogReturnerBasic tests basic syslog returner functionality
func TestSyslogReturnerBasic(t *testing.T) {
	// Start a UDP server to receive syslog messages
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve address: %v", err)
	}
	
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer conn.Close()
	
	// Get the actual port
	serverAddr := conn.LocalAddr().(*net.UDPAddr)
	serverPort := serverAddr.Port
	
	// Create syslog returner
	returner := returners.NewSyslogReturner(
		[]string{"127.0.0.1"},
		serverPort,
		77,
		nil,
		true,
		1,
	)
	
	// Test data
	data := map[string]interface{}{
		"test": "message",
		"value": 123,
	}
	
	// Send data
	jsonData, _ := json.Marshal(data)
	err = returner.Return(jsonData)
	if err != nil {
		t.Fatalf("Failed to send to syslog: %v", err)
	}
	
	// Wait a bit for message to arrive
	time.Sleep(100 * time.Millisecond)
	
	// Read message
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buffer := make([]byte, 4096)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	
	message := string(buffer[:n])
	t.Logf("Received syslog message: %s", message)
	
	// Verify message contains our data
	if !strings.Contains(message, "test") || !strings.Contains(message, "message") {
		t.Error("Message does not contain expected data")
	}
}

// TestSyslogReturnerWithPath tests syslog returner with path traversal
func TestSyslogReturnerWithPath(t *testing.T) {
	// Start a UDP server
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to resolve address: %v", err)
	}
	
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer conn.Close()
	
	serverPort := conn.LocalAddr().(*net.UDPAddr).Port
	
	// Create syslog returner with path
	returner := returners.NewSyslogReturner(
		[]string{"127.0.0.1"},
		serverPort,
		77,
		[]string{"interfaces", "0"},
		true,
		1,
	)
	
	// Test data with nested structure
	data := map[string]interface{}{
		"interfaces": []interface{}{
			map[string]interface{}{
				"name": "eth0",
				"ip":   "192.168.1.1",
			},
		},
	}
	
	jsonData, _ := json.Marshal(data)
	err = returner.Return(jsonData)
	if err != nil {
		t.Fatalf("Failed to send to syslog: %v", err)
	}
	
	// Wait for message
	time.Sleep(100 * time.Millisecond)
	
	// Read message
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buffer := make([]byte, 4096)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}
	
	message := string(buffer[:n])
	t.Logf("Received syslog message: %s", message)
	
	// Verify message contains path-traversed data
	if !strings.Contains(message, "eth0") {
		t.Error("Message does not contain path-traversed data")
	}
}

// TestParseSyslogOptions tests parsing syslog options
func TestParseSyslogOptions(t *testing.T) {
	options := map[string]interface{}{
		"servers":  "192.168.1.1,192.168.1.2",
		"port":     "514",
		"facility": "77",
		"path":     "interfaces",
		"iterate":  "true",
		"interval": "10",
	}
	
	returner, err := returners.ParseSyslogOptions(options)
	if err != nil {
		t.Fatalf("Failed to parse options: %v", err)
	}
	
	if len(returner.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(returner.Servers))
	}
	
	if returner.Port != 514 {
		t.Errorf("Expected port 514, got %d", returner.Port)
	}
	
	if returner.Facility != 77 {
		t.Errorf("Expected facility 77, got %d", returner.Facility)
	}
	
	if len(returner.Path) != 1 || returner.Path[0] != "interfaces" {
		t.Errorf("Expected path ['interfaces'], got %v", returner.Path)
	}
	
	if !returner.Iterate {
		t.Error("Expected iterate to be true")
	}
	
	if returner.Interval != 10 {
		t.Errorf("Expected interval 10, got %d", returner.Interval)
	}
}


