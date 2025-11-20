package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/returners"
)

// TestFileReturnerBasic tests basic file returner functionality
func TestFileReturnerBasic(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")
	
	returner := returners.NewFileReturner(outputFile)
	
	data := map[string]interface{}{
		"test": "data",
		"value": 123,
	}
	
	jsonData, _ := json.Marshal(data)
	err := returner.Return(jsonData)
	if err != nil {
		t.Fatalf("Failed to write to file: %v", err)
	}
	
	// Read back the file
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	
	// Verify content
	if !strings.Contains(string(content), "test") {
		t.Error("File content does not contain expected data")
	}
}

// TestFileReturnerString tests file returner with string data
func TestFileReturnerString(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")
	
	returner := returners.NewFileReturner(outputFile)
	
	data := "test output string"
	err := returner.ReturnString(data)
	if err != nil {
		t.Fatalf("Failed to write string to file: %v", err)
	}
	
	// Read back the file
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	
	if string(content) != data {
		t.Errorf("File content mismatch. Expected: %s, Got: %s", data, string(content))
	}
}

// TestFileReturnerNoPath tests file returner without path
func TestFileReturnerNoPath(t *testing.T) {
	returner := returners.NewFileReturner("")
	
	data := []byte("test")
	err := returner.Return(data)
	if err == nil {
		t.Error("Expected error when path is not specified")
	}
}

