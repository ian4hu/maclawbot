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
	"ilink/bot/getuploadurl":       true,  // Media upload
	"ilink/bot/get_bot_qrcode":    true,  // QR code for login
	"ilink/bot/get_qrcode_status": true,  // QR code scan status
}

// ProxyHandler handles HTTP requests from an AI gateway.
// It queues incoming messages for the router and forwards outbound messages to iLink.
// One ProxyHandler instance is shared across all accounts that use the same agent.
// Account context is determined by X-Queue-Name header or to_user_id in request body.
type ProxyHandler struct {
	pm           *ProxyManager   // Reference to proxy manager for queue lookup
	State        *router.State   // Shared state for access control
	ILinkBaseURL string          // iLink API base URL
	PollTimeout  time.Duration   // Max time to wait for messages in queue
}

// NewProxyHandler creates a new proxy handler for an agent.
// One handler instance is shared across all accounts using this agent.
// The pm reference allows queue lookup by accountID_agentName.
func NewProxyHandler(pm *ProxyManager, state *router.State, ilinkBaseURL string, pollTimeout time.Duration) *ProxyHandler {
	return &ProxyHandler{
		pm:           pm,
		State:        state,
		ILinkBaseURL: ilinkBaseURL,
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
		h.handleGetUpdates(w, r, body)
	case "ilink/bot/sendmessage":
		h.handleSendMessage(w, r, body)
	default:
		h.proxyPassthrough(w, r, ep, body)
	}
}

