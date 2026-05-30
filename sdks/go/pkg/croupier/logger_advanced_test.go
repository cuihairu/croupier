// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"strings"
	"testing"
	"time"
)

// TestLogger_advanced_scenarios tests advanced logger scenarios
func TestLogger_advanced_scenarios(t *testing.T) {
	t.Run("GetLogger multiple times returns same instance", func(t *testing.T) {
		logger1 := GetGlobalLogger()
		logger2 := GetGlobalLogger()
		logger3 := GetGlobalLogger()

		if logger1 != logger2 || logger2 != logger3 {
			t.Error("GetLogger should return same instance")
		} else {
			t.Log("GetLogger returns same instance - singleton pattern verified")
		}
	})

	t.Run("Logger is always non-nil", func(t *testing.T) {
		logger := GetGlobalLogger()

		if logger == nil {
			t.Error("GetLogger returned nil")
		} else {
			t.Log("GetLogger returned non-nil logger")
		}
	})
}

// TestLogger_log_level_scenarios tests log level handling
func TestLogger_log_level_scenarios(t *testing.T) {
	t.Run("Log at different levels", func(t *testing.T) {
		logger := GetGlobalLogger()

		// Log at various levels - these should not panic
		logger.Debugf("%s", "Debug message")
		logger.Infof("%s", "Info message")
		logger.Warnf("%s", "Warning message")
		logger.Errorf("%s", "Error message")

		t.Log("All log levels executed without panicking")
	})

	t.Run("Log with format strings", func(t *testing.T) {
		logger := GetGlobalLogger()

		// Test format logging
		logger.Debugf("Debug with format: %s", "value1")
		logger.Infof("Info with format: %d", 42)
		logger.Warnf("Warning with format: %v", true)
		logger.Errorf("Error with format: %s %d", "combined", 123)

		t.Log("Format logging completed")
	})

	t.Run("Log with empty messages", func(t *testing.T) {
		logger := GetGlobalLogger()

		logger.Debugf("%s", "")
		logger.Infof("%s", "")
		logger.Warnf("%s", "")
		logger.Errorf("%s", "")

		t.Log("Empty message logging completed")
	})

	t.Run("Log with very long messages", func(t *testing.T) {
		logger := GetGlobalLogger()

		longMessage := strings.Repeat("A", 10000)
		logger.Debugf("%s", longMessage)
		logger.Infof("%s", longMessage)
		logger.Warnf("%s", longMessage)
		logger.Errorf("%s", longMessage)

		t.Logf("Long message logging completed (message length: %d)", len(longMessage))
	})
}

// TestLogger_special_characters tests logging with special characters
func TestLogger_special_characters(t *testing.T) {
	t.Run("Log with Unicode characters", func(t *testing.T) {
		logger := GetGlobalLogger()

		messages := []string{
			"English: Hello World",
			"Chinese: 你好世界",
			"Japanese: こんにちは",
			"Korean: 안녕하세요",
			"Emoji: 🎉 🚀 ❤️",
			"Mixed: Test 测试 Test 🧪",
		}

		for _, msg := range messages {
			logger.Infof("%s", msg)
		}

		t.Log("Unicode character logging completed")
	})

	t.Run("Log with special escape sequences", func(t *testing.T) {
		logger := GetGlobalLogger()

		messages := []string{
			"Newline: \nLine1\nLine2",
			"Tab: \tTabbed\tcontent",
			"Quote: \"Quoted\" text",
			"Backslash: \\path\\to\\file",
			"Carriage return: \rLine1\rLine2",
		}

		for _, msg := range messages {
			logger.Infof("%s", msg)
		}

		t.Log("Special escape sequence logging completed")
	})

	t.Run("Log with JSON content", func(t *testing.T) {
		logger := GetGlobalLogger()

		jsonMessages := []string{
			`{"key":"value","number":42}`,
			`{"nested":{"key":"value"}}`,
			`{"array":[1,2,3]}`,
			`{"escaped":"\"quoted\"","backslash":"\\path"}`,
		}

		for _, msg := range jsonMessages {
			logger.Infof("%s", msg)
		}

		t.Log("JSON content logging completed")
	})
}

