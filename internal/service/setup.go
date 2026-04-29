package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"maclawbot/internal/router"
)

// SetupAgentConfig writes bot token/baseUrl to the agent's account config file.
// agentName must be "hermes" or "openclaw".
// Returns the config file path written, or an error.
func SetupAgentConfig(bot router.Bot, agent router.Agent, iLinkBaseURL string) (string, error) {
	switch agent.Name {
	case "openclaw":
		return setupOpenClaw(bot, agent)
	case "hermes":
		return setupHermes(bot, iLinkBaseURL)
	default:
		return "", fmt.Errorf("unsupported agent: %s", agent.Name)
	}
}

// openclawConfig is the JSON structure for an openclaw account file.
type openclawConfig struct {
	BaseURL string `json:"baseUrl"`
	Token   string `json:"token"`
	SavedAt string `json:"savedAt"`
}

// setupOpenClaw writes the account config for the openclaw agent.
// File: ~/.openclaw/openclaw-weixin/accounts/{prefix}-im-bot.json
// Also updates ~/.openclaw/openclaw-weixin/accounts.json registry.
func setupOpenClaw(bot router.Bot, agent router.Agent) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home dir: %w", err)
	}

	acctDir := filepath.Join(home, ".openclaw", "openclaw-weixin", "accounts")
	if err := os.MkdirAll(acctDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create accounts dir: %w", err)
	}

	// Bot file: {prefix}-im-bot.json
	prefix := strings.TrimSuffix(bot.BotID, "@im.bot")
	botFile := filepath.Join(acctDir, prefix+"-im-bot.json")

	cfg := openclawConfig{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d/", agent.Port),
		Token:   bot.Token,
		SavedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := writeJSON(botFile, cfg); err != nil {
		return "", fmt.Errorf("cannot write openclaw config: %w", err)
	}

	// Update accounts.json registry
	if err := updateOpenClawRegistry(acctDir, prefix+"-im-bot"); err != nil {
		return "", fmt.Errorf("cannot update openclaw registry: %w", err)
	}

	return botFile, nil
}

// updateOpenClawRegistry adds an account ID to the openclaw accounts.json array if not present.
func updateOpenClawRegistry(acctDir, acctID string) error {
	regFile := acctDir + ".json"

	// Read existing registry
	var ids []string
	if data, err := os.ReadFile(regFile); err == nil {
		if err := json.Unmarshal(data, &ids); err != nil {
			// Corrupt registry, start fresh
			ids = nil
		}
	}

	// Check if already registered
	for _, id := range ids {
		if id == acctID {
			return nil
		}
	}

	ids = append(ids, acctID)
	return writeJSON(regFile, ids)
}

// hermesConfig is the JSON structure for a hermes account file.
type hermesConfig struct {
	Token   string `json:"token"`
	BaseURL string `json:"base_url"`
	SavedAt string `json:"saved_at"`
}

// setupHermes writes the account config for the hermes agent.
// File: ~/.hermes/weixin/accounts/{botId}.json
func setupHermes(bot router.Bot, iLinkBaseURL string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot find home dir: %w", err)
	}

	acctDir := filepath.Join(home, ".hermes", "weixin", "accounts")
	if err := os.MkdirAll(acctDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create accounts dir: %w", err)
	}

	// Bot file: {botId}.json
	botFile := filepath.Join(acctDir, bot.BotID+".json")

	cfg := hermesConfig{
		Token:   bot.Token,
		BaseURL: iLinkBaseURL,
		SavedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := writeJSON(botFile, cfg); err != nil {
		return "", fmt.Errorf("cannot write hermes config: %w", err)
	}

	return botFile, nil
}

// writeJSON marshals v to an indented JSON file at path, using atomic write.
func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
