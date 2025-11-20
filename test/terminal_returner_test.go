package test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/returners"
)

// TestTerminalReturnerBasic tests basic terminal returner functionality
func TestTerminalReturnerBasic(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	returner := returners.NewTerminalReturner()
	
	data := map[string]interface{}{
		"test": "data",
		"value": 123,
	}
	
	jsonData, _ := json.Marshal(data)
	err := returner.Return(jsonData)
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	
	if err != nil {
		t.Fatalf("Failed to write to terminal: %v", err)
	}
	
	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	
	// Verify content was written
	if !strings.Contains(output, "test") {
		t.Error("Terminal output does not contain expected data")
	}
}

// TestTerminalReturnerString tests terminal returner with string data
func TestTerminalReturnerString(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	returner := returners.NewTerminalReturner()
	
	data := "test output string"
	err := returner.ReturnString(data)
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	
	if err != nil {
		t.Fatalf("Failed to write string to terminal: %v", err)
	}
	
	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	
	if output != data {
		t.Errorf("Terminal output mismatch. Expected: %s, Got: %s", data, output)
	}
}

// TestTerminalReturnerJSON tests terminal returner with JSON data
func TestTerminalReturnerJSON(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	returner := returners.NewTerminalReturner()
	
	data := []byte(`{"key": "value", "number": 42}`)
	err := returner.Return(data)
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	
	if err != nil {
		t.Fatalf("Failed to write JSON to terminal: %v", err)
	}
	
	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	
	// Verify JSON content
	if !strings.Contains(output, "key") || !strings.Contains(output, "value") {
		t.Error("Terminal output should contain JSON data")
	}
}

// TestTerminalReturnerEmptyData tests terminal returner with empty data
func TestTerminalReturnerEmptyData(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	returner := returners.NewTerminalReturner()
	
	err := returner.Return([]byte{})
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	
	if err != nil {
		t.Fatalf("Failed to write empty data to terminal: %v", err)
	}
	
	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	
	// Empty data should result in empty output
	if output != "" {
		t.Errorf("Expected empty output, got: %s", output)
	}
}

// TestTerminalReturnerEmptyString tests terminal returner with empty string
func TestTerminalReturnerEmptyString(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	returner := returners.NewTerminalReturner()
	
	err := returner.ReturnString("")
	
	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	
	if err != nil {
		t.Fatalf("Failed to write empty string to terminal: %v", err)
	}
	
	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()
	
	// Empty string should result in empty output
	if output != "" {
		t.Errorf("Expected empty output, got: %s", output)
	}
}

