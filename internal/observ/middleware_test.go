package observ_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/whither-link/whither/internal/observ"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRequestID_HeaderSet(t *testing.T) {
	handler := observ.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-Id")
	if id == "" {
		t.Fatal("X-Request-Id header not set")
	}
	if len(id) != 16 {
		t.Errorf("X-Request-Id length = %d, want 16 hex chars", len(id))
	}
}

func TestRequestID_PropagatedToContext(t *testing.T) {
	var contextID string
	handler := observ.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID = observ.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	headerID := rr.Header().Get("X-Request-Id")
	if contextID == "" {
		t.Fatal("request ID not propagated to context")
	}
	if contextID != headerID {
		t.Errorf("context ID %q != header ID %q", contextID, headerID)
	}
}

func TestRecovery_CatchesPanic(t *testing.T) {
	handler := observ.Recovery(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
}

func TestRecovery_DoesNotLeakPanicValue(t *testing.T) {
	handler := observ.Recovery(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("secret details")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if strings.Contains(body, "secret details") {
		t.Errorf("response body leaked panic value: %q", body)
	}
}

func TestAccessLog_CapturesStatus(t *testing.T) {
	handler := observ.AccessLog(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/some/path", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rr.Code)
	}
}
