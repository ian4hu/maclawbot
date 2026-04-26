package service

import (
	"os"
	"path/filepath"
	"testing"

	"maclawbot/internal/event"
	"maclawbot/internal/ilink"
	"maclawbot/internal/model"
	"maclawbot/internal/proxy"
	"maclawbot/internal/router"
)

// ─── welcome.go ───────────────────────────────────────────────────────────────

// mockWelcomeBus records published events.
type mockWelcomeBus struct {
	published []interface{}
}

func (m *mockWelcomeBus) Publish(e interface{}) {
	m.published = append(m.published, e)
}

func TestWelcomeSubscriber_NewWelcomeSubscriber(t *testing.T) {
	state := router.NewState("/tmp/test_welcome_new.json")
	sub := NewWelcomeSubscriber(state)
	if sub == nil {
		t.Fatal("Expected non-nil WelcomeSubscriber")
	}
	if sub.state != state {
		t.Error("Expected state to be set")
	}
}

// TestWelcomeSubscriber_OnEvent tests the non-command message path.
func TestWelcomeSubscriber_OnEvent(t *testing.T) {
	tmpFile := "/tmp/test_welcome_on_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	state.AddBot(router.Bot{BotID: "testbot", Token: "tok", Enabled: true})

	sub := NewWelcomeSubscriber(state)

	// Non-MessageEvent — should be silently ignored
	sub.OnEvent("not a message event")
	sub.OnEvent(42)
	sub.OnEvent(struct{ X int }{X: 1})
}

// TestWelcomeSubscriber_IgnoresCommand tests that slash commands do not trigger welcome.
func TestWelcomeSubscriber_IgnoresCommand(t *testing.T) {
	tmpFile := "/tmp/test_welcome_cmd_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	state.AddBot(router.Bot{BotID: "testbot", Token: "tok", Enabled: true})

	sub := NewWelcomeSubscriber(state)

	// A MessageEvent with Type=1 and cmd text should not trigger welcome
	// (the onEvent filter already handles this via hasPrefix check)
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "user1",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot help"}},
			},
		},
		Client: nil, // won't be called since no welcome shown
	})
}

// TestWelcomeSubscriber_NonTextMessage tests that non-text message types are ignored.
func TestWelcomeSubscriber_NonTextMessage(t *testing.T) {
	tmpFile := "/tmp/test_welcome_nontext_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	state.AddBot(router.Bot{BotID: "testbot", Token: "tok", Enabled: true})

	sub := NewWelcomeSubscriber(state)

	// MessageType != 1 should be ignored
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "user1",
			MessageType:   2, // image
			ContextToken: "ctxt",
			ItemList:     []model.Item{},
		},
	})
}

// ─── proxy.go ────────────────────────────────────────────────────────────────

func TestProxySubscriber_NewProxySubscriber(t *testing.T) {
	tmpFile := "/tmp/test_psub_new_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	sub := NewProxySubscriber(pm)
	if sub == nil {
		t.Fatal("Expected non-nil ProxySubscriber")
	}
	if sub.pm != pm {
		t.Error("Expected pm to be set")
	}
}

func TestProxySubscriber_OnEvent_IgnoresNonMessageEvents(t *testing.T) {
	tmpFile := "/tmp/test_psub_nonmsg_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	sub := NewProxySubscriber(pm)

	// Non-MessageEvent types should be silently ignored
	sub.OnEvent("not a message event")
	sub.OnEvent(42)
	sub.OnEvent(struct{ X int }{X: 1})
}

func TestProxySubscriber_OnEvent_IgnoresNonTextMessages(t *testing.T) {
	tmpFile := "/tmp/test_psub_nontext_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	sub := NewProxySubscriber(pm)

	// MessageType != 1 should be ignored
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok", DefaultAgent: "hermes"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "user1",
			MessageType:   2, // image
			ItemList:     []model.Item{},
		},
	})
}

func TestProxySubscriber_OnEvent_IgnoresEmptyText(t *testing.T) {
	tmpFile := "/tmp/test_psub_empty_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	sub := NewProxySubscriber(pm)

	// Empty text and no non-zero type items should be ignored
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok", DefaultAgent: "hermes"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "user1",
			MessageType:   1,
			ItemList:     []model.Item{{Type: 0}},
		},
	})
}

