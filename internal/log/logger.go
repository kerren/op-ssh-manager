package log

import (
	"io"
	"log/slog"
	"os"
)

var (
	logger  *slog.Logger
	verbose bool
)

// Init initializes the logger with the given verbosity level
func Init(v bool) {
	verbose = v

	var handler slog.Handler
	opts := &slog.HandlerOptions{}

	if verbose {
		opts.Level = slog.LevelDebug
	} else {
		opts.Level = slog.LevelInfo
	}

	handler = slog.NewTextHandler(os.Stderr, opts)
	logger = slog.New(handler)
}

// InitWithWriter initializes the logger with a custom writer (useful for testing)
func InitWithWriter(w io.Writer, v bool) {
	verbose = v

	opts := &slog.HandlerOptions{}
	if verbose {
		opts.Level = slog.LevelDebug
	} else {
		opts.Level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(w, opts)
	logger = slog.New(handler)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	if logger == nil {
		Init(false)
	}
	logger.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	if logger == nil {
		Init(false)
	}
	logger.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	if logger == nil {
		Init(false)
	}
	logger.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	if logger == nil {
		Init(false)
	}
	logger.Error(msg, args...)
}

// With returns a logger with additional context
func With(args ...any) *slog.Logger {
	if logger == nil {
		Init(false)
	}
	return logger.With(args...)
}

// IsVerbose returns whether verbose logging is enabled
func IsVerbose() bool {
	return verbose
}
