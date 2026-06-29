// Package config loads and validates service configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

var defaults = map[string]string{
	"WHITHER_ADDR":                     ":8080",
	"WHITHER_LOG_LEVEL":                "info",
	"WHITHER_LOG_FORMAT":               "json",
	"WHITHER_ENV":                      "production",
	"WHITHER_READ_HEADER_TIMEOUT":      "5s",
	"WHITHER_READ_TIMEOUT":             "10s",
	"WHITHER_WRITE_TIMEOUT":            "10s",
	"WHITHER_IDLE_TIMEOUT":             "60s",
	"WHITHER_SHUTDOWN_TIMEOUT":         "15s",
	"WHITHER_WIKI_API_BASE":            "https://en.wikipedia.org/w/api.php",
	"WHITHER_WIKIDATA_API_BASE":        "https://www.wikidata.org/w/api.php",
	"WHITHER_ARTICLE_HTML_BASE":        "https://en.wikipedia.org/api/rest_v1",
	"WHITHER_USER_AGENT_CONTACT":       "",
	"WHITHER_UPSTREAM_TIMEOUT":         "5s",
	"WHITHER_UPSTREAM_BACKOFF_BASE":    "100ms",
	"WHITHER_UPSTREAM_MAX_RETRIES":       "1",
	"WHITHER_UPSTREAM_MAX_CONCURRENCY":   "8",
	"WHITHER_UPSTREAM_MAX_WAITING":       "64",
	"WHITHER_UPSTREAM_ACQUIRE_TIMEOUT":   "1s",
	"WHITHER_REDIS_URL":                "redis://localhost:6379/0",
	"WHITHER_REDIS_TIMEOUT":            "200ms",
	"WHITHER_CACHE_TTL_POSITIVE":       "72h",
	"WHITHER_CACHE_TTL_NEGATIVE":       "12h",
	"WHITHER_CACHE_LANG":               "en",
	"WHITHER_CACHE_L1_ENABLED":         "true",
	"WHITHER_CACHE_L1_SIZE":            "1024",
	"WHITHER_CACHE_L1_TTL":             "60s",
	"WHITHER_CACHE_KEY_PREFIX":         "v1",

	// Resolver
	"WHITHER_WIKI_ARTICLE_BASE": "https://en.wikipedia.org/wiki/",
	"WHITHER_WIKI_SEARCH_BASE":  "https://en.wikipedia.org/w/index.php?search=",
	"WHITHER_OPENSEARCH_LIMIT":  "1",
	"WHITHER_INFOBOX_ENABLED":   "false", // stubbed; set true once F4 is implemented

	// HTTP API
	"WHITHER_CLIENT_CACHE_MAXAGE": "3600",
	"WHITHER_ATTRIBUTION_TEXT":    "Data from Wikipedia/Wikidata, CC BY-SA / CC0",
	"WHITHER_MAX_PATH_LEN":        "512",
}

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
	WikiAPIBase             string
	WikidataAPIBase         string
	ArticleHTMLBase         string
	UserAgentContact        string // required in production
	Version                 string // stamped at build time; defaults to "dev"
	UpstreamTimeout         time.Duration
	UpstreamMaxRetries      int
	UpstreamBackoffBase     time.Duration
	UpstreamMaxConcurrency  int
	UpstreamMaxWaiting      int
	UpstreamAcquireTimeout  time.Duration

	// Resolver
	WikiArticleBase string
	WikiSearchBase  string
	OpenSearchLimit int
	InfoboxEnabled  bool

	// HTTP API
	ClientCacheMaxAge int // seconds; emitted as Cache-Control: public, max-age=N
	AttributionText   string
	MaxPathLen        int

	// Cache / Redis
	RedisURL         string
	RedisTimeout     time.Duration
	CacheTTLPositive time.Duration
	CacheTTLNegative time.Duration
	CacheLang        string
	CacheL1Enabled   bool
	CacheL1Size      int
	CacheL1TTL       time.Duration
	CacheKeyPrefix   string
}