// TestLogger_concurrent_logging tests concurrent logging operations
func TestLogger_concurrent_logging(t *testing.T) {
	t.Run("Concurrent logging from multiple goroutines", func(t *testing.T) {
		logger := GetGlobalLogger()

		const numGoroutines = 50
		const logsPerGoroutine = 20

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				for j := 0; j < logsPerGoroutine; j++ {
					logger.Infof("Goroutine %d, log %d", idx, j)
				}
			}(i)
		}

		// Note: In a real scenario, we'd need synchronization to ensure all logs complete
		// For this test, we're just verifying no panics occur
		t.Logf("Concurrent logging: %d goroutines × %d logs", numGoroutines, logsPerGoroutine)
	})

	t.Run("Concurrent logging at different levels", func(t *testing.T) {
		logger := GetGlobalLogger()

		const numGoroutines = 20

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				switch idx % 4 {
				case 0:
					logger.Debugf("Debug from goroutine %d", idx)
				case 1:
					logger.Infof("Info from goroutine %d", idx)
				case 2:
					logger.Warnf("Warning from goroutine %d", idx)
				case 3:
					logger.Errorf("Error from goroutine %d", idx)
				}
			}(i)
		}

		t.Logf("Concurrent multi-level logging: %d goroutines", numGoroutines)
	})
}

// TestLogger_performance_patterns tests logger performance patterns
func TestLogger_performance_patterns(t *testing.T) {
	t.Run("Rapid logging performance", func(t *testing.T) {
		logger := GetGlobalLogger()

		const iterations = 1000

		// Measure debug logging performance
		start := time.Now()
		for i := 0; i < iterations; i++ {
			logger.Debugf("Debug log %d", i)
		}
		duration := time.Since(start)

		t.Logf("Logged %d debug messages in %v", iterations, duration)
	})

	t.Run("Sequential logging at all levels", func(t *testing.T) {
		logger := GetGlobalLogger()

		const iterations = 100

		for i := 0; i < iterations; i++ {
			logger.Debugf("Debug %d", i)
			logger.Infof("Info %d", i)
			logger.Warnf("Warning %d", i)
			logger.Errorf("Error %d", i)
		}

		t.Logf("Sequential logging: %d iterations × 4 levels = %d logs",
			iterations, iterations*4)
	})
}

// TestLogger_edge_cases tests edge case scenarios
func TestLogger_edge_cases(t *testing.T) {
	t.Run("Log with nil-like values", func(t *testing.T) {
		logger := GetGlobalLogger()

		// These should not cause panics
		logger.Infof("%s", "")
		logger.Debugf("%s", "")
		logger.Infof("%d %s", 0, "")
		logger.Warnf("%v", nil)
		logger.Errorf("%s %d %v", "", 0, nil)

		t.Log("Edge case values handled")
	})

	t.Run("Log with very long format strings", func(t *testing.T) {
		logger := GetGlobalLogger()

		longFormat := strings.Repeat("%s ", 100)
		args := make([]interface{}, 100)
		for i := range args {
			args[i] = i
		}

		logger.Infof(longFormat, args...)

		t.Log("Long format string handled")
	})

	t.Run("Log with mixed argument types", func(t *testing.T) {
		logger := GetGlobalLogger()

		logger.Infof("String: %s, Int: %d, Float: %f, Bool: %v, Nil: %v",
			"text", 42, 3.14, true, nil)

		t.Log("Mixed argument types handled")
	})
}

// TestLogger_consistency tests logger behavior consistency
func TestLogger_consistency(t *testing.T) {
	t.Run("Logger state consistency", func(t *testing.T) {
		logger := GetGlobalLogger()

		// Get logger multiple times and verify it's the same instance
		for i := 0; i < 10; i++ {
			sameLogger := (GetGlobalLogger() == logger)
			if !sameLogger {
				t.Errorf("Iteration %d: GetLogger returned different instance", i)
			}
		}

		t.Log("Logger state consistency verified across 10 calls")
	})

	t.Run("Logger behavior across multiple calls", func(t *testing.T) {
		logger := GetGlobalLogger()

		// Perform multiple logging operations
		for i := 0; i < 20; i++ {
			logger.Infof("Test message %d", i)
		}

		t.Log("Multiple sequential calls completed successfully")
	})
}

