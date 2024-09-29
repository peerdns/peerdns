package logger

import (
	"github.com/peerdns/peerdns/pkg/config"
)

// InitializeGlobalLogger initializes the global logger based on the provided configuration.
// It should be called once during application startup.
func InitializeGlobalLogger(cfg config.Logger) error {
	l, err := Factory(cfg)
	if err != nil {
		return err
	}
	return SetGlobalLogger(l)
}

// SyncGlobalLogger flushes any buffered log entries.
// It should be called before application exit to ensure all logs are written.
func SyncGlobalLogger() error {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		if zapLogger, ok := globalLogger.(*ZapLogger); ok && zapLogger.sugaredLogger != nil {
			return zapLogger.sugaredLogger.Sync()
		}
	}
	return nil
}
