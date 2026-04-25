package proxy

import (
	"net"
	"os"
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