// Package service provides the business logic layer for MAClawBot.
// It orchestrates message processing, agent lifecycle management,
// and bridges the poller, proxy, and state components.
package service

import (
	"fmt"
	"log"

	"maclawbot/internal/ilink"
	"maclawbot/internal/model"
	"maclawbot/internal/proxy"
	"maclawbot/internal/router"
)

// MessageService implements poller.MessageHandler to process incoming messages.
// It handles:
//   - Welcome messages for new users
//   - /clawbot commands
//   - Routing non-command messages to the appropriate agent queue
type MessageService struct {
	State *router.State
	PM    *proxy.ProxyManager
}

// NewMessageService creates a new MessageService.
func NewMessageService(state *router.State, pm *proxy.ProxyManager) *MessageService {
	return &MessageService{
		State: state,
		PM:    pm,
	}
}

// HandleMessage processes a single incoming message from iLink.
// Implements poller.MessageHandler.
func (s *MessageService) HandleMessage(bot *router.Bot, msg model.Message, client *ilink.Client) {
	accountID := msg.ToUserID
	uid := msg.FromUserID
	ctx := msg.ContextToken

	// Only process incoming messages (type 1)
	if msg.MessageType != 1 {
		return
	}

	log.Printf("Msg bot=%s from=%s... items=%d", bot.AccountID, uid[:minStr(16, len(uid))], len(msg.ItemList))

	txt := router.ExtractText(msg.ItemList)

	// Check if message has any content
	hasAny := txt != "" || hasNonZeroType(msg.ItemList)
	if !hasAny {
		return
	}

	// Show welcome message to new users (non-command messages)
	if s.State.ShouldShowStatus(accountID, uid) && !hasPrefix(txt, "/") {
		defaultAgent := s.State.GetDefaultAgentForBot(accountID)
		agent, _ := s.State.GetAgent(defaultAgent)
		welcomeMsg := fmt.Sprintf("**MAClawBot** by Github @ian4hu\n**Current agent**: **%s** (port %d)\n\n**Commands:**\n- `/clawbot` - Show clawbot help\n- `/clawbot list` - List all agents\n- `/clawbot new <name>` - Create new agent\n- `/clawbot set <name>` - Switch to agent", defaultAgent, agent.Port)
		client.SendText(uid, welcomeMsg, ctx)
		s.State.MarkStatusShown(accountID, uid)
	}

	// Handle slash commands
	if hasPrefix(txt, "/clawbot") {
		result := router.ProcessCommand(s.State, txt)
		if result.IsHandled {
			client.SendText(uid, result.Text, ctx)

			// If agent was added or removed, update running servers
			if hasPrefix(txt, "/clawbot new") || hasPrefix(txt, "/clawbot del") {
				HandleAgentChange(s.State, s.PM)
			}
			return
		}
	}

	s.PM.Enqueue(bot.AccountID, bot.DefaultAgent, msg)
}

// ---- utility functions ----

func hasNonZeroType(items []model.Item) bool {
	for _, it := range items {
		if it.Type != 0 {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func minStr(a, b int) int {
	if a < b {
		return a
	}
	return b
}
