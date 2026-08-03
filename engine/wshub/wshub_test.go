package wshub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"herminas/engine/auth"
	"herminas/kernel/permissions"
	"herminas/kernel/schemas"
)

// fakeRunner is a QueryRunner test double that records every SQL string it
// was asked to run and returns whatever rows the test queued up for the
// next call.
type fakeRunner struct {
	mu    sync.Mutex
	calls []string
	next  [][][]byte // one entry consumed per Execute call; last entry repeats once exhausted
}

func (f *fakeRunner) Execute(_ context.Context, _ string, sql string) (QueryResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sql)
	if len(f.next) == 0 {
		return QueryResult{}, nil
	}
	idx := len(f.calls) - 1
	if idx >= len(f.next) {
		idx = len(f.next) - 1
	}
	return QueryResult{Rows: f.next[idx]}, nil
}

func (f *fakeRunner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeRunner) lastSQL() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return ""
	}
	return f.calls[len(f.calls)-1]
}

func testJWTManager(t *testing.T) *auth.JWTManager {
	t.Helper()
	return auth.NewJWTManager([]byte("test-secret-32-bytes-minimum!!!!"), time.Hour)
}

func dialWS(t *testing.T, server *httptest.Server, token string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + token
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		status := "no response"
		if resp != nil {
			status = resp.Status
		}
		t.Fatalf("dial %s: %v (%s)", url, err, status)
	}
	return conn
}

func readEnvelope(t *testing.T, conn *websocket.Conn, timeout time.Duration) envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v (raw: %s)", err, data)
	}
	return env
}

func TestServeWSRejectsMissingOrInvalidToken(t *testing.T) {
	hub := New(nil)
	jwtMgr := testJWTManager(t)
	server := httptest.NewServer(ServeWS(hub, jwtMgr, &fakeRunner{}, false))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected dial without a token to fail")
	}
	if resp == nil || resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %v", resp)
	}

	_, resp, err = websocket.DefaultDialer.Dial(url+"?token=garbage", nil)
	if err == nil {
		t.Fatal("expected dial with an invalid token to fail")
	}
	if resp == nil || resp.StatusCode != 401 {
		t.Fatalf("expected 401 for invalid token, got %v", resp)
	}
}

func TestServeWSAcceptsValidTokenAndReceivesBroadcast(t *testing.T) {
	hub := New(nil)
	jwtMgr := testJWTManager(t)
	server := httptest.NewServer(ServeWS(hub, jwtMgr, &fakeRunner{}, false))
	defer server.Close()

	token, err := jwtMgr.Issue("alice", permissions.RoleViewer)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	conn := dialWS(t, server, token)
	defer conn.Close()

	// Registration happens in a goroutine right after Upgrade; poll rather
	// than race a fixed sleep against it.
	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 registered client, got %d", hub.ClientCount())
	}

	if err := hub.BroadcastSystemHealth("clickhouse", "healthy", "", "2026-08-02T10:00:00Z"); err != nil {
		t.Fatalf("broadcast: %v", err)
	}

	env := readEnvelope(t, conn, 2*time.Second)
	if env.Event != "system.health" {
		t.Fatalf("event = %q, want system.health", env.Event)
	}
}

func TestServeWSDisconnectUnregistersClient(t *testing.T) {
	hub := New(nil)
	jwtMgr := testJWTManager(t)
	server := httptest.NewServer(ServeWS(hub, jwtMgr, &fakeRunner{}, false))
	defer server.Close()

	token, _ := jwtMgr.Issue("alice", permissions.RoleViewer)
	conn := dialWS(t, server, token)

	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	conn.Close()

	deadline = time.Now().Add(2 * time.Second)
	for hub.ClientCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("expected client to be unregistered after close, got count %d", got)
	}
}

func TestSubscriptionPushesDashboardDataOnInterval(t *testing.T) {
	hub := New(nil)
	jwtMgr := testJWTManager(t)
	runner := &fakeRunner{next: [][][]byte{
		{[]byte(`{"count":1}`)},
	}}
	server := httptest.NewServer(ServeWS(hub, jwtMgr, runner, false))
	defer server.Close()

	token, _ := jwtMgr.Issue("alice", permissions.RoleViewer)
	conn := dialWS(t, server, token)
	defer conn.Close()

	sub := controlMessage{Action: "subscribe", WidgetID: "w1", SQL: "SELECT count() AS count FROM events", IntervalMS: 250}
	data, _ := json.Marshal(sub)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	env := readEnvelope(t, conn, 2*time.Second)
	if env.Event != "dashboard.data" {
		t.Fatalf("event = %q, want dashboard.data", env.Event)
	}
	payload, ok := env.Payload.(map[string]any)
	if !ok {
		t.Fatalf("payload is %T, want map", env.Payload)
	}
	if payload["widget_id"] != "w1" {
		t.Fatalf("widget_id = %v, want w1", payload["widget_id"])
	}
	rows, ok := payload["rows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("rows = %v, want a single-row array", payload["rows"])
	}

	// A second push should follow on the next tick without a further
	// subscribe message.
	env2 := readEnvelope(t, conn, 2*time.Second)
	if env2.Event != "dashboard.data" {
		t.Fatalf("second event = %q, want dashboard.data", env2.Event)
	}
	if runner.callCount() < 2 {
		t.Fatalf("expected at least 2 poll calls, got %d", runner.callCount())
	}
}

