package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Addr string
	LogLevel  slog.Level
	LogFormat string // "json" | "text"

	// HTTP timeouts
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	ShutdownTimeout time.Duration
	Env             string // "production" | "development"
}

// Load reads and validates configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:      getenv("WHITHER_ADDR", ":8080"),
		LogFormat: getenv("WHITHER_LOG_FORMAT", "json"),
		Env:       getenv("WHITHER_ENV", "production"),
	}

	var err error

	if cfg.LogLevel, err = parseLogLevel("WHITHER_LOG_LEVEL", "info"); err != nil {
		return nil, err
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		return nil, fmt.Errorf("WHITHER_LOG_FORMAT %q: must be json or text", cfg.LogFormat)
	}
	if cfg.ReadHeaderTimeout, err = parseDuration("WHITHER_READ_HEADER_TIMEOUT", "5s"); err != nil {
		return nil, err
	}
	if cfg.ReadTimeout, err = parseDuration("WHITHER_READ_TIMEOUT", "10s"); err != nil {
		return nil, err
	}
	if cfg.WriteTimeout, err = parseDuration("WHITHER_WRITE_TIMEOUT", "10s"); err != nil {
		return nil, err
	}
	if cfg.IdleTimeout, err = parseDuration("WHITHER_IDLE_TIMEOUT", "60s"); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = parseDuration("WHITHER_SHUTDOWN_TIMEOUT", "15s"); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseLogLevel(envKey, defaultVal string) (slog.Level, error) {
	raw := getenv(envKey, defaultVal)
	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return 0, fmt.Errorf("%s %q: must be one of debug, info, warn, error", envKey, raw)
	}
	return level, nil
}

func parseDuration(envKey, defaultVal string) (time.Duration, error) {
	raw := getenv(envKey, defaultVal)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %q: invalid duration: %w", envKey, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s %q: must be a positive duration", envKey, raw)
	}
	return d, nil
}