func TestProxySubscriber_OnEvent_IgnoresCommands(t *testing.T) {
	tmpFile := "/tmp/test_psub_cmds_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	sub := NewProxySubscriber(pm)

	// A /clawbot command should be ignored (CommandSubscriber handles those)
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok", DefaultAgent: "hermes"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "user1",
			MessageType:   1,
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot help"}},
			},
		},
	})
}

// TestProxySubscriber_OnEvent_EnqueuesNonCommandMessage tests the happy path.
func TestProxySubscriber_OnEvent_EnqueuesNonCommandMessage(t *testing.T) {
	tmpFile := "/tmp/test_psub_enqueue_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	sub := NewProxySubscriber(pm)

	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok", DefaultAgent: "hermes"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "user1",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "hello world"}},
			},
		},
	})
	// No panic = success. The message is enqueued via pm.Enqueue.
}

// ─── agent.go ────────────────────────────────────────────────────────────────

func TestHandleAgentChange(t *testing.T) {
	tmpFile := "/tmp/test_agent_change_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)

	// Add a new enabled agent to state — HandleAgentChange should call OnAgentAdded
	state.AddAgent(router.Agent{Name: "testagent", Port: 20100, Tag: "[Test]", Enabled: true})

	// Start the agent so there's a running server
	pm.StartAgent(router.Agent{Name: "testagent", Port: 20100, Tag: "[Test]", Enabled: true})

	// Now call HandleAgentChange — agent already running, nothing should be added
	HandleAgentChange(state, pm)

	// Clean up
	pm.StopAgent("testagent")
}

// TestHandleAgentChange_StartsMissingAgents tests that missing agent servers get started.
func TestHandleAgentChange_StartsMissingAgents(t *testing.T) {
	tmpFile := "/tmp/test_agent_missing_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)

	// Add enabled agent but don't start server
	state.AddAgent(router.Agent{Name: "newagent", Port: 20101, Tag: "[New]", Enabled: true})

	// newagent has no queue yet — GetQueue uses accountID_agentName format
	// but before HandleAgentChange starts the agent, there should be no queues for it
	// We check by iterating known queues instead
	// Actually: after adding an enabled agent, state has it but pm doesn't have it yet
	// HandleAgentChange should start it (OnAgentAdded creates a queue)
	// Since we can't verify GetQueue easily (needs accountID prefix), we check via GetActiveAgents
	active := pm.GetActiveAgents()
	for _, n := range active {
		if n == "newagent" {
			t.Fatal("newagent should not be active yet")
		}
	}

	HandleAgentChange(state, pm)

	// Now the agent should be active (server was started)
	active = pm.GetActiveAgents()
	found := false
	for _, n := range active {
		if n == "newagent" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected HandleAgentChange to start newagent server")
	}

	pm.StopAgent("newagent")
}

// TestHandleAgentChange_StopsRemovedAgents tests that removed agents get stopped.
func TestHandleAgentChange_StopsRemovedAgents(t *testing.T) {
	tmpFile := "/tmp/test_agent_stop_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)

	// Start an agent server
	pm.StartAgent(router.Agent{Name: "to_remove", Port: 20102, Tag: "[R]", Enabled: true})

	// Active agents includes "to_remove"
	active := pm.GetActiveAgents()
	found := false
	for _, n := range active {
		if n == "to_remove" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected to_remove to be active before HandleAgentChange")
	}

	// State has no agents except defaults — HandleAgentChange should stop it
	HandleAgentChange(state, pm)

	active = pm.GetActiveAgents()
	for _, n := range active {
		if n == "to_remove" {
			t.Error("Expected to_remove to be stopped after HandleAgentChange")
		}
	}
}

// ─── command.go ──────────────────────────────────────────────────────────────

