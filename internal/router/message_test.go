package router

import (
	"os"
	"testing"
)

// TestProcessCommand_ClawbotHelp tests /clawbot help command.
func TestProcessCommand_ClawbotHelp(t *testing.T) {
	tmpFile := "/tmp/test_state_cmd.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Test /clawbot without subcommand shows help
	result := ProcessCommand(state, "/clawbot")
	if !result.IsHandled {
		t.Error("Expected /clawbot to be handled")
	}
	if result.Text == "" {
		t.Error("Expected help text for /clawbot")
	}
}

// TestProcessCommand_ClawbotList tests /clawbot list command.
func TestProcessCommand_ClawbotList(t *testing.T) {
	tmpFile := "/tmp/test_state_list.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	result := ProcessCommand(state, "/clawbot list")
	if !result.IsHandled {
		t.Error("Expected /clawbot list to be handled")
	}
	if result.Text == "" {
		t.Error("Expected agent list output")
	}
}

// TestProcessCommand_ClawbotSet tests /clawbot set command.
func TestProcessCommand_ClawbotSet(t *testing.T) {
	tmpFile := "/tmp/test_state_set.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	if err := state.AddBot(Bot{AccountID: "test_account"}); err != nil {
		t.Fatalf("Failed to add account: %v", err)
	}

	// Switch to openclaw
	result := ProcessCommand(state, "/clawbot set openclaw")
	if !result.IsHandled {
		t.Error("Expected /clawbot set to be handled")
	}

	// Verify default agent changed
	defaultAgent := state.GetDefaultAgentForBot("test_account")
	if defaultAgent != "openclaw" {
		t.Errorf("Expected default agent to be openclaw, got %s", defaultAgent)
	}

	// Try to set non-existent agent
	result = ProcessCommand(state, "/clawbot set nonexistent")
	if !result.IsHandled {
		t.Error("Expected /clawbot set nonexistent to be handled")
	}
	if result.Text == "" {
		t.Error("Expected error message for non-existent agent")
	}
}

// TestProcessCommand_ClawbotNew tests /clawbot new command.
func TestProcessCommand_ClawbotNew(t *testing.T) {
	tmpFile := "/tmp/test_state_new.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Create new agent with default tag
	result := ProcessCommand(state, "/clawbot new claude")
	if !result.IsHandled {
		t.Error("Expected /clawbot new to be handled")
	}

	// Verify agent was created
	agent, ok := state.GetAgent("claude")
	if !ok {
		t.Fatal("Expected claude agent to be created")
	}
	if agent.Tag != "[Claude]" {
		t.Errorf("Expected default tag [Claude], got %s", agent.Tag)
	}

	// Create new agent with custom tag
	result = ProcessCommand(state, "/clawbot new gpt4 [GPT-4]")
	if !result.IsHandled {
		t.Error("Expected /clawbot new with tag to be handled")
	}

	agent, ok = state.GetAgent("gpt4")
	if !ok {
		t.Fatal("Expected gpt4 agent to be created")
	}
	// Note: command text is lowercased, but tag preserves case in brackets
	if agent.Tag != "[[gpt-4]]" {
		t.Errorf("Expected custom tag [[gpt-4]], got %s", agent.Tag)
	}
}

// TestProcessCommand_ClawbotDel tests /clawbot del command.
func TestProcessCommand_ClawbotDel(t *testing.T) {
	tmpFile := "/tmp/test_state_del.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Create and delete an agent
	ProcessCommand(state, "/clawbot new tempagent")
	result := ProcessCommand(state, "/clawbot del tempagent")
	if !result.IsHandled {
		t.Error("Expected /clawbot del to be handled")
	}

	// Verify agent was deleted
	_, ok := state.GetAgent("tempagent")
	if ok {
		t.Error("Expected tempagent to be deleted")
	}

	// Try to delete default agent (should fail)
	result = ProcessCommand(state, "/clawbot del hermes")
	if !result.IsHandled {
		t.Error("Expected /clawbot del hermes to be handled")
	}
	if result.Text == "" {
		t.Error("Expected error message when deleting default agent")
	}
}