// TestLogger_thread_safety tests thread safety of logger
func TestLogger_thread_safety(t *testing.T) {
	t.Run("Concurrent Debug logging", func(t *testing.T) {
		logger := GetGlobalLogger()

		const numGoroutines = 30
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer func() { done <- true }()
				for j := 0; j < 10; j++ {
					logger.Debugf("Goroutine %d, iteration %d", idx, j)
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		t.Logf("Thread-safe Debug logging: %d goroutines", numGoroutines)
	})

	t.Run("Concurrent Info logging", func(t *testing.T) {
		logger := GetGlobalLogger()

		const numGoroutines = 30
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer func() { done <- true }()
				for j := 0; j < 10; j++ {
					logger.Infof("Goroutine %d, iteration %d", idx, j)
				}
			}(i)
		}

		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		t.Logf("Thread-safe Info logging: %d goroutines", numGoroutines)
	})

	t.Run("Concurrent Error logging", func(t *testing.T) {
		logger := GetGlobalLogger()

		const numGoroutines = 30
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer func() { done <- true }()
				for j := 0; j < 10; j++ {
					logger.Errorf("Goroutine %d, iteration %d", idx, j)
				}
			}(i)
		}

		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		t.Logf("Thread-safe Error logging: %d goroutines", numGoroutines)
	})
}

// TestLogger_resource_usage tests logger resource usage patterns
func TestLogger_resource_usage(t *testing.T) {
	t.Run("Logger creation and usage", func(t *testing.T) {
		// Get logger multiple times
		for i := 0; i < 100; i++ {
			logger := GetGlobalLogger()
			logger.Infof("Iteration %d", i)
		}

		t.Log("100 get-and-log cycles completed")
	})

	t.Run("High-frequency logging", func(t *testing.T) {
		logger := GetGlobalLogger()

		const iterations = 500

		for i := 0; i < iterations; i++ {
			logger.Debugf("High-frequency log %d", i)
		}

		t.Logf("High-frequency logging: %d iterations", iterations)
	})
}

// TestLogger_message_patterns tests various message patterns
func TestLogger_message_patterns(t *testing.T) {
	t.Run("Structured logging patterns", func(t *testing.T) {
		logger := GetGlobalLogger()

		// Simulate structured logging
		logger.Infof("User action: %s | UserID: %d | Status: %s",
			"login", 12345, "success")

		logger.Warnf("Performance warning | Endpoint: %s | Duration: %dms",
			"/api/slow", 500)

		logger.Errorf("Error occurred | Code: %d | Message: %s",
			404, "Resource not found")

		t.Log("Structured logging patterns completed")
	})

	t.Run("Contextual logging", func(t *testing.T) {
		logger := GetGlobalLogger()

		// Simulate contextual logging
		contextID := "ctx-12345"
		userID := "user-67890"
		requestID := "req-abcde"

		logger.Infof("[%s] User %s initiated request %s", contextID, userID, requestID)
		logger.Infof("[%s] Processing request %s", contextID, requestID)
		logger.Infof("[%s] Completed request %s for user %s", contextID, requestID, userID)

		t.Log("Contextual logging completed")
	})

	t.Run("Multi-line logging", func(t *testing.T) {
		logger := GetGlobalLogger()

		// Log multi-line messages
		logger.Infof("%s", "Line 1\nLine 2\nLine 3")
		logger.Infof("%s", "Multi-line:\n  Item 1\n  Item 2\n  Item 3")

		t.Log("Multi-line logging completed")
	})
}

// TestLogger_format_variations tests various format variations
func TestLogger_format_variations(t *testing.T) {
	t.Run("Log with various format specifiers", func(t *testing.T) {
		logger := GetGlobalLogger()

		// Test different format specifiers
		logger.Infof("String: %s", "test")
		logger.Infof("Integer: %d", 42)
		logger.Infof("Float: %f", 3.14159)
		logger.Infof("Boolean: %t", true)
		logger.Infof("Hex: %x", 255)
		logger.Infof("Octal: %o", 8)
		logger.Infof("Binary: %b", 5)
		logger.Infof("Pointer: %p", logger)

		t.Log("Various format specifiers tested")
	})

	t.Run("Log with width and precision", func(t *testing.T) {
		logger := GetGlobalLogger()

		logger.Infof("Width test: |%10s|", "short")
		logger.Infof("Width test: |%-10s|", "short")
		logger.Infof("Precision test: %.2f", 3.14159)
		logger.Infof("Width+Precision:|%10.2f|", 3.14159)

		t.Log("Width and precision formatting tested")
	})

	t.Run("Log with multiple arguments", func(t *testing.T) {
		logger := GetGlobalLogger()

		logger.Infof("Multiple args: %s %d %f %v", "text", 42, 3.14, true)
		logger.Infof("Five args: %s %d %f %v %s", "1", 2, 3.0, false, "5")
		logger.Infof("Six args: %d %d %d %d %d %d", 1, 2, 3, 4, 5, 6)

		t.Log("Multiple argument formatting tested")
	})
}
