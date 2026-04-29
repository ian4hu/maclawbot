// Package event provides a lightweight synchronous pub/sub event bus
// for decoupling message processing and bot lifecycle events.
package event

import (
	"sync"

	"maclawbot/internal/ilink"
	"maclawbot/internal/model"
	"maclawbot/internal/router"
)

// MessageEvent is published when a new message arrives from iLink polling.
type MessageEvent struct {
	Bot    *router.Bot
	Msg    model.Message
	Client *ilink.Client
}

// BotAddedEvent is published when a new bot is added (via command or QR login).
type BotAddedEvent struct {
	Bot router.Bot
}

// BotRemovedEvent is published when a bot is removed.
type BotRemovedEvent struct {
	BotID string
}

// Subscriber receives events from the bus. Implementations use a type switch
// to filter events they care about.
type Subscriber interface {
	OnEvent(event interface{})
}

// Bus is a synchronous publish/subscribe event bus. Subscribers are called
// in registration order within the publishing goroutine.
type Bus struct {
	mu   sync.RWMutex
	subs []Subscriber
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{}
}

// Subscribe registers a subscriber. Subscribers are called in order of registration.
func (b *Bus) Subscribe(sub Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, sub)
}

// Publish delivers an event to all subscribers synchronously in registration order.
func (b *Bus) Publish(event interface{}) {
	b.mu.RLock()
	// Snapshot subscribers to avoid holding lock during callbacks
	subs := make([]Subscriber, len(b.subs))
	copy(subs, b.subs)
	b.mu.RUnlock()

	for _, sub := range subs {
		sub.OnEvent(event)
	}
}