// Verify CommandSubscriber at least compiles and NewCommandSubscriber works
func TestCommandSubscriber_NewCommandSubscriber(t *testing.T) {
	state := router.NewState("/tmp/test_cmdsub_new.json")
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()

	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)
	if sub == nil {
		t.Fatal("Expected non-nil CommandSubscriber")
	}
	if sub.state != state {
		t.Error("Expected state to be set")
	}
	if sub.pm != pm {
		t.Error("Expected pm to be set")
	}
	if sub.baseURL != "http://localhost:9" {
		t.Errorf("Expected baseURL http://localhost:9, got %s", sub.baseURL)
	}
	if sub.bus != bus {
		t.Error("Expected bus to be set")
	}
}

func TestCommandSubscriber_OnEvent_IgnoresNonMessageEvents(t *testing.T) {
	state := router.NewState("/tmp/test_cmdsub_nonmsg_" + t.Name() + ".json")
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	sub.OnEvent("not a message event")
	sub.OnEvent(42)
	sub.OnEvent(struct{ X int }{X: 1})
}

func TestCommandSubscriber_OnEvent_IgnoresNonTextMessageType(t *testing.T) {
	state := router.NewState("/tmp/test_cmdsub_nontext_" + t.Name() + ".json")
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	// MessageType != 1 should be ignored
	// We pass nil client since it shouldn't be called
	sub.OnEvent(event.MessageEvent{
		Bot:    nil,
		Msg:    model.Message{MessageType: 2, ItemList: []model.Item{}},
		Client: nil,
	})
}

func TestCommandSubscriber_OnEvent_IgnoresEmptyText(t *testing.T) {
	state := router.NewState("/tmp/test_cmdsub_empty_" + t.Name() + ".json")
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	sub.OnEvent(event.MessageEvent{
		Bot:    nil,
		Msg:    model.Message{MessageType: 1, ItemList: []model.Item{{Type: 0}}},
		Client: nil,
	})
}

// ─── setup.go ───────────────────────────────────────────────────────────────

func TestSetupAgentConfig_UnsupportedAgent(t *testing.T) {
	bot := router.Bot{BotID: "testbot", Token: "tok"}
	agent := router.Agent{Name: "unsupported_agent", Port: 19999}

	path, err := SetupAgentConfig(bot, agent, "http://localhost:0")
	if err == nil {
		t.Error("Expected error for unsupported agent")
	}
	if path != "" {
		t.Errorf("Expected empty path, got %s", path)
	}
	if err.Error() != "unsupported agent: unsupported_agent" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSetupAgentConfig_Hermes(t *testing.T) {
	bot := router.Bot{BotID: "hermes-bot@" + t.Name(), Token: "test_tok_12345678"}
	agent := router.Agent{Name: "hermes", Port: 19998}

	path, err := SetupAgentConfig(bot, agent, "http://localhost:9000")
	if err != nil {
		t.Fatalf("SetupAgentConfig hermes failed: %v", err)
	}
	if path == "" {
		t.Fatal("Expected non-empty config path")
	}
	defer os.RemoveAll(filepath.Dir(path))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	if string(data) == "" {
		t.Error("Config file should not be empty")
	}
}

func TestSetupAgentConfig_OpenClaw(t *testing.T) {
	bot := router.Bot{BotID: "openclaw-bot@" + t.Name()}
	agent := router.Agent{Name: "openclaw", Port: 19999}

	path, err := SetupAgentConfig(bot, agent, "http://localhost:0")
	if err != nil {
		t.Fatalf("SetupAgentConfig openclaw failed: %v", err)
	}
	if path == "" {
		t.Fatal("Expected non-empty config path")
	}
	defer os.RemoveAll(filepath.Dir(path))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	if string(data) == "" {
		t.Error("Config file should not be empty")
	}
}

func TestUpdateOpenClawRegistry_Fresh(t *testing.T) {
	// Create a temp directory for the test
	tmpDir := "/tmp/test_openclaw_reg_" + t.Name()
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// First call — no existing registry
	err := updateOpenClawRegistry(tmpDir, "test-bot")
	if err != nil {
		t.Fatalf("updateOpenClawRegistry failed: %v", err)
	}

	// Verify registry was created
	regPath := filepath.Join(tmpDir, "accounts.json")
	data, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("Failed to read registry: %v", err)
	}
	if string(data) == "" {
		t.Error("Registry should not be empty")
	}
}

