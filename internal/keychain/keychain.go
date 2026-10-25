package keychain

import (
	"errors"
	"fmt"
	"os/exec"
)

// ErrUnavailable reports that this system has no OS keychain vox can reach:
// the helper vox shells out to (secret-tool on Linux, security on macOS) is
// not installed. A headless server is the normal case, not a fault — callers
// treat it like "no key stored" and fall back to the environment or a flag,
// and tell the user that the keychain is not one of the options here.
var ErrUnavailable = errors.New("keychain: no OS keychain available on this system")

// Get retrieves a secret from the OS keychain.
func Get(service, key string) (string, error) {
	return get(service, key)
}

// Set stores a secret in the OS keychain.
func Set(service, key, value string) error {
	return set(service, key, value)
}

// Delete removes a secret from the OS keychain.
func Delete(service, key string) error {
	return del(service, key)
}

// HasKey returns true if the key exists in the keychain.
func HasKey(service, key string) bool {
	_, err := Get(service, key)
	return err == nil
}

// missingTool reports whether err from running the keychain helper means the
// helper binary itself is absent — as opposed to the helper running and
// failing (no such key, locked keyring, non-zero exit).
func missingTool(err error) bool {
	return errors.Is(err, exec.ErrNotFound)
}

// unavailable wraps ErrUnavailable with the name of the helper that is not
// installed, so the message tells the user what the system lacks.
func unavailable(tool string) error {
	return fmt.Errorf("%w: %s is not installed", ErrUnavailable, tool)
}
