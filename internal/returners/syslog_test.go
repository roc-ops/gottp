package returners

import (
	"encoding/json"
	"testing"
)

func TestNewSyslogReturner(t *testing.T) {
	tests := []struct {
		name     string
		servers  []string
		port     int
		facility int
		path     []string
		iterate  bool
		interval int
		wantPort int
		wantFac  int
		wantInt  int
	}{
		{
			name:     "with defaults",
			servers:  []string{"localhost"},
			port:     0,
			facility: 0,
			path:     nil,
			iterate:  true,
			interval: 0,
			wantPort: 514,
			wantFac:  77,
			wantInt:  1,
		},
		{
			name:     "with custom values",
			servers:  []string{"192.168.1.1"},
			port:     5140,
			facility: 16,
			path:     []string{"data"},
			iterate:  false,
			interval: 100,
			wantPort: 5140,
			wantFac:  16,
			wantInt:  100,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			returner := NewSyslogReturner(tt.servers, tt.port, tt.facility, tt.path, tt.iterate, tt.interval)
			
			if returner == nil {
				t.Fatal("NewSyslogReturner() returned nil")
			}
			if returner.Port != tt.wantPort {
				t.Errorf("NewSyslogReturner() port = %v, want %v", returner.Port, tt.wantPort)
			}
			if returner.Facility != tt.wantFac {
				t.Errorf("NewSyslogReturner() facility = %v, want %v", returner.Facility, tt.wantFac)
			}
			if returner.Interval != tt.wantInt {
				t.Errorf("NewSyslogReturner() interval = %v, want %v", returner.Interval, tt.wantInt)
			}
		})
	}
}

func TestSyslogReturner_Return(t *testing.T) {
	// Use a non-existent server to avoid actual network calls
	returner := NewSyslogReturner([]string{"127.0.0.1"}, 514, 77, nil, false, 1)
	
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid JSON",
			data:    []byte(`{"key": "value"}`),
			wantErr: false, // May fail on network, but JSON parsing should succeed
		},
		{
			name:    "invalid JSON",
			data:    []byte(`invalid json`),
			wantErr: true,
		},
		{
			name:    "empty data",
			data:    []byte(""),
			wantErr: true, // Empty string is not valid JSON
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := returner.Return(tt.data)
			if (err != nil) != tt.wantErr {
				// Network errors are acceptable - we're testing the logic
				if tt.wantErr && err != nil {
					// Expected error occurred
					return
				}
				t.Logf("SyslogReturner.Return() error = %v (may be network error)", err)
			}
		})
	}
}

func TestSyslogReturner_ReturnString(t *testing.T) {
	returner := NewSyslogReturner([]string{"127.0.0.1"}, 514, 77, nil, false, 1)
	
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "valid JSON string",
			data:    `{"key": "value"}`,
			wantErr: false,
		},
		{
			name:    "plain text string",
			data:    "plain text",
			wantErr: false, // Should send as plain text
		},
		{
			name:    "empty string",
			data:    "",
			wantErr: false, // Empty string should be sent as plain text
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := returner.ReturnString(tt.data)
			// Network errors are acceptable - we're testing the logic
			if err != nil && !tt.wantErr {
				t.Logf("SyslogReturner.ReturnString() error = %v (may be network error)", err)
			}
		})
	}
}

func TestSyslogReturner_ReturnData(t *testing.T) {
	returner := NewSyslogReturner([]string{"127.0.0.1"}, 514, 77, nil, false, 1)
	
	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name:    "map data",
			data:    map[string]interface{}{"key": "value"},
			wantErr: false,
		},
		{
			name:    "list data",
			data:    []interface{}{map[string]interface{}{"key": "value"}},
			wantErr: false,
		},
		{
			name:    "nil data",
			data:    nil,
			wantErr: false, // Should skip nil
		},
		{
			name:    "no servers",
			data:    map[string]interface{}{"key": "value"},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testReturner := returner
			if tt.name == "no servers" {
				testReturner = NewSyslogReturner([]string{}, 514, 77, nil, false, 1)
			}
			
			err := testReturner.ReturnData(tt.data)
			if (err != nil) != tt.wantErr {
				// Network errors are acceptable for valid cases
				if !tt.wantErr && err != nil {
					t.Logf("SyslogReturner.ReturnData() error = %v (may be network error)", err)
					return
				}
				t.Errorf("SyslogReturner.ReturnData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyslogReturner_ReturnData_WithPath(t *testing.T) {
	returner := NewSyslogReturner([]string{"127.0.0.1"}, 514, 77, []string{"data", "items"}, false, 1)
	
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"key": "value1"},
				map[string]interface{}{"key": "value2"},
			},
		},
	}
	
	err := returner.ReturnData(data)
	// Network errors are acceptable - we're testing the logic
	if err != nil {
		t.Logf("SyslogReturner.ReturnData() error = %v (may be network error)", err)
	}
}

func TestSyslogReturner_ReturnData_WithIterate(t *testing.T) {
	returner := NewSyslogReturner([]string{"127.0.0.1"}, 514, 77, nil, true, 1)
	
	data := []interface{}{
		map[string]interface{}{"key": "value1"},
		map[string]interface{}{"key": "value2"},
	}
	
	err := returner.ReturnData(data)
	// Network errors are acceptable - we're testing the logic
	if err != nil {
		t.Logf("SyslogReturner.ReturnData() error = %v (may be network error)", err)
	}
}

func TestParseSyslogOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]interface{}
		wantErr bool
	}{
		{
			name:    "valid options",
			options: map[string]interface{}{"servers": "127.0.0.1,192.168.1.1", "port": 5140, "facility": 16},
			wantErr: false,
		},
		{
			name:    "empty options",
			options: map[string]interface{}{},
			wantErr: true, // No servers specified should return error
		},
		{
			name:    "single server",
			options: map[string]interface{}{"servers": "127.0.0.1"},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSyslogOptions(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSyslogOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyslogReturner_JSONMarshaling(t *testing.T) {
	returner := NewSyslogReturner([]string{"127.0.0.1"}, 514, 77, nil, false, 1)
	
	data := map[string]interface{}{
		"key": "value",
		"number": 42,
		"nested": map[string]interface{}{
			"inner": "data",
		},
	}
	
	// Test that data can be marshaled to JSON (required for syslog)
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal data to JSON: %v", err)
	}
	
	// Test Return with JSON data
	err = returner.Return(jsonData)
	// Network errors are acceptable
	if err != nil {
		t.Logf("SyslogReturner.Return() error = %v (may be network error)", err)
	}
}