func TestSubscriptionOutputValidatesAgainstRealSchema(t *testing.T) {
	sch, err := schemas.LoadWebSocketSchemas()
	if err != nil {
		t.Fatalf("load schemas: %v", err)
	}
	hub := New(sch)
	jwtMgr := testJWTManager(t)
	runner := &fakeRunner{next: [][][]byte{{[]byte(`{"ts":"2026-08-02T10:00:00Z","value":42}`)}}}
	server := httptest.NewServer(ServeWS(hub, jwtMgr, runner, false))
	defer server.Close()

	token, _ := jwtMgr.Issue("alice", permissions.RoleViewer)
	conn := dialWS(t, server, token)
	defer conn.Close()

	sub := controlMessage{Action: "subscribe", WidgetID: "w1", SQL: "SELECT now() AS ts, 42 AS value", IntervalMS: 250}
	data, _ := json.Marshal(sub)
	_ = conn.WriteMessage(websocket.TextMessage, data)

	// If the payload wasn't schema-valid, Hub.send would have returned an
	// error and logged instead of queuing anything — receiving a message
	// at all is itself proof the real schema validator accepted it.
	env := readEnvelope(t, conn, 2*time.Second)
	if env.Event != "dashboard.data" {
		t.Fatalf("event = %q, want dashboard.data", env.Event)
	}
}

func TestSubscriptionUnsubscribeStopsFurtherPushes(t *testing.T) {
	hub := New(nil)
	jwtMgr := testJWTManager(t)
	runner := &fakeRunner{next: [][][]byte{{[]byte(`{"count":1}`)}}}
	server := httptest.NewServer(ServeWS(hub, jwtMgr, runner, false))
	defer server.Close()

	token, _ := jwtMgr.Issue("alice", permissions.RoleViewer)
	conn := dialWS(t, server, token)
	defer conn.Close()

	sub := controlMessage{Action: "subscribe", WidgetID: "w1", SQL: "SELECT 1", IntervalMS: 250}
	data, _ := json.Marshal(sub)
	_ = conn.WriteMessage(websocket.TextMessage, data)
	readEnvelope(t, conn, 2*time.Second) // first push

	unsub := controlMessage{Action: "unsubscribe", WidgetID: "w1"}
	data, _ = json.Marshal(unsub)
	_ = conn.WriteMessage(websocket.TextMessage, data)

	// Drain whatever was already in flight, then make sure nothing more
	// arrives within a couple of intervals' worth of time.
	time.Sleep(150 * time.Millisecond)
	countAfterUnsub := runner.callCount()
	time.Sleep(700 * time.Millisecond)
	if runner.callCount() > countAfterUnsub {
		t.Fatalf("poll continued after unsubscribe: %d calls before, %d after waiting", countAfterUnsub, runner.callCount())
	}
}

func TestIncrementalModeFiltersOnSinceColumnAfterFirstPoll(t *testing.T) {
	hub := New(nil)
	jwtMgr := testJWTManager(t)
	runner := &fakeRunner{next: [][][]byte{
		{[]byte(`{"ts":"2026-08-02T10:00:00Z"}`), []byte(`{"ts":"2026-08-02T10:00:05Z"}`)},
		{[]byte(`{"ts":"2026-08-02T10:00:10Z"}`)},
	}}
	server := httptest.NewServer(ServeWS(hub, jwtMgr, runner, false))
	defer server.Close()

	token, _ := jwtMgr.Issue("alice", permissions.RoleViewer)
	conn := dialWS(t, server, token)
	defer conn.Close()

	sub := controlMessage{
		Action: "subscribe", WidgetID: "w1", SQL: "SELECT ts FROM events",
		IntervalMS: 250, Mode: "incremental", SinceColumn: "ts",
	}
	data, _ := json.Marshal(sub)
	_ = conn.WriteMessage(websocket.TextMessage, data)

	readEnvelope(t, conn, 2*time.Second) // first push: 2 rows, no filter yet
	readEnvelope(t, conn, 2*time.Second) // second push: 1 new row

	lastSQL := runner.lastSQL()
	if !strings.Contains(lastSQL, "WHERE ts > '2026-08-02T10:00:05Z'") {
		t.Fatalf("expected second poll to filter on the max ts seen, got SQL: %q", lastSQL)
	}
}

