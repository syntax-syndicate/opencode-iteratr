package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
		wantErr  bool
	}{
		{"debug", LevelDebug, false},
		{"DEBUG", LevelDebug, false},
		{"info", LevelInfo, false},
		{"INFO", LevelInfo, false},
		{"warn", LevelWarn, false},
		{"WARN", LevelWarn, false},
		{"error", LevelError, false},
		{"ERROR", LevelError, false},
		{"invalid", LevelInfo, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseLevel(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for input %q", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("expected %v, got %v for input %q", tt.expected, got, tt.input)
			}
		})
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New()
	l.SetOutput(&buf)

	// Set to WARN level
	l.SetLevel(LevelWarn)

	// Debug and Info should not be logged
	l.Debug("debug message")
	l.Info("info message")

	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Error("debug message should not be logged at WARN level")
	}
	if strings.Contains(output, "info message") {
		t.Error("info message should not be logged at WARN level")
	}

	// Warn and Error should be logged
	l.Warn("warn message")
	l.Error("error message")

	output = buf.String()
	if !strings.Contains(output, "warn message") {
		t.Error("warn message should be logged at WARN level")
	}
	if !strings.Contains(output, "error message") {
		t.Error("error message should be logged at WARN level")
	}
}

func TestLogger_LogFormat(t *testing.T) {
	var buf bytes.Buffer
	l := New()
	l.SetOutput(&buf)
	l.SetLevel(LevelDebug)

	l.Info("test message with %s", "formatting")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Error("log output should contain level prefix")
	}
	if !strings.Contains(output, "test message with formatting") {
		t.Error("log output should contain formatted message")
	}
}

func TestLogger_EnvVarLogLevel(t *testing.T) {
	// Save and restore original env var
	original := os.Getenv("ITERATR_LOG_LEVEL")
	defer func() {
		if original != "" {
			os.Setenv("ITERATR_LOG_LEVEL", original)
		} else {
			os.Unsetenv("ITERATR_LOG_LEVEL")
		}
	}()

	os.Setenv("ITERATR_LOG_LEVEL", "debug")

	l := New()
	if l.level != LevelDebug {
		t.Errorf("expected debug level from env var, got %v", l.level)
	}
}

func TestLogger_EnvVarLogFile(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "iteratr-test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Save and restore original env var
	original := os.Getenv("ITERATR_LOG_FILE")
	defer func() {
		if original != "" {
			os.Setenv("ITERATR_LOG_FILE", original)
		} else {
			os.Unsetenv("ITERATR_LOG_FILE")
		}
	}()

	os.Setenv("ITERATR_LOG_FILE", tmpPath)

	l := New()
	defer l.Close()

	l.Info("test message")

	// Read the log file
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Error("log file should contain the test message")
	}
}

func TestLogger_Close(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "iteratr-test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	original := os.Getenv("ITERATR_LOG_FILE")
	defer func() {
		if original != "" {
			os.Setenv("ITERATR_LOG_FILE", original)
		} else {
			os.Unsetenv("ITERATR_LOG_FILE")
		}
	}()

	os.Setenv("ITERATR_LOG_FILE", tmpPath)

	l := New()
	if err := l.Close(); err != nil {
		t.Errorf("unexpected error closing logger: %v", err)
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	var buf bytes.Buffer
	Default.SetOutput(&buf)
	Default.SetLevel(LevelDebug)

	Debug("debug %s", "test")
	Info("info %s", "test")
	Warn("warn %s", "test")
	Error("error %s", "test")

	output := buf.String()
	if !strings.Contains(output, "debug test") {
		t.Error("output should contain debug message")
	}
	if !strings.Contains(output, "info test") {
		t.Error("output should contain info message")
	}
	if !strings.Contains(output, "warn test") {
		t.Error("output should contain warn message")
	}
	if !strings.Contains(output, "error test") {
		t.Error("output should contain error message")
	}
}
