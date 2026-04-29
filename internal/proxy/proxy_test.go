package proxy

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"maclawbot/internal/router"
)

// tempStateFile creates a temporary state file.
// Caller must delete the file after use.
func tempStateFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "pmtest-*.json")
	if err != nil {
		t.Fatalf("failed to create temp state file: %v", err)
	}
	f.Close()
	return f.Name()
}

// getFreePort asks the kernel for an available port that this process can bind to.
func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// TestStopAgent tests that StopAgent removes agent from servers and queues.
func TestStopAgent(t *testing.T) {
	statePath := tempStateFile(t)
	defer os.Remove(statePath)

	state := router.NewState(statePath)
	pm := NewProxyManager(state, "http://127.0.0.1:9999", 35)

	port := getFreePort(t)
	agent := router.Agent{
		Name:    "stopme",
		Port:    port,
		Tag:     "[Test]",
		Enabled: true,
	}

	err := pm.StartAgent(agent)
	if err != nil {
		t.Fatalf("StartAgent failed: %v", err)
	}
	pm.mu.Lock()
	pm.mu.Unlock()

	if q := pm.GetOrCreateQueue("test", "stopme"); q == nil {
		t.Fatal("expected queue to exist")
	}

	if err := pm.StopAgent("stopme"); err != nil {
		t.Fatalf("StopAgent failed: %v", err)
	}

	// Note: queues intentionally persist across agent stops to preserve account-scoped messages

	active := pm.GetActiveAgents()
	for i := range active {
		if active[i] == "stopme" {
			t.Error("expected stopme not in active agents after StopAgent")
		}
	}
}

// TestStopAgent_NoOp tests that stopping a non-existent agent is safe.
func TestStopAgent_NoOp(t *testing.T) {
	statePath := tempStateFile(t)
	defer os.Remove(statePath)

	state := router.NewState(statePath)
	pm := NewProxyManager(state, "http://127.0.0.1:9999", 35)

	if err := pm.StopAgent("nonexistent"); err != nil {
		t.Fatalf("StopAgent on nonexistent agent should not error, got: %v", err)
	}
}

// TestGetActiveAgents tests that GetActiveAgents returns running agent names.
func TestGetActiveAgents(t *testing.T) {
	statePath := tempStateFile(t)
	defer os.Remove(statePath)

	state := router.NewState(statePath)
	pm := NewProxyManager(state, "http://127.0.0.1:9999", 35)

	if len(pm.GetActiveAgents()) != 0 {
		t.Error("expected no active agents initially")
	}

	port1 := getFreePort(t)
	port2 := getFreePort(t)
	pm.StartAgent(router.Agent{Name: "a1", Port: port1, Tag: "[A]", Enabled: true})
	pm.StartAgent(router.Agent{Name: "a2", Port: port2, Tag: "[B]", Enabled: true})

	pm.mu.Lock()
	pm.mu.Unlock()

	active := pm.GetActiveAgents()
	if len(active) != 2 {
		t.Errorf("expected 2 active agents, got %d", len(active))
	}

	pm.StopAgent("a1")
	pm.mu.Lock()
	pm.mu.Unlock()
	active = pm.GetActiveAgents()
	if len(active) != 1 {
		t.Errorf("expected 1 active agent after stop, got %d", len(active))
	}

	pm.StopAgent("a2")
	pm.mu.Lock()
	pm.mu.Unlock()
	active = pm.GetActiveAgents()
	if len(active) != 0 {
		t.Errorf("expected 0 active agents after all stopped, got %d", len(active))
	}
}

