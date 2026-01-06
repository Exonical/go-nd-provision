package common

import (
	"strings"
	"testing"
)

func TestRequireNonEmpty_Valid(t *testing.T) {
	err := RequireNonEmpty("fieldName", "some value")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestRequireNonEmpty_Empty(t *testing.T) {
	err := RequireNonEmpty("fieldName", "")
	if err == nil {
		t.Fatal("expected error for empty value")
	}
	if !strings.Contains(err.Error(), "fieldName") {
		t.Errorf("expected error to contain field name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "is required") {
		t.Errorf("expected 'is required' in error, got: %v", err)
	}
}

func TestRequireNonEmpty_Whitespace(t *testing.T) {
	// RequireNonEmpty trims whitespace - whitespace-only is considered empty
	err := RequireNonEmpty("fieldName", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only value")
	}
	if !strings.Contains(err.Error(), "is required") {
		t.Errorf("expected 'is required' in error, got: %v", err)
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		nvPairs  map[string]interface{}
		key      string
		expected string
	}{
		{"string value", map[string]interface{}{"key": "value"}, "key", "value"},
		{"missing key", map[string]interface{}{"other": "value"}, "key", ""},
		{"nil map", nil, "key", ""},
		{"non-string value", map[string]interface{}{"key": 123}, "key", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetString(tt.nvPairs, tt.key)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name     string
		nvPairs  map[string]interface{}
		key      string
		expected bool
	}{
		{"bool true", map[string]interface{}{"key": true}, "key", true},
		{"bool false", map[string]interface{}{"key": false}, "key", false},
		{"string true", map[string]interface{}{"key": "true"}, "key", true},
		{"string True", map[string]interface{}{"key": "True"}, "key", true},
		{"string 1", map[string]interface{}{"key": "1"}, "key", true},
		{"string false", map[string]interface{}{"key": "false"}, "key", false},
		{"missing key", map[string]interface{}{"other": true}, "key", false},
		{"nil map", nil, "key", false},
		{"non-bool value", map[string]interface{}{"key": 123}, "key", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBool(tt.nvPairs, tt.key)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name     string
		nvPairs  map[string]interface{}
		key      string
		expected int
	}{
		{"int value", map[string]interface{}{"key": 42}, "key", 42},
		{"float64 value", map[string]interface{}{"key": float64(42)}, "key", 42},
		{"int64 value", map[string]interface{}{"key": int64(42)}, "key", 42},
		{"missing key", map[string]interface{}{"other": 42}, "key", 0},
		{"nil map", nil, "key", 0},
		{"string value", map[string]interface{}{"key": "42"}, "key", 0}, // strings not supported
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetInt(tt.nvPairs, tt.key)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
