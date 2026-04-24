package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	ilink "maclawbot/internal/ilink"
	"maclawbot/internal/router"
)

// allowedEndpoints defines which iLink API endpoints the proxy will forward.
// This is a security measure to prevent gateway from accessing other iLink APIs.
var allowedEndpoints = map[string]bool{
	"ilink/bot/getupdates":         true,  // Long-polling for new messages
	"ilink/bot/sendmessage":       true,  // Sending replies
	"ilink/bot/getuploadurl":      true,  // Media upload
	"ilink/bot/sendtyping":        true,  // Typing indicator
	"ilink/bot/getconfig":         true,  // Bot configuration
	"ilink/bot/get_bot_qrcode":    true,  // QR code for login
	"ilink/bot/get_qrcode_status": true,  // QR code scan status
}

// ProxyHandler handles HTTP requests from an AI gateway.
// It queues incoming messages for the router and forwards outbound messages to iLink.
type ProxyHandler struct {
	Queue        *MessageQueue // Queue for incoming messages destined for this agent
	State        *router.State // Shared state for access control
	ILinkBaseURL string        // iLink API base URL
	ILinkToken   string        // iLink authentication token
	PollTimeout  time.Duration // Max time to wait for messages in queue
}

// NewProxyHandler creates a new proxy handler for an agent.
func NewProxyHandler(queue *MessageQueue, state *router.State, ilinkBaseURL, ilinkToken string, pollTimeout time.Duration) *ProxyHandler {
	return &ProxyHandler{
		Queue:        queue,
		State:        state,
		ILinkBaseURL: ilinkBaseURL,
		ILinkToken:   ilinkToken,
		PollTimeout:  pollTimeout,
	}
}