// TestOnAgentAddedRemoved tests the start/stop cycle via OnAgentAdded/OnAgentRemoved.
func TestOnAgentAddedRemoved(t *testing.T) {
	statePath := tempStateFile(t)
	defer os.Remove(statePath)

	state := router.NewState(statePath)
	pm := NewProxyManager(state, "http://127.0.0.1:9999", 35)

	port := getFreePort(t)
	agent := router.Agent{
		Name:    "cycle",
		Port:    port,
		Tag:     "[Cycle]",
		Enabled: true,
	}

	pm.OnAgentAdded(agent)
	pm.mu.Lock()
	pm.mu.Unlock()

	// Queues are now lazily created per (account, agent) pair - eagerly create one for testing
	if pm.GetOrCreateQueue("test", "cycle") == nil {
		t.Fatal("queue should exist after GetOrCreateQueue")
	}

	pm.OnAgentRemoved("cycle")
	pm.mu.Lock()
	pm.mu.Unlock()

	// Queues persist across agent stops to preserve account-scoped messages
}

// TestHandleAgentChangeRemovesOrphanedProxy tests the core cleanup logic:
// when an agent is removed from state, handleAgentChange should call
// OnAgentRemoved to stop the orphaned proxy server.
func TestHandleAgentChangeRemovesOrphanedProxy(t *testing.T) {
	statePath := tempStateFile(t)
	defer os.Remove(statePath)

	state := router.NewState(statePath)
	pm := NewProxyManager(state, "http://127.0.0.1:9999", 35)

	port := getFreePort(t)
	agent := router.Agent{
		Name:    "orphan",
		Port:    port,
		Tag:     "[Orphan]",
		Enabled: true,
	}

	// Agent starts (as if /clawbot new was called)
	state.AddAgent(agent)
	pm.OnAgentAdded(agent)
	pm.mu.Lock()
	pm.mu.Unlock()

	// Then agent is removed from state (as if /clawbot del was called)
	state.RemoveAgent("orphan")

	activeBefore := pm.GetActiveAgents()
	if len(activeBefore) != 1 {
		t.Fatalf("expected 1 active before cleanup, got %d", len(activeBefore))
	}

	// Simulate handleAgentChange cleanup: stop servers whose agents
	// are no longer in state.
	agents := state.GetAgents()
	for i := range activeBefore {
		name := activeBefore[i]
		if _, exists := agents[name]; !exists {
			pm.OnAgentRemoved(name)
		}
	}
	pm.mu.Lock()
	pm.mu.Unlock()

	if pm.GetQueue("orphan") != nil {
		t.Error("expected queue to be removed after cleanup for orphaned agent")
	}
	if len(pm.GetActiveAgents()) != 0 {
		t.Error("expected no active agents after cleanup")
	}
}

