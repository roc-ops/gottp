package test

import (
	"testing"

	"github.com/roc-ops/gottp/internal/functions/match"
)

// TestUptimeparseDirect tests uptimeparse function directly
func TestUptimeparseDirect(t *testing.T) {
	registry := match.NewRegistry()
	
	// Test uptimeparse with seconds format
	fn, ok := registry.Get("uptimeparse")
	if !ok {
		t.Fatal("uptimeparse function not found")
	}
	
	// Test case 1: "27 weeks, 3 days, 10 hours, 46 minutes"
	result, err := fn("27 weeks, 3 days, 10 hours, 46 minutes", []string{}, nil)
	if err != nil {
		t.Fatalf("uptimeparse failed: %v", err)
	}
	
	// Should return integer seconds
	seconds, ok := result.(int)
	if !ok {
		t.Fatalf("Expected integer seconds, got %T: %v", result, result)
	}
	
	if seconds <= 0 {
		t.Errorf("Expected positive seconds, got %d", seconds)
	}
	
	t.Logf("Parsed uptime to %d seconds", seconds)
	
	// Test case 2: dict format
	result2, err := fn("27 weeks, 3 days, 10 hours, 46 minutes, 10 seconds", []string{"dict"}, nil)
	if err != nil {
		t.Fatalf("uptimeparse failed: %v", err)
	}
	
	// Should return dictionary
	dict, ok := result2.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected dictionary, got %T: %v", result2, result2)
	}
	
	if _, hasWeeks := dict["weeks"]; !hasWeeks {
		t.Error("Expected 'weeks' in dictionary")
	}
	if _, hasDays := dict["days"]; !hasDays {
		t.Error("Expected 'days' in dictionary")
	}
	
	t.Logf("Parsed uptime to dict: %v", dict)
}

