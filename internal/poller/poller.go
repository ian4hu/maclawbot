// Package poller provides message polling from iLink with exponential backoff.
// It is responsible only for fetching messages and passing them to a handler -
// it has no knowledge of state, routing, or agent management.
package poller

import (
	"context"
	"log"
	"time"

	"maclawbot/internal/ilink"
	"maclawbot/internal/model"
	"maclawbot/internal/router"
)

// MessageHandler is the callback invoked for each incoming message.
// The handler receives the bot context, the message, and the iLink client
// for sending replies. All routing, state, and agent logic is handled by
// the implementation.
type MessageHandler interface {
	HandleMessage(bot *router.Bot, msg model.Message, client *ilink.Client)
}

// Poller polls iLink for new messages on behalf of a single bot account.
// It handles error backoff and cursor management internally.
type Poller struct {
	Bot            *router.Bot
	Client         *ilink.Client
	Handler        MessageHandler
	PollTimeout    time.Duration
	HTTPTimeoutAdd time.Duration // Added to poll timeout for HTTP request
}

// New creates a new Poller for the given bot.
func New(bot *router.Bot, baseURL string, handler MessageHandler, pollTimeout time.Duration) *Poller {
	return &Poller{
		Bot:            bot,
		Client:         ilink.NewClient(baseURL, bot.Token),
		Handler:        handler,
		PollTimeout:    pollTimeout,
		HTTPTimeoutAdd: 5 * time.Second,
	}
}

// Run starts the polling loop. It blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	const (
		maxFails = 3                // Max consecutive failures before backoff
		backoff  = 30 * time.Second // Backoff duration after max failures
	)

	pollTimeoutHTTP := p.PollTimeout + p.HTTPTimeoutAdd
	buf := ""
	fails := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := p.Client.GetUpdates(buf, pollTimeoutHTTP)
		if err != nil {
			log.Printf("getUpdates error: %v", err)
			fails++
			fails = handleFailure(fails, maxFails, backoff)
			continue
		}

		if resp.Ret != 0 || resp.ErrCode != 0 {
			log.Printf("getUpdates err: ret=%d ec=%d", resp.Ret, resp.ErrCode)
			fails++
			fails = handleFailure(fails, maxFails, backoff)
			continue
		}

		fails = 0
		if resp.GetUpdatesBuf != "" {
			buf = resp.GetUpdatesBuf
		}

		for _, msg := range resp.Msgs {
			p.Handler.HandleMessage(p.Bot, msg, p.Client)
		}
	}
}

// handleFailure manages error backoff logic.
// Returns the updated failure count.
func handleFailure(fails, maxFails int, backoff time.Duration) int {
	if fails >= maxFails {
		time.Sleep(backoff)
		return 0 // Reset after backoff
	}
	time.Sleep(2 * time.Second)
	return fails
}
