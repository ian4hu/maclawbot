package router

import (
	"fmt"
	"os"
	"testing"
)

// TestNewState_CreatesDefaultAgents tests that NewState creates default agents
// when no state file exists (first run).
func TestNewState_CreatesDefaultAgents(t *testing.T) {
	// Create a temporary file path
	tmpFile := "/tmp/test_state_new.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Should have 2 default agents
	agents := state.GetAgents()
	if len(agents) != 2 {
		t.Errorf("Expected 2 default agents, got %d", len(agents))
	}

	// Check hermes agent
	hermes, ok := state.GetAgent("hermes")
	if !ok {
		t.Error("Expected hermes agent to exist")
	}
	if hermes.Port != 19998 {
		t.Errorf("Expected hermes port 19998, got %d", hermes.Port)
	}

	// Check openclaw agent
	openclaw, ok := state.GetAgent("openclaw")
	if !ok {
		t.Error("Expected openclaw agent to exist")
	}
	if openclaw.Port != 19999 {
		t.Errorf("Expected openclaw port 19999, got %d", openclaw.Port)
	}
}

// TestNewState_LoadsExistingState tests that NewState loads existing state from file.
func TestNewState_LoadsExistingState(t *testing.T) {
	tmpFile := "/tmp/test_state_load.json"
	defer os.Remove(tmpFile)

	// Create initial state
	state1 := NewState(tmpFile)
	
	// Add a custom agent
	agent := Agent{
		Name:    "claude",
		Port:    20000,
		Tag:     "[Claude]",
		Enabled: true,
	}
	if err := state1.AddAgent(agent); err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	// Create new state instance (should load from file)
	state2 := NewState(tmpFile)

	// Should have 3 agents (2 default + 1 custom)
	agents := state2.GetAgents()
	if len(agents) != 3 {
		t.Errorf("Expected 3 agents, got %d", len(agents))
	}

	// Custom agent should exist
	claude, ok := state2.GetAgent("claude")
	if !ok {
		t.Error("Expected claude agent to exist after reload")
	}
	if claude.Port != 20000 {
		t.Errorf("Expected claude port 20000, got %d", claude.Port)
	}
}

// TestAddAgent tests adding a new agent.
func TestAddAgent(t *testing.T) {
	tmpFile := "/tmp/test_state_add.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Add a new agent
	agent := Agent{
		Name:    "gpt4",
		Port:    20001,
		Tag:     "[GPT-4]",
		Enabled: true,
	}
	if err := state.AddAgent(agent); err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	// Verify agent was added
	gpt4, ok := state.GetAgent("gpt4")
	if !ok {
		t.Error("Expected gpt4 agent to exist")
	}
	if gpt4.Tag != "[GPT-4]" {
		t.Errorf("Expected tag [GPT-4], got %s", gpt4.Tag)
	}

	// Try to add duplicate agent
	err := state.AddAgent(agent)
	if err == nil {
		t.Error("Expected error when adding duplicate agent")
	}
}

// TestRemoveAgent tests removing an agent.
func TestRemoveAgent(t *testing.T) {
	tmpFile := "/tmp/test_state_remove.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Cannot remove default agent
	err := state.RemoveAgent("hermes")
	if err == nil {
		t.Error("Expected error when removing default agent hermes")
	}

	// Add and remove custom agent
	agent := Agent{
		Name:    "temp",
		Port:    20002,
		Tag:     "[Temp]",
		Enabled: true,
	}
	if err := state.AddAgent(agent); err != nil {
		t.Fatalf("Failed to add agent: %v", err)
	}

	if err := state.RemoveAgent("temp"); err != nil {
		t.Fatalf("Failed to remove agent: %v", err)
	}

	// Verify agent was removed
	_, ok := state.GetAgent("temp")
	if ok {
		t.Error("Expected temp agent to be removed")
	}

	// Remove non-existent agent
	err = state.RemoveAgent("nonexistent")
	if err == nil {
		t.Error("Expected error when removing non-existent agent")
	}
}

// TestSetDefaultAgent tests switching the default agent.
func TestSetDefaultAgent(t *testing.T) {
	tmpFile := "/tmp/test_state_default.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	if err := state.AddBot(Bot{AccountID: "test_account"}); err != nil {
		t.Fatalf("Failed to add account: %v", err)
	}

	// Initially hermes should be default (fallback)
	defaultAgent := state.GetDefaultAgentForBot("test_account")
	if defaultAgent != "hermes" {
		t.Errorf("Expected default agent to be hermes, got %s", defaultAgent)
	}

	// Switch to openclaw
	if err := state.SetBotDefaultAgent("test_account", "openclaw"); err != nil {
		t.Fatalf("Failed to set default agent: %v", err)
	}

	defaultAgent = state.GetDefaultAgentForBot("test_account")
	if defaultAgent != "openclaw" {
		t.Errorf("Expected default agent to be openclaw, got %s", defaultAgent)
	}

	// Try to set non-existent agent as default
	err := state.SetBotDefaultAgent("test_account", "nonexistent")
	if err == nil {
		t.Error("Expected error when setting non-existent agent as default")
	}
}

// TestGetNextAvailablePort tests port allocation.
func TestGetNextAvailablePort(t *testing.T) {
	tmpFile := "/tmp/test_state_port.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// First available port should be 20000 (after 19998, 19999)
	port := state.GetNextAvailablePort()
	if port != 20000 {
		t.Errorf("Expected next port 20000, got %d", port)
	}

	// Add an agent on port 20000
	agent := Agent{
		Name:    "agent1",
		Port:    20000,
		Tag:     "[Agent1]",
		Enabled: true,
	}
	state.AddAgent(agent)

	// Next port should be 20001
	port = state.GetNextAvailablePort()
	if port != 20001 {
		t.Errorf("Expected next port 20001, got %d", port)
	}
}

// TestShouldShowStatus tests welcome message tracking.
func TestShouldShowStatus(t *testing.T) {
	tmpFile := "/tmp/test_state_status.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	if err := state.AddBot(Bot{AccountID: "test_account"}); err != nil {
		t.Fatalf("Failed to add account: %v", err)
	}

	// New user should see status
	uid := "user123"
	if !state.ShouldShowStatus("test_account", uid) {
		t.Error("Expected new user to see status")
	}

	// Mark status as shown
	state.MarkStatusShown("test_account", uid)

	// User should not see status again
	if state.ShouldShowStatus("test_account", uid) {
		t.Error("Expected user to not see status after marked as shown")
	}
}

// TestConcurrentStateAccess tests concurrent access to state (no race conditions).
func TestConcurrentStateAccess(t *testing.T) {
	tmpFile := "/tmp/test_state_concurrent.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	if err := state.AddBot(Bot{AccountID: "test_account"}); err != nil {
		t.Fatalf("Failed to add account: %v", err)
	}

	done := make(chan bool)

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			state.GetAgents()
			state.GetDefaultAgentForBot("test_account")
			state.ShouldShowStatus("test_account", "user")
		}
		done <- true
	}()

	// Concurrent writes
	go func() {
		for i := 0; i < 10; i++ {
			agent := Agent{
				Name:    fmt.Sprintf("agent%d", i),
				Port:    20000 + i,
				Tag:     "[Agent]",
				Enabled: true,
			}
			state.AddAgent(agent)
			state.SetBotDefaultAgent("test_account", "hermes")
			state.MarkStatusShown("test_account", "user")
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done
}
