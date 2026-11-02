// Package bus is EVOLV's nervous system: every event the kernel or the
// organism produces is broadcast here and fanned out to every connected
// browser, so mutations appear live without a refresh.
package bus

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// Event is a single message on the bus.
type Event struct {
	Type    string          `json:"type"`              // e.g. "agent:token", "evolution", "fs:change"
	Payload json.RawMessage `json:"payload,omitempty"` // arbitrary payload
	Time    string          `json:"time"`              // RFC3339
	Seq     uint64          `json:"seq"`               // monotonically increasing
}

// Subscriber receives events pushed by the bus.
type Subscriber struct {
	id     uint64
	ch     chan Event
	closed chan struct{}
	once   sync.Once
}

// Bus is a broadcast hub. Safe for concurrent use.
type Bus struct {
	mu      sync.RWMutex
	subs    map[uint64]*Subscriber
	nextID  uint64
	seq     uint64
	last    []Event // replay ring for late joiners
	dropped atomic.Uint64
}

const replayLen = 128

// New creates a Bus.
func New() *Bus {
	return &Bus{subs: make(map[uint64]*Subscriber), last: make([]Event, 0, replayLen)}
}

// Subscribe registers a subscriber and returns it plus a replay of recent events.
func (b *Bus) Subscribe() (*Subscriber, []Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	s := &Subscriber{id: b.nextID, ch: make(chan Event, 256), closed: make(chan struct{})}
	b.subs[s.id] = s
	replay := make([]Event, len(b.last))
	copy(replay, b.last)
	return s, replay
}

// Unsubscribe removes a subscriber.
func (b *Bus) Unsubscribe(s *Subscriber) {
	b.mu.Lock()
	if _, ok := b.subs[s.id]; ok {
		delete(b.subs, s.id)
		close(s.closed)
	}
	b.mu.Unlock()
}

// Publish emits an event to all subscribers and remembers it for replay.
func (b *Bus) Publish(eventType string, payload any) Event {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw, _ = json.Marshal(map[string]string{"error": err.Error()})
	}
	b.mu.Lock()
	b.seq++
	ev := Event{Type: eventType, Payload: raw, Time: time.Now().UTC().Format(time.RFC3339Nano), Seq: b.seq}
	b.last = append(b.last, ev)
	if len(b.last) > replayLen {
		b.last = b.last[len(b.last)-replayLen:]
	}
	targets := make([]*Subscriber, 0, len(b.subs))
	for _, s := range b.subs {
		targets = append(targets, s)
	}
	b.mu.Unlock()

	for _, s := range targets {
		select {
		case <-s.closed:
		default:
			select {
			case s.ch <- ev:
			default: // slow consumer: drop rather than stall the organism
				b.dropped.Add(1)
			}
		}
	}
	return ev
}

// Events is the receive channel of a Subscriber.
func (s *Subscriber) Events() <-chan Event { return s.ch }

// Closed signals subscription shutdown.
func (s *Subscriber) Closed() <-chan struct{} { return s.closed }

// Dropped reports events that could not be delivered because a subscriber's
// queue was full. Non-zero means a browser/plugin is falling behind.
func (b *Bus) Dropped() uint64 { return b.dropped.Load() }

// SubscriberCount reports connected subscribers (WebSocket clients, plugins).
func (b *Bus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
