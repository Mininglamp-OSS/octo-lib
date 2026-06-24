package log

import (
	"strings"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSanitizeForLog(t *testing.T) {
	// Strategy: strip \r \n \t entirely (no replacement char) so adjacent tokens
	// collapse rather than introduce misleading whitespace separators.
	cases := []struct {
		in   string
		want string
	}{
		{"hello\nworld", "helloworld"},
		{"\r\n\t", ""},
		{"plain", "plain"},
		{"mixed\r\nvalue\twith\nall", "mixedvaluewithall"},
		{"", ""},
		{"unicode 中文 \n ok", "unicode 中文  ok"},
	}
	for _, tc := range cases {
		if got := SanitizeForLog(tc.in); got != tc.want {
			t.Errorf("SanitizeForLog(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// memoryCore captures emitted entries so wrapper sanitization can be asserted.
type memoryCore struct {
	zapcore.LevelEnabler
	entries *atomic.Pointer[[]capturedEntry]
}

type capturedEntry struct {
	msg    string
	fields []zapcore.Field
}

func (c *memoryCore) With(_ []zapcore.Field) zapcore.Core { return c }

func (c *memoryCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *memoryCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	for {
		cur := c.entries.Load()
		next := append([]capturedEntry{}, (*cur)...)
		next = append(next, capturedEntry{msg: ent.Message, fields: fields})
		if c.entries.CompareAndSwap(cur, &next) {
			return nil
		}
	}
}

func (c *memoryCore) Sync() error { return nil }

func TestWrappersSanitizeMessageAndFields(t *testing.T) {
	var captured atomic.Pointer[[]capturedEntry]
	empty := []capturedEntry{}
	captured.Store(&empty)
	core := &memoryCore{LevelEnabler: zap.DebugLevel, entries: &captured}

	// Swap in a logger backed by the memory core for the duration of this test.
	// Restore whatever was published (or absent) afterwards.
	prev := loggers.Load()
	t.Cleanup(func() {
		if prev == nil {
			loggers.Store(nil)
		} else {
			loggers.Store(prev)
		}
	})
	memLogger := zap.New(core)
	loggers.Store(&loggerSet{
		infoLogger:  memLogger,
		errorLogger: memLogger,
		warnLogger:  memLogger,
		testLogger:  memLogger,
	})

	Info("msg with\nnewline", zap.String("k", "v\nwith\nnewlines"), zap.Int("n", 1))

	got := captured.Load()
	if len(*got) != 1 {
		t.Fatalf("expected 1 captured entry, got %d", len(*got))
	}
	entry := (*got)[0]
	if strings.ContainsAny(entry.msg, "\r\n\t") {
		t.Errorf("msg not sanitized: %q", entry.msg)
	}
	if entry.msg != "msg withnewline" {
		t.Errorf("unexpected sanitized msg: %q", entry.msg)
	}

	var found bool
	for _, f := range entry.fields {
		if f.Key == "k" {
			found = true
			if f.Type != zapcore.StringType {
				t.Errorf("field k unexpectedly non-string: %v", f.Type)
			}
			if strings.ContainsAny(f.String, "\r\n\t") {
				t.Errorf("field k not sanitized: %q", f.String)
			}
			if f.String != "vwithnewlines" {
				t.Errorf("unexpected sanitized field value: %q", f.String)
			}
		}
		if f.Key == "n" && f.Integer != 1 {
			t.Errorf("non-string field n mutated: %d", f.Integer)
		}
	}
	if !found {
		t.Fatal("did not see field k in captured entry")
	}
}
