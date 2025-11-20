package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/roc-ops/gottp/internal/input"
)

// TestFileInputLoader tests loading input from a file using the input loader
func TestFileInputLoader(t *testing.T) {
	// Create a temporary file with test data
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_input.txt")
	
	data := `
interface Loopback0
 ip address 192.168.0.1/24
interface Vlan100
 ip address 10.0.0.1/24
`
	
	err := os.WriteFile(testFile, []byte(data), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	loader := input.NewLoader()
	loadedData, err := loader.LoadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to load file: %v", err)
	}
	
	if loadedData != data {
		t.Errorf("Loaded data does not match original. Expected length: %d, Got: %d", len(data), len(loadedData))
	}
	
	// Verify content
	if !strings.Contains(loadedData, "Loopback0") {
		t.Error("Loaded data does not contain expected content")
	}
}

// TestDirectoryInputLoader tests loading inputs from a directory using the input loader
func TestDirectoryInputLoader(t *testing.T) {
	// Create temporary directory with multiple files
	tmpDir := t.TempDir()
	
	// Create first file
	file1 := filepath.Join(tmpDir, "device1.txt")
	data1 := `interface Loopback0`
	os.WriteFile(file1, []byte(data1), 0644)
	
	// Create second file
	file2 := filepath.Join(tmpDir, "device2.txt")
	data2 := `interface Vlan100`
	os.WriteFile(file2, []byte(data2), 0644)
	
	loader := input.NewLoader()
	files, err := loader.LoadDirectory(tmpDir, []string{"txt"})
	if err != nil {
		t.Fatalf("Failed to load directory: %v", err)
	}
	
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}
	
	// Verify files are in the list
	found1 := false
	found2 := false
	for _, f := range files {
		if strings.Contains(f, "device1.txt") {
			found1 = true
		}
		if strings.Contains(f, "device2.txt") {
			found2 = true
		}
	}
	
	if !found1 || !found2 {
		t.Error("Expected both device files to be found")
	}
}

