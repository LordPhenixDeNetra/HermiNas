// Package wshub is the WebSocket hub (M2.1) that pushes real-time events —
// dashboard widget data, system health, agent connectivity — to the React
// frontend. Two goroutines (read + write pump) run per connected client,
// the idiomatic gorilla/websocket split so a slow reader can't stall
// outgoing pings and vice versa; Hub itself only tracks membership and
// fans broadcasts out, all guarded by one mutex so registration never
// races with a broadcast.
package wshub

import (
	"encoding/json"
	"fmt"
	"sync"

	"herminas/kernel/schemas"
)

type envelope struct {
	Event   string `json:"event"`
	Payload any    `json:"payload"`
}

// Hub owns the client registry and fans broadcasts out to every connected
// client's send queue. Safe for concurrent use.
type Hub struct {
	schemas *schemas.WebSocketSchemas // nil disables outgoing schema validation

	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// New builds a Hub. Pass nil for sch to skip outgoing schema validation
// (e.g. if kernel/schemas.LoadWebSocketSchemas failed) rather than refuse
// to start — a hub that broadcasts unvalidated is still better than no
// hub, and the schemas are self-consistency checks, not the wire format.
func New(sch *schemas.WebSocketSchemas) *Hub {
	return &Hub{schemas: sch, clients: make(map[*Client]struct{})}
}

func (h *Hub) add(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

// ClientCount reports the number of currently connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast encodes event+payload as the standard envelope, validates it
// against kernel/schemas/websocket (if a validator was supplied), and
// queues it on every connected client's send channel.
func (h *Hub) Broadcast(event string, payload any) error {
	data, err := h.encode(event, payload)
	if err != nil {
		return err
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.queue(data)
	}
	return nil
}

// BroadcastSystemHealth sends a system.health event to every client —
// the supervised-service heartbeat cahier des charges §6.1 calls for on
// the Overview page.
func (h *Hub) BroadcastSystemHealth(service, state, message, checkedAtRFC3339 string) error {
	return h.Broadcast("system.health", map[string]any{
		"service":    service,
		"state":      state,
		"message":    message,
		"checked_at": checkedAtRFC3339,
	})
}

// BroadcastAgentStatus sends an agent.status event to every client. No
// live producer calls this yet — Go's control plane doesn't track
// individual agent gRPC streams today (the receiver lives in the Rust
// dataplane, rust/bootstrap/src/receiver.rs, with no connect/disconnect
// registry surfaced to Go) — see tasks-herminas.md M2.1 for the honest
// status. The event contract and hub-side plumbing are real and tested;
// wiring a real producer is future work, not a silent gap.
func (h *Hub) BroadcastAgentStatus(agentID string, connected bool, lastSeenRFC3339, hostname string) error {
	return h.Broadcast("agent.status", map[string]any{
		"agent_id":     agentID,
		"connected":    connected,
		"last_seen_at": lastSeenRFC3339,
		"hostname":     hostname,
	})
}

// send delivers a pre-encoded envelope to a single client — used for
// subscription-scoped pushes like dashboard.data, versus Broadcast's
// send-to-everyone.
func (h *Hub) send(c *Client, event string, payload any) error {
	data, err := h.encode(event, payload)
	if err != nil {
		return err
	}
	c.queue(data)
	return nil
}

func (h *Hub) encode(event string, payload any) ([]byte, error) {
	if h.schemas != nil {
		if err := h.schemas.ValidateEnvelope(event, payload); err != nil {
			return nil, fmt.Errorf("wshub: refusing to send invalid %s message: %w", event, err)
		}
	}
	data, err := json.Marshal(envelope{Event: event, Payload: payload})
	if err != nil {
		return nil, fmt.Errorf("wshub: encode envelope: %w", err)
	}
	return data, nil
}
