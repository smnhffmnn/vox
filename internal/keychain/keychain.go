package keychain

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
