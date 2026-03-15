package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	Language      string
	Output        string
	Raw           bool
	Hotkey        string
	Mode          string // "hold" or "toggle"
	Notifications bool
	AudioFeedback bool

	// Backend
	STTBackend string // "openai" (default) or "local"
	STTURL     string // URL for local Whisper server
	LLMBackend string // "openai" (default), "ollama", "none"
	LLMURL     string // Base URL for Ollama
	LLMModel   string // Model name override

	// UI
	UIPort int // Web UI port (default: 7890)
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Language:      "de",
		Output:        "stdout",
		Raw:           false,
		Hotkey:        "right_option",
		Mode:          "hold",
		Notifications: true,
		AudioFeedback: true,
		STTBackend:    "openai",
		LLMBackend:    "openai",
		UIPort:        7890,
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
	b.WriteString("\n# UI\n")
	b.WriteString(fmt.Sprintf("ui_port: %d\n", cfg.UIPort))

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
		case "ui_port":
			if v, err := strconv.Atoi(value); err == nil {
				cfg.UIPort = v
			}
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
