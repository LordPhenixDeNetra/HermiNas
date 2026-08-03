package wshub

import (
	"sync"
	"time"
)

// messageRateLimiter caps inbound control messages (subscribe/unsubscribe)
// per client per second. Unlike engine/api.RateLimit's per-IP sliding
// window (shared across every request), each Client already is one
// connection's worth of isolated state, so a plain fixed-window counter
// reset every second is simpler and just as effective here.
type messageRateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Time
	count  int
}

func newMessageRateLimiter(perSecond int) *messageRateLimiter {
	return &messageRateLimiter{limit: perSecond, window: time.Now()}
}

func (r *messageRateLimiter) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Sub(r.window) >= time.Second {
		r.window = now
		r.count = 0
	}
	r.count++
	return r.count <= r.limit
}
