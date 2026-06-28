package config_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/whither-link/whither/internal/config"
)

var allVars = []string{
	"WHITHER_ADDR",
	"WHITHER_LOG_LEVEL",
	"WHITHER_LOG_FORMAT",
	"WHITHER_READ_HEADER_TIMEOUT",
	"WHITHER_READ_TIMEOUT",
	"WHITHER_WRITE_TIMEOUT",
	"WHITHER_IDLE_TIMEOUT",
	"WHITHER_SHUTDOWN_TIMEOUT",
	"WHITHER_ENV",
	"WHITHER_WIKI_API_BASE",
	"WHITHER_WIKIDATA_API_BASE",
	"WHITHER_ARTICLE_HTML_BASE",
	"WHITHER_USER_AGENT_CONTACT",
	"WHITHER_UPSTREAM_TIMEOUT",
	"WHITHER_UPSTREAM_MAX_RETRIES",
	"WHITHER_UPSTREAM_BACKOFF_BASE",
	"WHITHER_UPSTREAM_MAX_CONCURRENCY",
	"WHITHER_WIKI_ARTICLE_BASE",
	"WHITHER_WIKI_SEARCH_BASE",
	"WHITHER_OPENSEARCH_LIMIT",
	"WHITHER_INFOBOX_ENABLED",
	"WHITHER_REDIS_URL",
	"WHITHER_REDIS_TIMEOUT",
	"WHITHER_CACHE_TTL_POSITIVE",
	"WHITHER_CACHE_TTL_NEGATIVE",
	"WHITHER_CACHE_LANG",
	"WHITHER_CACHE_L1_ENABLED",
	"WHITHER_CACHE_L1_SIZE",
	"WHITHER_CACHE_L1_TTL",
	"WHITHER_CACHE_KEY_PREFIX",
	"WHITHER_CLIENT_CACHE_MAXAGE",
	"WHITHER_ATTRIBUTION_TEXT",
	"WHITHER_MAX_PATH_LEN",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range allVars {
		t.Setenv(k, "")
	}
}

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_USER_AGENT_CONTACT", "test@example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() with all defaults: %v", err)
	}

	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", cfg.LogFormat)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 5s", cfg.ReadHeaderTimeout)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want 10s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want 10s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", cfg.IdleTimeout)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.ShutdownTimeout)
	}
	if cfg.Env != "production" {
		t.Errorf("Env = %q, want production", cfg.Env)
	}
}

func TestLoad_Overrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_USER_AGENT_CONTACT", "test@example.com")
	t.Setenv("WHITHER_ADDR", ":9090")
	t.Setenv("WHITHER_LOG_LEVEL", "debug")
	t.Setenv("WHITHER_LOG_FORMAT", "text")
	t.Setenv("WHITHER_READ_TIMEOUT", "30s")
	t.Setenv("WHITHER_ENV", "development")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() with overrides: %v", err)
	}

	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want text", cfg.LogFormat)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", cfg.ReadTimeout)
	}
	if cfg.Env != "development" {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_ENV", "development")
	t.Setenv("WHITHER_LOG_LEVEL", "verbose")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() with invalid log level: expected error, got nil")
	}
}

func TestLoad_InvalidLogFormat(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_ENV", "development")
	t.Setenv("WHITHER_LOG_FORMAT", "yaml")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() with invalid log format: expected error, got nil")
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	cases := []struct {
		envKey string
		value  string
	}{
		{"WHITHER_READ_HEADER_TIMEOUT", "notaduration"},
		{"WHITHER_READ_TIMEOUT", "0s"},
		{"WHITHER_WRITE_TIMEOUT", "-5s"},
		{"WHITHER_IDLE_TIMEOUT", "abc"},
		{"WHITHER_SHUTDOWN_TIMEOUT", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.envKey+"="+tc.value, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("WHITHER_ENV", "development")
			t.Setenv(tc.envKey, tc.value)

			_, err := config.Load()
			if err == nil {
				t.Fatalf("Load() with %s=%q: expected error, got nil", tc.envKey, tc.value)
			}
		})
	}
}

func TestLoad_UserAgentContactRequiredInProduction(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_ENV", "production")
	// WHITHER_USER_AGENT_CONTACT left empty

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when UserAgentContact is empty in production")
	}
}

func TestLoad_InvalidUpstreamInt(t *testing.T) {
	cases := []struct{ key, val string }{
		{"WHITHER_UPSTREAM_MAX_RETRIES", "notanint"},
		{"WHITHER_UPSTREAM_MAX_RETRIES", "0"},
		{"WHITHER_UPSTREAM_MAX_CONCURRENCY", "-1"},
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.val, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("WHITHER_ENV", "development")
			t.Setenv(tc.key, tc.val)
			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error for %s=%q", tc.key, tc.val)
			}
		})
	}
}

func TestLoad_ResolverDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_USER_AGENT_CONTACT", "test@example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.WikiArticleBase != "https://en.wikipedia.org/wiki/" {
		t.Errorf("WikiArticleBase = %q", cfg.WikiArticleBase)
	}
	if cfg.WikiSearchBase != "https://en.wikipedia.org/w/index.php?search=" {
		t.Errorf("WikiSearchBase = %q", cfg.WikiSearchBase)
	}
	if cfg.OpenSearchLimit != 1 {
		t.Errorf("OpenSearchLimit = %d, want 1", cfg.OpenSearchLimit)
	}
	if cfg.InfoboxEnabled {
		t.Error("InfoboxEnabled should default to false")
	}
}

