//go:build darwin

package keychain

import (
	"strings"
)

// macOS ships security with the OS; the missing-tool branch exists so both
// exec-based backends behave the same and the error shape is one to test.
const tool = "security"

func get(service, key string) (string, error) {
	out, err := run(nil, "find-generic-password", "-s", service, "-a", key, "-w")
	if err != nil {
		return "", lookupError(key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func set(service, key, value string) error {
	if _, err := run(nil, "add-generic-password", "-U", "-s", service, "-a", key, "-w", value); err != nil {
		return writeError("set", key, err)
	}
	return nil
}

func del(service, key string) error {
	if _, err := run(nil, "delete-generic-password", "-s", service, "-a", key); err != nil {
		return writeError("delete", key, err)
	}
	return nil
}
