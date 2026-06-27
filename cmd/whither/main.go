// Package main is the entry point for the whither HTTP redirect service.
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

	"github.com/whither-link/whither/internal/cache"
	"github.com/whither-link/whither/internal/config"
	"github.com/whither-link/whither/internal/httpapi"
	"github.com/whither-link/whither/internal/observ"
	"github.com/whither-link/whither/internal/wiki"
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

	clients := wiki.NewClients(cfg)
	logger.Info("upstream clients ready",
		slog.String("wiki_api", cfg.WikiAPIBase),
		slog.String("wikidata_api", cfg.WikidataAPIBase),
	)
	_ = clients // TODO

	redisCache, err := cache.NewRedisCache(
		cfg.RedisURL,
		cfg.CacheTTLPositive,
		cfg.CacheTTLNegative,
		cfg.RedisTimeout,
		logger,
	)
	if err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	defer func() { _ = redisCache.Close() }()

	var c cache.Cache = redisCache
	if cfg.CacheL1Enabled {
		c, err = cache.NewTwoLevel(cfg.CacheL1Size, cfg.CacheL1TTL, redisCache, nil)
		if err != nil {
			return fmt.Errorf("cache l1: %w", err)
		}
	}
	logger.Info("cache ready", slog.Bool("l1_enabled", cfg.CacheL1Enabled))
	_ = c // TODO

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
