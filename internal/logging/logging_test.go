package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo}, // default
		{"", slog.LevelInfo},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("stdout json", func(t *testing.T) {
		logger, err := New(Config{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if logger == nil {
			t.Fatal("New() returned nil logger")
		}
	})

	t.Run("stderr text", func(t *testing.T) {
		logger, err := New(Config{
			Level:  "debug",
			Format: "text",
			Output: "stderr",
		})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if logger == nil {
			t.Fatal("New() returned nil logger")
		}
	})

	t.Run("file output", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "test.log")

		logger, err := New(Config{
			Level:  "info",
			Format: "json",
			Output: logPath,
		})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if logger == nil {
			t.Fatal("New() returned nil logger")
		}

		// Log something.
		logger.Info("test message")

		// Verify file was created.
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Error("log file was not created")
		}
	})

	t.Run("nested directory", func(t *testing.T) {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "nested", "dir", "test.log")

		logger, err := New(Config{
			Level:  "info",
			Format: "json",
			Output: logPath,
		})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if logger == nil {
			t.Fatal("New() returned nil logger")
		}

		// Verify directory was created.
		if _, err := os.Stat(filepath.Dir(logPath)); os.IsNotExist(err) {
			t.Error("log directory was not created")
		}
	})
}

func TestNewStdout(t *testing.T) {
	logger := NewStdout("info")
	if logger == nil {
		t.Fatal("NewStdout() returned nil")
	}
}

func TestNewStderr(t *testing.T) {
	logger := NewStderr("warn")
	if logger == nil {
		t.Fatal("NewStderr() returned nil")
	}
}

func TestNewDiscard(t *testing.T) {
	logger := NewDiscard()
	if logger == nil {
		t.Fatal("NewDiscard() returned nil")
	}

	// Logging should not panic.
	logger.Info("this should be discarded")
	logger.Error("this too")
}

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected output to contain 'test message', got %q", output)
	}
	if !strings.Contains(output, "key") {
		t.Errorf("expected output to contain 'key', got %q", output)
	}
	if !strings.Contains(output, "value") {
		t.Errorf("expected output to contain 'value', got %q", output)
	}
}