// handleGetUpdates implements long-polling for the gateway.
// Returns queued messages or waits until timeout for new messages.
// This allows the gateway to maintain a persistent connection waiting for messages.
// X-Queue-Name header specifies accountID_agentName to determine which queue to use.
func (h *ProxyHandler) handleGetUpdates(w http.ResponseWriter, r *http.Request, body []byte) {
	var req struct {
		GetUpdatesBuf string `json:"get_updates_buf"`
	}
	if len(body) > 0 {
		json.Unmarshal(body, &req)
	}

	// Get queue name from X-Queue-Name header (format: accountID_agentName)
	queueName := r.Header.Get("X-Queue-Name")
	if queueName == "" {
		http.Error(w, `{"ret":-1,"errmsg":"missing X-Queue-Name header"}`, http.StatusBadRequest)
		return
	}

	// Get the queue for this account+agent
	queue := h.pm.GetQueue(queueName)
	if queue == nil {
		// Queue doesn't exist yet for this account - create one lazily
		queue = h.pm.GetOrCreateQueueFromName(queueName)
	}

	// Dequeue messages with timeout (long-poll)
	msgs := queue.DequeueAll(h.PollTimeout)

	resp := map[string]interface{}{
		"ret":             0,
		"msgs":            msgs,
		"get_updates_buf": req.GetUpdatesBuf,
	}

	if len(msgs) > 0 {
		log.Printf("Proxy getupdates (%s) -> %d msgs", queueName, len(msgs))
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// handleSendMessage forwards outbound messages to iLink.
// The gateway sends replies through this endpoint.
// Token is looked up from state based on to_user_id in the request body.
func (h *ProxyHandler) handleSendMessage(w http.ResponseWriter, r *http.Request, body []byte) {
	var req map[string]interface{}
	if len(body) > 0 {
		json.Unmarshal(body, &req)
	}

	h.forwardToILink(w, r, "ilink/bot/sendmessage", req)
}

// tokenForBot looks up the iLink token for a given accountID.
// Returns empty string if account not found.
func (h *ProxyHandler) tokenForBot(accountID string) string {
	if accountID == "" {
		return ""
	}
	account, ok := h.State.GetBot(accountID)
	if !ok {
		log.Printf("Account not found: %s", accountID)
		return ""
	}
	return account.Token
}

// proxyPassthrough forwards other endpoints directly to iLink without modification.
func (h *ProxyHandler) proxyPassthrough(w http.ResponseWriter, r *http.Request, ep string, body []byte) {
	var reqBody interface{}
	if len(body) > 0 {
		json.Unmarshal(body, &reqBody)
	}
	h.forwardToILink(w, r, ep, reqBody)
}

// forwardToILink forwards a request to the real iLink API.
// Adds authentication headers and copies the response back to the gateway.
// Token is determined by: X-Queue-Name header (for getupdates) or to_user_id in body (for sendmessage).
func (h *ProxyHandler) forwardToILink(w http.ResponseWriter, r *http.Request, ep string, reqBody interface{}) {
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

	// Determine token: try X-Queue-Name header first, then fall back to to_user_id in body
	token := ""
	if queueName := r.Header.Get("X-Queue-Name"); queueName != "" {
		// Extract accountID from queueName (format: accountID_agentName)
		parts := strings.SplitN(queueName, "_", 2)
		if len(parts) >= 1 {
			token = h.tokenForBot(parts[0])
		}
	}

	// Fall back to to_user_id if no token found from header
	if token == "" {
		if reqMap, ok := reqBody.(map[string]interface{}); ok {
			if toUserID, ok := reqMap["to_user_id"].(string); ok {
				token = h.tokenForBot(toUserID)
			}
		}
	}

	if token == "" {
		log.Printf("Could not determine token for request to %s", ep)
		http.Error(w, `{"ret":-1,"errmsg":"account token not found"}`, http.StatusInternalServerError)
		return
	}

	// Set iLink authentication headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("iLink-App-Id", "")
	req.Header.Set("iLink-App-ClientVersion", ilink.ILINKCV)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-WECHAT-UIN", ilink.GenerateUIN()) // Required by iLink protocol

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
	queues       map[string]*MessageQueue // Message queues, keyed by queueName (accountID_agentName)
	state        *router.State          // Shared state for agent config
	ilinkBaseURL string                 // iLink API base URL
	pollTimeout  time.Duration          // Long-poll timeout
}

// NewProxyManager creates a new proxy manager.
func NewProxyManager(state *router.State, ilinkBaseURL string, pollTimeout int) *ProxyManager {
	return &ProxyManager{
		servers:      make(map[string]*http.Server),
		queues:       make(map[string]*MessageQueue),
		state:        state,
		ilinkBaseURL: ilinkBaseURL,
		pollTimeout:  time.Duration(pollTimeout) * time.Second,
	}
}

// GetQueue returns the message queue for a queueName (accountID_agentName), or nil if not found.
func (pm *ProxyManager) GetQueue(queueName string) *MessageQueue {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.queues[queueName]
}

// GetOrCreateQueueFromName gets or creates a queue by queueName (accountID_agentName).
func (pm *ProxyManager) GetOrCreateQueueFromName(queueName string) *MessageQueue {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if queue, exists := pm.queues[queueName]; exists {
		return queue
	}

	queue := NewMessageQueue()
	pm.queues[queueName] = queue
	return queue
}

// GetOrCreateQueue gets or creates a queue for the given accountID and agentName.
// Returns the queue keyed by "accountID_agentName".
func (pm *ProxyManager) GetOrCreateQueue(accountID, agentName string) *MessageQueue {
	queueName := accountID + "_" + agentName
	return pm.GetOrCreateQueueFromName(queueName)
}

// Enqueue adds a message to the queue for the given accountID and agentName.
// This is called when routing incoming messages to an agent.
func (pm *ProxyManager) Enqueue(accountID, agentName string, msg interface{}) {
	queue := pm.GetOrCreateQueue(accountID, agentName)
	queue.Enqueue(msg)
}

// GetActiveAgents returns names of agents with running proxy servers.
func (pm *ProxyManager) GetActiveAgents() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	names := make([]string, 0, len(pm.servers))
	for name := range pm.servers {
		names = append(names, name)
	}
	return names
}

// StartAgent starts the HTTP proxy server for an agent.
// Creates an HTTP server listening on the agent's port.
// One handler is shared across all accounts that use this agent.
func (pm *ProxyManager) StartAgent(agent router.Agent) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Skip if already running
	if _, exists := pm.servers[agent.Name]; exists {
		return nil
	}

	handler := NewProxyHandler(pm, pm.state, pm.ilinkBaseURL, pm.pollTimeout)

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
	// Note: queues are not deleted here as they may be shared across agent restarts
	// and are keyed by accountID_agentName, not just agentName

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