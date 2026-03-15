package keychain

import (
	"fmt"
	"os/exec"
	"strings"
)

func get(service, key string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", service, "-a", key, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain: key %q not found: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func set(service, key, value string) error {
	err := exec.Command("security", "add-generic-password", "-U", "-s", service, "-a", key, "-w", value).Run()
	if err != nil {
		return fmt.Errorf("keychain: failed to set %q: %w", key, err)
	}
	return nil
}

func del(service, key string) error {
	err := exec.Command("security", "delete-generic-password", "-s", service, "-a", key).Run()
	if err != nil {
		return fmt.Errorf("keychain: failed to delete %q: %w", key, err)
	}
	return nil
}
