package util

import (
	"testing"
)

func TestGetIPAddress_EmptyAPIKey(t *testing.T) {
	_, _, err := GetIPAddress("8.8.8.8", "")
	if err == nil {
		t.Error("expected error when apiKey is empty, got nil")
	}
	if err.Error() != "apiKey is required" {
		t.Errorf("expected error message 'apiKey is required', got '%s'", err.Error())
	}
}

func TestGetIPAddress_WithAPIKey(t *testing.T) {
	// This test verifies that the function accepts an API key parameter
	// We use a dummy key that will fail the actual API call,
	// but this confirms the function signature change works correctly
	_, _, err := GetIPAddress("8.8.8.8", "test-api-key")
	// We expect an error from the API (invalid key), not from our validation
	if err != nil && err.Error() == "apiKey is required" {
		t.Error("function incorrectly rejected a non-empty API key")
	}
}

func TestIsIntranet(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"192.168.1.1", true},
		{"192.168.0.100", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.15.0.1", false},
		{"172.32.0.1", false},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			result := IsIntranet(tt.ip)
			if result != tt.expected {
				t.Errorf("IsIntranet(%s) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}
