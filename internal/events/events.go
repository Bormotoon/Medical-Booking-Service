package events

import (
    "sync"
    "time"
)

// Event represents a lightweight domain event.
type Event struct {
    ID        int64
    Type      string
    Payload   []byte
    CreatedAt time.Time
    Processed bool
}

// EventHandler reacts to an event.
type EventHandler func(event Event) error

// EventBus provides in-process pub/sub for events.
type EventBus struct {
    subscribers map[string][]EventHandler
    mu          sync.RWMutex
}

// NewEventBus constructs an empty bus.
func NewEventBus() *EventBus {
    return &EventBus{subscribers: make(map[string][]EventHandler)}
}

// Subscribe registers a handler for a given event type.
func (b *EventBus) Subscribe(eventType string, handler EventHandler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.subscribers[eventType] = append(b.subscribers[eventType], handler)
}

// Publish notifies subscribers of the event type.
func (b *EventBus) Publish(event Event) {
    b.mu.RLock()
    handlers := append([]EventHandler(nil), b.subscribers[event.Type]...)
    b.mu.RUnlock()

    if event.CreatedAt.IsZero() {
        event.CreatedAt = time.Now()
    }

    for _, handler := range handlers {
        // Handlers run synchronously; caller decides concurrency model.
        _ = handler(event)
    }
}
