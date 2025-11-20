package input

import (
	"strings"
	"testing"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader()
	
	if loader == nil {
		t.Fatal("NewLoader() returned nil")
	}
}

func TestLoader_LoadText(t *testing.T) {
	loader := NewLoader()
	
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name:    "simple text",
			data:    "test data",
			want:    "test data",
			wantErr: false,
		},
		{
			name:    "empty text",
			data:    "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "multiline text",
			data:    "line1\nline2\nline3",
			want:    "line1\nline2\nline3",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.LoadText(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if resultStr, ok := result.(string); ok {
					if resultStr != tt.want {
						t.Errorf("LoadText() = %v, want %v", resultStr, tt.want)
					}
				} else {
					t.Errorf("LoadText() result type = %T, want string", result)
				}
			}
		})
	}
}

func TestLoader_LoadYAML(t *testing.T) {
	loader := NewLoader()
	
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "valid YAML",
			data:    "key: value\nnested:\n  inner: data",
			wantErr: false,
		},
		{
			name:    "invalid YAML",
			data:    "key: value\n  invalid: indentation",
			wantErr: true,
		},
		{
			name:    "empty YAML",
			data:    "",
			wantErr: false,
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

func TestLoader_LoadJSON(t *testing.T) {
	loader := NewLoader()
	
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "valid JSON",
			data:    `{"key": "value", "nested": {"inner": "data"}}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			data:    `{"key": "value", invalid}`,
			wantErr: true,
		},
		{
			name:    "empty JSON",
			data:    "",
			wantErr: true, // Empty string is not valid JSON
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

func TestLoader_LoadCSV(t *testing.T) {
	loader := NewLoader()
	
	tests := []struct {
		name      string
		data      string
		keyColumn string
		wantErr   bool
	}{
		{
			name:      "valid CSV",
			data:      "name,age\nJohn,30\nJane,25",
			keyColumn: "",
			wantErr:   false,
		},
		{
			name:      "CSV with key column",
			data:      "name,age\nJohn,30\nJane,25",
			keyColumn: "name",
			wantErr:   false,
		},
		{
			name:      "empty CSV",
			data:      "",
			keyColumn: "",
			wantErr:   false,
		},
		{
			name:      "CSV with headers only",
			data:      "name,age",
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
				t.Logf("LoadCSV() result = %v", result)
			}
		})
	}
}

func TestLoader_LoadFromReader(t *testing.T) {
	loader := NewLoader()
	
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name:    "simple text",
			data:    "test data",
			want:    "test data",
			wantErr: false,
		},
		{
			name:    "empty text",
			data:    "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "multiline text",
			data:    "line1\nline2\nline3",
			want:    "line1\nline2\nline3",
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.data)
			result, err := loader.LoadFromReader(reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadFromReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if result != tt.want {
					t.Errorf("LoadFromReader() = %v, want %v", result, tt.want)
				}
			}
		})
	}
}

