package querybroker

import (
	"sync"
	"time"
)

// Quota bounds what one user can do to the broker (M1.4: "quotas basiques :
// requêtes/min par utilisateur, max lignes scannées"). Zero means
// unlimited for that field — useful for an admin role or local dev.
type Quota struct {
	RequestsPerMinute int
	// MaxRowsToRead is passed to ClickHouse as the `max_rows_to_read`
	// setting, so the engine itself aborts an over-scoped query instead of
	// the broker checking after the fact — cheaper and can't be bypassed
	// by a query that would scan billions of rows before we ever see a
	// row count.
	MaxRowsToRead uint64
}

// QuotaTracker enforces RequestsPerMinute with a sliding window per user.
type QuotaTracker struct {
	mu      sync.Mutex
	quota   Quota
	windows map[string][]time.Time
}

func NewQuotaTracker(q Quota) *QuotaTracker {
	return &QuotaTracker{quota: q, windows: make(map[string][]time.Time)}
}

// Allow reports whether userID may make another request right now, and —
// if so — records it against their window.
func (t *QuotaTracker) Allow(userID string) bool {
	if t.quota.RequestsPerMinute <= 0 {
		return true
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	window := t.windows[userID]
	kept := window[:0]
	for _, ts := range window {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}

	if len(kept) >= t.quota.RequestsPerMinute {
		t.windows[userID] = kept
		return false
	}

	t.windows[userID] = append(kept, now)
	return true
}
