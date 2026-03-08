package util

import (
	"errors"
	"testing"
)

func TestMustNoErr_NilError(t *testing.T) {
	// Should not panic when err is nil
	MustNoErr(nil)
}

func TestMustNoErr_NonNilError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustNoErr should panic on non-nil error")
		}
	}()
	MustNoErr(errors.New("test error"))
}

func TestSubstr(t *testing.T) {
	tests := []struct {
		str    string
		start  int
		length int
		want   string
	}{
		{"hello", 0, 2, "he"},
		{"hello", 1, 3, "ell"},
		{"hello", 0, 0, ""},
		{"hello", -2, 2, "lo"},
		{"hello", 0, 10, "hello"},
	}

	for _, tt := range tests {
		got := Substr(tt.str, tt.start, tt.length)
		if got != tt.want {
			t.Errorf("Substr(%q, %d, %d) = %q, want %q", tt.str, tt.start, tt.length, got, tt.want)
		}
	}
}
