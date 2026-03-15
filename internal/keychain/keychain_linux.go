package keychain

import (
	"fmt"
	"os/exec"
	"strings"
)

func get(service, key string) (string, error) {
	out, err := exec.Command("secret-tool", "lookup", "service", service, "key", key).Output()
	if err != nil {
		return "", fmt.Errorf("keychain: key %q not found: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func set(service, key, value string) error {
	cmd := exec.Command("secret-tool", "store", "--label=vox: "+key, "service", service, "key", key)
	cmd.Stdin = strings.NewReader(value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("keychain: failed to set %q: %w", key, err)
	}
	return nil
}

func del(service, key string) error {
	err := exec.Command("secret-tool", "clear", "service", service, "key", key).Run()
	if err != nil {
		return fmt.Errorf("keychain: failed to delete %q: %w", key, err)
	}
	return nil
}