// Load reads and validates configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:             getenv("WHITHER_ADDR"),
		LogFormat:        getenv("WHITHER_LOG_FORMAT"),
		Env:              getenv("WHITHER_ENV"),
		WikiAPIBase:      getenv("WHITHER_WIKI_API_BASE"),
		WikidataAPIBase:  getenv("WHITHER_WIKIDATA_API_BASE"),
		ArticleHTMLBase:  getenv("WHITHER_ARTICLE_HTML_BASE"),
		UserAgentContact: getenv("WHITHER_USER_AGENT_CONTACT"),
		WikiArticleBase:  getenv("WHITHER_WIKI_ARTICLE_BASE"),
		WikiSearchBase:   getenv("WHITHER_WIKI_SEARCH_BASE"),
		RedisURL:         getenv("WHITHER_REDIS_URL"),
		CacheLang:        getenv("WHITHER_CACHE_LANG"),
		CacheKeyPrefix:   getenv("WHITHER_CACHE_KEY_PREFIX"),
	}

	var err error

	if cfg.LogLevel, err = parseLogLevel("WHITHER_LOG_LEVEL"); err != nil {
		return nil, err
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "text" {
		return nil, fmt.Errorf("WHITHER_LOG_FORMAT %q: must be json or text", cfg.LogFormat)
	}
	if cfg.ReadHeaderTimeout, err = parseDuration("WHITHER_READ_HEADER_TIMEOUT"); err != nil {
		return nil, err
	}
	if cfg.ReadTimeout, err = parseDuration("WHITHER_READ_TIMEOUT"); err != nil {
		return nil, err
	}
	if cfg.WriteTimeout, err = parseDuration("WHITHER_WRITE_TIMEOUT"); err != nil {
		return nil, err
	}
	if cfg.IdleTimeout, err = parseDuration("WHITHER_IDLE_TIMEOUT"); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = parseDuration("WHITHER_SHUTDOWN_TIMEOUT"); err != nil {
		return nil, err
	}

	// Upstream client settings
	if cfg.UserAgentContact == "" && cfg.Env == "production" {
		return nil, fmt.Errorf("WHITHER_USER_AGENT_CONTACT is required in production (Wikimedia API etiquette)")
	}
	if cfg.UpstreamTimeout, err = parseDuration("WHITHER_UPSTREAM_TIMEOUT"); err != nil {
		return nil, err
	}
	if cfg.UpstreamBackoffBase, err = parseDuration("WHITHER_UPSTREAM_BACKOFF_BASE"); err != nil {
		return nil, err
	}
	if cfg.UpstreamMaxRetries, err = parseInt("WHITHER_UPSTREAM_MAX_RETRIES"); err != nil {
		return nil, err
	}
	if cfg.UpstreamMaxConcurrency, err = parseInt("WHITHER_UPSTREAM_MAX_CONCURRENCY"); err != nil {
		return nil, err
	}
	if cfg.UpstreamMaxWaiting, err = parseInt("WHITHER_UPSTREAM_MAX_WAITING"); err != nil {
		return nil, err
	}
	if cfg.UpstreamAcquireTimeout, err = parseDuration("WHITHER_UPSTREAM_ACQUIRE_TIMEOUT"); err != nil {
		return nil, err
	}

	// Resolver
	if cfg.OpenSearchLimit, err = parseInt("WHITHER_OPENSEARCH_LIMIT"); err != nil {
		return nil, err
	}
	if cfg.InfoboxEnabled, err = parseBool("WHITHER_INFOBOX_ENABLED"); err != nil {
		return nil, err
	}

	// Cache / Redis
	if cfg.RedisTimeout, err = parseDuration("WHITHER_REDIS_TIMEOUT"); err != nil {
		return nil, err
	}
	if cfg.CacheTTLPositive, err = parseDuration("WHITHER_CACHE_TTL_POSITIVE"); err != nil {
		return nil, err
	}
	if cfg.CacheTTLNegative, err = parseDuration("WHITHER_CACHE_TTL_NEGATIVE"); err != nil {
		return nil, err
	}
	if cfg.CacheL1Enabled, err = parseBool("WHITHER_CACHE_L1_ENABLED"); err != nil {
		return nil, err
	}
	if cfg.CacheL1Size, err = parseInt("WHITHER_CACHE_L1_SIZE"); err != nil {
		return nil, err
	}
	if cfg.CacheL1TTL, err = parseDuration("WHITHER_CACHE_L1_TTL"); err != nil {
		return nil, err
	}

	// HTTP API
	cfg.AttributionText = getenv("WHITHER_ATTRIBUTION_TEXT")
	if cfg.ClientCacheMaxAge, err = parseInt("WHITHER_CLIENT_CACHE_MAXAGE"); err != nil {
		return nil, err
	}
	if cfg.MaxPathLen, err = parseInt("WHITHER_MAX_PATH_LEN"); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getenv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaults[key]
}

func parseLogLevel(envKey string) (slog.Level, error) {
	raw := getenv(envKey)
	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return 0, fmt.Errorf("%s %q: must be one of debug, info, warn, error", envKey, raw)
	}
	return level, nil
}

func parseInt(envKey string) (int, error) {
	raw := getenv(envKey)
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %q: must be an integer: %w", envKey, raw, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%s %d: must be positive", envKey, n)
	}
	return n, nil
}

func parseBool(envKey string) (bool, error) {
	raw := getenv(envKey)
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s %q: must be a boolean (true/false/1/0)", envKey, raw)
	}
	return b, nil
}

func parseDuration(envKey string) (time.Duration, error) {
	raw := getenv(envKey)
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %q: invalid duration: %w", envKey, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s %q: must be a positive duration", envKey, raw)
	}
	return d, nil
}
