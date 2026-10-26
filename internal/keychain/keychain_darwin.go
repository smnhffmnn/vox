//go:build darwin

package keychain

import (
	"errors"
	"os/exec"
	"strings"
)

// macOS ships security with the OS; the missing-tool branch exists so both
// exec-based backends behave the same and the error shape is one to test.
const tool = "security"

// itemNotFound is security's exit status for a keychain item that does not
// exist (errSecItemNotFound). Unlike secret-tool, which answers that case
// silently, security also writes "The specified item could not be found in
// the keychain." to stderr — the status, not the silence, says "no such key".
const itemNotFound = 44

// exitCode returns the helper's exit status, or -1 when err is not an exit
// error (helper missing, cut off by the timeout).
func exitCode(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return -1
	}
	return exitErr.ExitCode()
}

func get(service, key string) (string, error) {
	out, err := run(nil, "find-generic-password", "-s", service, "-a", key, "-w")
	if err != nil {
		if exitCode(err) == itemNotFound {
			return "", notFound(key, err)
		}
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

// del treats an item that is not there as deleted, as the Windows file store
// does: the state the caller asked for is reached.
func del(service, key string) error {
	if _, err := run(nil, "delete-generic-password", "-s", service, "-a", key); err != nil {
		if exitCode(err) == itemNotFound {
			return nil
		}
		return writeError("delete", key, err)
	}
	return nil
}
