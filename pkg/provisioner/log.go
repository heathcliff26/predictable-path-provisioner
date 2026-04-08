package provisioner

import (
	"log/slog"
	"os"
	"strings"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	logLevelEnv     = "LOG_LEVEL"
	logLevelDefault = slog.LevelInfo
)

func init() {
	initLogger()
}

func initLogger() {
	opts := &slog.HandlerOptions{
		Level: getLogLevel(),
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	ctrl.SetLogger(logr.FromSlogHandler(handler))
	slog.SetDefault(slog.New(handler))
}

func getLogLevel() slog.Level {
	levelStr := os.Getenv(logLevelEnv)
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "":
		return logLevelDefault
	default:
		slog.Warn("Unknown log level", "level", levelStr, "default", logLevelDefault)
		return logLevelDefault
	}
}
