package router

import (
	"os"
	"strings"
	"testing"
)

// processBotCommand receives text that still includes the "bot" keyword
// (e.g., "bot add mybot abc..." not "add mybot abc...").
// The comment in processClawbotCommand is misleading: it strips "/clawbot "
// but NOT "bot ". Tests use the actual input format.

// TestProcessBotCommand_ListEmpty tests listing with no bots configured.
func TestProcessBotCommand_ListEmpty(t *testing.T) {
	tmpFile := "/tmp/test_cmd_list.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// "bot list" — full text as received by processBotCommand from processClawbotCommand
	result := processBotCommand(state, "bot list")
	if !result.IsHandled {
		t.Error("Expected bot list to be handled")
	}
	if !strings.Contains(result.Text, "(none configured)") {
		t.Errorf("Expected '(none configured)', got: %s", result.Text)
	}
}

// TestProcessBotCommand_ListWithBots tests listing with bots configured.
func TestProcessBotCommand_ListWithBots(t *testing.T) {
	tmpFile := "/tmp/test_cmd_list_bots.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	state.AddBot(Bot{BotID: "botA", Token: "tok12345678abcdef", DefaultAgent: "hermes", Enabled: true})
	state.AddBot(Bot{BotID: "botB", Token: "xyz999999999abc", DefaultAgent: "openclaw", Enabled: false})

	result := processBotCommand(state, "bot list")
	if !result.IsHandled {
		t.Error("Expected bot list to be handled")
	}
	if !strings.Contains(result.Text, "botA") {
		t.Errorf("Expected list to contain botA, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "✅ enabled") {
		t.Errorf("Expected ✅ enabled, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "❌ disabled") {
		t.Errorf("Expected ❌ disabled, got: %s", result.Text)
	}
	if strings.Contains(result.Text, "tok12345678") {
		t.Error("Token should be masked but wasn't")
	}
}

// TestProcessBotCommand_Add tests bot add.
func TestProcessBotCommand_Add(t *testing.T) {
	tmpFile := "/tmp/test_cmd_add.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	// "bot add mybot abcxyz123456"
	result := processBotCommand(state, "bot add mybot abcxyz123456")
	if !result.IsHandled {
		t.Fatal("Expected bot add to be handled")
	}
	if !strings.Contains(result.Text, "mybot") {
		t.Errorf("Expected confirmation text to contain mybot, got: %s", result.Text)
	}
	if !strings.Contains(result.Text, "configure") {
		t.Errorf("Expected confirmation text to mention configure, got: %s", result.Text)
	}
	if result.Action != "bot_add" {
		t.Errorf("Expected action bot_add, got: %s", result.Action)
	}
	if result.BotID != "mybot" {
		t.Errorf("Expected BotID mybot, got: %s", result.BotID)
	}

	b, ok := state.GetBot("mybot")
	if !ok {
		t.Fatal("Expected mybot to be added")
	}
	if b.Token != "abcxyz123456" {
		t.Errorf("Expected token abcxyz123456, got %s", b.Token)
	}

	// Add with explicit agent
	result = processBotCommand(state, "bot add another openclaw_token openclaw")
	if !result.IsHandled {
		t.Fatal("Expected add with agent to be handled")
	}
	b2, ok := state.GetBot("another")
	if !ok {
		t.Fatal("Expected another bot to exist")
	}
	if b2.DefaultAgent != "openclaw" {
		t.Errorf("Expected default agent openclaw, got %s", b2.DefaultAgent)
	}

	// Add with non-existent agent
	result = processBotCommand(state, "bot add badbot tok nonexist")
	if !result.IsHandled {
		t.Fatal("Expected error for nonexistent agent")
	}
	if !strings.Contains(result.Text, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", result.Text)
	}

	// Duplicate add
	result = processBotCommand(state, "bot add mybot dup_token")
	if !result.IsHandled {
		t.Fatal("Expected duplicate error to be handled")
	}
	if !strings.Contains(result.Text, "already exists") {
		t.Errorf("Expected 'already exists' error, got: %s", result.Text)
	}
}

// TestProcessBotCommand_AddTooFewArgs tests add with insufficient args.
func TestProcessBotCommand_AddTooFewArgs(t *testing.T) {
	tmpFile := "/tmp/test_cmd_addfew.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	result := processBotCommand(state, "bot add")
	if !result.IsHandled {
		t.Fatal("Expected to be handled")
	}
	if !strings.Contains(result.Text, "Usage") {
		t.Errorf("Expected usage message, got: %s", result.Text)
	}

	result = processBotCommand(state, "bot add onlyone")
	if !result.IsHandled {
		t.Fatal("Expected to be handled with just bot_id")
	}
	if !strings.Contains(result.Text, "Usage") {
		t.Errorf("Expected usage message, got: %s", result.Text)
	}
}

// TestProcessBotCommand_Del tests bot del.
func TestProcessBotCommand_Del(t *testing.T) {
	tmpFile := "/tmp/test_cmd_del.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	state.AddBot(Bot{BotID: "todel", Token: "tok", Enabled: true})

	result := processBotCommand(state, "bot del todel")
	if !result.IsHandled {
		t.Fatal("Expected bot del to be handled")
	}
	if !strings.Contains(result.Text, "todel") {
		t.Errorf("Expected confirmation to mention todel, got: %s", result.Text)
	}
	if result.Action != "bot_del" || result.BotID != "todel" {
		t.Errorf("Expected action=bot_del BotID=todel, got action=%s BotID=%s", result.Action, result.BotID)
	}

	if _, ok := state.GetBot("todel"); ok {
		t.Error("Expected todel to be deleted")
	}

	// Missing bot_id
	result = processBotCommand(state, "bot del")
	if !result.IsHandled {
		t.Fatal("Expected Usage error for missing args")
	}
	if !strings.Contains(result.Text, "Usage") {
		t.Errorf("Expected Usage, got: %s", result.Text)
	}

	// Non-existent bot
	result = processBotCommand(state, "bot del nonexistent")
	if !result.IsHandled {
		t.Fatal("Expected error for nonexistent bot")
	}
	if !strings.Contains(result.Text, "not found") {
		t.Errorf("Expected 'not found', got: %s", result.Text)
	}
}

// TestProcessBotCommand_Set tests bot set (show current + update default agent).
func TestProcessBotCommand_Set(t *testing.T) {
	tmpFile := "/tmp/test_cmd_set.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	state.AddBot(Bot{BotID: "setbot", Token: "settok1234567", DefaultAgent: "hermes", Enabled: true})

	// Too few args — shows Usage
	result := processBotCommand(state, "bot set")
	if !result.IsHandled {
		t.Fatal("Expected to be handled")
	}
	if !strings.Contains(result.Text, "Usage") {
		t.Errorf("Expected Usage for missing bot_id, got: %s", result.Text)
	}

	// Bot exists — shows current settings
	result = processBotCommand(state, "bot set setbot")
	if !result.IsHandled {
		t.Fatal("Expected to be handled")
	}
	if !strings.Contains(result.Text, "setbot") {
		t.Errorf("Expected show bot info, got: %s", result.Text)
	}

	// Set default agent
	result = processBotCommand(state, "bot set setbot openclaw")
	if !result.IsHandled {
		t.Fatal("Expected bot set to be handled")
	}
	if !strings.Contains(result.Text, "openclaw") {
		t.Errorf("Expected confirmation of openclaw, got: %s", result.Text)
	}
	if got := state.GetDefaultAgentForBot("setbot"); got != "openclaw" {
		t.Errorf("Expected default agent openclaw, got %s", got)
	}

	// Set non-existent agent
	result = processBotCommand(state, "bot set setbot nonexistent")
	if !result.IsHandled {
		t.Fatal("Expected error for nonexistent agent")
	}
	if !strings.Contains(result.Text, "not found") {
		t.Errorf("Expected 'not found', got: %s", result.Text)
	}

	// Set on non-existent bot
	result = processBotCommand(state, "bot set ghostbot openclaw")
	if !result.IsHandled {
		t.Fatal("Expected error for nonexistent bot")
	}
	if !strings.Contains(result.Text, "not found") {
		t.Errorf("Expected 'not found', got: %s", result.Text)
	}
}

// TestProcessBotCommand_Setup tests bot setup.
func TestProcessBotCommand_Setup(t *testing.T) {
	tmpFile := "/tmp/test_cmd_setup.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)
	state.AddBot(Bot{BotID: "setupbot", Token: "tok", Enabled: true})

	// Too few args
	result := processBotCommand(state, "bot setup")
	if !result.IsHandled {
		t.Fatal("Expected to be handled")
	}
	if !strings.Contains(result.Text, "Usage") {
		t.Errorf("Expected Usage, got: %s", result.Text)
	}

	// Non-existent agent
	result = processBotCommand(state, "bot setup nonexistentAgent setupbot")
	if !result.IsHandled {
		t.Fatal("Expected error for nonexistent agent")
	}
	if !strings.Contains(result.Text, "not found") {
		t.Errorf("Expected 'not found', got: %s", result.Text)
	}

	// Non-existent bot
	result = processBotCommand(state, "bot setup hermes ghostbot")
	if !result.IsHandled {
		t.Fatal("Expected error for nonexistent bot")
	}
	if !strings.Contains(result.Text, "not found") {
		t.Errorf("Expected 'not found', got: %s", result.Text)
	}

	// Valid setup
	result = processBotCommand(state, "bot setup hermes setupbot")
	if !result.IsHandled {
		t.Fatal("Expected valid setup to be handled")
	}
	if result.Action != "bot_setup" {
		t.Errorf("Expected action=bot_setup, got %s", result.Action)
	}
	if result.BotID != "setupbot" {
		t.Errorf("Expected BotID=setupbot, got %s", result.BotID)
	}
	if result.AgentName != "hermes" {
		t.Errorf("Expected AgentName=hermes, got %s", result.AgentName)
	}
	if result.RestartAgent {
		t.Error("Expected RestartAgent=false without flag")
	}

	// With --restart-agent flag
	result = processBotCommand(state, "bot setup hermes setupbot --restart-agent")
	if !result.IsHandled {
		t.Fatal("Expected setup with restart to be handled")
	}
	if !result.RestartAgent {
		t.Error("Expected RestartAgent=true with flag")
	}
	if !strings.Contains(result.Text, "restarted") {
		t.Errorf("Expected restart message, got: %s", result.Text)
	}
}

// TestProcessBotCommand_Login tests "login" subcommand.
func TestProcessBotCommand_Login(t *testing.T) {
	tmpFile := "/tmp/test_cmd_login.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	result := processBotCommand(state, "bot login")
	if !result.IsHandled {
		t.Fatal("Expected login to be handled")
	}
	if result.Action != "login" {
		t.Errorf("Expected action=login, got %s", result.Action)
	}
	if !strings.Contains(result.Text, "登录二维码") {
		t.Errorf("Expected QR code text, got: %s", result.Text)
	}
}

// TestProcessBotCommand_Help tests "help" is alias for "list".
func TestProcessBotCommand_Help(t *testing.T) {
	tmpFile := "/tmp/test_cmd_help.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	result := processBotCommand(state, "bot help")
	if !result.IsHandled {
		t.Fatal("Expected help to be handled")
	}
	if !strings.Contains(result.Text, "Commands") {
		t.Errorf("Expected help text with commands, got: %s", result.Text)
	}
}

// TestProcessBotCommand_UnknownSubcommand tests unknown falls through to listBots.
func TestProcessBotCommand_UnknownSubcommand(t *testing.T) {
	tmpFile := "/tmp/test_cmd_unknown.json"
	defer os.Remove(tmpFile)

	state := NewState(tmpFile)

	result := processBotCommand(state, "bot unknown_subcmd")
	if !result.IsHandled {
		t.Fatal("Expected unknown subcommand to fall through to listBots")
	}
	if !strings.Contains(result.Text, "Commands") {
		t.Errorf("Expected commands list, got: %s", result.Text)
	}
}

// TestMaskTokenEdgeCases covers maskToken boundaries not yet tested.
func TestMaskTokenEdgeCases(t *testing.T) {
	// Empty string -> "****"
	masked := maskToken("")
	if masked != "****" {
		t.Errorf("Expected ****, got %s", masked)
	}

	// Exactly 12 chars -> "****"
	masked = maskToken("123456789012")
	if masked != "****" {
		t.Errorf("Expected ****, got %s", masked)
	}

	// 13 chars
	masked = maskToken("1234567890abc")
	if masked != "12345678...0abc" {
		t.Errorf("Expected 12345678...0abc, got %s", masked)
	}
}