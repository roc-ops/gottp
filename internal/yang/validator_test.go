package yang

import (
	"strings"
	"testing"
)

const testLeafListYANG = `
module test-leaflist {
    namespace "urn:test:leaflist";
    prefix tll;

    container test-data {
        leaf-list string-list {
            type string;
        }
        leaf-list decimal-list {
            type decimal64 {
                fraction-digits 1;
            }
        }
        leaf regular-leaf {
            type string;
        }
        leaf-list int-list {
            type uint32;
        }
    }
}
`

func setupValidator(t *testing.T) *Validator {
	t.Helper()

	ms := NewModuleSet()
	if err := ms.AddModule("test-leaflist", testLeafListYANG); err != nil {
		t.Fatalf("Failed to add YANG module: %v", err)
	}
	if err := ms.ProcessAllModules(); err != nil {
		t.Fatalf("Failed to process YANG modules: %v", err)
	}

	return NewValidator(ms)
}

func TestLeafListStringValidation(t *testing.T) {
	v := setupValidator(t)

	data := map[string]interface{}{
		"string-list": []interface{}{"a", "b", "c"},
	}

	result := v.Validate(data, "test-leaflist:test-data", "test")
	if !result.Valid {
		t.Errorf("Expected valid result, got errors: %v", formatErrors(result))
	}
}

func TestLeafListStringInvalidElement(t *testing.T) {
	v := setupValidator(t)

	data := map[string]interface{}{
		"string-list": []interface{}{"a", 123, "c"},
	}

	result := v.Validate(data, "test-leaflist:test-data", "test")
	if result.Valid {
		t.Error("Expected invalid result for non-string element in string leaf-list")
	}

	// Check that the error references element [1]
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "element [1]") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error referencing element [1], got: %v", formatErrors(result))
	}
}

func TestLeafListDecimal64WithFloats(t *testing.T) {
	v := setupValidator(t)

	data := map[string]interface{}{
		"decimal-list": []interface{}{1.5, 2.3, 3.7},
	}

	result := v.Validate(data, "test-leaflist:test-data", "test")
	if !result.Valid {
		t.Errorf("Expected valid result for float64 values in decimal64 leaf-list, got errors: %v", formatErrors(result))
	}
}

func TestLeafListDecimal64WithIntegers(t *testing.T) {
	v := setupValidator(t)

	data := map[string]interface{}{
		"decimal-list": []interface{}{52, 52, 52},
	}

	result := v.Validate(data, "test-leaflist:test-data", "test")
	if !result.Valid {
		t.Errorf("Expected valid result for integer values in decimal64 leaf-list, got errors: %v", formatErrors(result))
	}
}

func TestLeafListDecimal64Mixed(t *testing.T) {
	v := setupValidator(t)

	data := map[string]interface{}{
		"decimal-list": []interface{}{20.2, 20.2, 17},
	}

	result := v.Validate(data, "test-leaflist:test-data", "test")
	if !result.Valid {
		t.Errorf("Expected valid result for mixed int/float values in decimal64 leaf-list, got errors: %v", formatErrors(result))
	}
}

func TestRegularLeafStillWorks(t *testing.T) {
	v := setupValidator(t)

	data := map[string]interface{}{
		"regular-leaf": "hello",
	}

	result := v.Validate(data, "test-leaflist:test-data", "test")
	if !result.Valid {
		t.Errorf("Expected valid result for regular string leaf, got errors: %v", formatErrors(result))
	}
}

func TestRegularLeafInvalidType(t *testing.T) {
	v := setupValidator(t)

	data := map[string]interface{}{
		"regular-leaf": 123,
	}

	result := v.Validate(data, "test-leaflist:test-data", "test")
	if result.Valid {
		t.Error("Expected invalid result for integer value in string leaf")
	}

	// Verify the error mentions the field name
	found := false
	for _, err := range result.Errors {
		if strings.Contains(err.Message, "regular-leaf") && strings.Contains(err.Message, "invalid type") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected type error for 'regular-leaf', got: %v", formatErrors(result))
	}
}

func formatErrors(result *ValidationResult) string {
	var msgs []string
	for _, err := range result.Errors {
		msgs = append(msgs, err.Message)
	}
	return strings.Join(msgs, "; ")
}
