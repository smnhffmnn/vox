package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Snippet maps a voice trigger to replacement text.
type Snippet struct {
	Trigger string `json:"trigger"`
	Text    string `json:"text"`
}

// LoadSnippets reads ~/.config/vox/snippets.yaml and returns parsed snippets.
// Returns nil without error if the file doesn't exist.
//
// Expected format:
//
//	- trigger: "greeting"
//	  text: "Hello, thanks for reaching out!"
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
			// Handle escaped newlines: \\n → literal \n, \n → newline
			current.Text = strings.ReplaceAll(current.Text, `\\n`, "\x00")
			current.Text = strings.ReplaceAll(current.Text, `\n`, "\n")
			current.Text = strings.ReplaceAll(current.Text, "\x00", `\n`)
		}
	}

	if current != nil {
		snippets = append(snippets, *current)
	}

	return snippets, nil
}

// extractYAMLValue is defined in config.go — reuse it here via package scope.

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
