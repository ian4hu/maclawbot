package service

import (
	"maclawbot/internal/event"
	"maclawbot/internal/proxy"
	"maclawbot/internal/router"
)

// ProxySubscriber routes non-command messages to the appropriate agent proxy queue.
type ProxySubscriber struct {
	pm *proxy.ProxyManager
}

// NewProxySubscriber creates a new ProxySubscriber.
func NewProxySubscriber(pm *proxy.ProxyManager) *ProxySubscriber {
	return &ProxySubscriber{pm: pm}
}

// OnEvent handles MessageEvents by enqueuing non-command messages. Implements event.Subscriber.
func (p *ProxySubscriber) OnEvent(e interface{}) {
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

	// Skip slash commands (handled by CommandSubscriber)
	if hasPrefix(txt, "/clawbot") {
		return
	}

	p.pm.Enqueue(msg.Bot.BotID, msg.Bot.DefaultAgent, msg.Msg)
}
