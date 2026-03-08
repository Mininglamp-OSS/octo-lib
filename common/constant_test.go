package common

import (
	"strings"
	"testing"
)

func TestCMDMessageEraseSpelling(t *testing.T) {
	// Ensure CMDMessageErase value does not contain double 'e' typo
	if strings.Contains(CMDMessageErase, "Eerase") || strings.Contains(CMDMessageErase, "eerase") {
		t.Errorf("CMDMessageErase contains typo: got %q, want %q", CMDMessageErase, "messageErase")
	}

	// Ensure the value is exactly what we expect
	expected := "messageErase"
	if CMDMessageErase != expected {
		t.Errorf("CMDMessageErase = %q, want %q", CMDMessageErase, expected)
	}
}
