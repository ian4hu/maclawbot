package router

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Bot represents a WeChat bot account.
// Each account has its own iLink token and can be independently enabled/disabled.
type Bot struct {
	BotID        string `json:"bot_id"`        // Unique identifier (iLink ToUserID of the bot)
	Token        string `json:"token"`         // iLink authentication token for this account
	DefaultAgent string `json:"default_agent"` // Default agent for this account
	Enabled      bool   `json:"enabled"`       // Whether this account's poll loop is active
}

// Agent represents a configured AI agent gateway.
// Each agent has its own HTTP proxy server listening on a dedicated port.
// Agents are shared across all accounts.
type Agent struct {
	Name    string `json:"name"`    // Unique identifier for the agent
	Port    int    `json:"port"`    // Local HTTP proxy port for this agent
	Tag     string `json:"tag"`     // Prefix tag for messages (e.g., "[Claude]")
	Enabled bool   `json:"enabled"` // Whether the agent proxy is active
}

// StatusShown tracks which WeChat users have already received the welcome message.
// Key: accountID, Value: map of userID → bool
type StatusShown map[string]map[string]bool

// State manages accounts, agents, and user preferences with thread-safe access.
// All changes are persisted to a JSON file for durability across restarts.
type State struct {
	filepath    string           // Path to the persisted state file
	mu          sync.RWMutex     // Read-write lock for concurrent access
	bots        []Bot            // All configured accounts (ordered for stable iteration)
	agents      map[string]Agent // All configured agents, keyed by name
	statusShown StatusShown      // Tracks which users have seen welcome message per account
}

// NewState creates a new State instance, loading persisted data if available.
// If no state file exists, initializes with default hermes and openclaw agents.
func NewState(fp string) *State {
	s := &State{
		filepath:    fp,
		bots:        make([]Bot, 0),
		agents:      make(map[string]Agent),
		statusShown: make(StatusShown),
	}
	s.load()
	s.ensureDefaultAgents()
	return s
}

// ensureDefaultAgents initializes the default agents (hermes and openclaw)
// only when no agents are configured (first run).
func (s *State) ensureDefaultAgents() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.agents) == 0 {
		s.agents["hermes"] = Agent{
			Name:    "hermes",
			Port:    19998,
			Tag:     "[Hermes Agent]",
			Enabled: true,
		}
		s.agents["openclaw"] = Agent{
			Name:    "openclaw",
			Port:    19999,
			Tag:     "[OpenClaw]",
			Enabled: true,
		}
		s.saveLocked()
	}
}

