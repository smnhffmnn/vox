package main

import (
	"testing"
	"time"
)

// The status the frontend shows must match what the pipeline will find: a
// key from OPENAI_API_KEY counts as configured, the same as one in the OS
// keychain. No helper on PATH keeps the real keychain out of the test.
func TestHasAPIKey_SeesEnvironment(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	isolateConfig(t)
	a := &App{state: "idle", started: time.Now()}

	t.Setenv("OPENAI_API_KEY", "")
	if a.HasAPIKey(openAIKeyName) {
		t.Error("HasAPIKey: got true with no key anywhere")
	}
	if a.GetStatus().HasKey {
		t.Error("GetStatus.HasKey: got true with no key anywhere")
	}

	t.Setenv("OPENAI_API_KEY", "test-key")
	if !a.HasAPIKey(openAIKeyName) {
		t.Error("HasAPIKey: got false with OPENAI_API_KEY set")
	}
	if !a.GetStatus().HasKey {
		t.Error("GetStatus.HasKey: got false with OPENAI_API_KEY set")
	}
}
