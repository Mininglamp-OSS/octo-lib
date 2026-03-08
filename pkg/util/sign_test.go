package util

import (
	"testing"
)

func TestGetSignStr(t *testing.T) {
	tests := []struct {
		name     string
		params   map[string]interface{}
		expected string
	}{
		{
			name: "multiple params sorted",
			params: map[string]interface{}{
				"c": "3",
				"a": "1",
				"b": "2",
			},
			expected: "a=1&b=2&c=3",
		},
		{
			name: "single param",
			params: map[string]interface{}{
				"key": "value",
			},
			expected: "key=value",
		},
		{
			name:     "empty params",
			params:   map[string]interface{}{},
			expected: "",
		},
		{
			name: "with integer values",
			params: map[string]interface{}{
				"count": 10,
				"name":  "test",
			},
			expected: "count=10&name=test",
		},
		{
			name: "skip empty string values",
			params: map[string]interface{}{
				"a": "1",
				"b": "",
				"c": "3",
			},
			expected: "a=1&c=3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSignStr(tt.params)
			if result != tt.expected {
				t.Errorf("GetSignStr() = %q, want %q", result, tt.expected)
			}
		})
	}
}
