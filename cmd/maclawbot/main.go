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
	"maclawbot/internal/poller"
	"maclawbot/internal/proxy"
	"maclawbot/internal/router"
	"maclawbot/internal/service"
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

	// Initialize state management (loads persisted agents and bots)
	state := router.NewState(cfg.StateFile)

	bots := state.GetEnabledBots()
	if len(bots) == 0 && cfg.ILinkToken == "" {
		log.Fatal("No ILINK_TOKEN set and no bots configured!")
	}

	_, hasDefaultBot := state.GetBot("default")
	_, tokenUsed := state.GetBotByToken(cfg.ILinkToken)

	if cfg.ILinkBaseURL != "" && !hasDefaultBot && !tokenUsed {
		log.Println("No default bot configured, using ILINK_TOKEN create default bot")
		state.AddBot(router.Bot{
			BotID:        "default",
			DefaultAgent: "",
			Enabled:      true,
			Token:        cfg.ILinkToken,
		})
		bots = state.GetEnabledBots()
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

	// Create message service (bridges poller, state, and proxy)
	msgService := service.NewMessageService(state, pm, cfg.ILinkBaseURL)

	// Start the poll loop(s) in background — one per enabled bot
	ctx, cancel := context.WithCancel(context.Background())
	for _, bot := range bots {
		p := poller.New(&bot, cfg.ILinkBaseURL, msgService, time.Duration(cfg.PollTimeout)*time.Second)
		go p.Run(ctx)
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
