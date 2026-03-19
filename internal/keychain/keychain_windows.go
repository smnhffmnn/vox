//go:build windows

package keychain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func get(service, key string) (string, error) {
	path := credPath(service, key)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("keychain: key %q not found: %w", key, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func set(service, key, value string) error {
	path := credPath(service, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("keychain: failed to create dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		return fmt.Errorf("keychain: failed to set %q: %w", key, err)
	}
	return nil
}

func del(service, key string) error {
	path := credPath(service, key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("keychain: failed to delete %q: %w", key, err)
	}
	return nil
}

func credPath(service, key string) string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, service, "credentials", key)
}
