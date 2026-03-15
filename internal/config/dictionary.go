package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadDictionary reads ~/.config/vox/dictionary.txt and returns non-empty,
// non-comment lines. Returns nil without error if the file doesn't exist.
func LoadDictionary() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(home, ".config", "vox", "dictionary.txt")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var words []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		words = append(words, line)
	}
	return words, scanner.Err()
}
