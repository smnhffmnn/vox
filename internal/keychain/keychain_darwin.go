//go:build darwin

package keychain

import (
	"fmt"
	"os/exec"
	"strings"
)

// macOS ships security with the OS; the missing-tool branch exists so both
// exec-based backends behave the same and the error shape is one to test.
const tool = "security"

func get(service, key string) (string, error) {
	out, err := exec.Command(tool, "find-generic-password", "-s", service, "-a", key, "-w").Output()
	if err != nil {
		if missingTool(err) {
			return "", unavailable(tool)
		}
		return "", fmt.Errorf("keychain: key %q not found: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func set(service, key, value string) error {
	err := exec.Command(tool, "add-generic-password", "-U", "-s", service, "-a", key, "-w", value).Run()
	if err != nil {
		if missingTool(err) {
			return unavailable(tool)
		}
		return fmt.Errorf("keychain: failed to set %q: %w", key, err)
	}
	return nil
}

func del(service, key string) error {
	err := exec.Command(tool, "delete-generic-password", "-s", service, "-a", key).Run()
	if err != nil {
		if missingTool(err) {
			return unavailable(tool)
		}
		return fmt.Errorf("keychain: failed to delete %q: %w", key, err)
	}
	return nil
}
