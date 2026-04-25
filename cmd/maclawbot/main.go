package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"maclawbot/internal/config"
	"maclawbot/internal/ilink"
	"maclawbot/internal/proxy"
	"maclawbot/internal/router"
)

var (
	Version = "2.1.0"
)

func main() {
	// Parse command line flags
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("MAClawBot v%s\n", Version)
		return
	}

	// Load .env file if present
	envPath := getWorkDir() + "/.env"
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			log.Printf("Warning: failed to load .env: %v", err)
		}
	}

	cfg := config.Load()
	setupLogging(cfg.LogFile)

	// Initialize state management (loads persisted agents and accounts)
	state := router.NewState(cfg.StateFile)

	accounts := state.GetEnabledBots()
	if len(accounts) == 0 && cfg.ILinkToken == "" {
		log.Fatal("No ILINK_TOKEN set and no accounts configured!")
	}

	// Initialize proxy manager and start all agent servers
	pm := proxy.NewProxyManager(state, cfg.ILinkBaseURL, cfg.PollTimeout)
	pm.StartAll()

	// Log startup information
	log.Println("================================================================================")
	log.Printf("MAClawBot v%s -- dynamic agent proxy", Version)
	log.Printf("iLink: %s", cfg.ILinkBaseURL)
	log.Printf("Agents:")
	for name, agent := range state.GetAgents() {
		log.Printf("  - %s: 127.0.0.1:%d", name, agent.Port)
	}
	log.Println("================================================================================")

	// Start the poll loop(s) in background
	ctx, cancel := context.WithCancel(context.Background())
	if len(accounts) > 0 {
		for _, bot := range accounts {
			go pollLoop(ctx, bot.Token, cfg.ILinkBaseURL, state, pm, time.Duration(cfg.PollTimeout)*time.Second)
		}
	} else if cfg.ILinkToken != "" {
		go pollLoop(ctx, cfg.ILinkToken, cfg.ILinkBaseURL, state, pm, time.Duration(cfg.PollTimeout)*time.Second)
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	cancel()
	pm.StopAll()
	log.Println("Stopped")
}

// pollLoop continuously polls iLink for new messages and routes them to agents.
// Implements exponential backoff on errors to prevent hammering the API.
// One pollLoop is started per enabled account.
func pollLoop(ctx context.Context, token string, baseURL string, state *router.State, pm *proxy.ProxyManager, pollTimeout time.Duration) {
	const (
		maxFails = 3                // Max consecutive failures before backoff
		backoff  = 30 * time.Second // Backoff duration after max failures
	)

	client := ilink.NewClient(baseURL, token)

	buf := ""
	fails := 0
	pollTimeoutHTTP := pollTimeout + 5*time.Second // HTTP timeout slightly longer than iLink timeout

	for {
		select {
		case <-ctx.Done():
			return // Shutdown requested
		default:
		}

		// Poll iLink for updates
		resp, err := client.GetUpdates(buf, pollTimeoutHTTP)
		if err != nil {
			log.Printf("getUpdates error: %v", err)
			fails++
			fails = handleFailure(fails, maxFails, backoff)
			continue
		}

		// Check for API errors
		if resp.Ret != 0 || resp.ErrCode != 0 {
			log.Printf("getUpdates err: ret=%d ec=%d", resp.Ret, resp.ErrCode)
			fails++
			fails = handleFailure(fails, maxFails, backoff)
			continue
		}

		// Success - reset failure counter
		fails = 0
		if resp.GetUpdatesBuf != "" {
			buf = resp.GetUpdatesBuf // Update cursor for next poll
		}

		// Process each incoming message
		for _, msg := range resp.Msgs {
			procMsg(msg, client, state, pm)
		}
	}
}

// handleFailure manages error backoff logic.
// Returns the updated failure count.
func handleFailure(fails, maxFails int, backoff time.Duration) int {
	if fails >= maxFails {
		time.Sleep(backoff)
		return 0 // Reset after backoff
	}
	time.Sleep(2 * time.Second)
	return fails
}

// procMsg processes a single incoming message from iLink.
// Handles commands, shows welcome message to new users, and routes to agents.
func procMsg(msg router.Message, client *ilink.Client, state *router.State, pm *proxy.ProxyManager) {
	accountID := msg.ToUserID
	uid := msg.FromUserID
	ctx := msg.ContextToken

	// Only process incoming messages (type 1)
	if msg.MessageType != 1 {
		return
	}

	log.Printf("Msg from=%s... items=%d", uid[:min(16, len(uid))], len(msg.ItemList))

	txt := router.ExtractText(msg.ItemList)

	// Check if message has any content
	hasAny := txt != "" || hasNonZeroType(msg.ItemList)
	if !hasAny {
		return
	}

	// Show welcome message to new users (non-command messages)
	if state.ShouldShowStatus(accountID, uid) && !hasPrefix(txt, "/") {
		// Build welcome message directly instead of using /whoami command
		defaultAgent := state.GetDefaultAgentForBot(accountID)
		agent, _ := state.GetAgent(defaultAgent)
		welcomeMsg := fmt.Sprintf("**MAClawBot** by Github @ian4hu\n**Current agent**: **%s** (port %d)\n\n**Commands:**\n- `/clawbot` - Show clawbot help\n- `/clawbot list` - List all agents\n- `/clawbot new <name>` - Create new agent\n- `/clawbot set <name>` - Switch to agent", defaultAgent, agent.Port)
		client.SendText(uid, welcomeMsg, ctx)
		state.MarkStatusShown(accountID, uid)
	}

	// Handle slash commands
	if hasPrefix(txt, "/clawbot") {
		result := router.ProcessCommand(state, txt)
		if result.IsHandled {
			client.SendText(uid, result.Text, ctx)

			// If agent was added or removed, update running servers
			if hasPrefix(txt, "/clawbot new") || hasPrefix(txt, "/clawbot del") {
				handleAgentChange(state, pm)
			}
			return
		}
	}

	// Route message to the account's default agent
	bot, ok := state.GetBot(accountID)
	if !ok {
		bot, ok = state.GetBot("default")
	}
	if !ok {
		log.Printf("Bot not found: %s", accountID)
		return
	}
	pm.Enqueue(bot.AccountID, bot.DefaultAgent, msg)
}

// handleAgentChange ensures proxy servers match the configured agents.
// Called after /clawbot new or /clawbot del commands.
func handleAgentChange(state *router.State, pm *proxy.ProxyManager) {
	agents := state.GetAgents()

	// Start servers for agents that don't have one running
	for name, agent := range agents {
		if pm.GetQueue(name) == nil && agent.Enabled {
			pm.OnAgentAdded(agent)
		}
	}

	// Stop servers for agents that were removed from state
	for _, name := range pm.GetActiveAgents() {
		if _, exists := agents[name]; !exists {
			pm.OnAgentRemoved(name)
		}
	}
}

// Utility functions

func hasNonZeroType(items []router.Item) bool {
	for _, it := range items {
		if it.Type != 0 {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getWorkDir() string {
	execPath, _ := os.Executable()
	if execPath != "" {
		return filepath.Dir(execPath)
	}
	dir, _ := os.Getwd()
	return dir
}

func setupLogging(logFile string) {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.SetOutput(os.Stdout)
		log.Printf("Warning: failed to open log file: %v", err)
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(f)
}
