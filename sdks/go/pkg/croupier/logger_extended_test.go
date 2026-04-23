// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

import (
	"io"
	"testing"
)

// TestNoOpLogger_extended tests extended NoOp logger functionality
func TestNoOpLogger_extended(t *testing.T) {
	t.Run("Debugf calls", func(t *testing.T) {
		logger := &NoOpLogger{}

		// Multiple Debugf calls
		for i := 0; i < 10; i++ {
			logger.Debugf("Debug message %d", i)
		}

		t.Log("10 Debugf calls completed")
	})

	t.Run("Infof calls", func(t *testing.T) {
		logger := &NoOpLogger{}

		for i := 0; i < 10; i++ {
			logger.Infof("Info message %d", i)
		}

		t.Log("10 Infof calls completed")
	})

	t.Run("Warnf calls", func(t *testing.T) {
		logger := &NoOpLogger{}

		for i := 0; i < 10; i++ {
			logger.Warnf("Warning message %d", i)
		}

		t.Log("10 Warnf calls completed")
	})

	t.Run("Errorf calls", func(t *testing.T) {
		logger := &NoOpLogger{}

		for i := 0; i < 10; i++ {
			logger.Errorf("Error message %d", i)
		}

		t.Log("10 Errorf calls completed")
	})

	t.Run("mixed log calls", func(t *testing.T) {
		logger := &NoOpLogger{}

		logger.Debugf("Debug: %s", "test")
		logger.Infof("Info: %d", 42)
		logger.Warnf("Warn: %v", true)
		logger.Errorf("Error: %f", 3.14)

		t.Log("Mixed log calls completed")
	})

	t.Run("empty format strings", func(t *testing.T) {
		logger := &NoOpLogger{}

		logger.Debugf("")
		logger.Infof("")
		logger.Warnf("")
		logger.Errorf("")

		t.Log("Empty format strings completed")
	})

	t.Run("complex format strings", func(t *testing.T) {
		logger := &NoOpLogger{}

		logger.Debugf("Multiple: %s %d %v %f", "test", 42, true, 3.14)
		logger.Infof("Struct: %+v", struct{ Name string }{"test"})
		logger.Warnf("Array: %v", []int{1, 2, 3})
		logger.Errorf("Map: %v", map[string]int{"key": 42})

		t.Log("Complex format strings completed")
	})

	t.Run("special characters", func(t *testing.T) {
		logger := &NoOpLogger{}

		logger.Debugf("Special: \n\t\r")
		logger.Infof("Unicode: 世界")
		logger.Warnf("Emoji: 😀")
		logger.Errorf("Mixed: %s", "测试\x00")

		t.Log("Special characters completed")
	})
}

// TestDefaultLogger_extended tests extended default logger functionality
func TestDefaultLogger_extended(t *testing.T) {
	t.Run("default logger with debug enabled", func(t *testing.T) {
		logger := NewDefaultLogger(true, io.Discard)

		logger.Debugf("Debug message: %s", "test")
		logger.Infof("Info message: %d", 42)
		logger.Warnf("Warning: %v", true)
		logger.Errorf("Error: %f", 3.14)

		t.Log("Default logger with debug enabled completed")
	})

	t.Run("default logger with debug disabled", func(t *testing.T) {
		logger := NewDefaultLogger(false, io.Discard)

		logger.Debugf("This should not appear: %s", "test")
		logger.Infof("Info: %d", 42)
		logger.Warnf("Warning: %v", true)
		logger.Errorf("Error: %f", 3.14)

		t.Log("Default logger with debug disabled completed")
	})

	t.Run("default logger with nil writer", func(t *testing.T) {
		logger := NewDefaultLogger(true, nil)

		logger.Debugf("Debug with nil writer: %s", "test")
		logger.Infof("Info with nil writer: %d", 42)
		logger.Warnf("Warning with nil writer: %v", true)
		logger.Errorf("Error with nil writer: %f", 3.14)

		t.Log("Default logger with nil writer completed")
	})

	t.Run("multiple default loggers", func(t *testing.T) {
		loggers := []Logger{
			NewDefaultLogger(true, io.Discard),
			NewDefaultLogger(false, io.Discard),
			NewDefaultLogger(true, nil),
		}

		for i, logger := range loggers {
			logger.Infof("Logger %d: test", i)
		}

		t.Log("Multiple default loggers completed")
	})
}

