package wiki

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/whither-link/whither/internal/config"
)

// sleepFunc is injectable so tests can skip real sleeps.
type sleepFunc func(ctx context.Context, d time.Duration) error

// semaphore caps concurrent upstream requests and bounds the waiting queue to
// prevent unbounded goroutine/heap growth under a cold traffic flood.
//
// With MaxConcurrency=8 and ~1–3 s per cold resolution, draining 8 slots takes
// ~1–3 s; maxWaiting=64 caps parked goroutines to a small bounded set and gives
// an effective admitted backlog of a few seconds — past that we shed fast rather
// than pile up. acquireTO=1s keeps a single waiter's tail latency bounded so a
// request making up to ~4 sequential upstream calls stays under WriteTimeout.
type semaphore struct {
	slots      chan struct{}
	waiting    int64 // atomic: goroutines currently parked waiting for a slot
	maxWaiting int64
	acquireTO  time.Duration
}

func newSemaphore(n, maxWaiting int, acquireTO time.Duration) *semaphore {
	return &semaphore{
		slots:      make(chan struct{}, n),
		maxWaiting: int64(maxWaiting),
		acquireTO:  acquireTO,
	}
}

func (s *semaphore) acquire(ctx context.Context) error {
	// Fast path: a slot is free right now.
	select {
	case s.slots <- struct{}{}:
		return nil
	default:
	}
	// Admission control: refuse to queue past maxWaiting.
	if atomic.AddInt64(&s.waiting, 1) > s.maxWaiting {
		atomic.AddInt64(&s.waiting, -1)
		slog.Warn("upstream semaphore shed: queue full")
		return ErrUpstreamUnavailable
	}
	defer atomic.AddInt64(&s.waiting, -1)

	t := time.NewTimer(s.acquireTO)
	defer t.Stop()
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-t.C:
		slog.Warn("upstream semaphore shed: acquire timeout")
		return ErrUpstreamUnavailable
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *semaphore) release() { <-s.slots }

// baseClient holds the shared HTTP transport, UA string, semaphore, and retry policy
// used by all three Wikimedia client types.
type baseClient struct {
	hc          *http.Client
	userAgent   string
	sem         *semaphore
	maxRetries  int
	backoffBase time.Duration
	sleep       sleepFunc
}

// Option configures a baseClient; used primarily to inject test helpers.
type Option func(*baseClient)

// WithSleepFn replaces the real sleep with fn in tests to avoid real delays.
func WithSleepFn(fn sleepFunc) Option {
	return func(bc *baseClient) { bc.sleep = fn }
}

func newBaseClient(cfg *config.Config, opts ...Option) *baseClient {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: cfg.UpstreamTimeout,
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
	}
	v := cfg.Version
	if v == "" || v == "dev" {
		v = "1.0"
	}
	bc := &baseClient{
		hc: &http.Client{
			Transport: transport,
			Timeout:   cfg.UpstreamTimeout + 5*time.Second,
		},
		userAgent:   fmt.Sprintf("Whither/%s (bot; https://whither.link; %s) Go/%s", v, cfg.UserAgentContact, strings.TrimPrefix(runtime.Version(), "go")),
		sem:         newSemaphore(cfg.UpstreamMaxConcurrency, cfg.UpstreamMaxWaiting, cfg.UpstreamAcquireTimeout),
		maxRetries:  cfg.UpstreamMaxRetries,
		backoffBase: cfg.UpstreamBackoffBase,
		sleep:       realSleep,
	}
	for _, o := range opts {
		o(bc)
	}
	return bc
}

// do executes req against the upstream, retrying on transient errors with
// exponential backoff + jitter. The semaphore is held for the duration of all
// attempts. Callers must close the returned response body.
func (c *baseClient) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.sem.acquire(ctx); err != nil {
		// ErrUpstreamUnavailable returned bare (shed path) so errors.Is holds
		// up the call chain and triggers stale-serve / 503. ctx.Err() is
		// returned as-is; client already gone, response is moot.
		return nil, err
	}
	defer c.sem.release()

	req.Header.Set("User-Agent", c.userAgent)

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.hc.Do(req.Clone(ctx))
		if err != nil {
			if attempt == c.maxRetries {
				return nil, ErrUpstreamUnavailable
			}
			if sleepErr := c.sleep(ctx, c.jitter(attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		switch {
		case resp.StatusCode == http.StatusTooManyRequests:
			d := retryAfterDuration(resp, c.jitter(attempt))
			drainClose(resp)
			if attempt == c.maxRetries {
				return nil, ErrUpstreamUnavailable
			}
			if sleepErr := c.sleep(ctx, d); sleepErr != nil {
				return nil, sleepErr
			}

		case resp.StatusCode >= 500:
			drainClose(resp)
			if attempt == c.maxRetries {
				return nil, ErrUpstreamUnavailable
			}
			if sleepErr := c.sleep(ctx, c.jitter(attempt)); sleepErr != nil {
				return nil, sleepErr
			}

		case resp.StatusCode >= 400:
			drainClose(resp)
			return nil, ErrNotFound

		default:
			return resp, nil
		}
	}
	return nil, ErrUpstreamUnavailable
}

// jitter returns an exponential backoff duration for attempt n with ±50% jitter.
func (c *baseClient) jitter(attempt int) time.Duration {
	exp := c.backoffBase * (1 << uint(attempt))
	if exp > 30*time.Second {
		exp = 30 * time.Second
	}
	// add uniform jitter in [0, exp/2]
	j := time.Duration(rand.Int64N(int64(exp/2) + 1))
	return exp + j
}

func retryAfterDuration(resp *http.Response, fallback time.Duration) time.Duration {
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return fallback
}

func drainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func realSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
