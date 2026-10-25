//go:build linux

package keychain

import (
	"strings"
)

// The Linux keychain is libsecret, reached through secret-tool. Desktops have
// it; a server usually does not — then every call ends in ErrUnavailable
// instead of surfacing exec's "executable file not found", and the key comes
// from OPENAI_API_KEY or the -api-key flag instead.
const tool = "secret-tool"

func get(service, key string) (string, error) {
	out, err := run(nil, "lookup", "service", service, "key", key)
	if err != nil {
		return "", lookupError(key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func set(service, key, value string) error {
	if _, err := run(strings.NewReader(value), "store", "--label=vox: "+key, "service", service, "key", key); err != nil {
		return writeError("set", key, err)
	}
	return nil
}

func del(service, key string) error {
	if _, err := run(nil, "clear", "service", service, "key", key); err != nil {
		return writeError("delete", key, err)
	}
	return nil
}