// TestGlobalLogger_operations tests global logger operations
func TestGlobalLogger_operations(t *testing.T) {
	t.Run("set and get NoOp logger", func(t *testing.T) {
		SetGlobalLogger(&NoOpLogger{})

		logger := GetGlobalLogger()
		if logger == nil {
			t.Error("GetGlobalLogger should not return nil")
		}

		logger.Infof("Test message")
	})

	t.Run("set and get default logger", func(t *testing.T) {
		SetGlobalLogger(NewDefaultLogger(false, io.Discard))

		logger := GetGlobalLogger()
		if logger == nil {
			t.Error("GetGlobalLogger should not return nil")
		}

		logger.Infof("Test message")
	})

	t.Run("multiple set operations", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			SetGlobalLogger(&NoOpLogger{})
			logger := GetGlobalLogger()
			if logger == nil {
				t.Errorf("Iteration %d: GetGlobalLogger returned nil", i)
			}
			logger.Infof("Iteration %d", i)
		}
	})

	t.Run("switch between loggers", func(t *testing.T) {
		noOpLogger := &NoOpLogger{}
		defaultLogger := NewDefaultLogger(false, io.Discard)

		SetGlobalLogger(noOpLogger)
		GetGlobalLogger().Infof("NoOp message")

		SetGlobalLogger(defaultLogger)
		GetGlobalLogger().Infof("Default message")

		SetGlobalLogger(noOpLogger)
		GetGlobalLogger().Infof("NoOp message again")
	})
}

// TestLogger_performance tests logger performance
func TestLogger_performance(t *testing.T) {
	t.Run("NoOp logger performance", func(t *testing.T) {
		logger := &NoOpLogger{}

		const iterations = 1000
		for i := 0; i < iterations; i++ {
			logger.Debugf("Debug %d", i)
			logger.Infof("Info %d", i)
			logger.Warnf("Warn %d", i)
			logger.Errorf("Error %d", i)
		}

		t.Logf("Completed %d iterations with all log levels", iterations)
	})

	t.Run("default logger performance", func(t *testing.T) {
		logger := NewDefaultLogger(false, io.Discard)

		const iterations = 1000
		for i := 0; i < iterations; i++ {
			logger.Infof("Info %d", i)
		}

		t.Logf("Completed %d iterations with default logger", iterations)
	})
}

// TestLogger_edgeCases tests logger edge cases
func TestLogger_edgeCases(t *testing.T) {
	t.Run("very long messages", func(t *testing.T) {
		logger := &NoOpLogger{}

		longMsg := make([]byte, 10000)
		for i := range longMsg {
			longMsg[i] = 'a'
		}

		logger.Debugf("Long: %s", string(longMsg))
		logger.Infof("Long: %s", string(longMsg))
		logger.Warnf("Long: %s", string(longMsg))
		logger.Errorf("Long: %s", string(longMsg))

		t.Log("Very long messages completed")
	})

	t.Run("nil values in format", func(t *testing.T) {
		logger := &NoOpLogger{}

		logger.Debugf("Nil: %v", nil)
		logger.Infof("Nil string: %s", (string)(""))
		logger.Warnf("Nil slice: %v", []int(nil))
		logger.Errorf("Nil map: %v", map[string]int(nil))

		t.Log("Nil values completed")
	})

	t.Run("format without args", func(t *testing.T) {
		logger := &NoOpLogger{}

		logger.Debugf("Simple string")
		logger.Infof("Simple string")
		logger.Warnf("Simple string")
		logger.Errorf("Simple string")

		t.Log("Format without args completed")
	})

	t.Run("format with too many args", func(t *testing.T) {
		logger := &NoOpLogger{}

		// Extra arguments should be ignored
		logger.Debugf("Single arg: %s", "test", "extra1", "extra2")
		logger.Infof("No arg: %s %d")

		t.Log("Format with too many args completed")
	})
}
