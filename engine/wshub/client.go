package wshub

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1 << 16 // 64 KiB: control messages are small (SQL + options), not data payloads
	sendQueueSize  = 32
)

// Client wraps one WebSocket connection: an outbound queue drained by
// writePump, and whatever widget subscriptions this connection currently
// owns.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	out  chan []byte

	subs *subscriptionSet
	rl   *messageRateLimiter
}

func newClient(hub *Hub, conn *websocket.Conn, runner QueryRunner, userID string) *Client {
	c := &Client{
		hub:  hub,
		conn: conn,
		out:  make(chan []byte, sendQueueSize),
		rl:   newMessageRateLimiter(20), // 20 control msgs/sec/client: generous for UI churn, tight enough to stop a scripted flood
	}
	c.subs = newSubscriptionSet(c, runner, userID)
	return c
}

// queue enqueues a pre-encoded message. If the client's outbound buffer is
// already full (a stalled reader on the other end), the connection is
// closed instead of blocking the caller or buffering unboundedly; readPump
// and writePump both observe the resulting error and unwind.
func (c *Client) queue(data []byte) {
	select {
	case c.out <- data:
	default:
		_ = c.conn.Close()
	}
}

func (c *Client) readPump() {
	defer func() {
		c.subs.cancelAll()
		c.hub.remove(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if !c.rl.allow() {
			continue // drop the message; a spammy client is throttled, not disconnected
		}
		c.handleControlMessage(data)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case data, ok := <-c.out:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// controlMessage is what a client sends *to* the hub — subscribe to or
// drop a dashboard widget. Everything the hub sends back goes through
// envelope (hub.go) instead.
type controlMessage struct {
	Action      string `json:"action"` // "subscribe" | "unsubscribe"
	WidgetID    string `json:"widget_id"`
	SQL         string `json:"sql"`
	IntervalMS  int    `json:"interval_ms"`
	Mode        string `json:"mode"`         // "" (full refresh) | "incremental"
	SinceColumn string `json:"since_column"` // required for incremental mode
}

func (c *Client) handleControlMessage(data []byte) {
	var msg controlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("wshub: dropping malformed control message: %v", err)
		return
	}
	switch msg.Action {
	case "subscribe":
		c.subs.subscribe(msg)
	case "unsubscribe":
		c.subs.unsubscribe(msg.WidgetID)
	default:
		log.Printf("wshub: unknown control message action %q", msg.Action)
	}
}
