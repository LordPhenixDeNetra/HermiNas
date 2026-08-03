// Package bus is HermiNas' internal Go event bus (cahier des charges
// §5.3.1: "bus/ — Event bus interne Go (channels)"). It decouples
// producers — the supervisor's health loop, alerting, anomaly events
// forwarded from the Rust data plane's StreamEvents — from consumers, chief
// among them the WebSocket hub (M2.1) that turns bus events into the
// envelopes described by kernel/schemas/websocket/*.json.
package bus

import (
	"sync"
)

// Topic names match the WebSocket event names 1:1 (see
// kernel/schemas/websocket/envelope.schema.json's event enum), so a
// producer's Publish(bus.TopicSystemHealth, status) maps directly onto the
// "system.health" envelope the hub sends to React.
type Topic string

const (
	TopicDashboardData   Topic = "dashboard.data"
	TopicQueryRows       Topic = "query.rows"
	TopicAlertFired      Topic = "alert.fired"
	TopicAnomalyDetected Topic = "anomaly.detected"
	TopicPipelineStatus  Topic = "pipeline.status"
	TopicSystemHealth    Topic = "system.health"
	TopicAgentStatus     Topic = "agent.status"
)

// Unsubscribe removes a subscription and closes its channel. Safe to call
// more than once.
type Unsubscribe func()

type subscriber struct {
	id int
	ch chan any
}

// Bus is a simple, in-process pub/sub: Publish never blocks on a slow
// subscriber — a subscriber whose buffer is full has that message dropped
// (and counted) rather than stalling every other subscriber and the
// publisher. WebSocket clients are exactly the kind of consumer this
// protects against: one laggy browser tab must not back up alerting.
type Bus struct {
	// A single exclusive mutex, not RWMutex: Publish's fan-out and
	// Unsubscribe's close must never interleave for the same channel (that
	// would panic with "send on closed channel"), and a plain Mutex is the
	// simplest way to guarantee that. This bus carries dashboard/alerting/
	// health events, not the Rust data plane's hot path — the brief
	// per-subscriber critical section during Publish is not a bottleneck
	// here.
	mu          sync.Mutex
	subscribers map[Topic][]subscriber
	bufferSize  int
	nextID      int

	droppedMu sync.Mutex
	dropped   map[Topic]uint64
}

func New(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = 16
	}
	return &Bus{
		subscribers: make(map[Topic][]subscriber),
		bufferSize:  bufferSize,
		dropped:     make(map[Topic]uint64),
	}
}

// Subscribe returns a receive-only channel of events published to topic
// from now on, and a function to unsubscribe and release it. Callers must
// call Unsubscribe when done to avoid leaking the channel and its slot in
// the subscriber list.
func (b *Bus) Subscribe(topic Topic) (<-chan any, Unsubscribe) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	sub := subscriber{id: id, ch: make(chan any, b.bufferSize)}
	b.subscribers[topic] = append(b.subscribers[topic], sub)

	var once sync.Once
	unsub := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			subs := b.subscribers[topic]
			for i, s := range subs {
				if s.id == id {
					b.subscribers[topic] = append(subs[:i], subs[i+1:]...)
					close(s.ch)
					break
				}
			}
		})
	}

	return sub.ch, unsub
}

// Publish fans event out to every current subscriber of topic. Delivery is
// best-effort per subscriber: a full buffer means that subscriber misses
// this event (see Dropped) rather than the publisher blocking.
func (b *Bus) Publish(topic Topic, event any) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, s := range b.subscribers[topic] {
		select {
		case s.ch <- event:
		default:
			b.droppedMu.Lock()
			b.dropped[topic]++
			b.droppedMu.Unlock()
		}
	}
}

// Dropped reports how many events were dropped for topic because some
// subscriber's buffer was full at publish time. Exposed for the
// Prometheus metrics engine/health will register (cahier §5.3.1 health/).
func (b *Bus) Dropped(topic Topic) uint64 {
	b.droppedMu.Lock()
	defer b.droppedMu.Unlock()
	return b.dropped[topic]
}

// SubscriberCount reports how many active subscribers topic currently has.
func (b *Bus) SubscriberCount(topic Topic) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subscribers[topic])
}
