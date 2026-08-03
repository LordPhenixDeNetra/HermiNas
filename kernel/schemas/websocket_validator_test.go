package schemas

import "testing"

func TestLoadWebSocketSchemasValidatesRealMessages(t *testing.T) {
	v, err := LoadWebSocketSchemas()
	if err != nil {
		t.Fatalf("LoadWebSocketSchemas: %v", err)
	}

	if err := v.ValidateEnvelope("system.health", map[string]any{
		"service":    "clickhouse",
		"state":      "healthy",
		"checked_at": "2026-08-02T10:00:00Z",
	}); err != nil {
		t.Errorf("expected valid system.health payload to pass: %v", err)
	}

	if err := v.ValidateEnvelope("system.health", map[string]any{"service": "clickhouse"}); err == nil {
		t.Error("expected system.health payload missing required fields to be rejected")
	}

	if err := v.ValidateEnvelope("dashboard.data", map[string]any{
		"widget_id": "w-1",
		"rows":      []any{map[string]any{"count": 1}},
	}); err != nil {
		t.Errorf("expected valid dashboard.data payload to pass: %v", err)
	}

	if err := v.ValidateEnvelope("not.a.real.event", map[string]any{}); err == nil {
		t.Error("expected an unknown event name to be rejected")
	}
}
