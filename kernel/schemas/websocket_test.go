// Package schemas validates kernel/schemas/websocket/*.json — the WebSocket
// contracts Go pushes to React over the hub (M2.1). Each file must be a
// well-formed JSON Schema (draft 2020-12) that accepts a realistic sample
// payload and rejects an invalid one; that's more than "is it valid JSON",
// which is why this pulls in a real schema validator rather than just
// calling json.Unmarshal.
package schemas

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func compile(t *testing.T, path string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(path)
	if err != nil {
		t.Fatalf("compiling %s: %v", path, err)
	}
	return sch
}

func validate(t *testing.T, sch *jsonschema.Schema, sample map[string]any) error {
	t.Helper()
	b, err := json.Marshal(sample)
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}
	return sch.Validate(v)
}

func TestEnvelopeSchemaAcceptsKnownEventsOnly(t *testing.T) {
	sch := compile(t, "websocket/envelope.schema.json")

	valid := map[string]any{"event": "system.health", "payload": map[string]any{}}
	if err := validate(t, sch, valid); err != nil {
		t.Errorf("expected valid envelope to pass: %v", err)
	}

	invalid := map[string]any{"event": "not.a.real.event", "payload": map[string]any{}}
	if err := validate(t, sch, invalid); err == nil {
		t.Error("expected unknown event name to be rejected")
	}
}

func TestPayloadSchemas(t *testing.T) {
	cases := []struct {
		name    string
		schema  string
		valid   map[string]any
		invalid map[string]any
	}{
		{
			name:   "dashboard.data",
			schema: "websocket/dashboard.data.schema.json",
			valid: map[string]any{
				"widget_id":    "w-1",
				"rows":         []any{map[string]any{"ts": "2026-08-02T10:00:00Z", "value": 42}},
				"generated_at": "2026-08-02T10:00:00Z",
			},
			invalid: map[string]any{"rows": []any{}}, // missing widget_id
		},
		{
			name:   "query.rows",
			schema: "websocket/query.rows.schema.json",
			valid: map[string]any{
				"query_id": "q-1",
				"rows":     []any{map[string]any{"count": 1}},
				"done":     true,
			},
			invalid: map[string]any{"query_id": "q-1", "done": true}, // missing rows
		},
		{
			name:   "alert.fired",
			schema: "websocket/alert.fired.schema.json",
			valid: map[string]any{
				"alert_id": "a-1",
				"rule_id":  "r-1",
				"severity": "critical",
				"message":  "fraud score above threshold",
				"fired_at": "2026-08-02T10:00:00Z",
			},
			invalid: map[string]any{
				"alert_id": "a-1",
				"rule_id":  "r-1",
				"severity": "apocalyptic", // not in enum
				"message":  "x",
				"fired_at": "2026-08-02T10:00:00Z",
			},
		},
		{
			name:   "anomaly.detected",
			schema: "websocket/anomaly.detected.schema.json",
			valid: map[string]any{
				"dataset":     "transactions",
				"metric":      "amount_zscore",
				"score":       4.2,
				"detected_at": "2026-08-02T10:00:00Z",
			},
			invalid: map[string]any{"dataset": "transactions"}, // missing metric/score/detected_at
		},
		{
			name:   "pipeline.status",
			schema: "websocket/pipeline.status.schema.json",
			valid: map[string]any{
				"pipeline_id": "p-1",
				"state":       "running",
				"events_in":   1000,
				"events_out":  998,
			},
			invalid: map[string]any{"pipeline_id": "p-1", "state": "vibing"}, // not in enum
		},
		{
			name:   "system.health",
			schema: "websocket/system.health.schema.json",
			valid: map[string]any{
				"service":    "clickhouse",
				"state":      "healthy",
				"checked_at": "2026-08-02T10:00:00Z",
			},
			invalid: map[string]any{"service": "clickhouse", "state": "on_fire"}, // not in enum + missing checked_at
		},
		{
			name:   "agent.status",
			schema: "websocket/agent.status.schema.json",
			valid: map[string]any{
				"agent_id":     "agent-1",
				"connected":    true,
				"last_seen_at": "2026-08-02T10:00:00Z",
			},
			invalid: map[string]any{"agent_id": "agent-1"}, // missing connected/last_seen_at
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sch := compile(t, tc.schema)
			if err := validate(t, sch, tc.valid); err != nil {
				t.Errorf("expected valid %s sample to pass: %v", tc.name, err)
			}
			if err := validate(t, sch, tc.invalid); err == nil {
				t.Errorf("expected invalid %s sample to be rejected", tc.name)
			}
		})
	}
}

// TestAllSchemasAreListedInEnvelopeEnum keeps the envelope's event enum and
// the set of payload schema files honest with each other — add a schema
// file without updating the envelope (or vice versa) and this fails.
func TestAllSchemasAreListedInEnvelopeEnum(t *testing.T) {
	envelopeBytes, err := os.ReadFile("websocket/envelope.schema.json")
	if err != nil {
		t.Fatalf("reading envelope schema: %v", err)
	}
	var envelope struct {
		Properties struct {
			Event struct {
				Enum []string `json:"enum"`
			} `json:"event"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(envelopeBytes, &envelope); err != nil {
		t.Fatalf("unmarshal envelope schema: %v", err)
	}

	entries, err := os.ReadDir("websocket")
	if err != nil {
		t.Fatalf("reading websocket schema dir: %v", err)
	}

	known := map[string]bool{}
	for _, e := range envelope.Properties.Event.Enum {
		known[e] = true
	}

	for _, entry := range entries {
		name := entry.Name()
		if name == "envelope.schema.json" || filepath.Ext(name) != ".json" {
			continue
		}
		eventName := name[:len(name)-len(".schema.json")]
		if !known[eventName] {
			t.Errorf("schema file %s has no matching entry in envelope.schema.json's event enum", name)
		}
	}
}
