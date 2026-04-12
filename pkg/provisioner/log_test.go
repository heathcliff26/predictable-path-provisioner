package provisioner

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLogLevel(t *testing.T) {
	t.Cleanup(initLogger)
	tMatrix := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"error":   slog.LevelError,
		"Debug":   slog.LevelDebug,
		"WARN":    slog.LevelWarn,
		"unknown": logLevelDefault,
	}
	for levelStr, logLevel := range tMatrix {
		t.Run(levelStr, func(t *testing.T) {
			t.Setenv(logLevelEnv, levelStr)

			assert.Equal(t, logLevel, getLogLevel(), "Should return correct log level")

			// Show the actual output here for manual verification
			initLogger()
			slog.Debug("Test log message at debug level")
			slog.Info("Test log message at info level")
			slog.Warn("Test log message at warn level")
			slog.Error("This is an error message")
		})
	}
	t.Run("VariableEmpty", func(t *testing.T) {
		t.Setenv(logLevelEnv, "")
		assert.Equal(t, logLevelDefault, getLogLevel(), "Should return default value")
	})
}
