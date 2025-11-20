package match

import (
	"testing"
)

func TestConditionFunctions_ErrorHandling(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()

	tests := []struct {
		name    string
		fnName  string
		value   interface{}
		args    []string
		wantErr bool
	}{
		{
			name:    "contains_re with invalid regex",
			fnName:  "contains_re",
			value:   "test",
			args:    []string{"[invalid"},
			wantErr: true,
		},
		{
			name:    "startswith_re with invalid regex",
			fnName:  "startswith_re",
			value:   "test",
			args:    []string{"[invalid"},
			wantErr: true,
		},
		{
			name:    "endswith_re with invalid regex",
			fnName:  "endswith_re",
			value:   "test",
			args:    []string{"[invalid"},
			wantErr: true,
		},
		{
			name:    "exclude_re with invalid regex",
			fnName:  "exclude_re",
			value:   "test",
			args:    []string{"[invalid"},
			wantErr: true,
		},
		{
			name:    "notstartswith_re with invalid regex",
			fnName:  "notstartswith_re",
			value:   "test",
			args:    []string{"[invalid"},
			wantErr: true,
		},
		{
			name:    "notendswith_re with invalid regex",
			fnName:  "notendswith_re",
			value:   "test",
			args:    []string{"[invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := registry.Get(tt.fnName)
			if !ok {
				t.Fatalf("Function %s not found", tt.fnName)
			}

			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s() error = %v, wantErr %v", tt.fnName, err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					t.Logf("%s() condition = %v", tt.fnName, condResult.Condition)
				}
			}
		})
	}
}

func TestConditionFunctions_EdgeCases(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()

	tests := []struct {
		name   string
		fnName string
		value  interface{}
		args   []string
	}{
		{
			name:   "contains with empty string",
			fnName: "contains",
			value:  "",
			args:   []string{"pattern"},
		},
		{
			name:   "contains with empty pattern",
			fnName: "contains",
			value:  "test",
			args:   []string{""},
		},
		{
			name:   "startswith with empty string",
			fnName: "startswith",
			value:  "",
			args:   []string{"prefix"},
		},
		{
			name:   "startswith with empty prefix",
			fnName: "startswith",
			value:  "test",
			args:   []string{""},
		},
		{
			name:   "endswith with empty string",
			fnName: "endswith",
			value:  "",
			args:   []string{"suffix"},
		},
		{
			name:   "endswith with empty suffix",
			fnName: "endswith",
			value:  "test",
			args:   []string{""},
		},
		{
			name:   "equal with nil value",
			fnName: "equal",
			value:  nil,
			args:   []string{"test"},
		},
		{
			name:   "equal with empty string",
			fnName: "equal",
			value:  "",
			args:   []string{""},
		},
		{
			name:   "notequal with nil value",
			fnName: "notequal",
			value:  nil,
			args:   []string{"test"},
		},
		{
			name:   "exclude with empty string",
			fnName: "exclude",
			value:  "",
			args:   []string{"pattern"},
		},
		{
			name:   "contains_re with empty pattern",
			fnName: "contains_re",
			value:  "test",
			args:   []string{""},
		},
		{
			name:   "contains_re with very long pattern",
			fnName: "contains_re",
			value:  "test",
			args:   []string{"a{1000}"}, // Very long pattern
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := registry.Get(tt.fnName)
			if !ok {
				t.Fatalf("Function %s not found", tt.fnName)
			}

			result, err := fn(tt.value, tt.args, nil)
			if err != nil {
				t.Logf("%s() error = %v (may be expected)", tt.fnName, err)
				return
			}
			if condResult, ok := result.(*ConditionResult); ok {
				t.Logf("%s() condition = %v", tt.fnName, condResult.Condition)
			}
		})
	}
}

func TestConditionFunctions_TypeMismatches(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()

	tests := []struct {
		name   string
		fnName string
		value  interface{}
		args   []string
	}{
		{
			name:   "contains with integer",
			fnName: "contains",
			value:  123,
			args:   []string{"12"},
		},
		{
			name:   "contains with float",
			fnName: "contains",
			value:  3.14,
			args:   []string{"3"},
		},
		{
			name:   "contains with boolean",
			fnName: "contains",
			value:  true,
			args:   []string{"true"},
		},
		{
			name:   "contains with map",
			fnName: "contains",
			value:  map[string]interface{}{"key": "value"},
			args:   []string{"key"},
		},
		{
			name:   "contains with list",
			fnName: "contains",
			value:  []interface{}{"item1", "item2"},
			args:   []string{"item1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, ok := registry.Get(tt.fnName)
			if !ok {
				t.Fatalf("Function %s not found", tt.fnName)
			}

			result, err := fn(tt.value, tt.args, nil)
			if err != nil {
				t.Logf("%s() error = %v", tt.fnName, err)
				return
			}
			if condResult, ok := result.(*ConditionResult); ok {
				t.Logf("%s() condition = %v", tt.fnName, condResult.Condition)
			}
		})
	}
}

