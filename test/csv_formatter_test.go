package test

import (
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/formatters"
)

// TestCSVFormatter tests CSV formatter functionality
func TestCSVFormatter(t *testing.T) {
	formatter := formatters.NewCSVFormatter()

	data := []map[string]interface{}{
		{"interface": "Loopback0", "ip": "192.168.0.1", "mask": "24"},
		{"interface": "Vlan100", "ip": "10.0.0.1", "mask": "24"},
	}

	options := &formatters.CSVOptions{
		Sep:     ",",
		Quote:   "\"",
		Missing: "",
	}

	result, err := formatter.FormatString(data, options)
	if err != nil {
		t.Fatalf("Failed to format CSV: %v", err)
	}

	t.Logf("CSV Result:\n%s", result)

	// Verify CSV format
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines (header + 2 data), got %d", len(lines))
	}

	// Check header
	if !strings.Contains(lines[0], "interface") {
		t.Error("Expected header to contain 'interface'")
	}

	// Check data rows
	if !strings.Contains(lines[1], "Loopback0") {
		t.Error("Expected first data row to contain 'Loopback0'")
	}
	if !strings.Contains(lines[2], "Vlan100") {
		t.Error("Expected second data row to contain 'Vlan100'")
	}
}

// TestCSVFormatterWithCustomHeaders tests CSV formatter with custom headers
func TestCSVFormatterWithCustomHeaders(t *testing.T) {
	formatter := formatters.NewCSVFormatter()

	data := []map[string]interface{}{
		{"interface": "Loopback0", "ip": "192.168.0.1", "mask": "24"},
	}

	options := &formatters.CSVOptions{
		Sep:     ",",
		Headers: []string{"interface", "ip", "mask"},
		Missing: "",
	}

	result, err := formatter.FormatString(data, options)
	if err != nil {
		t.Fatalf("Failed to format CSV: %v", err)
	}

	t.Logf("CSV Result:\n%s", result)

	// Verify headers are in correct order
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) < 1 {
		t.Fatal("Expected at least header line")
	}

	header := lines[0]
	if !strings.HasPrefix(header, "interface") {
		t.Error("Expected header to start with 'interface'")
	}
}

