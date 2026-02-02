// Package logging provides a structured logging system based on zap.
// It supports configurable log levels and output formats (JSON/text).
package logging

import (
	"log"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	logger *zap.Logger
	sugar  *zap.SugaredLogger
)

// Config holds logging configuration.
type Config struct {
	Level  string // debug, info, warn, error
	Format string // json, text
}

func init() {
	// Initialize with a default logger (development mode)
	// This ensures logging works even if Init() is not called
	logger, _ = zap.NewDevelopment()
	sugar = logger.Sugar()
}

// Init initializes the logging system with the given configuration.
// It should be called early in the application startup.
func Init(cfg *Config) error {
	level := parseLevel(cfg.Level)
	encoder := createEncoder(cfg.Format)

	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	logger = zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1), // Skip the wrapper functions
	)
	sugar = logger.Sugar()

	// Redirect standard library log output to zap (for third-party libraries like go-fuse)
	// This outputs at WARN level since most stdlib log calls are warnings/errors
	redirectStdLog()

	return nil
}

// stdLogWriter implements io.Writer to redirect standard log output to zap.
type stdLogWriter struct{}

func (w *stdLogWriter) Write(p []byte) (n int, err error) {
	// Remove trailing newline if present
	msg := strings.TrimSuffix(string(p), "\n")
	// Remove timestamp prefix if present (e.g., "2006/01/02 15:04:05 ")
	if len(msg) > 20 && msg[4] == '/' && msg[7] == '/' && msg[10] == ' ' {
		msg = msg[20:]
	}
	// Log at WARN level with source marker
	sugar.Warnw(msg, "source", "stdlib")
	return len(p), nil
}

// redirectStdLog redirects standard library log output to zap.
func redirectStdLog() {
	log.SetFlags(0) // Remove default timestamp (we'll use zap's)
	log.SetOutput(&stdLogWriter{})
}

// parseLevel converts a string level to zapcore.Level.
func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// createEncoder creates the appropriate encoder based on format.
func createEncoder(format string) zapcore.Encoder {
	var encoderConfig zapcore.EncoderConfig

	if strings.ToLower(format) == "json" {
		// Production JSON format
		encoderConfig = zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}
		return zapcore.NewJSONEncoder(encoderConfig)
	}

	// Development text format with colors
	encoderConfig = zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006/01/02 15:04:05"),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// Sync flushes any buffered log entries.
// Should be called before the application exits.
func Sync() error {
	return logger.Sync()
}

// L returns the underlying zap.Logger for advanced usage.
func L() *zap.Logger {
	return logger
}

// S returns the underlying zap.SugaredLogger for advanced usage.
func S() *zap.SugaredLogger {
	return sugar
}

// =============================================================================
// Structured logging functions (with zap.Field)
// =============================================================================

// Debug logs a message at DebugLevel with structured fields.
func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

// Info logs a message at InfoLevel with structured fields.
func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

// Warn logs a message at WarnLevel with structured fields.
func Warn(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

// Error logs a message at ErrorLevel with structured fields.
func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

// Fatal logs a message at FatalLevel with structured fields, then calls os.Exit(1).
func Fatal(msg string, fields ...zap.Field) {
	logger.Fatal(msg, fields...)
}

// =============================================================================
// Printf-style logging functions (for easier migration from log/fmt)
// =============================================================================

// Debugf logs a formatted message at DebugLevel.
func Debugf(template string, args ...interface{}) {
	sugar.Debugf(template, args...)
}

// Infof logs a formatted message at InfoLevel.
func Infof(template string, args ...interface{}) {
	sugar.Infof(template, args...)
}

// Warnf logs a formatted message at WarnLevel.
func Warnf(template string, args ...interface{}) {
	sugar.Warnf(template, args...)
}

// Errorf logs a formatted message at ErrorLevel.
func Errorf(template string, args ...interface{}) {
	sugar.Errorf(template, args...)
}

// Fatalf logs a formatted message at FatalLevel, then calls os.Exit(1).
func Fatalf(template string, args ...interface{}) {
	sugar.Fatalf(template, args...)
}

// =============================================================================
// Helper functions for common field types
// =============================================================================

// String creates a string field.
func String(key, value string) zap.Field {
	return zap.String(key, value)
}

// Int creates an int field.
func Int(key string, value int) zap.Field {
	return zap.Int(key, value)
}

// Int64 creates an int64 field.
func Int64(key string, value int64) zap.Field {
	return zap.Int64(key, value)
}

// Err creates an error field with key "error".
func Err(err error) zap.Field {
	return zap.Error(err)
}

// Any creates a field with any value (uses reflection).
func Any(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

// Duration creates a duration field.
func Duration(key string, value interface{}) zap.Field {
	switch v := value.(type) {
	case int64:
		return zap.Int64(key+"_ms", v)
	default:
		return zap.Any(key, value)
	}
}
