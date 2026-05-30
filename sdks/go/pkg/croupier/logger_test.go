package croupier

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestNewDefaultLogger(t *testing.T) {
	t.Parallel()

	logger := NewDefaultLogger(true, nil)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.debug != true {
		t.Error("expected debug to be true")
	}
}

func TestNewDefaultLoggerWithNilWriter(t *testing.T) {
	t.Parallel()

	logger := NewDefaultLogger(false, nil)
	if logger.out == nil {
		t.Error("expected default stdout writer")
	}
}

func TestNewDefaultLoggerWithCustomWriter(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewDefaultLogger(true, &buf)

	logger.Infof("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected log message in output, got: %s", buf.String())
	}
}

func TestDefaultLogger_Debugf(t *testing.T) {
	t.Parallel()

	t.Run("writes when debug is true", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := NewDefaultLogger(true, &buf)

		logger.Debugf("debug %s", "message")
		output := buf.String()

		if !strings.Contains(output, "[DEBUG]") {
			t.Error("expected [DEBUG] prefix")
		}
		if !strings.Contains(output, "debug message") {
			t.Error("expected debug message")
		}
	})

	t.Run("does not write when debug is false", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		logger := NewDefaultLogger(false, &buf)

		logger.Debugf("debug %s", "message")
		output := buf.String()

		if output != "" {
			t.Errorf("expected empty output, got: %s", output)
		}
	})
}

func TestDefaultLogger_Infof(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewDefaultLogger(true, &buf)

	logger.Infof("info %s", "message")
	output := buf.String()

	if !strings.Contains(output, "[INFO]") {
		t.Error("expected [INFO] prefix")
	}
	if !strings.Contains(output, "info message") {
		t.Error("expected info message")
	}
}

func TestDefaultLogger_Warnf(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewDefaultLogger(true, &buf)

	logger.Warnf("warning %s", "message")
	output := buf.String()

	if !strings.Contains(output, "[WARN]") {
		t.Error("expected [WARN] prefix")
	}
	if !strings.Contains(output, "warning message") {
		t.Error("expected warning message")
	}
}

func TestDefaultLogger_Errorf(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewDefaultLogger(true, &buf)

	logger.Errorf("error %s", "message")
	output := buf.String()

	if !strings.Contains(output, "[ERROR]") {
		t.Error("expected [ERROR] prefix")
	}
	if !strings.Contains(output, "error message") {
		t.Error("expected error message")
	}
}

func TestNoOpLogger(t *testing.T) {
	t.Parallel()

	var logger NoOpLogger

	// These should not panic
	logger.Debugf("debug")
	logger.Infof("info")
	logger.Warnf("warn")
	logger.Errorf("error")
}

func TestSetGlobalLogger(t *testing.T) {
	// Note: This test cannot be parallel because it modifies global state.
	// atomic.Value requires consistent types, so we cannot run this in parallel
	// with other tests that might set different logger types.

	// Save original logger
	original := GetGlobalLogger()
	defer SetGlobalLogger(original)

	customLogger := &NoOpLogger{}
	SetGlobalLogger(customLogger)

	retrieved := GetGlobalLogger()
	if retrieved != customLogger {
		t.Error("expected custom logger to be returned")
	}
}

func TestGetGlobalLogger(t *testing.T) {
	t.Parallel()

	logger := GetGlobalLogger()
	if logger == nil {
		t.Error("expected non-nil global logger")
	}
}

func TestLogHelpers(t *testing.T) {
	t.Parallel()

	t.Run("logDebugf", func(t *testing.T) {
		t.Parallel()

		// Should not panic
		logDebugf("test %s", "debug")
	})

	t.Run("logInfof", func(t *testing.T) {
		t.Parallel()

		// Should not panic
		logInfof("test %s", "info")
	})

	t.Run("logWarnf", func(t *testing.T) {
		t.Parallel()

		// Should not panic
		logWarnf("test %s", "warn")
	})

	t.Run("logErrorf", func(t *testing.T) {
		t.Parallel()

		// Should not panic
		logErrorf("test %s", "error")
	})
}

func TestNoOpLogger_AllMethods(t *testing.T) {
	t.Parallel()

	var logger NoOpLogger

	// Test all methods to ensure they don't panic and are proper no-ops
	logger.Debugf("debug message %d", 123)
	logger.Infof("info message %s", "test")
	logger.Warnf("warn message %v", map[string]string{"key": "value"})
	logger.Errorf("error message %f", 3.14)

	// If we got here without panicking, the test passes
}

func TestDefaultLogger_AllLevelsWithoutDebug(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewDefaultLogger(false, &buf)

	// Debug messages should not appear when debug is false
	logger.Debugf("should not appear")
	if buf.String() != "" {
		t.Error("expected no output for debug when debug=false")
	}

	// But other levels should appear
	logger.Infof("info message")
	if !strings.Contains(buf.String(), "[INFO]") {
		t.Error("expected INFO log")
	}

	logger.Warnf("warn message")
	if !strings.Contains(buf.String(), "[WARN]") {
		t.Error("expected WARN log")
	}

	logger.Errorf("error message")
	if !strings.Contains(buf.String(), "[ERROR]") {
		t.Error("expected ERROR log")
	}
}

func TestDefaultLogger_FormatString(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewDefaultLogger(true, &buf)

	// Test various format strings
	logger.Debugf("debug: %s %d %v", "test", 42, true)
	logger.Infof("info: %.2f", 3.14159)
	logger.Warnf("warn: %#v", map[string]int{"a": 1})
	logger.Errorf("error: %v", []string{"x", "y"})

	output := buf.String()

	// Verify all log levels are present
	expectedPrefixes := []string{"[DEBUG]", "[INFO]", "[WARN]", "[ERROR]"}
	for _, prefix := range expectedPrefixes {
		if !strings.Contains(output, prefix) {
			t.Errorf("expected %s prefix in output", prefix)
		}
	}

	// Verify formatted content
	if !strings.Contains(output, "debug: test 42 true") {
		t.Error("expected debug formatted message")
	}
	if !strings.Contains(output, "info: 3.14") {
		t.Error("expected info formatted message")
	}
}

func TestDefaultLogger_MultipleMessages(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewDefaultLogger(true, &buf)

	// Write multiple messages
	for i := 0; i < 5; i++ {
		logger.Infof("message %d", i)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}

	// Verify each message is present
	for i := 0; i < 5; i++ {
		expected := fmt.Sprintf("message %d", i)
		if !strings.Contains(output, expected) {
			t.Errorf("expected message %q not found", expected)
		}
	}
}