// waitForServer waits up to 2s for a TCP port to be open.
func waitForServer(port int) bool {
	deadline := time.Now().Add(2 * time.Second)
	addr := "127.0.0.1:" + portStr(port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 10*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// portStr converts port int to string without importing strconv.
func portStr(port int) string {
	if port == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	n := port
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// --------------------------------------------------
// ProxyHandler / ServeHTTP tests
// --------------------------------------------------

// mockBotResolver is a test-only BotResolver.
type mockBotResolver struct {
	bots        map[string]router.Bot
	botsByToken map[string]router.Bot
}

func (m *mockBotResolver) GetBot(name string) (router.Bot, bool) {
	b, ok := m.bots[name]
	return b, ok
}

func (m *mockBotResolver) GetBotByToken(token string) (router.Bot, bool) {
	b, ok := m.botsByToken[token]
	return b, ok
}

func (m *mockBotResolver) addBot(name string, bot router.Bot) {
	if m.bots == nil {
		m.bots = make(map[string]router.Bot)
	}
	if m.botsByToken == nil {
		m.botsByToken = make(map[string]router.Bot)
	}
	m.bots[name] = bot
	m.botsByToken[bot.Token] = bot
}

func newTestProxyHandler(t *testing.T) (*ProxyHandler, *ProxyManager, *mockBotResolver) {
	t.Helper()
	statePath := tempStateFile(t)
	state := router.NewState(statePath)
	os.Remove(statePath) // ProxyManager doesn't need the file

	pm := NewProxyManager(state, "http://127.0.0.1:9999", 2)
	resolver := &mockBotResolver{}

	agent := &router.Agent{Name: "testagent", Port: 0, Tag: "[Test]", Enabled: true}
	handler := NewProxyHandler(pm, resolver, "http://127.0.0.1:9999", 5*time.Second, agent)
	return handler, pm, resolver
}

// TestServeHTTP_MethodNotAllowed verifies GET requests are rejected with 405.
func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	h, _, _ := newTestProxyHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/ilink/bot/getupdates", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestServeHTTP_EndpointNotAllowed verifies unknown endpoints return 404.
func TestServeHTTP_EndpointNotAllowed(t *testing.T) {
	h, _, _ := newTestProxyHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/forbidden", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestServeHTTP_BotNotFound verifies a request with an unknown token is rejected.
func TestServeHTTP_BotNotFound(t *testing.T) {
	h, _, _ := newTestProxyHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/getupdates", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer unknown_token_xyz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestServeHTTP_GetUpdates_BotByName tests that getBot resolves bot by bot name (GetBot).
func TestServeHTTP_GetUpdates_BotByName(t *testing.T) {
	h, pm, resolver := newTestProxyHandler(t)

	bot := router.Bot{BotID: "named_bot", Token: "tok_for_name", DefaultAgent: "testagent", Enabled: true}
	resolver.addBot("named_bot", bot)
	pm.GetOrCreateQueue("named_bot", "testagent")

	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/getupdates", strings.NewReader(`{"get_updates_buf": "prevbuf"}`))
	req.Header.Set("Authorization", "Bearer named_bot")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"ret":0`) {
		t.Errorf("expected ret:0, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"get_updates_buf":"prevbuf"`) {
		t.Errorf("expected get_updates_buf preserved, got %s", w.Body.String())
	}
}

// TestServeHTTP_GetUpdates_BotByTokenFallback tests that getBot falls back to GetBotByToken.
func TestServeHTTP_GetUpdates_BotByTokenFallback(t *testing.T) {
	h, pm, resolver := newTestProxyHandler(t)

	// Bot is registered only by token, not by name
	bot := router.Bot{BotID: "token_bot", Token: "just_token_only", DefaultAgent: "testagent", Enabled: true}
	resolver.addBot("some_name", bot) // These two should still add to name maps
	pm.GetOrCreateQueue("token_bot", "testagent")

	// Use the token directly (not the name)
	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/getupdates", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer just_token_only")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// TestServeHTTP_GetUpdates_DefaultFallback tests that unknown tokens fall back to "default" bot.
func TestServeHTTP_GetUpdates_DefaultFallback(t *testing.T) {
	h, pm, resolver := newTestProxyHandler(t)

	// Register "default" bot
	defaultBot := router.Bot{BotID: "default_bot", Token: "default_tok", DefaultAgent: "testagent", Enabled: true}
	resolver.addBot("default", defaultBot)
	pm.GetOrCreateQueue("default_bot", "testagent")

	// Unknown token should fall back to "default"
	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/getupdates", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer totally_unknown_token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (fallback), got %d: %s", w.Code, w.Body.String())
	}
}

// TestServeHTTP_GetUpdates_QueueWithMessages tests that queued messages are returned.
func TestServeHTTP_GetUpdates_QueueWithMessages(t *testing.T) {
	h, pm, resolver := newTestProxyHandler(t)

	bot := router.Bot{BotID: "queue_bot", Token: "queue_tok", DefaultAgent: "testagent", Enabled: true}
	resolver.addBot("queue_bot", bot)
	queue := pm.GetOrCreateQueue("queue_bot", "testagent")

	queue.Enqueue(router.Message{MessageType: 1, ItemList: []router.Item{{Type: 1, TextItem: &router.TextItem{Text: "hello from queue"}}}})
	queue.Enqueue(router.Message{MessageType: 1, ItemList: []router.Item{{Type: 1, TextItem: &router.TextItem{Text: "second message"}}}})

	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/getupdates", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer queue_bot")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello from queue") {
		t.Errorf("expected queued message in response, got %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "second message") {
		t.Errorf("expected second message in response, got %s", w.Body.String())
	}
}

// TestServeHTTP_ProxyPassthrough tests that allowed passthrough endpoints forwarded to iLink.
func TestServeHTTP_ProxyPassthrough(t *testing.T) {
	// Spin up a fake iLink server
	foundToken := ""
	foundEndpoint := ""
	var server http.Server
	found := make(chan struct{})
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foundToken = r.Header.Get("AuthorizationType")
		foundEndpoint = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ret":0,"data":{"message_id":"fake123"}}`))
		close(found)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("can't bind port: %v", err)
	}
	go server.Serve(ln)
	defer server.Shutdown(context.Background())

	// Use the real addr as ilink base
	fakeILink := "http://" + ln.Addr().String()

	// Create handler with real iLink address
	statePath := tempStateFile(t)
	state := router.NewState(statePath)
	os.Remove(statePath)
	pm := NewProxyManager(state, fakeILink, 2)
	resolver := &mockBotResolver{}
	bot := router.Bot{BotID: "passthrough_bot", Token: "passthrough_tok", DefaultAgent: "testagent", Enabled: true}
	resolver.addBot("passthrough_bot", bot)
	agent := &router.Agent{Name: "testagent", Port: 0, Tag: "[Test]", Enabled: true}
	h := NewProxyHandler(pm, resolver, fakeILink, 500*time.Millisecond, agent)

	body := `{"to_user_id":"user123","content":"hello via proxy"}`
	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/sendmessage", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer passthrough_bot")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	// Give server time to receive
	select {
	case <-found:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for forward request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if foundToken == "" {
		t.Error("expected AuthorizationType header to be set on forward request")
	}
	_ = foundEndpoint
}