func TestLoad_CacheDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_USER_AGENT_CONTACT", "test@example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.RedisURL != "redis://localhost:6379/0" {
		t.Errorf("RedisURL = %q, want redis://localhost:6379/0", cfg.RedisURL)
	}
	if cfg.RedisTimeout != 200*time.Millisecond {
		t.Errorf("RedisTimeout = %v, want 200ms", cfg.RedisTimeout)
	}
	if cfg.CacheTTLPositive != 24*time.Hour {
		t.Errorf("CacheTTLPositive = %v, want 24h", cfg.CacheTTLPositive)
	}
	if cfg.CacheTTLNegative != 2*time.Hour {
		t.Errorf("CacheTTLNegative = %v, want 2h", cfg.CacheTTLNegative)
	}
	if cfg.CacheLang != "en" {
		t.Errorf("CacheLang = %q, want en", cfg.CacheLang)
	}
	if cfg.CacheL1Enabled {
		t.Error("CacheL1Enabled should default to false")
	}
	if cfg.CacheL1Size != 1024 {
		t.Errorf("CacheL1Size = %d, want 1024", cfg.CacheL1Size)
	}
	if cfg.CacheL1TTL != 60*time.Second {
		t.Errorf("CacheL1TTL = %v, want 60s", cfg.CacheL1TTL)
	}
	if cfg.CacheKeyPrefix != "v1" {
		t.Errorf("CacheKeyPrefix = %q, want v1", cfg.CacheKeyPrefix)
	}
}

func TestLoad_CacheL1Enabled(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_ENV", "development")
	t.Setenv("WHITHER_CACHE_L1_ENABLED", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !cfg.CacheL1Enabled {
		t.Error("CacheL1Enabled should be true when WHITHER_CACHE_L1_ENABLED=true")
	}
}

func TestLoad_InvalidCacheDuration(t *testing.T) {
	cases := []struct{ key, val string }{
		{"WHITHER_REDIS_TIMEOUT", "notaduration"},
		{"WHITHER_CACHE_TTL_POSITIVE", "0s"},
		{"WHITHER_CACHE_TTL_NEGATIVE", "-1h"},
		{"WHITHER_CACHE_L1_TTL", "abc"},
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.val, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("WHITHER_ENV", "development")
			t.Setenv(tc.key, tc.val)
			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error for %s=%q", tc.key, tc.val)
			}
		})
	}
}

func TestLoad_HTTPAPIDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_USER_AGENT_CONTACT", "test@example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.ClientCacheMaxAge != 3600 {
		t.Errorf("ClientCacheMaxAge = %d, want 3600", cfg.ClientCacheMaxAge)
	}
	if cfg.AttributionText != "Data from Wikipedia/Wikidata, CC BY-SA / CC0" {
		t.Errorf("AttributionText = %q", cfg.AttributionText)
	}
	if cfg.MaxPathLen != 512 {
		t.Errorf("MaxPathLen = %d, want 512", cfg.MaxPathLen)
	}
}

func TestLoad_HTTPAPIOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_ENV", "development")
	t.Setenv("WHITHER_CLIENT_CACHE_MAXAGE", "7200")
	t.Setenv("WHITHER_MAX_PATH_LEN", "256")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.ClientCacheMaxAge != 7200 {
		t.Errorf("ClientCacheMaxAge = %d, want 7200", cfg.ClientCacheMaxAge)
	}
	if cfg.MaxPathLen != 256 {
		t.Errorf("MaxPathLen = %d, want 256", cfg.MaxPathLen)
	}
}

func TestLoad_InvalidHTTPAPI(t *testing.T) {
	cases := []struct{ key, val string }{
		{"WHITHER_CLIENT_CACHE_MAXAGE", "notanint"},
		{"WHITHER_CLIENT_CACHE_MAXAGE", "0"},
		{"WHITHER_MAX_PATH_LEN", "-1"},
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.val, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("WHITHER_ENV", "development")
			t.Setenv(tc.key, tc.val)
			_, err := config.Load()
			if err == nil {
				t.Fatalf("expected error for %s=%q", tc.key, tc.val)
			}
		})
	}
}

func TestLoad_InvalidCacheL1Enabled(t *testing.T) {
	clearEnv(t)
	t.Setenv("WHITHER_ENV", "development")
	t.Setenv("WHITHER_CACHE_L1_ENABLED", "notabool")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid WHITHER_CACHE_L1_ENABLED")
	}
}

func TestLoad_AllLogLevels(t *testing.T) {
	levels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}

	for raw, want := range levels {
		t.Run(raw, func(t *testing.T) {
			clearEnv(t)
			t.Setenv("WHITHER_ENV", "development")
			t.Setenv("WHITHER_LOG_LEVEL", raw)

			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Load() with log level %q: %v", raw, err)
			}
			if cfg.LogLevel != want {
				t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, want)
			}
		})
	}
}
