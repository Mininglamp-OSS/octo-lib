package log

import (
	"strings"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSanitizeForLog(t *testing.T) {
	// Strategy (must stay in sync with SanitizeForLog's doc comment):
	//   - strip \r \n \t \x00 (ASCII control bytes — adjacent tokens collapse)
	//   - replace U+2028 / U+2029 with a single space (multi-byte Unicode line
	//     separators — replacement avoids running words together)
	const ls = " " // LINE SEPARATOR, bytes E2 80 A8
	const ps = " " // PARAGRAPH SEPARATOR, bytes E2 80 A9
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain unchanged", "plain", "plain"},
		{"empty", "", ""},
		{"cr alone stripped", "a\rb", "ab"},
		{"lf alone stripped", "a\nb", "ab"},
		{"tab alone stripped", "a\tb", "ab"},
		{"crlf stripped", "a\r\nb", "ab"},
		{"nul stripped", "a\x00b", "ab"},
		{"all ascii controls", "\r\n\t\x00", ""},
		{"mixed ascii controls", "mixed\r\nvalue\twith\nall\x00", "mixedvaluewithall"},
		{"unicode preserved around lf", "unicode 中文 \n ok", "unicode 中文  ok"},
		{"u2028 replaced with space", "a" + ls + "b", "a b"},
		{"u2029 replaced with space", "a" + ps + "b", "a b"},
		{"u2028 and u2029 mixed", "x" + ls + "y" + ps + "z", "x y z"},
		{"all vectors combined", "a\r\nb\tc\x00d" + ls + "e" + ps + "f", "abcd e f"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeForLog(tc.in); got != tc.want {
				t.Errorf("SanitizeForLog(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
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

// TestWrappersSanitizeMessageAndFields is the anti-假绿 / mutation check for
// the wrapper wiring. It captures emitted entries through an in-memory zap core
// and asserts both message and string-typed fields are sanitized.
//
// Mutation check: if you remove the SanitizeForLog call in Info/Debug/Error/Warn,
// or the sanitizeFields call alongside it, this test must turn red. If you make
// that change and this test stays green, the test is broken — fix the test, not
// the production code.
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
