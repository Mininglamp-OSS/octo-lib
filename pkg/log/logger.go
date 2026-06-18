package log

import (
	"fmt"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// loggerSet bundles the configured loggers so they can be published as a single
// unit. Readers load the whole set through an atomic.Pointer, so they observe
// either a fully-built set or none at all — never a half-initialized one. This
// mirrors the boxed atomic.Pointer pattern already used in pkg/wkhttp.
type loggerSet struct {
	infoLogger  *zap.Logger
	errorLogger *zap.Logger
	warnLogger  *zap.Logger
	testLogger  *zap.Logger
}

// The package loggers are configured lazily on first use (or eagerly via
// Configure). configMu serializes (re)configuration so concurrent first-time
// callers cannot race on the lazy init, while loggers is read lock-free on the
// hot logging path once configured. atom is shared by the cores and is itself
// goroutine-safe.
var (
	configMu sync.Mutex
	loggers  atomic.Pointer[loggerSet]
	atom     = zap.NewAtomicLevel()
)

// Configure (re)configures the package loggers. It is safe to call concurrently
// and alongside lazy first-use initialization.
func Configure(opts *Options) {
	configMu.Lock()
	defer configMu.Unlock()
	configureLocked(opts)
}

// configureLocked builds the loggers, publishes them as one set, and returns it.
// Callers must hold configMu.
func configureLocked(opts *Options) *loggerSet {
	atom.SetLevel(opts.Level)

	stdoutCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(newEncoderConfig()),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)),
		atom,
	)
	var test *zap.Logger
	if opts.LineNum {
		test = zap.New(stdoutCore, zap.AddCaller())
	} else {
		test = zap.New(stdoutCore)
	}

	infoWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path.Join(opts.LogDir, "info.log"),
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	})
	infoCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(newEncoderConfig()),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(infoWriter)),
		atom,
	)
	var info *zap.Logger
	if opts.LineNum {
		info = zap.New(infoCore, zap.AddCaller())
	} else {
		info = zap.New(infoCore)
	}

	errorWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path.Join(opts.LogDir, "error.log"),
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	})
	errorCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(newEncoderConfig()),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(errorWriter)),
		zap.ErrorLevel,
	)
	var errLogger *zap.Logger
	if opts.LineNum {
		errLogger = zap.New(errorCore, zap.AddCaller())
	} else {
		errLogger = zap.New(errorCore)
	}

	warnWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path.Join(opts.LogDir, "warn.log"),
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	})
	warnCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(newEncoderConfig()),
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout), zapcore.AddSync(warnWriter)),
		zap.WarnLevel,
	)
	var warn *zap.Logger
	if opts.LineNum {
		warn = zap.New(warnCore, zap.AddCaller())
	} else {
		warn = zap.New(warnCore)
	}

	set := &loggerSet{
		infoLogger:  info,
		errorLogger: errLogger,
		warnLogger:  warn,
		testLogger:  test,
	}
	loggers.Store(set)
	return set
}

// current returns the published logger set, lazily configuring it with default
// options on first use. Because the set is published as a single atomic pointer,
// the returned value always has every logger built — there is no window where one
// logger is set and another is still nil. The lock-free fast path keeps the hot
// logging path free of contention once configured.
func current() *loggerSet {
	if ls := loggers.Load(); ls != nil {
		return ls
	}
	configMu.Lock()
	defer configMu.Unlock()
	if ls := loggers.Load(); ls != nil { // another goroutine configured while we waited
		return ls
	}
	return configureLocked(NewOptions())
}

func newEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:       "time",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "linenum",
		MessageKey:    "msg",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.LowercaseLevelEncoder, // 小写编码器
		EncodeCaller:  zapcore.FullCallerEncoder,     // 全路径编码器
		EncodeName:    zapcore.FullNameEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format("2006-01-02 15:04:05"))
		},
		EncodeDuration: func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendInt64(int64(d) / 1000000)
		},
	}
}
func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

// Info Info
func Info(msg string, fields ...zap.Field) {
	current().infoLogger.Info(msg, fields...)
}

// Debug Debug
func Debug(msg string, fields ...zap.Field) {
	current().infoLogger.Debug(msg, fields...)
}

// Error Error
func Error(msg string, fields ...zap.Field) {
	current().errorLogger.Error(msg, fields...)
}

// Warn Warn
func Warn(msg string, fields ...zap.Field) {
	current().warnLogger.Warn(msg, fields...)
}

// Log Log
type Log interface {
	Info(msg string, fields ...zap.Field)
	Debug(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
}

// LIMLog TLog
type TLog struct {
	prefix string // 日志前缀
}

// NewLIMLog NewLIMLog
func NewTLog(prefix string) *TLog {

	return &TLog{prefix: prefix}
}

// Info Info
func (t *TLog) Info(msg string, fields ...zap.Field) {
	Info(fmt.Sprintf("【%s】%s", t.prefix, msg), fields...)
}

// Debug Debug
func (t *TLog) Debug(msg string, fields ...zap.Field) {
	Debug(fmt.Sprintf("【%s】%s", t.prefix, msg), fields...)
}

// Error Error
func (t *TLog) Error(msg string, fields ...zap.Field) {
	Error(fmt.Sprintf("【%s】%s", t.prefix, msg), fields...)
}

// Warn Warn
func (t *TLog) Warn(msg string, fields ...zap.Field) {
	Warn(fmt.Sprintf("【%s】%s", t.prefix, msg), fields...)
}
