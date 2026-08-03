package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimitAllowsUpToTheLimit(t *testing.T) {
	handler := RateLimit(2, func(*http.Request) string { return "same-key" })(okHandler())

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: expected 429, got %d", rec.Code)
	}
}

func TestRateLimitIsPerKey(t *testing.T) {
	handler := RateLimit(1, func(r *http.Request) string { return r.Header.Get("X-Key") })(okHandler())

	for _, key := range []string{"a", "b"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Key", key)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("key %q: expected 200, got %d", key, rec.Code)
		}
	}
}

func TestRateLimitZeroMeansUnlimited(t *testing.T) {
	handler := RateLimit(0, func(*http.Request) string { return "same-key" })(okHandler())
	for i := 0; i < 100; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200 under an unlimited rate, got %d", i, rec.Code)
		}
	}
}

func TestByRemoteIPExtractsHostFromHostPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:54321"
	if got := ByRemoteIP(req); got != "192.0.2.1" {
		t.Fatalf("expected 192.0.2.1, got %q", got)
	}
}