// ServeHTTP is the main entry point for HTTP requests from the gateway.
// Only POST requests to allowed endpoints are processed; others return 404.
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate endpoint is allowed
	ep := strings.TrimPrefix(r.URL.Path, "/")
	if !allowedEndpoints[ep] {
		http.Error(w, `{"ret":-1,"errmsg":"not allowed"}`, http.StatusNotFound)
		log.Printf("Proxy blocked: %s", ep)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"ret":-1,"errmsg":"read error"}`, http.StatusBadRequest)
		return
	}

	// Route to appropriate handler
	switch ep {
	case "ilink/bot/getupdates":
		h.handleGetUpdates(w, body)
	case "ilink/bot/sendmessage":
		h.handleSendMessage(w, body)
	default:
		h.proxyPassthrough(w, ep, body)
	}
}

// handleGetUpdates implements long-polling for the gateway.
// Returns queued messages or waits until timeout for new messages.
// This allows the gateway to maintain a persistent connection waiting for messages.
func (h *ProxyHandler) handleGetUpdates(w http.ResponseWriter, body []byte) {
	var req struct {
		GetUpdatesBuf string `json:"get_updates_buf"`
	}
	if len(body) > 0 {
		json.Unmarshal(body, &req)
	}

	// Dequeue messages with timeout (long-poll)
	msgs := h.Queue.DequeueAll(h.PollTimeout)

	resp := map[string]interface{}{
		"ret":             0,
		"msgs":            msgs,
		"get_updates_buf": req.GetUpdatesBuf,
	}

	if len(msgs) > 0 {
		log.Printf("Proxy getupdates -> %d msgs", len(msgs))
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// handleSendMessage forwards outbound messages to iLink.
// The gateway sends replies through this endpoint.
func (h *ProxyHandler) handleSendMessage(w http.ResponseWriter, body []byte) {
	var req map[string]interface{}
	if len(body) > 0 {
		json.Unmarshal(body, &req)
	}

	h.forwardToILink(w, "ilink/bot/sendmessage", req)
}

// proxyPassthrough forwards other endpoints directly to iLink without modification.
func (h *ProxyHandler) proxyPassthrough(w http.ResponseWriter, ep string, body []byte) {
	var reqBody interface{}
	if len(body) > 0 {
		json.Unmarshal(body, &reqBody)
	}
	h.forwardToILink(w, ep, reqBody)
}

// forwardToILink forwards a request to the real iLink API.
// Adds authentication headers and copies the response back to the gateway.
func (h *ProxyHandler) forwardToILink(w http.ResponseWriter, ep string, reqBody interface{}) {
	url := h.ILinkBaseURL + "/" + ep

	body, err := json.Marshal(reqBody)
	if err != nil {
		http.Error(w, `{"ret":-1,"errmsg":"json error"}`, http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, `{"ret":-1,"errmsg":"request error"}`, http.StatusInternalServerError)
		return
	}

	// Set iLink authentication headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("iLink-App-Id", "")
	req.Header.Set("iLink-App-ClientVersion", ilink.ILINKCV)
	req.Header.Set("Authorization", "Bearer "+h.ILinkToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Proxy forward error: %v", err)
		errResp := map[string]interface{}{"ret": -1, "errmsg": err.Error()}
		h.writeJSON(w, http.StatusBadGateway, errResp)
		return
	}
	defer resp.Body.Close()

	// Copy response headers (except hop-by-hop headers)
	for key, values := range resp.Header {
		lowKey := strings.ToLower(key)
		if lowKey == "transfer-encoding" || lowKey == "content-encoding" || lowKey == "connection" {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// writeJSON writes a JSON response with proper headers.
func (h *ProxyHandler) writeJSON(w http.ResponseWriter, code int, obj interface{}) {
	data, err := json.Marshal(obj)
	if err != nil {
		http.Error(w, "json error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(data)
}

// ProxyManager manages multiple agent proxy servers dynamically.
// It creates, starts, stops, and tracks all agent HTTP servers.
type ProxyManager struct {
	mu           sync.RWMutex          // Protects servers and queues maps
	servers      map[string]*http.Server // Active HTTP servers, keyed by agent name
	queues       map[string]*MessageQueue // Message queues, keyed by agent name
	state        *router.State          // Shared state for agent config
	ilinkBaseURL string                 // iLink API base URL
	ilinkToken   string                 // iLink authentication token
	pollTimeout  time.Duration          // Long-poll timeout
}

// NewProxyManager creates a new proxy manager.
func NewProxyManager(state *router.State, ilinkBaseURL, ilinkToken string, pollTimeout int) *ProxyManager {
	return &ProxyManager{
		servers:      make(map[string]*http.Server),
		queues:       make(map[string]*MessageQueue),
		state:        state,
		ilinkBaseURL: ilinkBaseURL,
		ilinkToken:   ilinkToken,
		pollTimeout:  time.Duration(pollTimeout) * time.Second,
	}
}

// GetQueue returns the message queue for an agent, or nil if not found.
func (pm *ProxyManager) GetQueue(agentName string) *MessageQueue {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.queues[agentName]
}

// StartAgent starts the HTTP proxy server for an agent.
// Creates a new message queue and HTTP server listening on the agent's port.
func (pm *ProxyManager) StartAgent(agent router.Agent) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Skip if already running
	if _, exists := pm.servers[agent.Name]; exists {
		return nil
	}

	queue := NewMessageQueue()
	handler := NewProxyHandler(queue, pm.state, pm.ilinkBaseURL, pm.ilinkToken, pm.pollTimeout)

	addr := fmt.Sprintf("127.0.0.1:%d", agent.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Channel to signal server is ready
	ready := make(chan error, 1)

	// Start server in background
	go func() {
		log.Printf("Starting agent %s on %s", agent.Name, addr)
		
		// Create a listener to know when server is ready
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			ready <- fmt.Errorf("failed to listen on %s: %v", addr, err)
			return
		}
		
		ready <- nil // Signal server is ready
		
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("Agent %s error: %v", agent.Name, err)
		}
	}()

	// Wait for server to be ready
	if err := <-ready; err != nil {
		return err
	}

	pm.servers[agent.Name] = srv
	pm.queues[agent.Name] = queue

	return nil
}

// StopAgent gracefully stops an agent's HTTP server.
func (pm *ProxyManager) StopAgent(agentName string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	srv, exists := pm.servers[agentName]
	if !exists {
		return nil
	}

	delete(pm.servers, agentName)
	delete(pm.queues, agentName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return srv.Shutdown(ctx)
}

// StartAll starts proxy servers for all enabled agents from state.
func (pm *ProxyManager) StartAll() {
	agents := pm.state.GetAgents()
	for _, agent := range agents {
		if agent.Enabled {
			pm.StartAgent(agent)
		}
	}
}

// StopAll gracefully stops all agent proxy servers.
func (pm *ProxyManager) StopAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for name := range pm.servers {
		if srv, ok := pm.servers[name]; ok {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(ctx)
		}
	}
	pm.servers = make(map[string]*http.Server)
	pm.queues = make(map[string]*MessageQueue)
}

// OnAgentAdded is called when a new agent is added via /clawbot new.
// Starts the proxy server for the new agent.
func (pm *ProxyManager) OnAgentAdded(agent router.Agent) {
	pm.StartAgent(agent)
}

// OnAgentRemoved is called when an agent is removed via /clawbot del.
// Stops the proxy server for the removed agent.
func (pm *ProxyManager) OnAgentRemoved(agentName string) {
	pm.StopAgent(agentName)
}
