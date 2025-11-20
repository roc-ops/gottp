package input

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestLoader_LoadFile_ErrorHandling(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "non-existent file",
			path:    "/nonexistent/path/file.txt",
			wantErr: true,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "path to directory",
			path:    "/tmp",
			wantErr: false, // May succeed or fail depending on implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.LoadFile(tt.path)
			if (err != nil) != tt.wantErr {
				// Directory path might succeed, so be lenient
				if tt.name == "path to directory" && err == nil {
					t.Logf("LoadFile() succeeded for directory (may be expected)")
					return
				}
				t.Errorf("LoadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != "" {
				t.Logf("LoadFile() result length = %d", len(result))
			}
		})
	}
}

func TestLoader_LoadDirectory_ErrorHandling(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		name       string
		dirPath    string
		extensions []string
		wantErr    bool
	}{
		{
			name:       "non-existent directory",
			dirPath:    "/nonexistent/directory",
			extensions: nil,
			wantErr:    true,
		},
		{
			name:       "empty path",
			dirPath:    "",
			extensions: nil,
			wantErr:    true,
		},
		{
			name:       "path to file (not directory)",
			dirPath:    "/etc/passwd", // Assuming this exists
			extensions: nil,
			wantErr:    true,
		},
		{
			name:       "valid directory with extensions",
			dirPath:    "/tmp",
			extensions: []string{"txt", "log"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.LoadDirectory(tt.dirPath, tt.extensions)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadDirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				t.Logf("LoadDirectory() found %d files", len(result))
			}
		})
	}
}

func TestLoader_LoadYAML_ErrorHandling(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "invalid YAML syntax",
			data:    "key: value\n  invalid: indentation",
			wantErr: true,
		},
		{
			name:    "unclosed string",
			data:    "key: \"unclosed string",
			wantErr: true,
		},
		{
			name:    "invalid list syntax",
			data:    "- item1\n- item2\n  invalid",
			wantErr: false, // YAML parser (gopkg.in/yaml.v3) is lenient and accepts this as valid YAML
		},
		{
			name:    "invalid map syntax",
			data:    "key1: value1\nkey2:",
			wantErr: false, // YAML parser (gopkg.in/yaml.v3) is lenient and accepts this as valid YAML
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.LoadYAML(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				t.Logf("LoadYAML() result = %v", result)
			}
		})
	}
}

func TestLoader_LoadJSON_ErrorHandling(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "invalid JSON syntax",
			data:    `{"key": "value", invalid}`,
			wantErr: true,
		},
		{
			name:    "unclosed object",
			data:    `{"key": "value"`,
			wantErr: true,
		},
		{
			name:    "unclosed array",
			data:    `["item1", "item2"`,
			wantErr: true,
		},
		{
			name:    "invalid escape sequence",
			data:    `{"key": "value\x"}`,
			wantErr: true,
		},
		{
			name:    "trailing comma",
			data:    `{"key": "value",}`,
			wantErr: true,
		},
		{
			name:    "duplicate keys",
			data:    `{"key": "value1", "key": "value2"}`,
			wantErr: false, // JSON allows duplicate keys (last one wins)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.LoadJSON(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				t.Logf("LoadJSON() result = %v", result)
			}
		})
	}
}

func TestLoader_LoadCSV_ErrorHandling(t *testing.T) {
	loader := NewLoader()

	tests := []struct {
		name      string
		data      string
		keyColumn string
		wantErr   bool
	}{
		{
			name:      "malformed CSV - unclosed quote",
			data:      "name,age\n\"John,30",
			keyColumn: "",
			wantErr:   true,
		},
		{
			name:      "empty CSV with headers",
			data:      "name,age\n",
			keyColumn: "",
			wantErr:   false,
		},
		{
			name:      "CSV with inconsistent columns",
			data:      "name,age\nJohn,30\nJane",
			keyColumn: "",
			wantErr:   true, // CSV reader errors on inconsistent columns
		},
		{
			name:      "key column not in CSV",
			data:      "name,age\nJohn,30",
			keyColumn: "nonexistent",
			wantErr:   false, // Returns empty map
		},
		{
			name:      "very large CSV",
			data:      createLargeCSV(10000),
			keyColumn: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.LoadCSV(tt.data, tt.keyColumn)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadCSV() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && result != nil {
				t.Logf("LoadCSV() result type = %T", result)
			}
		})
	}
}

func TestLoader_LoadFromReader_ErrorHandling(t *testing.T) {
	loader := NewLoader()

	// Test with a reader that will error
	t.Run("reader error", func(t *testing.T) {
		// Create a reader that will error on read
		reader := &errorReader{}
		result, err := loader.LoadFromReader(reader)
		if err == nil {
			t.Error("LoadFromReader() expected error from error reader, got nil")
		}
		if result != "" {
			t.Errorf("LoadFromReader() result = %v, want empty string", result)
		}
	})
}

// Helper functions

func createLargeCSV(rows int) string {
	var csv strings.Builder
	csv.WriteString("name,age,value\n")
	for i := 0; i < rows; i++ {
		csv.WriteString(fmt.Sprintf("Item%d,%d,%d\n", i, 30+i, i*2))
	}
	return csv.String()
}

// errorReader is a reader that always returns an error
type errorReader struct{}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, os.ErrPermission
}
