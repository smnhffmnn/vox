package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Snippet maps a voice trigger to replacement text.
type Snippet struct {
	Trigger string
	Text    string
}

// LoadSnippets reads ~/.config/vox/snippets.yaml and returns parsed snippets.
// Returns nil without error if the file doesn't exist.
//
// Expected format:
//
//	- trigger: "kalenderlink"
//	  text: "https://cal.com/simon/30min"
func LoadSnippets() ([]Snippet, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".config", "vox", "snippets.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return parseSnippets(string(data))
}

func parseSnippets(data string) ([]Snippet, error) {
	var snippets []Snippet
	var current *Snippet

	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "- trigger:") {
			if current != nil {
				snippets = append(snippets, *current)
			}
			current = &Snippet{
				Trigger: extractYAMLValue(trimmed[len("- trigger:"):]),
			}
		} else if strings.HasPrefix(trimmed, "trigger:") {
			if current != nil {
				snippets = append(snippets, *current)
			}
			current = &Snippet{
				Trigger: extractYAMLValue(trimmed[len("trigger:"):]),
			}
		} else if strings.HasPrefix(trimmed, "text:") && current != nil {
			current.Text = extractYAMLValue(trimmed[len("text:"):])
			// Handle escaped newlines
			current.Text = strings.ReplaceAll(current.Text, `\n`, "\n")
		}
	}

	if current != nil {
		snippets = append(snippets, *current)
	}

	return snippets, nil
}

func extractYAMLValue(s string) string {
	s = strings.TrimSpace(s)
	// Remove surrounding quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

// MatchSnippet checks if the cleaned text exactly matches a snippet trigger (case-insensitive).
// Returns the snippet text and true if matched, otherwise empty string and false.
func MatchSnippet(text string, snippets []Snippet) (string, bool) {
	normalized := strings.TrimSpace(strings.ToLower(text))
	for _, s := range snippets {
		if strings.ToLower(s.Trigger) == normalized {
			return s.Text, true
		}
	}
	return "", false
}
