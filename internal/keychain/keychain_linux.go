//go:build linux

package keychain

import (
	"fmt"
	"os/exec"
	"strings"
)

// The Linux keychain is libsecret, reached through secret-tool. Desktops have
// it; a server usually does not — then every call ends in ErrUnavailable
// instead of surfacing exec's "executable file not found", and the key comes
// from OPENAI_API_KEY or the -api-key flag instead.
const tool = "secret-tool"

func get(service, key string) (string, error) {
	out, err := exec.Command(tool, "lookup", "service", service, "key", key).Output()
	if err != nil {
		if missingTool(err) {
			return "", unavailable(tool)
		}
		return "", fmt.Errorf("keychain: key %q not found: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func set(service, key, value string) error {
	cmd := exec.Command(tool, "store", "--label=vox: "+key, "service", service, "key", key)
	cmd.Stdin = strings.NewReader(value)
	if err := cmd.Run(); err != nil {
		if missingTool(err) {
			return unavailable(tool)
		}
		return fmt.Errorf("keychain: failed to set %q: %w", key, err)
	}
	return nil
}

func del(service, key string) error {
	err := exec.Command(tool, "clear", "service", service, "key", key).Run()
	if err != nil {
		if missingTool(err) {
			return unavailable(tool)
		}
		return fmt.Errorf("keychain: failed to delete %q: %w", key, err)
	}
	return nil
}
