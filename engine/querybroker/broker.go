// Package querybroker executes SQL against the embedded ClickHouse engine
// on behalf of the control plane (M1.4): timeout/cancellation via context,
// chunked streaming for large results, per-user rate limiting, a native
// ClickHouse-side row-scan cap, a short-TTL result cache, and an
// append-only audit trail. This is what Query Studio (M1.6) and the
// eventual NL→SQL path (M4.1) both call into — neither talks to
// ClickHouse directly.
package querybroker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"herminas/kernel/errors"
)

type Config struct {
	ClickHouseURL string
	Quota         Quota
	CacheTTL      time.Duration
	AuditLogPath  string
}

type Broker struct {
	baseURL string
	client  *http.Client
	cache   *Cache
	quotas  *QuotaTracker
	audit   *AuditLog
}

func New(cfg Config) (*Broker, error) {
	audit, err := OpenAuditLog(cfg.AuditLogPath)
	if err != nil {
		return nil, err
	}
	return &Broker{
		baseURL: cfg.ClickHouseURL,
		client:  &http.Client{},
		cache:   NewCache(cfg.CacheTTL),
		quotas:  NewQuotaTracker(cfg.Quota),
		audit:   audit,
	}, nil
}

func (b *Broker) Close() error {
	return b.audit.Close()
}

// Result is Execute's buffered response: one []byte per row (ClickHouse
// JSONEachRow, one JSON object per line).
type Result struct {
	Rows     [][]byte
	RowCount int
	Duration time.Duration
	Cached   bool
}

// Execute runs sql for userID and buffers the full result. sql must be a
// bare statement without its own FORMAT clause — Execute always appends
// `FORMAT JSONEachRow` so results are line-delimited for both caching and
// (in Stream) incremental delivery.
func (b *Broker) Execute(ctx context.Context, userID, sql string) (*Result, error) {
	if !b.quotas.Allow(userID) {
		err := errors.New(errors.CodeResourceExhausted, fmt.Sprintf("rate limit exceeded for user %q", userID))
		b.audit.Record(AuditEntry{UserID: userID, SQL: sql, StartedAt: time.Now(), Success: false, Error: err.Error()})
		return nil, err
	}

	if cached, ok := b.cache.Get(sql); ok {
		rows := splitLines(cached)
		b.audit.Record(AuditEntry{UserID: userID, SQL: sql, StartedAt: time.Now(), RowCount: len(rows), Cached: true, Success: true})
		return &Result{Rows: rows, RowCount: len(rows), Cached: true}, nil
	}

	start := time.Now()
	body, err := b.doQuery(ctx, sql)
	duration := time.Since(start)

	entry := AuditEntry{UserID: userID, SQL: sql, StartedAt: start, DurationMs: duration.Milliseconds()}
	if err != nil {
		entry.Success = false
		entry.Error = err.Error()
		b.audit.Record(entry)
		return nil, err
	}

	rows := splitLines(body)
	entry.Success = true
	entry.RowCount = len(rows)
	b.audit.Record(entry)

	b.cache.Set(sql, body)
	return &Result{Rows: rows, RowCount: len(rows), Duration: duration}, nil
}

// Stream runs sql for userID and calls onRow for each result row as it
// arrives off the wire — ClickHouse's response is read incrementally
// (bufio.Scanner over the live HTTP body), not buffered first, so a large
// result starts reaching the caller immediately. Not cached: Execute
// serves the "same dashboard query every few seconds" case; Stream is for
// results too large or too live to want cached as one blob.
func (b *Broker) Stream(ctx context.Context, userID, sql string, onRow func(line []byte) error) (int, error) {
	if !b.quotas.Allow(userID) {
		err := errors.New(errors.CodeResourceExhausted, fmt.Sprintf("rate limit exceeded for user %q", userID))
		b.audit.Record(AuditEntry{UserID: userID, SQL: sql, StartedAt: time.Now(), Success: false, Error: err.Error()})
		return 0, err
	}

	start := time.Now()
	resp, err := b.request(ctx, sql)
	if err != nil {
		b.audit.Record(AuditEntry{UserID: userID, SQL: sql, StartedAt: start, DurationMs: time.Since(start).Milliseconds(), Success: false, Error: err.Error()})
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := clickHouseError(resp)
		b.audit.Record(AuditEntry{UserID: userID, SQL: sql, StartedAt: start, DurationMs: time.Since(start).Milliseconds(), Success: false, Error: err.Error()})
		return 0, err
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	count := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if cbErr := onRow(line); cbErr != nil {
			b.audit.Record(AuditEntry{
				UserID: userID, SQL: sql, StartedAt: start, DurationMs: time.Since(start).Milliseconds(),
				RowCount: count, Success: false, Error: cbErr.Error(),
			})
			return count, cbErr
		}
		count++
	}

	duration := time.Since(start)
	if scanErr := scanner.Err(); scanErr != nil {
		wrapped := errors.Wrap(errors.CodeInternal, "reading clickhouse response", scanErr)
		b.audit.Record(AuditEntry{UserID: userID, SQL: sql, StartedAt: start, DurationMs: duration.Milliseconds(), RowCount: count, Success: false, Error: wrapped.Error()})
		return count, wrapped
	}

	b.audit.Record(AuditEntry{UserID: userID, SQL: sql, StartedAt: start, DurationMs: duration.Milliseconds(), RowCount: count, Success: true})
	return count, nil
}

func (b *Broker) doQuery(ctx context.Context, sql string) ([]byte, error) {
	resp, err := b.request(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, clickHouseErrorFromBody(resp.StatusCode, body)
	}
	if readErr != nil {
		return nil, errors.Wrap(errors.CodeInternal, "reading clickhouse response", readErr)
	}
	return body, nil
}

func (b *Broker) request(ctx context.Context, sql string) (*http.Response, error) {
	q := url.Values{}
	q.Set("query", sql+" FORMAT JSONEachRow")
	if b.quotas.quota.MaxRowsToRead > 0 {
		q.Set("max_rows_to_read", strconv.FormatUint(b.quotas.quota.MaxRowsToRead, 10))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL, nil)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "build clickhouse request", err)
	}
	req.URL.RawQuery = q.Encode()
	// ClickHouse's HTTP interface returns 411 Length Required for any POST
	// without a Content-Length header, even an intentionally empty body
	// (see engine/schemamgr's integration test for how this was found).
	req.Header.Set("Content-Length", "0")

	resp, err := b.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.Wrap(errors.CodeUnavailable, "query canceled or timed out", ctx.Err())
		}
		return nil, errors.Wrap(errors.CodeUnavailable, "clickhouse request failed", err)
	}
	return resp, nil
}

func clickHouseError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return clickHouseErrorFromBody(resp.StatusCode, body)
}

func clickHouseErrorFromBody(status int, body []byte) error {
	msg := strings.TrimSpace(string(body))
	if strings.Contains(msg, "TOO_MANY_ROWS") || strings.Contains(msg, "LIMIT_EXCEEDED") {
		return errors.New(errors.CodeResourceExhausted, fmt.Sprintf("query exceeded max_rows_to_read: %s", msg))
	}
	return errors.New(errors.CodeInternal, fmt.Sprintf("clickhouse returned %d: %s", status, msg))
}

func splitLines(body []byte) [][]byte {
	var rows [][]byte
	for _, line := range strings.Split(string(body), "\n") {
		if line == "" {
			continue
		}
		rows = append(rows, []byte(line))
	}
	return rows
}
