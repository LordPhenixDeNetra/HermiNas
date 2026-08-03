package wshub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// QueryResult is the subset of querybroker.Result a subscription needs.
// It's its own small type (rather than importing engine/querybroker
// directly) so this package is wired to a broker by bootstrap.go via the
// QueryRunner interface, the same layering pattern engine/api's Deps
// struct already uses for its dependencies.
type QueryResult struct {
	Rows [][]byte // one JSON object per row (ClickHouse JSONEachRow), same encoding querybroker.Result uses
}

// QueryRunner is the read path a widget subscription polls.
// engine/querybroker.Broker satisfies this already (see bootstrap.go);
// tests use a fake.
type QueryRunner interface {
	Execute(ctx context.Context, userID, sql string) (QueryResult, error)
}

// minInterval guards against a misconfigured widget (0ms, 1ms) hammering
// ClickHouse on every hub tick.
const minInterval = 250 * time.Millisecond

type subscription struct {
	cancel context.CancelFunc
}

// subscriptionSet is the widget subscriptions one client currently owns —
// "requête + intervalle → push dashboard.data" (M2.1). Each subscription
// runs its own poll goroutine so a slow widget query never delays another
// widget's refresh on the same connection.
type subscriptionSet struct {
	client *Client
	runner QueryRunner
	userID string

	mu   sync.Mutex
	subs map[string]*subscription
}

func newSubscriptionSet(c *Client, runner QueryRunner, userID string) *subscriptionSet {
	return &subscriptionSet{client: c, runner: runner, userID: userID, subs: make(map[string]*subscription)}
}

func (s *subscriptionSet) subscribe(msg controlMessage) {
	if msg.WidgetID == "" || msg.SQL == "" {
		return
	}
	interval := time.Duration(msg.IntervalMS) * time.Millisecond
	if interval < minInterval {
		interval = minInterval
	}

	s.mu.Lock()
	if existing, ok := s.subs[msg.WidgetID]; ok {
		existing.cancel() // re-subscribing (e.g. the widget's query was edited) replaces the old poller
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.subs[msg.WidgetID] = &subscription{cancel: cancel}
	s.mu.Unlock()

	go s.run(ctx, msg, interval)
}

func (s *subscriptionSet) unsubscribe(widgetID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sub, ok := s.subs[widgetID]; ok {
		sub.cancel()
		delete(s.subs, widgetID)
	}
}

func (s *subscriptionSet) cancelAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sub := range s.subs {
		sub.cancel()
		delete(s.subs, id)
	}
}

// run polls the widget's query on msg's interval and pushes dashboard.data.
// In incremental mode ("mode streaming: requêtes incrémentales depuis
// dernier timestamp", M2.1) it wraps the query so only rows newer than the
// last observed value of since_column are fetched and pushed, instead of
// re-sending the full result set every tick.
func (s *subscriptionSet) run(ctx context.Context, msg controlMessage, interval time.Duration) {
	incremental := msg.Mode == "incremental" && msg.SinceColumn != ""
	var lastSeen string

	poll := func() {
		q := msg.SQL
		if incremental && lastSeen != "" {
			q = fmt.Sprintf("SELECT * FROM (%s) AS w WHERE %s > '%s'", msg.SQL, msg.SinceColumn, lastSeen)
		}
		result, err := s.runner.Execute(ctx, s.userID, q)
		if err != nil {
			log.Printf("wshub: widget %s poll failed: %v", msg.WidgetID, err)
			return
		}
		if incremental {
			lastSeen = maxSinceColumn(result.Rows, msg.SinceColumn, lastSeen)
			if len(result.Rows) == 0 {
				return // nothing new since the last tick — don't push an empty refresh
			}
		}
		if err := s.client.hub.send(s.client, "dashboard.data", dashboardDataPayload(msg.WidgetID, result.Rows)); err != nil {
			log.Printf("wshub: widget %s payload rejected: %v", msg.WidgetID, err)
		}
	}

	poll()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll()
		}
	}
}

func dashboardDataPayload(widgetID string, rows [][]byte) map[string]any {
	raw := make([]json.RawMessage, len(rows))
	for i, r := range rows {
		raw[i] = json.RawMessage(r)
	}
	return map[string]any{
		"widget_id":    widgetID,
		"rows":         raw,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
}

// maxSinceColumn returns the largest value of column seen across rows,
// starting from current. Comparison is lexicographic on the column's raw
// JSON text — correct for ClickHouse's fixed-width DateTime/DateTime64
// string rendering and for monotonically-formatted numeric strings, but
// not a general-purpose type-aware comparator; picking a non-time,
// non-fixed-width since_column is a misuse of incremental mode, not
// something this function tries to guard against.
func maxSinceColumn(rows [][]byte, column, current string) string {
	max := current
	for _, r := range rows {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(r, &obj); err != nil {
			continue
		}
		raw, ok := obj[column]
		if !ok {
			continue
		}
		val := string(raw)
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			val = s
		}
		if val > max {
			max = val
		}
	}
	return max
}
