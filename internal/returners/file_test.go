package returners

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileReturner(t *testing.T) {
	path := "/tmp/test_file.txt"
	returner := NewFileReturner(path)
	
	if returner == nil {
		t.Fatal("NewFileReturner() returned nil")
	}
	if returner.Path != path {
		t.Errorf("NewFileReturner() path = %v, want %v", returner.Path, path)
	}
}

func TestFileReturner_Return(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "write string data",
			data:    []byte("test data"),
			wantErr: false,
		},
		{
			name:    "write empty data",
			data:    []byte(""),
			wantErr: false,
		},
		{
			name:    "write binary data",
			data:    []byte{0x00, 0x01, 0x02, 0xFF},
			wantErr: false,
		},
		{
			name:    "write multiline data",
			data:    []byte("line1\nline2\nline3"),
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique file for each test
			testFile := filepath.Join(tmpDir, tt.name+".txt")
			testReturner := NewFileReturner(testFile)
			
			err := testReturner.Return(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileReturner.Return() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				// Verify file was created and contains correct data
				readData, err := os.ReadFile(testFile)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
					return
				}
				if string(readData) != string(tt.data) {
					t.Errorf("File content = %v, want %v", string(readData), string(tt.data))
				}
			}
		})
	}
}

func TestFileReturner_ReturnString(t *testing.T) {
	tmpDir := t.TempDir()
	
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "write string",
			data:    "test string",
			wantErr: false,
		},
		{
			name:    "write empty string",
			data:    "",
			wantErr: false,
		},
		{
			name:    "write multiline string",
			data:    "line1\nline2\nline3",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name+".txt")
			testReturner := NewFileReturner(testFile)
			
			err := testReturner.ReturnString(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileReturner.ReturnString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !tt.wantErr {
				// Verify file was created and contains correct data
				readData, err := os.ReadFile(testFile)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
					return
				}
				if string(readData) != tt.data {
					t.Errorf("File content = %v, want %v", string(readData), tt.data)
				}
			}
		})
	}
}

func TestFileReturner_Return_NoPath(t *testing.T) {
	returner := NewFileReturner("")
	
	err := returner.Return([]byte("test"))
	if err == nil {
		t.Error("FileReturner.Return() expected error for empty path, got nil")
	}
}

func TestFileReturner_Return_InvalidPath(t *testing.T) {
	// Try to write to a directory (should fail)
	tmpDir := t.TempDir()
	returner := NewFileReturner(tmpDir)
	
	err := returner.Return([]byte("test"))
	if err == nil {
		t.Error("FileReturner.Return() expected error for directory path, got nil")
	}
}

func TestFileReturner_Return_PermissionDenied(t *testing.T) {
	// On Unix systems, try to write to /dev/null (should work)
	// On other systems, this might behave differently
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}
	
	// Try to write to a path in a non-existent directory
	returner := NewFileReturner("/nonexistent/path/file.txt")
	
	err := returner.Return([]byte("test"))
	if err == nil {
		t.Error("FileReturner.Return() expected error for invalid path, got nil")
	}
}