// load reads and parses the persisted state file.
func (s *State) load() {
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return // File doesn't exist yet, use defaults
	}

	var raw struct {
		Accounts    []Bot            `json:"accounts"`
		Agents      map[string]Agent `json:"agents"`
		StatusShown StatusShown      `json:"status_shown"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return // Corrupted file, use defaults
	}

	if raw.Accounts != nil {
		s.bots = raw.Accounts
	}
	if raw.Agents != nil {
		s.agents = raw.Agents
	}
	if raw.StatusShown != nil {
		s.statusShown = raw.StatusShown
	}
}

// save atomically writes the current state to disk.
func (s *State) save() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveLocked()
}

// saveLocked writes the current state to disk. Must be called with s.mu held.
func (s *State) saveLocked() {
	data := struct {
		Accounts    []Bot            `json:"accounts"`
		Agents      map[string]Agent `json:"agents"`
		StatusShown StatusShown      `json:"status_shown"`
	}{
		Accounts:    s.bots,
		Agents:      s.agents,
		StatusShown: s.statusShown,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	tmp := s.filepath + ".tmp"
	if err := os.WriteFile(tmp, jsonData, 0644); err != nil {
		return
	}
	os.Rename(tmp, s.filepath)
}

// GetAgents returns a copy of all configured agents.
func (s *State) GetAgents() map[string]Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]Agent)
	for k, v := range s.agents {
		result[k] = v
	}
	return result
}

// GetAgent returns the agent with the given name and whether it exists.
func (s *State) GetAgent(name string) (Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, ok := s.agents[name]
	return agent, ok
}

// AddAgent registers a new agent. Returns error if agent name already exists.
func (s *State) AddAgent(agent Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.agents[agent.Name]; exists {
		return fmt.Errorf("agent %s already exists", agent.Name)
	}
	s.agents[agent.Name] = agent
	s.saveLocked() // Call internal save since we already hold the lock
	return nil
}

// RemoveAgent deletes an agent by name.
// Default agents (hermes, openclaw) cannot be removed.
func (s *State) RemoveAgent(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Prevent removal of default agents
	if name == "hermes" || name == "openclaw" {
		return fmt.Errorf("cannot remove default agent %s", name)
	}

	if _, exists := s.agents[name]; !exists {
		return fmt.Errorf("agent %s not found", name)
	}

	delete(s.agents, name)
	s.saveLocked() // Call internal save since we already hold the lock
	return nil
}

// Bot management

// GetBots returns a copy of all configured accounts.
func (s *State) GetBots() []Bot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Bot, len(s.bots))
	copy(result, s.bots)
	return result
}

// GetEnabledBots returns all enabled accounts.
func (s *State) GetEnabledBots() []Bot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Bot
	for _, b := range s.bots {
		if b.Enabled {
			result = append(result, b)
		}
	}
	return result
}

// GetBot returns the bot with the given ID and whether it exists.
func (s *State) GetBot(botID string) (Bot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.bots {
		if b.BotID == botID {
			return b, true
		}
	}
	return Bot{}, false
}

// GetBotByToken returns the bot with the given token and whether it exists.
func (s *State) GetBotByToken(token string) (Bot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.bots {
		if b.Token == token {
			return b, true
		}
	}
	return Bot{}, false
}

// AddBot adds a new bot. Returns error if bot_id already exists.
func (s *State) AddBot(bot Bot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.bots {
		if b.BotID == bot.BotID {
			return fmt.Errorf("bot %s already exists", bot.BotID)
		}
	}
	// Default to hermes as default agent if not set
	if bot.DefaultAgent == "" {
		bot.DefaultAgent = "hermes"
	}
	s.bots = append(s.bots, bot)
	s.saveLocked()
	return nil
}

// RemoveBot removes a bot by bot_id. Returns error if not found.
func (s *State) RemoveBot(botID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.bots {
		if b.BotID == botID {
			s.bots = append(s.bots[:i], s.bots[i+1:]...)
			s.saveLocked()
			return nil
		}
	}
	return fmt.Errorf("bot %s not found", botID)
}

// SetBotDefaultAgent sets the default agent for a bot.
func (s *State) SetBotDefaultAgent(botID, agentName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.agents[agentName]; !exists {
		return fmt.Errorf("agent %s not found", agentName)
	}
	for i, b := range s.bots {
		if b.BotID == botID {
			s.bots[i].DefaultAgent = agentName
			s.saveLocked()
			return nil
		}
	}
	return fmt.Errorf("bot %s not found", botID)
}

// SetBotEnabled enables or disables a bot.
func (s *State) SetBotEnabled(botID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, b := range s.bots {
		if b.BotID == botID {
			s.bots[i].Enabled = enabled
			s.saveLocked()
			return nil
		}
	}
	return fmt.Errorf("bot %s not found", botID)
}

// GetDefaultAgentForBot returns the default agent name for a bot.
// Returns "hermes" as fallback if the bot has no default or bot doesn't exist.
func (s *State) GetDefaultAgentForBot(botID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.bots {
		if b.BotID == botID && b.DefaultAgent != "" {
			return b.DefaultAgent
		}
	}
	return "hermes"
}

// ShouldShowStatus checks if a user has already received the welcome message
// for a specific account.
func (s *State) ShouldShowStatus(accountID, userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.statusShown[accountID] == nil {
		return true
	}
	return !s.statusShown[accountID][userID]
}

// MarkStatusShown records that a user has seen the welcome message for an account.
func (s *State) MarkStatusShown(accountID, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.statusShown[accountID] == nil {
		s.statusShown[accountID] = make(map[string]bool)
	}
	s.statusShown[accountID][userID] = true
	s.saveLocked()
}

// GetNextAvailablePort returns an unused port number for new agents.
// Starts scanning from 19999 and returns the next available port after the max.
func (s *State) GetNextAvailablePort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	maxPort := 19999
	for _, agent := range s.agents {
		if agent.Port > maxPort {
			maxPort = agent.Port
		}
	}
	return maxPort + 1
}
