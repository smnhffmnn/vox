//go:build !windows

package keychain

import (
	"errors"
	"testing"
)

// With no helper on PATH — the headless-server case — every operation must
// report ErrUnavailable, and none may reach a real keychain. An empty PATH
// directory makes exec's lookup fail before anything runs.
func TestAllOperations_HelperMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	if _, err := Get("vox-test", "k"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Get: got %v, want ErrUnavailable", err)
	}
	if err := Set("vox-test", "k", "v"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Set: got %v, want ErrUnavailable", err)
	}
	if err := Delete("vox-test", "k"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Delete: got %v, want ErrUnavailable", err)
	}
	if HasKey("vox-test", "k") {
		t.Error("HasKey: got true without a keychain")
	}
}