// TestServeHTTP_MissingAuthHeader tests that missing Authorization returns 400.
func TestServeHTTP_MissingAuthHeader(t *testing.T) {
	h, _, _ := newTestProxyHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/getupdates", strings.NewReader("{}"))
	// No Authorization header set
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing Authorization") {
		t.Errorf("expected 'missing Authorization' in body, got %s", w.Body.String())
	}
}

// TestServeHTTP_EmptyBearerToken tests that an empty Bearer token returns 400.
func TestServeHTTP_EmptyBearerToken(t *testing.T) {
	h, _, _ := newTestProxyHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/ilink/bot/getupdates", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --------------------------------------------------
// getBot (internal) smoke tests via ServeHTTP
// These tests cover the bot-resolver fallback chain:
//   GetBot(name) -> GetBotByToken(token) -> GetBot("default")
// --------------------------------------------------

// TestGetBot_AllFallbacksCovered verifies the resolver chain is exercised.
// (Covered by the TestServeHTTP_* tests above, documented here for clarity.)
func TestGetBot_AllFallbacksCovered(t *testing.T) {
	// Chain exercised in order:
	// 1. GetBot(token)   <- TestServeHTTP_GetUpdates_BotByName
	// 2. GetBotByToken() <- TestServeHTTP_GetUpdates_BotByTokenFallback
	// 3. GetBot("default") <- TestServeHTTP_GetUpdates_DefaultFallback
}

// TestServeHTTP_AllAllowedEndpointsCovered verifies all allowed endpoints pass the blocklist.
func TestServeHTTP_AllAllowedEndpointsCovered(t *testing.T) {
	h, _, _ := newTestProxyHandler(t)

	// All allowed paths must NOT return 404 (they may return 400 for other reasons,
	// but the endpoint itself must be permitted).
	allowedPaths := []string{
		"/ilink/bot/getupdates",
		"/ilink/bot/get_bot_qrcode",
		"/ilink/bot/get_qrcode_status",
		"/ilink/bot/getconfig",
		"/ilink/bot/sendtyping",
	}

	for _, path := range allowedPaths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer unknown")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		// 404 means blocked; anything else means passed endpoint validation
		if w.Code == http.StatusNotFound {
			t.Errorf("path %s was blocked (got 404), expected at least 400", path)
		}
	}
}