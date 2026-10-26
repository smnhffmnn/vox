package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// The status the frontend shows must match what the pipeline will find: a
// key from OPENAI_API_KEY counts as configured, the same as one in the OS
// keychain. No helper on PATH and an empty APPDATA (the Windows credential
// file) keep the real keychain out of the test.
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

// fakeKeychain stands in for the OS keychain holding value (or nothing, for
// ""): a helper script alone on PATH on macOS and Linux, the credential file
// under a temporary APPDATA on Windows. The real keychain is never reached.
func fakeKeychain(t *testing.T, value string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	t.Setenv("APPDATA", dir)
	switch runtime.GOOS {
	case "windows":
		if value == "" {
			return
		}
		cred := filepath.Join(dir, "vox", "credentials", openAIKeyName)
		if err := os.MkdirAll(filepath.Dir(cred), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(cred, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	case "darwin", "linux":
		helper := map[string]string{"linux": "secret-tool", "darwin": "security"}[runtime.GOOS]
		script := "#!/bin/sh\nexit 1\n"
		if value != "" {
			script = "#!/bin/sh\nprintf '%s\\n' '" + value + "'\n"
		}
		if err := os.WriteFile(filepath.Join(dir, helper), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	default:
		t.Skipf("no keychain stand-in for %s", runtime.GOOS)
	}
}

// The Backends view decides from GetAPIKeyStatus what it shows and whether it
// offers Delete: a key from OPENAI_API_KEY is in use but not the app's to
// delete, a keychain key is. Every combination must be told apart.
func TestGetAPIKeyStatus_BothSources(t *testing.T) {
	for _, tc := range []struct {
		name     string
		env      string
		keychain string
		want     APIKeyStatus
	}{
		{"neither", "", "", APIKeyStatus{}},
		{"environment only", "env-key", "", APIKeyStatus{Environment: true}},
		{"keychain only", "", "keychain-key", APIKeyStatus{Keychain: true}},
		{"both", "env-key", "keychain-key", APIKeyStatus{Environment: true, Keychain: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateConfig(t)
			fakeKeychain(t, tc.keychain)
			t.Setenv("OPENAI_API_KEY", tc.env)
			a := &App{state: "idle", started: time.Now()}

			if got := a.GetAPIKeyStatus(); got != tc.want {
				t.Errorf("GetAPIKeyStatus = %+v, want %+v", got, tc.want)
			}
			if got, want := a.HasAPIKey(openAIKeyName), tc.env != "" || tc.keychain != ""; got != want {
				t.Errorf("HasAPIKey = %v, want %v", got, want)
			}
		})
	}
}
