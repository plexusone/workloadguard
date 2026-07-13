// Package logging provides logging utilities for workloadguard.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// ParseLevel parses a log level string into slog.Level.
func ParseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Config holds logger configuration.
type Config struct {
	Level  string
	Format string // "json" or "text"
	Output string // path or "stdout", "stderr"
}

// New creates a new logger from configuration.
func New(cfg Config) (*slog.Logger, error) {
	level := ParseLevel(cfg.Level)

	var writer io.Writer
	switch cfg.Output {
	case "", "stdout":
		writer = os.Stdout
	case "stderr":
		writer = os.Stderr
	default:
		// Ensure directory exists.
		dir := filepath.Dir(cfg.Output)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}

		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		writer = f
	}

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch cfg.Format {
	case "text":
		handler = slog.NewTextHandler(writer, opts)
	default:
		handler = slog.NewJSONHandler(writer, opts)
	}

	return slog.New(handler), nil
}

// NewStdout creates a JSON logger writing to stdout.
func NewStdout(level string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: ParseLevel(level),
	})
	return slog.New(handler)
}

// NewStderr creates a JSON logger writing to stderr.
func NewStderr(level string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: ParseLevel(level),
	})
	return slog.New(handler)
}

// NewDiscard creates a logger that discards all output.
func NewDiscard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
