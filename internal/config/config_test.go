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