func TestIncrementalModeSkipsPushWhenNoNewRows(t *testing.T) {
	hub := New(nil)
	jwtMgr := testJWTManager(t)
	runner := &fakeRunner{next: [][][]byte{
		{[]byte(`{"ts":"2026-08-02T10:00:00Z"}`)},
		{}, // nothing new on the second tick
		{[]byte(`{"ts":"2026-08-02T10:00:10Z"}`)},
	}}
	server := httptest.NewServer(ServeWS(hub, jwtMgr, runner, false))
	defer server.Close()

	token, _ := jwtMgr.Issue("alice", permissions.RoleViewer)
	conn := dialWS(t, server, token)
	defer conn.Close()

	sub := controlMessage{
		Action: "subscribe", WidgetID: "w1", SQL: "SELECT ts FROM events",
		IntervalMS: 200, Mode: "incremental", SinceColumn: "ts",
	}
	data, _ := json.Marshal(sub)
	_ = conn.WriteMessage(websocket.TextMessage, data)

	first := readEnvelope(t, conn, 2*time.Second)
	second := readEnvelope(t, conn, 3*time.Second) // must skip the empty tick and wait for the third poll

	firstPayload := first.Payload.(map[string]any)
	secondPayload := second.Payload.(map[string]any)
	firstRows := firstPayload["rows"].([]any)
	secondRows := secondPayload["rows"].([]any)
	if len(firstRows) != 1 || len(secondRows) != 1 {
		t.Fatalf("expected 1 row in each non-empty push, got %d then %d", len(firstRows), len(secondRows))
	}
	if runner.callCount() < 3 {
		t.Fatalf("expected the empty tick to still poll (just not push), got %d calls", runner.callCount())
	}
}

func TestMessageRateLimiterCapsThenResetsPerSecond(t *testing.T) {
	rl := newMessageRateLimiter(3)
	for i := 0; i < 3; i++ {
		if !rl.allow() {
			t.Fatalf("call %d: expected allowed within limit", i)
		}
	}
	if rl.allow() {
		t.Fatal("expected 4th call within the same second to be denied")
	}

	rl.window = time.Now().Add(-2 * time.Second) // simulate the window elapsing
	if !rl.allow() {
		t.Fatal("expected a new window to allow again")
	}
}

func TestHubEncodeRejectsPayloadFailingSchema(t *testing.T) {
	sch, err := schemas.LoadWebSocketSchemas()
	if err != nil {
		t.Fatalf("load schemas: %v", err)
	}
	hub := New(sch)

	if err := hub.Broadcast("system.health", map[string]any{}); err == nil {
		t.Fatal("expected an incomplete system.health payload to be rejected by the real schema")
	}
	if err := hub.BroadcastAgentStatus("agent-1", true, "2026-08-02T10:00:00Z", "host-1"); err != nil {
		t.Fatalf("expected a valid agent.status payload to pass: %v", err)
	}
	if err := hub.Broadcast("not.a.real.event", map[string]any{}); err == nil {
		t.Fatal("expected an unknown event name to be rejected")
	}
}

func TestMaxSinceColumnPicksLexicographicMax(t *testing.T) {
	rows := [][]byte{
		[]byte(`{"ts":"2026-08-02T10:00:05Z"}`),
		[]byte(`{"ts":"2026-08-02T10:00:01Z"}`),
		[]byte(`{"other":"field"}`), // missing column: ignored, not an error
	}
	got := maxSinceColumn(rows, "ts", "")
	if got != "2026-08-02T10:00:05Z" {
		t.Fatalf("maxSinceColumn = %q, want the largest ts", got)
	}

	got = maxSinceColumn(nil, "ts", "2026-08-02T10:00:09Z")
	if got != "2026-08-02T10:00:09Z" {
		t.Fatalf("maxSinceColumn with no rows should keep current, got %q", got)
	}
}

func init() {
	// Fail fast with a clear message if the embedded schemas can't be
	// loaded at all, instead of every test that depends on them failing
	// with a less obvious error.
	if _, err := schemas.LoadWebSocketSchemas(); err != nil {
		panic(fmt.Sprintf("wshub tests require kernel/schemas.LoadWebSocketSchemas to succeed: %v", err))
	}
}
