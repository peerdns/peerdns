package logger

import (
	"sync"
)

// Logger is the interface that wraps basic logging methods.
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
	Fatal(msg string, keysAndValues ...any)
}

// Global logger instance and mutex for thread-safe operations
var (
	globalLogger Logger
	mu           sync.RWMutex
)

// SetGlobalLogger sets the global logger instance.
// It should be called once during application initialization.
func SetGlobalLogger(l Logger) error {
	mu.Lock()
	defer mu.Unlock()
	globalLogger = l
	return nil
}

// G retrieves the global logger instance.
// Returns a no-op logger if no global logger is set.
func G() Logger {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		return globalLogger
	}
	return NewNoOpLogger()
}
