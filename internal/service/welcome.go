package service

import (
	"fmt"

	"maclawbot/internal/event"
	"maclawbot/internal/router"
)

// WelcomeSubscriber sends a welcome message to first-time users.
// It listens for MessageEvents and only acts on non-command messages from new users.
type WelcomeSubscriber struct {
	state *router.State
}

// NewWelcomeSubscriber creates a new WelcomeSubscriber.
func NewWelcomeSubscriber(state *router.State) *WelcomeSubscriber {
	return &WelcomeSubscriber{state: state}
}

// OnEvent handles MessageEvents. Implements event.Subscriber.
func (w *WelcomeSubscriber) OnEvent(e interface{}) {
	msg, ok := e.(event.MessageEvent)
	if !ok {
		return
	}

	accountID := msg.Msg.ToUserID
	uid := msg.Msg.FromUserID
	txt := router.ExtractText(msg.Msg.ItemList)

	// Only incoming messages with content
	if msg.Msg.MessageType != 1 {
		return
	}
	if txt == "" && !hasNonZeroType(msg.Msg.ItemList) {
		return
	}

	// Show welcome message to new users (non-command messages only)
	if !w.state.ShouldShowStatus(accountID, uid) || hasPrefix(txt, "/") {
		return
	}

	defaultAgent := w.state.GetDefaultAgentForBot(accountID)
	agent, _ := w.state.GetAgent(defaultAgent)
	welcomeMsg := fmt.Sprintf("**MAClawBot** by Github @ian4hu\n**Current agent**: **%s** (port %d)\n\n**Commands:**\n- `/clawbot` - Show clawbot help\n- `/clawbot list` - List all agents\n- `/clawbot new <name>` - Create new agent\n- `/clawbot set <name>` - Switch to agent", defaultAgent, agent.Port)
	msg.Client.SendText(uid, welcomeMsg, msg.Msg.ContextToken)
	w.state.MarkStatusShown(accountID, uid)
}
