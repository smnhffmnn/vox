//go:build darwin

package keychain

import (
	"strings"
	"testing"
)

// security answers a missing item with exit status 44 and a message on
// stderr (measured: "security: SecKeychainSearchCopyNext: The specified item
// could not be found in the keychain."). That is "not found", not a failed
// lookup — the message alone must not turn it into one.
const itemNotFoundHelper = `echo 'security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain.' >&2; exit 44`

func TestGet_ItemNotFound(t *testing.T) {
	fakeHelper(t, itemNotFoundHelper)

	_, err := Get("vox-test", "k")
	if err == nil {
		t.Fatal("expected an error for a missing item")
	}
	if !strings.Contains(err.Error(), `key "k" not found`) {
		t.Errorf("a missing item should read as not found, got %q", err)
	}
	if strings.Contains(err.Error(), "lookup of") {
		t.Errorf("a missing item is not a failed lookup, got %q", err)
	}
	if HasKey("vox-test", "k") {
		t.Error("HasKey: got true for a missing item")
	}
}

// Deleting an item that is not there reaches the state the caller asked for,
// as on Windows; it is not an error.
func TestDelete_ItemNotFound(t *testing.T) {
	fakeHelper(t, itemNotFoundHelper)

	if err := Delete("vox-test", "k"); err != nil {
		t.Errorf("Delete of a missing item: got %v, want nil", err)
	}
}

// Any other failure of security still carries its message.
func TestGet_OtherFailureKeepsMessage(t *testing.T) {
	fakeHelper(t, `echo 'security: SecKeychainCopySearchList: User interaction is not allowed.' >&2; exit 36`)

	_, err := Get("vox-test", "k")
	if err == nil || !strings.Contains(err.Error(), "User interaction is not allowed") {
		t.Errorf("error should carry security's message, got %v", err)
	}
	if err := Delete("vox-test", "k"); err == nil {
		t.Error("Delete: a failure other than a missing item must be reported")
	}
}
