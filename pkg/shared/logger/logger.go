// Package logger provides structured, leveled logging shared across all
// Vesper components. Uses uber-go/zap under the hood.
package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps zap.Logger with framework-specific context.
type Logger struct {
	*zap.SugaredLogger
	component string
}

// Config for logger initialization.
type Config struct {
	Level     string // debug, info, warn, error
	Format    string // text, json
	Output    string // stdout, file
	File      string // file path if output=file
	Component string // component name for field tagging
}

// New creates a new Logger with the given configuration.
func New(cfg Config) (*Logger, error) {
	level := parseLevel(cfg.Level)
	encoder := buildEncoder(cfg.Format)
	writer := buildWriter(cfg)
	core := zapcore.NewCore(encoder, writer, level)

	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	sugared := zapLogger.Sugar()

	if cfg.Component != "" {
		sugared = sugared.With("component", cfg.Component)
	}

	return &Logger{
		SugaredLogger: sugared,
		component:     cfg.Component,
	}, nil
}

// NewDefault returns a logger with sensible defaults for quick setup.
func NewDefault(component string) *Logger {
	logger, _ := New(Config{
		Level:     "info",
		Format:    "text",
		Output:    "stdout",
		Component: component,
	})
	return logger
}

// NewSilent returns a logger that discards all output (for testing).
func NewSilent() *Logger {
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(zapcore.NewMultiWriteSyncer()),
		zapcore.FatalLevel,
	)
	logger := zap.New(core)
	return &Logger{SugaredLogger: logger.Sugar()}
}

// With adds structured context fields to the logger.
func (l *Logger) With(args ...interface{}) *Logger {
	return &Logger{
		SugaredLogger: l.SugaredLogger.With(args...),
		component:     l.component,
	}
}

// Named returns a child logger with the given name appended.
func (l *Logger) Named(name string) *Logger {
	return &Logger{
		SugaredLogger: l.SugaredLogger.Named(name),
		component:     l.component,
	}
}

// Sync flushes any buffered log entries.
func (l *Logger) Sync() {
	_ = l.SugaredLogger.Sync()
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

func buildEncoder(format string) zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	if format == "json" {
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		return zapcore.NewJSONEncoder(encoderConfig)
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func buildWriter(cfg Config) zapcore.WriteSyncer {
	switch cfg.Output {
	case "file":
		if cfg.File == "" {
			fmt.Fprintf(os.Stderr, "logger: output=file but no file path set, falling back to stdout\n")
			return zapcore.AddSync(os.Stdout)
		}
		lj := &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    100, // MB
			MaxBackups: 5,
			MaxAge:     30, // days
			Compress:   true,
		}
		return zapcore.AddSync(lj)
	default:
		return zapcore.AddSync(os.Stdout)
	}
}

// SensitiveString wraps a sensitive value and redacts it in logs.
// Use to prevent keys, tokens, passwords from being written to disk or stdout.
type SensitiveString string

func (s SensitiveString) String() string {
	if s == "" {
		return ""
	}
	return "***REDACTED***"
}

func (s SensitiveString) Raw() string {
	return string(s)
}

func (s SensitiveString) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("value", "***REDACTED***")
	return nil
}
