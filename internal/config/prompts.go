package config

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadCustomPrompts reads all .txt files from ~/.config/vox/prompts/
// and returns a map of category → prompt text.
// Valid categories: chat, email, ide, docs, browser, default.
func LoadCustomPrompts() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dir := filepath.Join(home, ".config", "vox", "prompts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	prompts := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		category := strings.TrimSuffix(e.Name(), ".txt")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(data))
		if text != "" {
			prompts[category] = text
		}
	}

	if len(prompts) == 0 {
		return nil
	}
	return prompts
}
