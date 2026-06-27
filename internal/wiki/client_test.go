package wiki_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/whither-link/whither/internal/wiki"
)

// noSleep is a sleepFunc that returns immediately, for fast retry tests.
func noSleep(_ context.Context, _ time.Duration) error { return nil }

// --- Retry behaviour ---------------------------------------------------------

func TestRetry_SucceedsAfterTransientFailures(t *testing.T) {
	const failFirst = 2
	var attempts atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n <= failFirst {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		body := readFixture(t, "mediawiki-normalize-found.json")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := testConfig(t, srv.URL)
	cfg.UpstreamMaxRetries = 3
	clients := wiki.NewClients(cfg, wiki.WithSleepFn(noSleep))

	_, err := clients.MediaWiki.Normalize(context.Background(), "test")
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if got := int(attempts.Load()); got != failFirst+1 {
		t.Errorf("attempts = %d, want %d", got, failFirst+1)
	}
}

func TestRetry_ExhaustedReturnsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	cfg := testConfig(t, srv.URL)
	cfg.UpstreamMaxRetries = 2
	clients := wiki.NewClients(cfg, wiki.WithSleepFn(noSleep))

	_, err := clients.MediaWiki.Normalize(context.Background(), "test")
	if !errors.Is(err, wiki.ErrUpstreamUnavailable) {
		t.Errorf("err = %v, want ErrUpstreamUnavailable", err)
	}
}

func TestRetry_NoRetryOn4xx(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := testConfig(t, srv.URL)
	cfg.UpstreamMaxRetries = 3
	clients := wiki.NewClients(cfg, wiki.WithSleepFn(noSleep))

	_, err := clients.MediaWiki.Normalize(context.Background(), "test")
	if !errors.Is(err, wiki.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if got := int(attempts.Load()); got != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 4xx)", got)
	}
}

func TestRetry_HonorsRetryAfter(t *testing.T) {
	var sleepDurations []time.Duration
	captureSleep := func(_ context.Context, d time.Duration) error {
		sleepDurations = append(sleepDurations, d)
		return nil
	}

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		body := readFixture(t, "mediawiki-normalize-found.json")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := testConfig(t, srv.URL)
	cfg.UpstreamMaxRetries = 2
	clients := wiki.NewClients(cfg, wiki.WithSleepFn(captureSleep))

	_, err := clients.MediaWiki.Normalize(context.Background(), "test")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if len(sleepDurations) == 0 {
		t.Fatal("expected at least one sleep for Retry-After")
	}
	if sleepDurations[0] != 2*time.Second {
		t.Errorf("sleep duration = %v, want 2s (from Retry-After header)", sleepDurations[0])
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := testConfig(t, srv.URL)
	cfg.UpstreamMaxRetries = 5
	cancelSleep := func(ctx context.Context, _ time.Duration) error {
		return ctx.Err()
	}
	clients := wiki.NewClients(cfg, wiki.WithSleepFn(cancelSleep))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := clients.MediaWiki.Normalize(ctx, "test")
	if err == nil {
		t.Fatal("expected error after context cancellation, got nil")
	}
}

// --- Concurrency limiter -----------------------------------------------------

func TestConcurrencyLimiter_CapsInflightRequests(t *testing.T) {
	const concLimit = 2
	var inflight atomic.Int32
	var maxSeen atomic.Int32

	slow := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := inflight.Add(1)
		defer inflight.Add(-1)
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		<-slow // block until released
		body := readFixture(t, "mediawiki-normalize-found.json")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	cfg := testConfig(t, srv.URL)
	cfg.UpstreamMaxConcurrency = concLimit
	cfg.UpstreamMaxRetries = 0
	clients := wiki.NewClients(cfg, wiki.WithSleepFn(noSleep))

	const goroutines = 5
	errs := make(chan error, goroutines)
	for range goroutines {
		go func() {
			_, err := clients.MediaWiki.Normalize(context.Background(), "test")
			errs <- err
		}()
	}

	// Let goroutines start and block in the handler.
	time.Sleep(50 * time.Millisecond)

	if got := maxSeen.Load(); got > int32(concLimit) {
		t.Errorf("max inflight = %d, want ≤ %d", got, concLimit)
	}

	// Release all blocked requests.
	close(slow)
	for range goroutines {
		if err := <-errs; err != nil {
			t.Errorf("goroutine error: %v", err)
		}
	}
}
