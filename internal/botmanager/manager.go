// Package botmanager manages bot lifecycle: starting/stopping poll loops
// and publishing incoming messages to the event bus.
package botmanager

import (
	"context"
	"log"
	"sync"
	"time"

	"maclawbot/internal/event"
	"maclawbot/internal/ilink"
	"maclawbot/internal/router"
)

// Manager manages bot poll loops and publishes message events to the bus.
// It implements event.Subscriber to handle bot lifecycle events (added/removed).
type Manager struct {
	mu          sync.Mutex
	state       *router.State
	baseURL     string
	pollTimeout time.Duration
	bus         *event.Bus
	cancels     map[string]context.CancelFunc // botID -> cancel
}

// New creates a new Manager.
func New(state *router.State, baseURL string, pollTimeout time.Duration, bus *event.Bus) *Manager {
	return &Manager{
		state:       state,
		baseURL:     baseURL,
		pollTimeout: pollTimeout,
		bus:         bus,
		cancels:     make(map[string]context.CancelFunc),
	}
}

// StartBot starts a poll loop for the given bot. If a loop is already running
// for this bot, it is stopped first (idempotent).
func (m *Manager) StartBot(bot router.Bot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop existing loop if any
	if cancel, ok := m.cancels[bot.BotID]; ok {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancels[bot.BotID] = cancel
	go m.runPollLoop(ctx, bot)
}

// StopBot stops the poll loop for the given bot.
func (m *Manager) StopBot(botID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, ok := m.cancels[botID]; ok {
		cancel()
		delete(m.cancels, botID)
	}
}

// StartAll starts poll loops for all enabled bots.
func (m *Manager) StartAll() {
	for _, bot := range m.state.GetEnabledBots() {
		m.StartBot(bot)
	}
}

// StopAll stops all poll loops.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for botID, cancel := range m.cancels {
		cancel()
		delete(m.cancels, botID)
	}
}

// OnEvent handles lifecycle events. Implements event.Subscriber.
func (m *Manager) OnEvent(e interface{}) {
	switch evt := e.(type) {
	case event.BotAddedEvent:
		m.StartBot(evt.Bot)
	case event.BotRemovedEvent:
		m.StopBot(evt.BotID)
	}
	// MessageEvent and others are ignored
}

// ActiveBots returns the bot IDs currently being polled.
func (m *Manager) ActiveBots() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := make([]string, 0, len(m.cancels))
	for id := range m.cancels {
		ids = append(ids, id)
	}
	return ids
}

// runPollLoop is the long-polling loop for a single bot.
// It publishes MessageEvent to the bus for each incoming message.
func (m *Manager) runPollLoop(ctx context.Context, bot router.Bot) {
	const (
		maxFails = 3
		backoff  = 30 * time.Second
	)

	httpTimeoutAdd := 5 * time.Second
	pollTimeoutHTTP := m.pollTimeout + httpTimeoutAdd

	client := ilink.NewClient(m.baseURL, bot.Token)
	buf := ""
	fails := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := client.GetUpdates(buf, pollTimeoutHTTP)
		if err != nil {
			log.Printf("getUpdates error (bot=%s): %v", bot.BotID, err)
			fails++
			fails = handleFailure(fails, maxFails, backoff)
			continue
		}

		if resp.Ret != 0 || resp.ErrCode != 0 {
			log.Printf("getUpdates err (bot=%s): ret=%d ec=%d", bot.BotID, resp.Ret, resp.ErrCode)
			fails++
			fails = handleFailure(fails, maxFails, backoff)
			continue
		}

		fails = 0
		if resp.GetUpdatesBuf != "" {
			buf = resp.GetUpdatesBuf
		}

		botPtr := &bot
		for _, msg := range resp.Msgs {
			log.Printf("Incoming message (bot=%s): fromUserId=%s", bot.BotID, msg.FromUserID)
			m.bus.Publish(event.MessageEvent{
				Bot:    botPtr,
				Msg:    msg,
				Client: client,
			})
		}
	}
}

// handleFailure manages error backoff logic.
func handleFailure(fails, maxFails int, backoff time.Duration) int {
	if fails >= maxFails {
		time.Sleep(backoff)
		return 0
	}
	time.Sleep(2 * time.Second)
	return fails
}
