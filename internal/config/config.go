// Package config loads and validates service configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Addr      string
	LogLevel  slog.Level
	LogFormat string // "json" | "text"

	// HTTP timeouts
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

	ShutdownTimeout time.Duration
	Env             string // "production" | "development"

	// Upstream clients
	WikiAPIBase            string
	WikidataAPIBase        string
	ArticleHTMLBase        string
	UserAgentContact       string // required in production
	UpstreamTimeout        time.Duration
	UpstreamMaxRetries     int
	UpstreamBackoffBase    time.Duration
	UpstreamMaxConcurrency int
}

// Load reads and validates configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:             getenv("WHITHER_ADDR", ":8080"),
		LogFormat:        getenv("WHITHER_LOG_FORMAT", "json"),
		Env:              getenv("WHITHER_ENV", "production"),
		WikiAPIBase:      getenv("WHITHER_WIKI_API_BASE", "https://en.wikipedia.org/w/api.php"),
		WikidataAPIBase:  getenv("WHITHER_WIKIDATA_API_BASE", "https://www.wikidata.org/w/api.php"),
		ArticleHTMLBase:  getenv("WHITHER_ARTICLE_HTML_BASE", "https://en.wikipedia.org/api/rest_v1"),
		UserAgentContact: getenv("WHITHER_USER_AGENT_CONTACT", ""),
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

	// Upstream client settings
	if cfg.UserAgentContact == "" && cfg.Env == "production" {
		return nil, fmt.Errorf("WHITHER_USER_AGENT_CONTACT is required in production (Wikimedia API etiquette)")
	}
	if cfg.UpstreamTimeout, err = parseDuration("WHITHER_UPSTREAM_TIMEOUT", "5s"); err != nil {
		return nil, err
	}
	if cfg.UpstreamBackoffBase, err = parseDuration("WHITHER_UPSTREAM_BACKOFF_BASE", "100ms"); err != nil {
		return nil, err
	}
	if cfg.UpstreamMaxRetries, err = parseInt("WHITHER_UPSTREAM_MAX_RETRIES", 3); err != nil {
		return nil, err
	}
	if cfg.UpstreamMaxConcurrency, err = parseInt("WHITHER_UPSTREAM_MAX_CONCURRENCY", 8); err != nil {
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

func parseInt(envKey string, defaultVal int) (int, error) {
	raw := os.Getenv(envKey)
	if raw == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %q: must be an integer: %w", envKey, raw, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s %d: must be positive", envKey, n)
	}
	return n, nil
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
