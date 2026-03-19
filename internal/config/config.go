package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Config holds the application configuration.
type Config struct {
	mu            sync.RWMutex
	Language      string
	Output        string
	Raw           bool
	Hotkey        string
	Mode             string // "hold" or "toggle"
	HandsfreeTimeout int    // Hands-free timeout in seconds (0 = no limit)
	DoubletapWindow  int    // Double-tap detection window in milliseconds
	Notifications    bool
	AudioFeedback    bool

	// Backend
	STTBackend string // "openai" (default) or "local"
	STTURL     string // URL for local Whisper server
	LLMBackend string // "openai" (default), "ollama", "none"
	LLMURL     string // Base URL for Ollama
	LLMModel   string // Model name override

}

// Lock acquires a write lock on the config.
func (c *Config) Lock()   { c.mu.Lock() }
func (c *Config) Unlock() { c.mu.Unlock() }

// RLock acquires a read lock on the config.
func (c *Config) RLock()   { c.mu.RLock() }
func (c *Config) RUnlock() { c.mu.RUnlock() }

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Language:      "de",
		Output:        "wtype",
		Raw:           false,
		Hotkey:        "right_option",
		Mode:             "hold",
		HandsfreeTimeout: 360,
		DoubletapWindow:  400,
		Notifications:    true,
		AudioFeedback: true,
		STTBackend:    "openai",
		LLMBackend:    "openai",
	}
}

// ConfigDir returns the vox config directory (~/.config/vox).
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "vox"), nil
}

// Load reads ~/.config/vox/config.yaml and returns the parsed config.
// Returns defaults if the file doesn't exist.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	dir, err := ConfigDir()
	if err != nil {
		return cfg, nil
	}

	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	parseConfig(string(data), cfg)
	return cfg, nil
}

// Save writes the config to ~/.config/vox/config.yaml.
func (cfg *Config) Save() error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("language: %s\n", cfg.Language))
	b.WriteString(fmt.Sprintf("output: %s\n", cfg.Output))
	b.WriteString(fmt.Sprintf("raw: %v\n", cfg.Raw))
	b.WriteString(fmt.Sprintf("hotkey: %s\n", cfg.Hotkey))
	b.WriteString(fmt.Sprintf("mode: %s\n", cfg.Mode))
	b.WriteString(fmt.Sprintf("handsfree_timeout: %d\n", cfg.HandsfreeTimeout))
	b.WriteString(fmt.Sprintf("doubletap_window: %d\n", cfg.DoubletapWindow))
	b.WriteString(fmt.Sprintf("notifications: %v\n", cfg.Notifications))
	b.WriteString(fmt.Sprintf("audio_feedback: %v\n", cfg.AudioFeedback))
	b.WriteString("\n# Backend\n")
	b.WriteString(fmt.Sprintf("stt_backend: %s\n", cfg.STTBackend))
	if cfg.STTURL != "" {
		b.WriteString(fmt.Sprintf("stt_url: %s\n", cfg.STTURL))
	}
	b.WriteString(fmt.Sprintf("llm_backend: %s\n", cfg.LLMBackend))
	if cfg.LLMURL != "" {
		b.WriteString(fmt.Sprintf("llm_url: %s\n", cfg.LLMURL))
	}
	if cfg.LLMModel != "" {
		b.WriteString(fmt.Sprintf("llm_model: %s\n", cfg.LLMModel))
	}
	path := filepath.Join(dir, "config.yaml")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func parseConfig(data string, cfg *Config) {
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		key, value, ok := parseKV(trimmed)
		if !ok {
			continue
		}

		switch key {
		case "language":
			cfg.Language = value
		case "output":
			cfg.Output = value
		case "raw":
			cfg.Raw = value == "true"
		case "hotkey":
			cfg.Hotkey = value
		case "mode":
			cfg.Mode = value
		case "handsfree_timeout":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.HandsfreeTimeout = v
			}
		case "doubletap_window":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.DoubletapWindow = v
			}
		case "notifications":
			cfg.Notifications = value == "true"
		case "audio_feedback":
			cfg.AudioFeedback = value == "true"
		case "stt_backend":
			cfg.STTBackend = value
		case "stt_url":
			cfg.STTURL = value
		case "llm_backend":
			cfg.LLMBackend = value
		case "llm_url":
			cfg.LLMURL = value
		case "llm_model":
			cfg.LLMModel = value
		}
	}
}

func parseKV(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := extractYAMLValue(line[idx+1:])
	return key, value, true
}