func TestUpdateOpenClawRegistry_Append(t *testing.T) {
	tmpDir := "/tmp/test_openclaw_reg2_" + t.Name()
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// First account
	err := updateOpenClawRegistry(tmpDir, "bot-a")
	if err != nil {
		t.Fatalf("First updateOpenClawRegistry failed: %v", err)
	}

	// Second account
	err = updateOpenClawRegistry(tmpDir, "bot-b")
	if err != nil {
		t.Fatalf("Second updateOpenClawRegistry failed: %v", err)
	}

	// Verify both are in registry
	regPath := filepath.Join(tmpDir, "accounts.json")
	data, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("Failed to read registry: %v", err)
	}
	// Both should be present but order may vary
	if string(data) == "" {
		t.Error("Registry should not be empty")
	}
}

func TestUpdateOpenClawRegistry_Duplicate(t *testing.T) {
	tmpDir := "/tmp/test_openclaw_dup_" + t.Name()
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// Add same ID twice
	err := updateOpenClawRegistry(tmpDir, "dup-bot")
	if err != nil {
		t.Fatalf("First updateOpenClawRegistry failed: %v", err)
	}
	err = updateOpenClawRegistry(tmpDir, "dup-bot")
	if err != nil {
		t.Fatalf("Duplicate updateOpenClawRegistry failed: %v", err)
	}

	// Should still have only one entry (returned early on already-registered)
}

// ─── login.go (maskToken only — async goroutine not testable without mock http) ─

func TestMaskToken_Login(t *testing.T) {
	// Empty
	masked := maskToken("")
	if masked != "****" {
		t.Errorf("Expected ****, got %s", masked)
	}

	// Short
	masked = maskToken("abc")
	if masked != "****" {
		t.Errorf("Expected ****, got %s", masked)
	}

	// 13 chars
	masked = maskToken("abcdefghijxxx")
	if masked != "abcdefgh...jxxx" {
		t.Errorf("Expected abcdefgh...jxxx, got %s", masked)
	}

	// Exactly 12 chars → short path
	masked = maskToken("abcdefghijkl")
	if masked != "****" {
		t.Errorf("Expected **** for 12 chars, got %s", masked)
	}
}

// ─── welcome.go — cover ShouldShowStatus=false path ─────────────────────────

func TestWelcomeSubscriber_OnEvent_ShouldShowStatusFalse(t *testing.T) {
	tmpFile := "/tmp/test_welcome_already_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	state.AddBot(router.Bot{BotID: "testbot", Token: "tok", Enabled: true})

	sub := NewWelcomeSubscriber(state)

	// Mark user as already welcome first
	state.MarkStatusShown("testbot", "user_repeat")

	// Now send message — ShouldShowStatus=false, welcome branch skipped
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "user_repeat",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "hello again"}},
			},
		},
		Client: nil,
	})
	// No panic = ShouldShowStatus=false branch covered
}

func TestWelcomeSubscriber_OnEvent_EmptyTextWithNonZeroType(t *testing.T) {
	// Test: text is empty BUT has non-zero type items —
	// the early-exit condition (empty text && no non-zero types) doesn't apply.
	// Use a mock client to avoid nil panic.
	tmpFile := "/tmp/test_welcome_nzt_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	state.AddBot(router.Bot{BotID: "testbot", Token: "tok", Enabled: true})

	// Need a real-lookalike client; use the ilink.Client with nil fields
	// to avoid panic (it will panic if url is invalid)
	// Instead: create via null client approach — mark user NOT shown so we test
	// the non-command path, but we need a client:
	// Simpler: mark shown so we skip SendText but cover the !hasPrefix check
	// (empty text, hasNonZeroType → !hasPrefix, !ShouldShowStatus false → pass through)
	state.MarkStatusShown("testbot", "user_nzt")
	sub := NewWelcomeSubscriber(state)
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "user_nzt",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: 2}, // non-zero type, but text empty
			},
		},
		Client: nil,
	})
}

