package common

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewColoredTextHandler(t *testing.T) {
	tests := []struct {
		name string
		opts *slog.HandlerOptions
	}{
		{
			name: "nil options",
			opts: nil,
		},
		{
			name: "with options",
			opts: &slog.HandlerOptions{Level: slog.LevelDebug},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := newColoredTextHandler(&buf, tt.opts)
			assert.NotNil(t, handler)
		})
	}
}

func TestColoredTextHandler_Handle(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
	}{
		{"debug level", slog.LevelDebug},
		{"info level", slog.LevelInfo},
		{"warn level", slog.LevelWarn},
		{"error level", slog.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := newColoredTextHandler(&buf, nil)

			record := slog.NewRecord(time.Now(), tt.level, "test message", 0)
			err := handler.Handle(context.Background(), record)
			assert.NoError(t, err)
			assert.NotEmpty(t, buf.String())
		})
	}
}

func TestCountHandler(t *testing.T) {
	var buf bytes.Buffer
	// Use debug level to enable all log levels
	baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	countHandler := &countHandler{next: baseHandler}

	// Test Enabled - all levels should be enabled with debug level
	assert.True(t, countHandler.Enabled(context.Background(), slog.LevelInfo))
	assert.True(t, countHandler.Enabled(context.Background(), slog.LevelDebug))
	assert.True(t, countHandler.Enabled(context.Background(), slog.LevelWarn))
	assert.True(t, countHandler.Enabled(context.Background(), slog.LevelError))

	// Test Handle
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)
	err := countHandler.Handle(context.Background(), record)
	assert.NoError(t, err)

	// Test WithAttrs
	newHandler := countHandler.WithAttrs([]slog.Attr{slog.String("key", "value")})
	assert.NotNil(t, newHandler)

	// Test WithGroup
	newHandler2 := countHandler.WithGroup("test")
	assert.NotNil(t, newHandler2)
}

func TestGetLogCounters(t *testing.T) {
	counters := GetLogCounters()
	assert.NotNil(t, counters)
	assert.Contains(t, counters, "debug")
	assert.Contains(t, counters, "info")
	assert.Contains(t, counters, "warn")
	assert.Contains(t, counters, "error")
	assert.Contains(t, counters, "total")
}

func TestMergeLogSection(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*viper.Viper)
		expected map[string]interface{}
	}{
		{
			name: "no log section",
			setup: func(v *viper.Viper) {
				v.Set("server.port", 8080)
			},
			expected: map[string]interface{}{},
		},
		{
			name: "with log section",
			setup: func(v *viper.Viper) {
				v.Set("log.level", "debug")
				v.Set("log.format", "json")
			},
			expected: map[string]interface{}{
				"log.level":  "debug",
				"log.format": "json",
			},
		},
		{
			name: "partial log section",
			setup: func(v *viper.Viper) {
				v.Set("log.level", "warn")
			},
			expected: map[string]interface{}{
				"log.level": "warn",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			tt.setup(v)

			MergeLogSection(v)

			for key, expectedValue := range tt.expected {
				actualValue := v.Get(key)
				assert.Equal(t, expectedValue, actualValue, "key: %s", key)
			}
		})
	}
}

func TestWriterFunc(t *testing.T) {
	var buf bytes.Buffer
	wf := writerFunc(func(p []byte) (int, error) {
		return buf.Write(p)
	})

	n, err := wf.Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, "test", buf.String())
}

func TestSetupLoggerWithFile(t *testing.T) {
	// Test with console output only
	SetupLoggerWithFile("info", "json", "", 10, 3, 7, false)

	// Test with file output
	tmpDir := t.TempDir()
	logFile := tmpDir + "/test.log"
	SetupLoggerWithFile("debug", "console", logFile, 10, 3, 7, false)

	// Write a log message to trigger file creation
	slog.Info("test message")

	// Verify log file was created
	_, err := os.Stat(logFile)
	assert.NoError(t, err)
}
