// Package observ provides structured logging and HTTP middleware for observability.
package observ

import (
	"context"
	"log/slog"
	"os"

	"github.com/whither-link/whither/internal/config"
)

type loggerKey struct{}

// NewLogger returns a configured slog.Logger writing to stderr.
func NewLogger(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}

// ContextWithLogger returns a new context carrying logger.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// LoggerFromContext retrieves the logger stored in ctx, falling back to slog.Default().
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
