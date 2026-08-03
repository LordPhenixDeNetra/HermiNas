package querybroker

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"herminas/kernel/errors"
)

var errCallbackStop = stderrors.New("callback requested stop")

// requireClickHouse skips the test (not fails) if no ClickHouse is
// reachable, consistent with every other real-service test in this repo —
// run `go run bootstrap.go run` (M0.5 supervisor) to exercise these.
func requireClickHouse(t *testing.T) string {
	t.Helper()
	baseURL := os.Getenv("HERMINAS_CLICKHOUSE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8123"
	}
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/ping", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("no ClickHouse reachable at %s (%v) — run `go run bootstrap.go run` to exercise this test", baseURL, err)
	}
	resp.Body.Close()
	return baseURL
}

func newTestBroker(t *testing.T, quota Quota, cacheTTL time.Duration) *Broker {
	t.Helper()
	baseURL := requireClickHouse(t)
	b, err := New(Config{
		ClickHouseURL: baseURL,
		Quota:         quota,
		CacheTTL:      cacheTTL,
		AuditLogPath:  filepath.Join(t.TempDir(), "audit.jsonl"),
	})
	if err != nil {
		t.Fatalf("New broker failed: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestExecuteReturnsRealRowsFromClickHouse(t *testing.T) {
	b := newTestBroker(t, Quota{}, time.Minute)

	result, err := b.Execute(context.Background(), "alice", "SELECT number FROM system.numbers LIMIT 5")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.RowCount != 5 || len(result.Rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", result.RowCount)
	}
	if result.Cached {
		t.Fatal("first execution should not be a cache hit")
	}

	var row map[string]any
	if err := json.Unmarshal(result.Rows[0], &row); err != nil {
		t.Fatalf("decode row: %v", err)
	}
	if _, ok := row["number"]; !ok {
		t.Fatalf("expected a %q field, got %v", "number", row)
	}
}

func TestExecuteServesIdenticalQueryFromCache(t *testing.T) {
	b := newTestBroker(t, Quota{}, time.Minute)
	sql := "SELECT number FROM system.numbers LIMIT 3"

	first, err := b.Execute(context.Background(), "alice", sql)
	if err != nil {
		t.Fatalf("first Execute failed: %v", err)
	}
	if first.Cached {
		t.Fatal("first execution should not be cached")
	}

	second, err := b.Execute(context.Background(), "alice", sql)
	if err != nil {
		t.Fatalf("second Execute failed: %v", err)
	}
	if !second.Cached {
		t.Fatal("second execution of the same SQL should be served from cache")
	}
	if second.RowCount != first.RowCount {
		t.Fatalf("cached result row count %d differs from original %d", second.RowCount, first.RowCount)
	}
}

func TestExecuteEnforcesRequestsPerMinute(t *testing.T) {
	b := newTestBroker(t, Quota{RequestsPerMinute: 1}, 0)

	if _, err := b.Execute(context.Background(), "alice", "SELECT 1"); err != nil {
		t.Fatalf("first Execute should be allowed: %v", err)
	}

	_, err := b.Execute(context.Background(), "alice", "SELECT 2")
	if !errors.IsResourceExhausted(err) {
		t.Fatalf("expected resource_exhausted for the 2nd request, got %v", err)
	}
}

func TestExecuteEnforcesMaxRowsToRead(t *testing.T) {
	b := newTestBroker(t, Quota{MaxRowsToRead: 1000}, 0)

	_, err := b.Execute(context.Background(), "alice", "SELECT sum(number) FROM numbers(1000000)")
	if err == nil {
		t.Fatal("expected an error: query should have exceeded max_rows_to_read")
	}
	if !errors.IsResourceExhausted(err) {
		t.Fatalf("expected resource_exhausted, got %v", err)
	}
}

func TestStreamDeliversRowsIncrementally(t *testing.T) {
	b := newTestBroker(t, Quota{}, 0)

	var lines [][]byte
	count, err := b.Stream(context.Background(), "alice", "SELECT number FROM system.numbers LIMIT 10", func(line []byte) error {
		cp := append([]byte(nil), line...)
		lines = append(lines, cp)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	if count != 10 || len(lines) != 10 {
		t.Fatalf("expected 10 streamed rows, got count=%d collected=%d", count, len(lines))
	}
}

func TestStreamRespectsContextCancellation(t *testing.T) {
	b := newTestBroker(t, Quota{}, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := b.Stream(ctx, "alice", "SELECT sleep(3)", func(_ []byte) error { return nil })
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error from a query that outlives its context timeout")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Stream took %s — context cancellation should abort well before the 3s sleep() completes", elapsed)
	}
}

func TestStreamStopsWhenCallbackReturnsError(t *testing.T) {
	b := newTestBroker(t, Quota{}, 0)

	callCount := 0
	_, err := b.Stream(context.Background(), "alice", "SELECT number FROM system.numbers LIMIT 100", func(_ []byte) error {
		callCount++
		if callCount == 3 {
			return errCallbackStop
		}
		return nil
	})
	if err != errCallbackStop {
		t.Fatalf("expected errCallbackStop, got %v", err)
	}
	if callCount != 3 {
		t.Fatalf("expected exactly 3 callback invocations before stopping, got %d", callCount)
	}
}
