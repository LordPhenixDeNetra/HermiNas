package schemas

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed websocket/*.json
var websocketFS embed.FS

// WebSocketSchemas validates outgoing hub messages (M2.1) against the
// envelope + per-event contracts in kernel/schemas/websocket/. It's backed
// by go:embed rather than relative file paths (unlike websocket_test.go's
// own compiler, which only needs to work from `go test`'s package-dir
// working directory) so engine/wshub can validate from any process
// working directory, embedded in the compiled binary.
type WebSocketSchemas struct {
	envelope *jsonschema.Schema
	payloads map[string]*jsonschema.Schema
}

// LoadWebSocketSchemas compiles every schema once; call it at process
// startup and reuse the result, not per-message.
func LoadWebSocketSchemas() (*WebSocketSchemas, error) {
	c := jsonschema.NewCompiler()

	envelope, err := loadEmbeddedSchema(c, "envelope.schema.json")
	if err != nil {
		return nil, err
	}

	entries, err := websocketFS.ReadDir("websocket")
	if err != nil {
		return nil, fmt.Errorf("read embedded websocket schemas: %w", err)
	}

	payloads := make(map[string]*jsonschema.Schema, len(entries)-1)
	for _, e := range entries {
		name := e.Name()
		if name == "envelope.schema.json" {
			continue
		}
		sch, err := loadEmbeddedSchema(c, name)
		if err != nil {
			return nil, err
		}
		event := name[:len(name)-len(".schema.json")]
		payloads[event] = sch
	}

	return &WebSocketSchemas{envelope: envelope, payloads: payloads}, nil
}

func loadEmbeddedSchema(c *jsonschema.Compiler, name string) (*jsonschema.Schema, error) {
	data, err := websocketFS.ReadFile("websocket/" + name)
	if err != nil {
		return nil, fmt.Errorf("read embedded schema %s: %w", name, err)
	}
	url := "embed://herminas/websocket/" + name
	if err := c.AddResource(url, bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("add schema resource %s: %w", name, err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", name, err)
	}
	return sch, nil
}

// ValidateEnvelope checks that event is a known event name and that
// payload satisfies both the envelope shape and the event-specific payload
// schema. payload is marshaled to JSON and back so the same map/struct
// values used to build the actual wire message are what get validated.
func (s *WebSocketSchemas) ValidateEnvelope(event string, payload any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	var payloadVal any
	if err := json.Unmarshal(payloadJSON, &payloadVal); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	envelopeVal := map[string]any{"event": event, "payload": payloadVal}
	envelopeJSON, err := json.Marshal(envelopeVal)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	var v any
	if err := json.Unmarshal(envelopeJSON, &v); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	if err := s.envelope.Validate(v); err != nil {
		return fmt.Errorf("envelope: %w", err)
	}

	payloadSchema, ok := s.payloads[event]
	if !ok {
		return fmt.Errorf("no payload schema registered for event %q", event)
	}
	if err := payloadSchema.Validate(payloadVal); err != nil {
		return fmt.Errorf("payload for event %q: %w", event, err)
	}
	return nil
}
