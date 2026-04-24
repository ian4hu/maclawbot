package ilink

import (
	"sync"
	"time"
)

// TypingManager manages typing indicators for multiple users.
// It caches typing tickets and handles automatic keepalive.
type TypingManager struct {
	client   *Client
	mu       sync.RWMutex
	tickets  map[string]*typingTicket // Cache per user
}

// typingTicket stores ticket info for a user.
type typingTicket struct {
	ticket    string
	expiresAt time.Time // Tickets are valid ~24h
	lastSent  time.Time // Last time we sent typing indicator
}

// NewTypingManager creates a new typing manager.
func NewTypingManager(client *Client) *TypingManager {
	return &TypingManager{
		client:  client,
		tickets: make(map[string]*typingTicket),
	}
}

// StartTyping sends a typing indicator to a user.
// Automatically fetches and caches ticket if needed.
func (tm *TypingManager) StartTyping(userId string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ticket, ok := tm.tickets[userId]
	
	// Check if we need to fetch a new ticket
	if !ok || time.Now().After(ticket.expiresAt) {
		// Fetch new ticket
		newTicket, err := tm.client.GetTypingTicket(userId)
		if err != nil {
			return err
		}
		ticket = &typingTicket{
			ticket:    newTicket,
			expiresAt: time.Now().Add(24 * time.Hour), // Valid for 24h
		}
		tm.tickets[userId] = ticket
	}

	// Send typing indicator
	if err := tm.client.SendTyping(userId, ticket.ticket, 1); err != nil {
		return err
	}

	ticket.lastSent = time.Now()
	return nil
}

// StopTyping stops the typing indicator for a user.
func (tm *TypingManager) StopTyping(userId string) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	ticket, ok := tm.tickets[userId]
	if !ok {
		return nil // No ticket, nothing to stop
	}

	return tm.client.SendTyping(userId, ticket.ticket, 2)
}

// Clear removes all cached tickets (e.g., on session expiry).
func (tm *TypingManager) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.tickets = make(map[string]*typingTicket)
}
