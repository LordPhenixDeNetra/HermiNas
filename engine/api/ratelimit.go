// Package api is HermiNas' L3 HTTP server (M1.5): wires auth, RBAC, rate
// limiting and the querybroker/schemamgr handlers built in M1.3/M1.4 into
// one real `http.Server`.
package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimit is a sliding-window request cap per key (M1.5: "rate limiting
// HTTP par IP/token") — a coarser, cheaper first line of defense than
// querybroker's own per-user query quota (M1.4): it guards every route,
// not just /api/v1/query, and runs before auth so an unauthenticated
// flood can't reach the (more expensive) credential-checking path either.
func RateLimit(requestsPerMinute int, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	limiter := newSlidingWindow(requestsPerMinute)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.allow(keyFunc(r)) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ByRemoteIP is the default RateLimit key: the caller's IP, unauthenticated
// or not. Falls back to the raw RemoteAddr if it isn't a valid host:port
// (e.g. under some test harnesses).
func ByRemoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type slidingWindow struct {
	mu      sync.Mutex
	limit   int
	windows map[string][]time.Time
}

func newSlidingWindow(limit int) *slidingWindow {
	return &slidingWindow{limit: limit, windows: make(map[string][]time.Time)}
}

func (s *slidingWindow) allow(key string) bool {
	if s.limit <= 0 {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	window := s.windows[key]
	kept := window[:0]
	for _, ts := range window {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}

	if len(kept) >= s.limit {
		s.windows[key] = kept
		return false
	}

	s.windows[key] = append(kept, now)
	return true
}
