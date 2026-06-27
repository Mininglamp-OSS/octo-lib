package log

import (
	"testing"

	"go.uber.org/zap"
)

func TestLogger(t *testing.T) {
	opts := NewOptions()
	opts.Level = zap.DebugLevel
	opts.LineNum = true
	Configure(opts)

	Info("this is info")
	Debug("this is debug")
	Error("this is error", zap.String("key", "value"))
}

func TestSanitizeForLog(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no injection", "normal message", "normal message"},
		{"newline injection", "user\nadmin=true", "user\\nadmin=true"},
		{"carriage return", "user\r\nadmin=true", "user\\r\\nadmin=true"},
		{"tab", "user\tinjected", "user\\tinjected"},
		{"mixed", "a\nb\rc\td", "a\\nb\\rc\\td"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeForLog(tt.in)
			if got != tt.want {
				t.Errorf("SanitizeForLog(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