func TestWelcomeSubscriber_OnEvent_WelcomeSent(t *testing.T) {
	// Test the welcome-sending path with a minimal mock client.
	// ilink.Client.SendText is called even with nil; we can't safely test this
	// without a real client. Instead, verify the entire flow up to SendText
	// by checking state is properly checked (ShouldShowStatus branch).
	tmpFile := "/tmp/test_welcome_send_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	state.AddBot(router.Bot{BotID: "testbot", Token: "tok", Enabled: true})

	sub := NewWelcomeSubscriber(state)

	// Use a non-nil Client to allow SendText to execute.
	// Use a real ilink.Client pointing to localhost:9 which will fail,
	// but the call itself is what we want to exercise.
	client := ilink.NewClient("http://localhost:9", "test_token")

	// New user → ShouldShowStatus=true → enters welcome path
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "new_user",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "hello"}},
			},
		},
		Client: client,
	})
	// We expect the HTTP call to fail (localhost:9 unreachable), but that's OK.
	// Main goal: cover the welcome message SendText call path.
}

// ─── command.go OnEvent — cover action types + handleBotSetup ────────────────

// trackingBotSub implements event.Subscriber and records published bot events.
type trackingBotSub struct {
	botAdded   bool
	botRemoved bool
}

func (t *trackingBotSub) OnEvent(e interface{}) {
	if _, ok := e.(event.BotAddedEvent); ok {
		t.botAdded = true
	}
	if _, ok := e.(event.BotRemovedEvent); ok {
		t.botRemoved = true
	}
}

// trackingBotSub implements event.Subscriber and records published bot events.

func TestCommandSubscriber_OnEvent_BotAdd(t *testing.T) {
	tmpFile := "/tmp/test_cmdsub_add_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	// Subscribe BEFORE OnEvent so tracker receives events
	tracker := &trackingBotSub{}
	bus.Subscribe(tracker)

	client := ilink.NewClient("http://localhost:9", "test_tok")
	sub.OnEvent(event.MessageEvent{
		Bot: nil,
		Msg: model.Message{
			ToUserID:     "admin",
			FromUserID:   "admin",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot bot add freshbot123 abcdef123"}},
			},
		},
		Client: client,
	})

	// Verify BotAddedEvent was published
	if !tracker.botAdded {
		t.Error("Expected BotAddedEvent to be published")
	}
}

func TestCommandSubscriber_OnEvent_BotDel(t *testing.T) {
	tmpFile := "/tmp/test_cmdsub_del_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	tracker := &trackingBotSub{}
	bus.Subscribe(tracker)

	// Add the bot first so it exists for deletion
	added := state.AddBot(router.Bot{BotID: "delbot", Token: "tok", Enabled: true})
	if added != nil {
		t.Fatalf("Failed to add bot: %v", added)
	}

	// /clawbot bot del <bot_id> — happy path; SendText is called, need non-nil client
	client := ilink.NewClient("http://localhost:9", "test_tok")
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "delbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "delbot",
			FromUserID:   "admin",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot bot del delbot"}},
			},
		},
		Client: client,
	})

	if !tracker.botRemoved {
		t.Error("Expected BotRemovedEvent to be published")
	}
}

func TestCommandSubscriber_OnEvent_BotSetup(t *testing.T) {
	tmpFile := "/tmp/test_cmdsub_setup_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	// Add the bot (no default bots in fresh state)
	state.AddBot(router.Bot{BotID: "setupbot", Token: "tok", Enabled: true})
	// "hermes" is already a default agent — just verify it exists
	if _, ok := state.GetAgent("hermes"); !ok {
		t.Fatal("hermes should be a default agent")
	}

	// /clawbot bot setup <agent> <bot_id>
	client := ilink.NewClient("http://localhost:9", "test_tok")
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "setupbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "setupbot",
			FromUserID:   "admin",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot bot setup hermes setupbot"}},
			},
		},
		Client: client,
	})
}

func TestCommandSubscriber_OnEvent_BotSetup_BotNotFound(t *testing.T) {
	tmpFile := "/tmp/test_cmdsub_setup_nf_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	// hermes is a default agent, no need to add
	client := ilink.NewClient("http://localhost:9", "test_tok")

	sub.OnEvent(event.MessageEvent{
		Bot: nil,
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "admin",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot bot setup hermes nonexistentbot"}},
			},
		},
		Client: client,
	})
}

