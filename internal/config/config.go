package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	Language string
	Output   string
	Raw      bool
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Language: "de",
		Output:   "stdout",
		Raw:      false,
	}
}

// Load reads ~/.config/vox/config.yaml and returns the parsed config.
// Returns defaults if the file doesn't exist.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil
	}

	path := filepath.Join(home, ".config", "vox", "config.yaml")
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
