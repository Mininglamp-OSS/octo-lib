package log

import (
	"errors"
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
	const ls = "\u2028" // LINE SEPARATOR, bytes E2 80 A8
	const ps = "\u2029" // PARAGRAPH SEPARATOR, bytes E2 80 A9
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
		{"unicode preserved around lf", "unicode café \n ok", "unicode café  ok"},
		{"u2028 replaced with space", "a" + ls + "b", "a b"},
		{"u2029 replaced with space", "a" + ps + "b", "a b"},
		{"u2028 and u2029 mixed", "x" + ls + "y" + ps + "z", "x y z"},
		{"all vectors combined", "a\r\nb\tc\x00d" + ls + "e" + ps + "f", "abcd e f"},
		{"mixed", "x\r\n\t\x00" + ls + ps + "y", "x  y"},
		{"invalid utf-8 preserved", "a\xffb\xfec", "a\xffb\xfec"},
		{"invalid utf-8 mixed with vectors", "a\xff\nb\xfe\tc", "a\xffb\xfec"},
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

// TestWrappersSanitizeMessageAndFields is the anti-fake-pass / mutation check for
// the wrapper wiring. It captures emitted entries through an in-memory zap core
// and asserts both message and string-typed fields are sanitized.
//
// Mutation canary: this test is the safety-net for the wrapper sanitization
// wiring across ALL four package-level wrappers. The PR description promises
// that removing SanitizeForLog from ANY of Info/Debug/Error/Warn must turn this
// test red. An earlier version only exercised Info, leaving Debug/Error/Warn
// silently uncovered (caught by reviewer 李飞飞 on PR #93). Do NOT collapse this
// back to a single-wrapper test without re-proving the universal-coverage
// property.
//
// Reproduction (for each wrapper W in {Info, Debug, Error, Warn}):
//  1. Stub: replace `current().<W>Logger.<W>(SanitizeForLog(msg), sanitizeFields(fields)...)`
//     with `current().<W>Logger.<W>(msg, fields...)` in pkg/log/logger.go
//  2. Run: go test ./pkg/log/ -count=1 -run TestWrappersSanitizeMessageAndFields
//  3. Must FAIL on subtest TestWrappersSanitizeMessageAndFields/W. If PASS, the
//     canary for wrapper W is broken; fix the test BEFORE merging any change
//     that touches the wrappers.
func TestWrappersSanitizeMessageAndFields(t *testing.T) {
	// Save/restore the published logger set once around all subtests so the
	// process-global state is restored after the table finishes.
	prev := loggers.Load()
	t.Cleanup(func() {
		if prev == nil {
			loggers.Store(nil)
		} else {
			loggers.Store(prev)
		}
	})

	wrappers := []struct {
		name string
		fn   func(string, ...zap.Field)
	}{
		{"Info", Info},
		{"Debug", Debug},
		{"Error", Error},
		{"Warn", Warn},
	}
	for _, w := range wrappers {
		t.Run(w.name, func(t *testing.T) {
			// Fresh memoryCore + loggerSet per subtest so wrappers cannot
			// contaminate each other's captured entries.
			var captured atomic.Pointer[[]capturedEntry]
			empty := []capturedEntry{}
			captured.Store(&empty)
			core := &memoryCore{LevelEnabler: zap.DebugLevel, entries: &captured}
			memLogger := zap.New(core)
			loggers.Store(&loggerSet{
				infoLogger:  memLogger,
				errorLogger: memLogger,
				warnLogger:  memLogger,
				testLogger:  memLogger,
			})

			w.fn("msg with\nnewline", zap.String("k", "v\nwith\nnewlines"), zap.Int("n", 1))

			got := captured.Load()
			if len(*got) != 1 {
				t.Fatalf("wrapper %s: expected 1 captured entry, got %d", w.name, len(*got))
			}
			entry := (*got)[0]
			if strings.ContainsAny(entry.msg, "\r\n\t") {
				t.Errorf("wrapper %s: msg not sanitized: %q", w.name, entry.msg)
			}
			if entry.msg != "msg withnewline" {
				t.Errorf("wrapper %s: unexpected sanitized msg: %q", w.name, entry.msg)
			}

			var found bool
			for _, f := range entry.fields {
				if f.Key == "k" {
					found = true
					if f.Type != zapcore.StringType {
						t.Errorf("wrapper %s: field k unexpectedly non-string: %v", w.name, f.Type)
					}
					if strings.ContainsAny(f.String, "\r\n\t") {
						t.Errorf("wrapper %s: field k not sanitized: %q", w.name, f.String)
					}
					if f.String != "vwithnewlines" {
						t.Errorf("wrapper %s: unexpected sanitized field value: %q", w.name, f.String)
					}
				}
				if f.Key == "n" && f.Integer != 1 {
					t.Errorf("wrapper %s: non-string field n mutated: %d", w.name, f.Integer)
				}
			}
			if !found {
				t.Errorf("wrapper %s: expected field k not found", w.name)
			}
		})
	}
}

// Reverse assertion: SanitizeForLog must only sanitize StringType fields.
// zap.Error(err) uses ErrorType field and must pass through unchanged — this
// proves the layering is clean and we're not accidentally stripping data from
// structured error fields (which would lose stack traces, wrapped error
// chains, etc.).
func TestSanitizeFieldsLeavesErrorFieldUnchanged(t *testing.T) {
	var captured atomic.Pointer[[]capturedEntry]
	empty := []capturedEntry{}
	captured.Store(&empty)
	core := &memoryCore{LevelEnabler: zap.DebugLevel, entries: &captured}

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

	err := errors.New("error with\nnewline in message")
	Error("test msg", zap.Error(err))

	got := captured.Load()
	if len(*got) != 1 {
		t.Fatalf("expected 1 captured entry, got %d", len(*got))
	}
	entry := (*got)[0]

	var found bool
	for _, f := range entry.fields {
		if f.Key != "error" {
			continue
		}
		found = true
		if f.Type != zapcore.ErrorType {
			t.Errorf("error field unexpectedly non-error type: %v", f.Type)
		}
		got, ok := f.Interface.(error)
		if !ok {
			t.Fatalf("error field Interface not error: %T", f.Interface)
		}
		if !strings.Contains(got.Error(), "\n") {
			t.Errorf("error field message was sanitized: %q (expected to contain \\n)", got.Error())
		}
	}
	if !found {
		t.Fatal("did not see error field in captured entry")
	}
}