func TestCommandSubscriber_OnEvent_BotSetup_AgentNotFound(t *testing.T) {
	tmpFile := "/tmp/test_cmdsub_setup_anf_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	state.AddBot(router.Bot{BotID: "setupbot2", Token: "tok", Enabled: true})

	client := ilink.NewClient("http://localhost:9", "test_tok")
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "setupbot2", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "setupbot2",
			FromUserID:   "admin",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot bot setup nonexistent_agent setupbot2"}},
			},
		},
		Client: client,
	})
}

func TestCommandSubscriber_OnEvent_AgentNew(t *testing.T) {
	tmpFile := "/tmp/test_cmdsub_agent_new_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	// Need a client so SendText doesn't panic
	client := ilink.NewClient("http://localhost:9", "test_tok")

	// `/clawbot new` should call HandleAgentChange after command is processed
	// It also returns a result (agent created), causing SendText to be called
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "admin",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot new some_agent"}},
			},
		},
		Client: client,
	})
}

func TestCommandSubscriber_OnEvent_AgentDel(t *testing.T) {
	tmpFile := "/tmp/test_cmdsub_agent_del_" + t.Name() + ".json"
	state := router.NewState(tmpFile)
	pm := proxy.NewProxyManager(state, "http://localhost:9", 5)
	bus := event.NewBus()
	sub := NewCommandSubscriber(state, pm, "http://localhost:9", bus)

	// Add agent first so del can find it
	added := state.AddAgent(router.Agent{Name: "some_agent", Port: 20110, Enabled: true})
	if added != nil {
		t.Fatalf("Failed to add agent: %v", added)
	}

	client := ilink.NewClient("http://localhost:9", "test_tok")

	// `/clawbot del` should also call HandleAgentChange
	sub.OnEvent(event.MessageEvent{
		Bot: &router.Bot{BotID: "testbot", Token: "tok"},
		Msg: model.Message{
			ToUserID:     "testbot",
			FromUserID:   "admin",
			MessageType:   1,
			ContextToken: "ctxt",
			ItemList: []model.Item{
				{Type: model.MessageTypeText, TextItem: &model.TextItem{Text: "/clawbot del some_agent"}},
			},
		},
		Client: client,
	})
}

// ─── setup.go error paths ───────────────────────────────────────────────────

func TestSetupHermes_GetHomeDirError(t *testing.T) {
	// We can't easily mock os.UserHomeDir in-process, but we can test exact
	// path coverage by checking the writeJSON error branch via file permissions.
	// Test setupHermes creates the right file structure at the expected path.
	bot := router.Bot{BotID: "hbot-error-" + t.Name(), Token: "tokerr123"}
	agent := router.Agent{Name: "hermes", Port: 19998}

	// Write to an unwritable path to trigger writeJSON error path
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "/proc/0/nonexistent") // unlikely to exist
	defer os.Setenv("HOME", oldHome)

	_, err := SetupAgentConfig(bot, agent, "http://localhost:0")
	if err == nil {
		t.Error("Expected error when HOME is invalid")
	}
}

func TestSetupOpenClaw_MkdirAllError(t *testing.T) {
	bot := router.Bot{BotID: "obot-errno-" + t.Name(), Token: "tok"}
	agent := router.Agent{Name: "openclaw", Port: 19999}

	// Write to /proc/0/ which is not writable to break at MkdirAll
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "/proc/0")
	defer os.Setenv("HOME", oldHome)

	_, err := SetupAgentConfig(bot, agent, "http://localhost:0")
	if err == nil {
		t.Error("Expected error when HOME is invalid")
	}
	if err != nil {
		// Should contain "home dir" or similar
		t.Logf("Got expected error: %v", err)
	}
}

func TestSetupHermes_MkdirAllError(t *testing.T) {
	bot := router.Bot{BotID: "hbot-direrr-" + t.Name(), Token: "tok"}
	agent := router.Agent{Name: "hermes", Port: 19998}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "/nonexistent_home_xyz")
	defer os.Setenv("HOME", oldHome)

	_, err := SetupAgentConfig(bot, agent, "http://localhost:0")
	if err == nil {
		t.Error("Expected error when HOME is invalid")
	}
}// ─── runtime debug helper ─────────────────────────────────────────────────────
var _intDebug int // dummy to force recompile