// TestProcessCommand_ClawbotInfo tests /clawbot info command.
func TestProcessCommand_ClawbotInfo(t *testing.T) {
	tmpFile := "/tmp/test_state_info.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Get info for default agent
	result := ProcessCommand(state, "/clawbot info hermes")
	if !result.IsHandled {
		t.Error("Expected /clawbot info to be handled")
	}
	if result.Text == "" {
		t.Error("Expected agent info output")
	}

	// Get info for non-existent agent
	result = ProcessCommand(state, "/clawbot info nonexistent")
	if !result.IsHandled {
		t.Error("Expected /clawbot info nonexistent to be handled")
	}
}

// TestProcessCommand_CaseInsensitive tests that commands are case-insensitive.
func TestProcessCommand_CaseInsensitive(t *testing.T) {
	tmpFile := "/tmp/test_state_case.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	if err := state.AddBot(Bot{AccountID: "test_account"}); err != nil {
		t.Fatalf("Failed to add account: %v", err)
	}

	// Test uppercase command
	result := ProcessCommand(state, "/CLAWBOT SET openclaw")
	if !result.IsHandled {
		t.Error("Expected uppercase command to be handled")
	}

	defaultAgent := state.GetDefaultAgentForBot("test_account")
	if defaultAgent != "openclaw" {
		t.Errorf("Expected default agent to be openclaw, got %s", defaultAgent)
	}
}

// TestProcessCommand_NonCommand tests that non-command text is not handled.
func TestProcessCommand_NonCommand(t *testing.T) {
	tmpFile := "/tmp/test_state_noncmd.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// Regular text should not be handled
	result := ProcessCommand(state, "Hello, world!")
	if result.IsHandled {
		t.Error("Expected regular text to not be handled")
	}

	// Unknown command should not be handled
	result = ProcessCommand(state, "/unknown")
	if result.IsHandled {
		t.Error("Expected unknown command to not be handled")
	}
}

// TestExtractText_TextMessage tests extracting text from text messages.
func TestExtractText_TextMessage(t *testing.T) {
	items := []Item{
		{
			Type:     MessageTypeText,
			TextItem: &TextItem{Text: "Hello, world!"},
		},
	}

	text := ExtractText(items)
	if text != "Hello, world!" {
		t.Errorf("Expected 'Hello, world!', got '%s'", text)
	}
}

// TestExtractText_VoiceMessage tests extracting transcription from voice messages.
func TestExtractText_VoiceMessage(t *testing.T) {
	items := []Item{
		{
			Type:      MessageTypeVoice,
			VoiceItem: &VoiceItem{Text: "This is a voice message"},
		},
	}

	text := ExtractText(items)
	expected := "[The user sent a voice message. Here's what they said: \"This is a voice message\"]"
	if text != expected {
		t.Errorf("Expected '%s', got '%s'", expected, text)
	}
}

// TestExtractText_MultipleItems tests extracting text from multiple items.
func TestExtractText_MultipleItems(t *testing.T) {
	items := []Item{
		{
			Type:     MessageTypeText,
			TextItem: &TextItem{Text: "First message"},
		},
		{
			Type:      MessageTypeVoice,
			VoiceItem: &VoiceItem{Text: "Voice transcription"},
		},
		{
			Type:     MessageTypeText,
			TextItem: &TextItem{Text: "Second message"},
		},
	}

	text := ExtractText(items)
	expected := "First message\n[The user sent a voice message. Here's what they said: \"Voice transcription\"]\nSecond message"
	if text != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, text)
	}
}

// TestExtractText_EmptyItems tests extracting text from empty items.
func TestExtractText_EmptyItems(t *testing.T) {
	items := []Item{}

	text := ExtractText(items)
	if text != "" {
		t.Errorf("Expected empty string, got '%s'", text)
	}
}
