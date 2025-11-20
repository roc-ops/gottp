package match

import (
	"testing"
)

func TestContains(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("contains")
	if !ok {
		t.Fatal("contains function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    *ConditionResult
		wantErr bool
	}{
		{
			name:    "contains substring",
			value:   "hello world",
			args:    []string{"world"},
			want:    &ConditionResult{Condition: true, Value: "hello world"},
			wantErr: false,
		},
		{
			name:    "does not contain",
			value:   "hello world",
			args:    []string{"universe"},
			want:    &ConditionResult{Condition: false, Value: "hello world"},
			wantErr: false,
		},
		{
			name:    "multiple patterns - first matches",
			value:   "hello world",
			args:    []string{"world", "test"},
			want:    &ConditionResult{Condition: true, Value: "hello world"},
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    &ConditionResult{Condition: true, Value: "test"}, // No args means condition is true
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("contains() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want.Condition {
						t.Errorf("contains() condition = %v, want %v", condResult.Condition, tt.want.Condition)
					}
				} else {
					t.Errorf("contains() result type = %T, want *ConditionResult", result)
				}
			}
		})
	}
}

func TestContainsRe(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("contains_re")
	if !ok {
		t.Fatal("contains_re function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "matches regex",
			value:   "hello123world",
			args:    []string{"\\d+"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "does not match regex",
			value:   "hello world",
			args:    []string{"\\d+"},
			want:    false,
			wantErr: false,
		},
		{
			name:    "invalid regex",
			value:   "test",
			args:    []string{"[invalid"},
			want:    false,
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("contains_re() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("contains_re() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestStartswith(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("startswith")
	if !ok {
		t.Fatal("startswith function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "starts with prefix",
			value:   "hello world",
			args:    []string{"hello"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "does not start with prefix",
			value:   "hello world",
			args:    []string{"world"},
			want:    false,
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    true, // No args means condition is true
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("startswith() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("startswith() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestEndswith(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("endswith")
	if !ok {
		t.Fatal("endswith function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "ends with suffix",
			value:   "hello world",
			args:    []string{"world"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "does not end with suffix",
			value:   "hello world",
			args:    []string{"hello"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("endswith() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("endswith() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestEqual(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("equal")
	if !ok {
		t.Fatal("equal function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "equal values",
			value:   "test",
			args:    []string{"test"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "not equal values",
			value:   "test",
			args:    []string{"other"},
			want:    false,
			wantErr: false,
		},
		{
			name:    "no args",
			value:   "test",
			args:    nil,
			want:    true, // No args means condition is true
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("equal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("equal() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestNotEqual(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("notequal")
	if !ok {
		t.Fatal("notequal function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "not equal values",
			value:   "test",
			args:    []string{"other"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "equal values",
			value:   "test",
			args:    []string{"test"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("notequal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("notequal() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestExclude(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("exclude")
	if !ok {
		t.Fatal("exclude function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "does not contain",
			value:   "hello world",
			args:    []string{"universe"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "contains",
			value:   "hello world",
			args:    []string{"world"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("exclude() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("exclude() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestExcludeRe(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("exclude_re")
	if !ok {
		t.Fatal("exclude_re function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "does not match regex",
			value:   "hello world",
			args:    []string{"\\d+"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "matches regex",
			value:   "hello123world",
			args:    []string{"\\d+"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("exclude_re() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("exclude_re() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestNotStartswith(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("notstartswith")
	if !ok {
		t.Fatal("notstartswith function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "does not start with",
			value:   "hello world",
			args:    []string{"world"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "starts with",
			value:   "hello world",
			args:    []string{"hello"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("notstartswith() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("notstartswith() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestNotEndswith(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("notendswith")
	if !ok {
		t.Fatal("notendswith function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "does not end with",
			value:   "hello world",
			args:    []string{"hello"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "ends with",
			value:   "hello world",
			args:    []string{"world"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("notendswith() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("notendswith() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestStartswithRe(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("startswith_re")
	if !ok {
		t.Fatal("startswith_re function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "matches regex at start",
			value:   "123hello",
			args:    []string{"\\d+"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "does not match regex at start",
			value:   "hello123",
			args:    []string{"\\d+"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("startswith_re() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("startswith_re() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestEndswithRe(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("endswith_re")
	if !ok {
		t.Fatal("endswith_re function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "matches regex at end",
			value:   "hello123",
			args:    []string{"\\d+"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "does not match regex at end",
			value:   "123hello",
			args:    []string{"\\d+"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("endswith_re() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("endswith_re() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestNotStartswithRe(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("notstartswith_re")
	if !ok {
		t.Fatal("notstartswith_re function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "does not match regex at start",
			value:   "hello123",
			args:    []string{"\\d+"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "matches regex at start",
			value:   "123hello",
			args:    []string{"\\d+"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("notstartswith_re() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("notstartswith_re() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

func TestNotEndswithRe(t *testing.T) {
	registry := NewRegistry()
	registry.RegisterConditionFunctions()
	
	fn, ok := registry.Get("notendswith_re")
	if !ok {
		t.Fatal("notendswith_re function not found")
	}
	
	tests := []struct {
		name    string
		value   interface{}
		args    []string
		want    bool
		wantErr bool
	}{
		{
			name:    "does not match regex at end",
			value:   "123hello",
			args:    []string{"\\d+"},
			want:    true,
			wantErr: false,
		},
		{
			name:    "matches regex at end",
			value:   "hello123",
			args:    []string{"\\d+"},
			want:    false,
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fn(tt.value, tt.args, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("notendswith_re() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if condResult, ok := result.(*ConditionResult); ok {
					if condResult.Condition != tt.want {
						t.Errorf("notendswith_re() condition = %v, want %v", condResult.Condition, tt.want)
					}
				}
			}
		})
	}
}

