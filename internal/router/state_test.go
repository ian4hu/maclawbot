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
	if err := state.AddBot(Bot{BotID: "test_account"}); err != nil {
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
	if err := state.AddBot(Bot{BotID: "test_account"}); err != nil {
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
	if err := state.AddBot(Bot{BotID: "test_account"}); err != nil {
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

// TestGetBot tests GetBot lookup and not-found path.
func TestGetBot(t *testing.T) {
	tmpFile := "/tmp/test_state_getbot.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Not found
	_, ok := state.GetBot("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent bot")
	}

	// Add bots with the same Token to test GetBotByToken
	botA := Bot{BotID: "botA", Token: "tokA", DefaultAgent: "hermes", Enabled: true}
	botB := Bot{BotID: "botB", Token: "tokB", DefaultAgent: "openclaw", Enabled: false}
	if err := state.AddBot(botA); err != nil {
		t.Fatalf("AddBot botA: %v", err)
	}
	if err := state.AddBot(botB); err != nil {
		t.Fatalf("AddBot botB: %v", err)
	}

	// Found by ID
	b, ok := state.GetBot("botA")
	if !ok {
		t.Fatal("expected botA to be found")
	}
	if b.DefaultAgent != "hermes" || b.Token != "tokA" {
		t.Errorf("unexpected botA fields: agent=%s token=%s", b.DefaultAgent, b.Token)
	}

	b2, ok := state.GetBot("botB")
	if !ok {
		t.Fatal("expected botB to be found")
	}
	if b2.Enabled {
		t.Error("expected botB to be disabled")
	}

	// Still not found after adding
	_, ok = state.GetBot("botC")
	if ok {
		t.Error("expected not found for botC")
	}
}

// TestGetBotByToken tests Token-based lookup.
func TestGetBotByToken(t *testing.T) {
	tmpFile := "/tmp/test_state_getbot_token.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Not found
	_, ok := state.GetBotByToken("unknown_tok")
	if ok {
		t.Error("expected not found for unknown token")
	}

	bot := Bot{BotID: "botX", Token: "secret_tok", DefaultAgent: "openclaw", Enabled: true}
	if err := state.AddBot(bot); err != nil {
		t.Fatalf("AddBot: %v", err)
	}

	b, ok := state.GetBotByToken("secret_tok")
	if !ok {
		t.Fatal("expected token lookup to succeed")
	}
	if b.BotID != "botX" {
		t.Errorf("expected BotID=botX, got %s", b.BotID)
	}

	// Wrong token still not found
	_, ok = state.GetBotByToken("wrong_tok")
	if ok {
		t.Error("expected not found for wrong token")
	}
}

// TestGetEnabledBots tests filtering by Enabled flag.
func TestGetEnabledBots(t *testing.T) {
	tmpFile := "/tmp/test_state_enabled.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	bots := state.GetEnabledBots()
	if len(bots) != 0 {
		t.Errorf("expected 0 enabled bots initially, got %d", len(bots))
	}

	// Default agents have no bots, ensure no false positives
	if err := state.AddBot(Bot{BotID: "disabled_bot", Token: "t1", Enabled: false}); err != nil {
		t.Fatalf("AddBot disabled: %v", err)
	}
	if err := state.AddBot(Bot{BotID: "enabled_bot", Token: "t2", Enabled: true}); err != nil {
		t.Fatalf("AddBot enabled: %v", err)
	}
	if err := state.AddBot(Bot{BotID: "another_disabled", Token: "t3", Enabled: false}); err != nil {
		t.Fatalf("AddBot another_disabled: %v", err)
	}

	enabled := state.GetEnabledBots()
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled bot, got %d", len(enabled))
	}
	if enabled[0].BotID != "enabled_bot" {
		t.Errorf("expected enabled_bot, got %s", enabled[0].BotID)
	}
}

// TestRemoveBot tests bot removal and persistence.
func TestRemoveBot(t *testing.T) {
	tmpFile := "/tmp/test_state_remove_bot.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Remove non-existent
	err := state.RemoveBot("ghost")
	if err == nil {
		t.Error("expected error removing nonexistent bot")
	}

	bot := Bot{BotID: "removeme", Token: "tok_rem", DefaultAgent: "hermes", Enabled: true}
	if err := state.AddBot(bot); err != nil {
		t.Fatalf("AddBot: %v", err)
	}

	// Verify it exists
	if _, ok := state.GetBot("removeme"); !ok {
		t.Fatal("bot should exist before removal")
	}

	// Remove
	if err := state.RemoveBot("removeme"); err != nil {
		t.Fatalf("RemoveBot: %v", err)
	}

	// Verify gone
	if _, ok := state.GetBot("removeme"); ok {
		t.Error("bot should be gone after removal")
	}

	// Simulate reload from disk
	state2 := NewState(tmpFile)
	if _, ok := state2.GetBot("removeme"); ok {
		t.Error("bot should not persist after removal")
	}
}

// TestSetBotEnabled tests enable/disable and persistence across reload.
func TestSetBotEnabled(t *testing.T) {
	tmpFile := "/tmp/test_state_bot_enabled.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	bot := Bot{BotID: "toggle_me", Token: "tok_toggle", Enabled: false}
	if err := state.AddBot(bot); err != nil {
		t.Fatalf("AddBot: %v", err)
	}

	// Reload and verify disabled
	state2 := NewState(tmpFile)
	if b, ok := state2.GetBot("toggle_me"); !ok {
		t.Fatal("bot should exist after reload")
	} else if b.Enabled {
		t.Error("expected bot to be disabled after reload")
	}

	// Enable
	if err := state.SetBotEnabled("toggle_me", true); err != nil {
		t.Fatalf("SetBotEnabled true: %v", err)
	}
	if b, ok := state.GetBot("toggle_me"); !ok || !b.Enabled {
		t.Error("expected bot enabled after SetBotEnabled(true)")
	}

	// Persisted?
	state3 := NewState(tmpFile)
	if b, ok := state3.GetBot("toggle_me"); !ok {
		t.Fatal("bot should persist")
	} else if !b.Enabled {
		t.Error("expected enabled after reload")
	}

	// Disable again
	if err := state.SetBotEnabled("toggle_me", false); err != nil {
		t.Fatalf("SetBotEnabled false: %v", err)
	}

	// Non-existent bot
	err := state.SetBotEnabled("ghost_bot", true)
	if err == nil {
		t.Error("expected error for nonexistent bot")
	}

	// Persisted disabled state
	state4 := NewState(tmpFile)
	if b, ok := state4.GetBot("toggle_me"); !ok {
		t.Fatal("bot should persist")
	} else if b.Enabled {
		t.Error("expected disabled after reload")
	}
}

// TestBotPersistence_BotLookup tests that GetBot and GetBotByToken survive persist/reload.
func TestBotPersistence_BotLookup(t *testing.T) {
	tmpFile := "/tmp/test_state_persist_lookup.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	if err := state.AddBot(Bot{BotID: "persist_bot", Token: "persist_tok", DefaultAgent: "openclaw", Enabled: true}); err != nil {
		t.Fatalf("AddBot: %v", err)
	}

	// Reload
	state2 := NewState(tmpFile)

	b, ok := state2.GetBot("persist_bot")
	if !ok {
		t.Fatal("GetBot failed after reload")
	}
	if b.Token != "persist_tok" || b.DefaultAgent != "openclaw" {
		t.Errorf("unexpected fields after reload: token=%s agent=%s", b.Token, b.DefaultAgent)
	}

	b2, ok := state2.GetBotByToken("persist_tok")
	if !ok {
		t.Fatal("GetBotByToken failed after reload")
	}
	if b2.BotID != "persist_bot" {
		t.Errorf("expected BotID=persist_bot, got %s", b2.BotID)
	}
}

// TestSaveLocked_Errors tests that save errors are handled gracefully.
func TestSaveLocked_Errors(t *testing.T) {
	// Save to a directory (not a file) should fail
	state := NewState("/tmp/cant_write_this")
	defer os.Remove("/tmp/cant_write_this")

	// Silently returns; just verify no panic occurs
	b := Bot{BotID: "test", Token: "t", Enabled: true}
	state.AddBot(b)

	// Adding another bot triggers another save
	b2 := Bot{BotID: "test2", Token: "t2", Enabled: false}
	state.AddBot(b2)
}

// TestMaskToken tests that token masking is not empty and shorter than original.
func TestMaskToken(t *testing.T) {
	masked := maskToken("verylongtokenstring12345")
	if masked == "" {
		t.Error("masked token should not be empty")
	}
	if len(masked) >= len("verylongtokenstring12345") {
		t.Error("masked token should be shorter than original")
	}
	// Short token should still produce some output
	maskedShort := maskToken("a")
	if maskedShort == "" {
		t.Error("masked short token should not be empty")
	}
}
