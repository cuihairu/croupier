package croupier

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Logger interface for configurable logging
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// DefaultLogger is the default logger implementation
type DefaultLogger struct {
	debug bool
	out   io.Writer
}

// NewDefaultLogger creates a new default logger
func NewDefaultLogger(debug bool, out io.Writer) *DefaultLogger {
	if out == nil {
		out = os.Stdout
	}
	return &DefaultLogger{
		debug: debug,
		out:   out,
	}
}

// Debugf logs debug messages
func (l *DefaultLogger) Debugf(format string, args ...interface{}) {
	if l.debug {
		fmt.Fprintf(l.out, "[DEBUG] "+format+"\n", args...)
	}
}

// Infof logs info messages
func (l *DefaultLogger) Infof(format string, args ...interface{}) {
	fmt.Fprintf(l.out, "[INFO] "+format+"\n", args...)
}

// Warnf logs warning messages
func (l *DefaultLogger) Warnf(format string, args ...interface{}) {
	fmt.Fprintf(l.out, "[WARN] "+format+"\n", args...)
}

// Errorf logs error messages
func (l *DefaultLogger) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(l.out, "[ERROR] "+format+"\n", args...)
}

// NoOpLogger is a logger that does nothing
type NoOpLogger struct{}

// Debugf does nothing
func (l *NoOpLogger) Debugf(format string, args ...interface{}) {}

// Infof does nothing
func (l *NoOpLogger) Infof(format string, args ...interface{}) {}

// Warnf does nothing
func (l *NoOpLogger) Warnf(format string, args ...interface{}) {}

// Errorf does nothing
func (l *NoOpLogger) Errorf(format string, args ...interface{}) {}

// global logger instance - can be replaced by user
// Using mutex for thread-safe access to support different logger types
var (
	globalLogger Logger = NewDefaultLogger(false, os.Stdout)
	loggerMutex  sync.RWMutex
)

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger Logger) {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	globalLogger = logger
}

// GetGlobalLogger returns the current global logger
func GetGlobalLogger() Logger {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	return globalLogger
}

// Log helpers using global logger
func logDebugf(format string, args ...interface{}) {
	GetGlobalLogger().Debugf(format, args...)
}

func logInfof(format string, args ...interface{}) {
	GetGlobalLogger().Infof(format, args...)
}

func logWarnf(format string, args ...interface{}) {
	GetGlobalLogger().Warnf(format, args...)
}

func logErrorf(format string, args ...interface{}) {
	GetGlobalLogger().Errorf(format, args...)
}
