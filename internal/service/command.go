package service

import (
	"maclawbot/internal/event"
	"maclawbot/internal/proxy"
	"maclawbot/internal/router"
)

// CommandSubscriber handles /clawbot slash commands and publishes bot lifecycle events.
type CommandSubscriber struct {
	state   *router.State
	pm      *proxy.ProxyManager
	baseURL string
	bus     *event.Bus
}

// NewCommandSubscriber creates a new CommandSubscriber.
func NewCommandSubscriber(state *router.State, pm *proxy.ProxyManager, baseURL string, bus *event.Bus) *CommandSubscriber {
	return &CommandSubscriber{
		state:   state,
		pm:      pm,
		baseURL: baseURL,
		bus:     bus,
	}
}

// OnEvent handles MessageEvents for /clawbot commands. Implements event.Subscriber.
func (c *CommandSubscriber) OnEvent(e interface{}) {
	msg, ok := e.(event.MessageEvent)
	if !ok {
		return
	}

	// Only incoming messages
	if msg.Msg.MessageType != 1 {
		return
	}

	txt := router.ExtractText(msg.Msg.ItemList)
	if txt == "" && !hasNonZeroType(msg.Msg.ItemList) {
		return
	}

	// Only handle slash commands
	if !hasPrefix(txt, "/clawbot") {
		return
	}

	uid := msg.Msg.FromUserID
	ctx := msg.Msg.ContextToken

	result := router.ProcessCommand(c.state, txt)
	if !result.IsHandled {
		return
	}

	msg.Client.SendText(uid, result.Text, ctx)

	// Publish lifecycle events based on command action
	switch result.Action {
	case "bot_add":
		bot, exists := c.state.GetBot(result.BotID)
		if exists {
			c.bus.Publish(event.BotAddedEvent{Bot: bot})
		}
	case "bot_del":
		c.bus.Publish(event.BotRemovedEvent{BotID: result.BotID})
	case "login":
		go StartBotLogin(c.baseURL, uid, ctx, msg.Client, c.state, c.bus)
	}

	// If agent was added or removed, update running servers
	if hasPrefix(txt, "/clawbot new") || hasPrefix(txt, "/clawbot del") {
		HandleAgentChange(c.state, c.pm)
	}
}
