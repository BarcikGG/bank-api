package logger

import (
	"log/slog"
	"os"
)

func New(env, process string) *slog.Logger {
	options := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: env == "local",
	}

	var handler slog.Handler
	if env == "local" {
		handler = slog.NewTextHandler(os.Stdout, options)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, options)
	}

	return slog.New(handler).With(
		slog.String("service", "notifications"),
		slog.String("process", process),
		slog.String("env", env),
	)
}
