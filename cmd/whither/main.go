package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/httpapi"
	"github.com/whither-link/whither/internal/observ"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "whither: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}

	logger := observ.NewLogger(cfg)
	slog.SetDefault(logger)

	router := httpapi.NewRouter(cfg, logger)
	srv := httpapi.NewServer(cfg, router)

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", slog.String("addr", cfg.Addr), slog.String("env", cfg.Env))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
		return nil
	case sig := <-quit:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("server stopped")
	return nil
}
