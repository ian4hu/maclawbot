package router

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Agent represents a configured AI agent gateway.
// Each agent has its own HTTP proxy server listening on a dedicated port.
type Agent struct {
	Name         string `json:"name"`                   // Unique identifier for the agent
	Port         int    `json:"port"`                   // Local HTTP proxy port for this agent
	Tag          string `json:"tag"`                    // Prefix added to messages in /both mode (e.g., "[Claude]")
	Enabled      bool   `json:"enabled"`                 // Whether the agent proxy is active
	DefaultAgent bool   `json:"default"`                // Whether this is the currently selected agent
}

// StatusShown tracks which WeChat users have already received the welcome message.
// Key: user ID, Value: true if shown
type StatusShown map[string]bool

// State manages agent configuration and user preferences with thread-safe access.
// All changes are persisted to a JSON file for durability across restarts.
type State struct {
	filepath    string              // Path to the persisted state file
	mu          sync.RWMutex        // Read-write lock for concurrent access
	agents      map[string]Agent   // All configured agents, keyed by name
	statusShown StatusShown         // Tracks which users have seen welcome message
}

// NewState creates a new State instance, loading persisted data if available.
// If no state file exists, initializes with default hermes and openclaw agents.
func NewState(fp string) *State {
	s := &State{
		filepath:    fp,
		agents:      make(map[string]Agent),
		statusShown: make(map[string]bool),
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
			Name:         "hermes",
			Port:         19998,
			Tag:          "[Hermes Agent]",
			Enabled:      true,
			DefaultAgent: true, // hermes is the initial default
		}
		s.agents["openclaw"] = Agent{
			Name:    "openclaw",
			Port:    19999,
			Tag:     "[OpenClaw]",
			Enabled: true,
		}
		s.saveLocked() // Call internal save that doesn't try to acquire the lock
	}
}

// load reads and parses the persisted state file.
func (s *State) load() {
	data, err := os.ReadFile(s.filepath)
	if err != nil {
		return // File doesn't exist yet, use defaults
	}

	var raw struct {
		Agents      map[string]Agent `json:"agents"`
		StatusShown StatusShown      `json:"status_shown"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return // Corrupted file, use defaults
	}

	if raw.Agents != nil {
		s.agents = raw.Agents
	}
	if raw.StatusShown != nil {
		s.statusShown = raw.StatusShown
	}
}

// save atomically writes the current state to disk.
// Uses a temporary file and rename for atomicity.
func (s *State) save() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveLocked()
}

// saveLocked writes the current state to disk. Must be called with s.mu held.
func (s *State) saveLocked() {

	data := struct {
		Agents      map[string]Agent `json:"agents"`
		StatusShown StatusShown     `json:"status_shown"`
	}{
		Agents:      s.agents,
		StatusShown: s.statusShown,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	// Write to temp file first, then rename for atomic update
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

// GetDefaultAgent returns the name of the currently selected default agent.
func (s *State) GetDefaultAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for name, agent := range s.agents {
		if agent.DefaultAgent {
			return name
		}
	}
	return "hermes" // Fallback
}

// SetDefaultAgent switches the default agent.
// All new messages will be routed to this agent.
func (s *State) SetDefaultAgent(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.agents[name]; !exists {
		return fmt.Errorf("agent %s not found", name)
	}

	// Only one agent can be default at a time
	for n, agent := range s.agents {
		if n == name {
			agent.DefaultAgent = true
		} else {
			agent.DefaultAgent = false
		}
		s.agents[n] = agent
	}
	s.saveLocked() // Call internal save since we already hold the lock
	return nil
}

// ShouldShowStatus checks if a user has already received the welcome message.
func (s *State) ShouldShowStatus(uid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.statusShown[uid]
}

// MarkStatusShown records that a user has seen the welcome message.
func (s *State) MarkStatusShown(uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusShown[uid] = true
	s.saveLocked() // Call internal save since we already hold the lock
}

// GetNextAvailablePort returns an unused port number for new agents.
// Starts scanning from 19999 and returns the next available port.
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
