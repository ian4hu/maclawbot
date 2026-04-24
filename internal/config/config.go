package config

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// Config holds all configuration for MAClawBot.
// Configuration is loaded from environment variables.
type Config struct {
	ILinkBaseURL string // iLink API base URL
	ILinkToken   string // iLink authentication token
	StateFile    string // Path to persistent state file
	LogFile      string // Path to log file
	PollTimeout  int    // Long-poll timeout in seconds

	config map[string]string // Raw config map (unused, kept for future)
}

var (
	cfg  *Config
	once sync.Once
)

// Load returns the singleton Config instance.
// Environment variables are loaded on first call.
func Load() *Config {
	once.Do(func() {
		cfg = &Config{
			config: make(map[string]string),
		}
		cfg.loadFromEnv()
	})
	return cfg
}

// loadFromEnv reads configuration from environment variables with defaults.
func (c *Config) loadFromEnv() {
	c.ILinkBaseURL = getEnv("ILINK_BASE_URL", "https://ilinkai.weixin.qq.com")
	c.ILinkToken = getEnv("ILINK_TOKEN", "")
	c.StateFile = getEnv("STATE_FILE", filepath.Join(getWorkDir(), "maclawbot_state.json"))
	c.LogFile = getEnv("LOG_FILE", filepath.Join(getWorkDir(), "maclawbot.log"))
	c.PollTimeout, _ = strconv.Atoi(getEnv("LONG_POLL_TIMEOUT", "35"))
}

// getEnv returns environment variable value or default if not set.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getWorkDir returns the directory containing the executable.
func getWorkDir() string {
	execPath, _ := os.Executable()
	if execPath != "" {
		return filepath.Dir(execPath)
	}
	return "."
}
