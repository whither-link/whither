package wiki

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/whither-link/whither/internal/config"
)

// sleepFunc is injectable so tests can skip real sleeps.
type sleepFunc func(ctx context.Context, d time.Duration) error

// semaphore caps the number of concurrent upstream requests using a buffered channel.
type semaphore chan struct{}

func newSemaphore(n int) semaphore {
	return make(semaphore, n)
}

func (s semaphore) acquire(ctx context.Context) error {
	select {
	case s <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s semaphore) release() {
	<-s
}

// baseClient holds the shared HTTP transport, UA string, semaphore, and retry policy
// used by all three Wikimedia client types.
type baseClient struct {
	hc          *http.Client
	userAgent   string
	sem         semaphore
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
	bc := &baseClient{
		hc: &http.Client{
			Transport: transport,
			Timeout:   cfg.UpstreamTimeout + 5*time.Second,
		},
		userAgent:   fmt.Sprintf("Whither/dev (+%s)", cfg.UserAgentContact),
		sem:         newSemaphore(cfg.UpstreamMaxConcurrency),
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
		return nil, fmt.Errorf("acquire semaphore: %w", err)
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
