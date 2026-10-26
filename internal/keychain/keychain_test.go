package keychain

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// A helper that is not on PATH is the headless-server case: it must be told
// apart from a helper that ran and failed, because only the former means
// "there is no keychain here" rather than "the key is not in it".
func TestMissingTool_AbsentBinary(t *testing.T) {
	_, err := exec.Command("vox-keychain-helper-that-does-not-exist", "lookup").Output()
	if err == nil {
		t.Fatal("expected an error for a binary that does not exist")
	}
	if !missingTool(err) {
		t.Errorf("missingTool(%v) = false, want true", err)
	}
}

func TestMissingTool_OtherFailures(t *testing.T) {
	for _, err := range []error{
		&exec.ExitError{},          // helper ran, non-zero exit (no such key)
		errors.New("ioctl failed"), // anything else
	} {
		if missingTool(err) {
			t.Errorf("missingTool(%T %v) = true, want false", err, err)
		}
	}
}

func TestUnavailable_WrapsSentinelAndNamesTool(t *testing.T) {
	err := unavailable("secret-tool")
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("errors.Is(err, ErrUnavailable) = false for %v", err)
	}
	if !strings.Contains(err.Error(), "secret-tool") {
		t.Errorf("message should name the missing helper, got %q", err)
	}
	if strings.Contains(err.Error(), "exec") {
		t.Errorf("message should not leak the exec error, got %q", err)
	}
}
