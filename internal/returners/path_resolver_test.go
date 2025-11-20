package returners

import (
	"testing"
)

func TestTraversePath_ComplexPaths(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		path    []string
		want    interface{}
		wantErr bool
	}{
		{
			name:    "simple path",
			data:    map[string]interface{}{"key": "value"},
			path:    []string{"key"},
			want:    "value",
			wantErr: false,
		},
		{
			name:    "nested path",
			data:    map[string]interface{}{"outer": map[string]interface{}{"inner": "value"}},
			path:    []string{"outer", "inner"},
			want:    "value",
			wantErr: false,
		},
		{
			name:    "deeply nested path",
			data:    map[string]interface{}{"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{"d": "value"}}}},
			path:    []string{"a", "b", "c", "d"},
			want:    "value",
			wantErr: false,
		},
		{
			name:    "path to list index",
			data:    map[string]interface{}{"items": []interface{}{"item0", "item1", "item2"}},
			path:    []string{"items", "0"},
			want:    "item0",
			wantErr: false,
		},
		{
			name:    "path to nested list",
			data:    map[string]interface{}{"data": []interface{}{map[string]interface{}{"key": "value"}}},
			path:    []string{"data", "0", "key"},
			want:    "value",
			wantErr: false,
		},
		{
			name:    "non-existent path",
			data:    map[string]interface{}{"key": "value"},
			path:    []string{"nonexistent"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "path through non-map value",
			data:    map[string]interface{}{"key": "value"},
			path:    []string{"key", "nested"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty path",
			data:    map[string]interface{}{"key": "value"},
			path:    []string{},
			wantErr: false,
		},
		{
			name:    "path with empty keys",
			data:    map[string]interface{}{"": map[string]interface{}{"inner": "value"}},
			path:    []string{"", "inner"},
			want:    "value",
			wantErr: false,
		},
		{
			name:    "path to nil value",
			data:    map[string]interface{}{"key": nil},
			path:    []string{"key"},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := traversePath(tt.data, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("traversePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if tt.name == "empty path" {
					// For empty path, result should be the original data
					if result == nil && tt.data != nil {
						t.Errorf("traversePath() = nil, expected original data")
					}
				} else if result != tt.want {
					t.Errorf("traversePath() = %v, want %v", result, tt.want)
				}
			}
		})
	}
}

func TestTraversePath_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    interface{}
		path    []string
		wantErr bool
	}{
		{
			name:    "nil data",
			data:    nil,
			path:    []string{"key"},
			wantErr: true,
		},
		{
			name:    "non-map data",
			data:    "string",
			path:    []string{"key"},
			wantErr: true,
		},
		{
			name:    "list data",
			data:    []interface{}{"item1", "item2"},
			path:    []string{"0"},
			wantErr: true, // traversePath expects map at root
		},
		{
			name:    "very long path",
			data:    createDeepNestedMap(100),
			path:    createLongPath(100),
			wantErr: false,
		},
		{
			name:    "path with numeric string keys",
			data:    map[string]interface{}{"0": "zero", "1": "one"},
			path:    []string{"0"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := traversePath(tt.data, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("traversePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.name == "path with numeric string keys" {
				// Check if we got a value
				if result == nil {
					t.Errorf("traversePath() = nil, expected a value")
				}
			}
		})
	}
}

// Helper functions

func createDeepNestedMap(depth int) map[string]interface{} {
	if depth == 0 {
		return map[string]interface{}{"value": "leaf"}
	}
	return map[string]interface{}{"level": createDeepNestedMap(depth - 1)}
}

func createLongPath(length int) []string {
	path := make([]string, length)
	for i := 0; i < length; i++ {
		path[i] = "level"
	}
	return path
}

