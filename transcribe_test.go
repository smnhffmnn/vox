package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/smnhffmnn/vox/internal/keychain"
)

// fakeSTTServer returns a test server that echoes a fixed transcription.
func fakeSTTServer(transcription string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"text": transcription})
	}))
}

// fakeLLMServer returns a test server that simulates an OpenAI chat completions endpoint.
// It returns the user message prefixed with "CLEANED: " so tests can verify the cleanup was called.
func fakeLLMServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.Unmarshal(body, &req)

		// Extract text between sentinel tags from user message
		userMsg := ""
		for _, m := range req.Messages {
			if m.Role == "user" {
				userMsg = m.Content
			}
		}

		// Strip sentinel tags, trim whitespace
		cleaned := userMsg
		if idx := strings.Index(cleaned, "\n"); idx >= 0 {
			cleaned = cleaned[idx+1:]
		}
		if idx := strings.LastIndex(cleaned, "\n"); idx >= 0 {
			cleaned = cleaned[:idx]
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "CLEANED: " + strings.TrimSpace(cleaned),
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func tmpAudioFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	f := filepath.Join(dir, "test.wav")
	os.WriteFile(f, []byte("fake-audio-data"), 0644)
	return f
}

// isolateConfig points the config directory at an empty home, so a test
// neither reads the developer's ~/.config/vox nor depends on it. Returns the
// home directory for tests that want to plant a file there.
func isolateConfig(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows home
	return home
}

// withTestKey makes the key resolution stop at the environment: no test may
// reach the OS keychain, which on a developer Mac holds a real key.
func withTestKey(t *testing.T) {
	t.Helper()
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "test-key")
}

// withNoKey leaves no key anywhere: environment empty, no keychain helper on
// PATH (secret-tool, security), and the Windows credential file nowhere.
func withNoKey(t *testing.T) {
	t.Helper()
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("PATH", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
}

// runCapturing runs the transcribe command with stdout and stderr captured.
func runCapturing(t *testing.T, args []string) (code int, stdout, stderr string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	code = runTranscribe(args)

	wOut.Close()
	wErr.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	out, _ := io.ReadAll(rOut)
	errOut, _ := io.ReadAll(rErr)
	return code, string(out), string(errOut)
}

func TestTranscribe_DefaultRunsCleanup(t *testing.T) {
	withTestKey(t)

	sttSrv := fakeSTTServer("ähm hallo welt")
	defer sttSrv.Close()

	llmSrv := fakeLLMServer()
	defer llmSrv.Close()

	audioFile := tmpAudioFile(t)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runTranscribe([]string{
		"-f", audioFile,
		"-backend", "local",
		"-stt-url", sttSrv.URL,
		"-llm-url", llmSrv.URL,
		"-llm-backend", "openai",
		"-api-key", "test-key",
	})

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := string(out)
	if !strings.Contains(output, "CLEANED:") {
		t.Errorf("expected cleanup to run by default, got: %q", output)
	}
}

func TestTranscribe_RawSkipsCleanup(t *testing.T) {
	withTestKey(t)

	sttSrv := fakeSTTServer("ähm hallo welt")
	defer sttSrv.Close()

	audioFile := tmpAudioFile(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runTranscribe([]string{
		"-f", audioFile,
		"-backend", "local",
		"-stt-url", sttSrv.URL,
		"-raw",
	})

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := string(out)
	if output != "ähm hallo welt" {
		t.Errorf("expected raw STT output, got: %q", output)
	}
}

func TestTranscribe_RawJSON(t *testing.T) {
	withTestKey(t)

	sttSrv := fakeSTTServer("hello world")
	defer sttSrv.Close()

	audioFile := tmpAudioFile(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runTranscribe([]string{
		"-f", audioFile,
		"-backend", "local",
		"-stt-url", sttSrv.URL,
		"-raw",
		"-json",
		"-l", "en",
	})

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var result TranscribeResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result.Text != "hello world" {
		t.Errorf("expected raw text 'hello world', got %q", result.Text)
	}
	if result.Backend != "local" {
		t.Errorf("expected backend 'local', got %q", result.Backend)
	}
}

func TestTranscribe_CleanupJSON(t *testing.T) {
	withTestKey(t)

	sttSrv := fakeSTTServer("ähm hallo welt")
	defer sttSrv.Close()

	llmSrv := fakeLLMServer()
	defer llmSrv.Close()

	audioFile := tmpAudioFile(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runTranscribe([]string{
		"-f", audioFile,
		"-backend", "local",
		"-stt-url", sttSrv.URL,
		"-llm-url", llmSrv.URL,
		"-llm-backend", "openai",
		"-api-key", "test-key",
		"-json",
	})

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var result TranscribeResult
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !strings.Contains(result.Text, "CLEANED:") {
		t.Errorf("expected cleaned text in JSON, got %q", result.Text)
	}
}

func TestTranscribe_LLMBackendNoneSkipsCleanup(t *testing.T) {
	withTestKey(t)

	sttSrv := fakeSTTServer("raw text here")
	defer sttSrv.Close()

	audioFile := tmpAudioFile(t)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := runTranscribe([]string{
		"-f", audioFile,
		"-backend", "local",
		"-stt-url", sttSrv.URL,
		"-llm-backend", "none",
	})

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old

	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	output := string(out)
	if output != "raw text here" {
		t.Errorf("expected raw text with llm-backend=none, got: %q", output)
	}
}

func TestTranscribe_MissingFile(t *testing.T) {
	code := runTranscribe([]string{"-f", "/nonexistent/file.wav"})
	if code != 1 {
		t.Errorf("expected exit 1 for missing file, got %d", code)
	}
}

func TestTranscribe_MissingFileFlag(t *testing.T) {
	code := runTranscribe([]string{})
	if code != 1 {
		t.Errorf("expected exit 1 for missing -f flag, got %d", code)
	}
}

// --- API key resolution ---

// The keychain stubs stand in for a system with and without an OS keychain;
// the real one is never consulted here, because on a developer machine it may
// hold a real key and would make the "no key" cases pass or fail by accident.
func keychainUnavailable(service, key string) (string, error) {
	return "", fmt.Errorf("%w: secret-tool is not installed", keychain.ErrUnavailable)
}

func keychainWithout(service, key string) (string, error) {
	return "", errors.New("keychain: key \"openai-api-key\" not found: exit status 1")
}

func keychainWith(value string) func(service, key string) (string, error) {
	return func(service, key string) (string, error) { return value, nil }
}

func envWith(value string) func(string) string {
	return func(name string) string {
		if name == "OPENAI_API_KEY" {
			return value
		}
		return ""
	}
}

func TestLookupAPIKey_FlagWins(t *testing.T) {
	key, err := lookupAPIKey("from-flag", envWith("from-env"), keychainWith("from-keychain"))
	if err != nil || key != "from-flag" {
		t.Errorf("got (%q, %v), want from-flag", key, err)
	}
}

func TestLookupAPIKey_EnvBeforeKeychain(t *testing.T) {
	key, err := lookupAPIKey("", envWith("from-env"), keychainWith("from-keychain"))
	if err != nil || key != "from-env" {
		t.Errorf("got (%q, %v), want from-env", key, err)
	}
}

func TestLookupAPIKey_KeychainLast(t *testing.T) {
	key, err := lookupAPIKey("", envWith(""), keychainWith("from-keychain"))
	if err != nil || key != "from-keychain" {
		t.Errorf("got (%q, %v), want from-keychain", key, err)
	}
}

// A headless server has no secret-tool: the environment variable must still
// be honoured, and the keychain's absence must not surface as an exec error.
func TestLookupAPIKey_EnvWhenKeychainUnavailable(t *testing.T) {
	key, err := lookupAPIKey("", envWith("from-env"), keychainUnavailable)
	if err != nil || key != "from-env" {
		t.Errorf("got (%q, %v), want from-env", key, err)
	}
}

func TestLookupAPIKey_NothingHeadless(t *testing.T) {
	key, err := lookupAPIKey("", envWith(""), keychainUnavailable)
	if key != "" || err == nil {
		t.Fatalf("got (%q, %v), want no key and an error", key, err)
	}
	msg := err.Error()
	for _, want := range []string{"OPENAI_API_KEY", "-api-key", "secret-tool"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q, got %q", want, msg)
		}
	}
	for _, leak := range []string{"exec", "executable file not found"} {
		if strings.Contains(msg, leak) {
			t.Errorf("error should not leak %q, got %q", leak, msg)
		}
	}
	if strings.Contains(msg, "store it in the OS keychain") {
		t.Errorf("error must not offer the keychain when there is none, got %q", msg)
	}
}

func TestLookupAPIKey_NothingWithKeychain(t *testing.T) {
	key, err := lookupAPIKey("", envWith(""), keychainWithout)
	if key != "" || err == nil {
		t.Fatalf("got (%q, %v), want no key and an error", key, err)
	}
	msg := err.Error()
	for _, want := range []string{"OPENAI_API_KEY", "-api-key", "store it in the OS keychain"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q, got %q", want, msg)
		}
	}
}

// A keychain that is there but could not be asked — secret-tool without a
// Secret Service, a helper that did not answer in time — has a reason, and
// the hint carries it instead of offering a keychain that does not work here.
func TestLookupAPIKey_KeychainErrorIsQuoted(t *testing.T) {
	cause := errors.New("keychain: lookup of \"openai-api-key\" failed: exit status 1: secret-tool: Cannot autolaunch D-Bus without X11 $DISPLAY")
	keychainBroken := func(service, key string) (string, error) { return "", cause }

	key, err := lookupAPIKey("", envWith(""), keychainBroken)
	if key != "" || err == nil {
		t.Fatalf("got (%q, %v), want no key and an error", key, err)
	}
	if !strings.Contains(err.Error(), "Cannot autolaunch D-Bus") {
		t.Errorf("error should quote the keychain's reason, got %q", err)
	}
	if !errors.Is(err, cause) {
		t.Errorf("the keychain error should stay wrapped, got %v", err)
	}
}

// ErrUnavailable must survive the wrapping too, so a caller can still tell a
// system without a keychain from one whose keychain has no key.
func TestLookupAPIKey_UnavailableStaysWrapped(t *testing.T) {
	_, err := lookupAPIKey("", envWith(""), keychainUnavailable)
	if !errors.Is(err, keychain.ErrUnavailable) {
		t.Errorf("errors.Is(err, ErrUnavailable) = false for %v", err)
	}
}

// A keychain that answers without an error but with an empty value is "no
// key" too — not a key of length zero.
func TestLookupAPIKey_EmptyKeychainValue(t *testing.T) {
	key, err := lookupAPIKey("", envWith(""), keychainWith(""))
	if key != "" || err == nil {
		t.Fatalf("got (%q, %v), want no key and an error", key, err)
	}
	if !strings.Contains(err.Error(), "store it in the OS keychain") {
		t.Errorf("error should offer the keychain that answered, got %q", err)
	}
}

// --- runTranscribe: key and config errors as the user sees them ---

func TestTranscribe_NoKey_ErrorOutput(t *testing.T) {
	withNoKey(t)
	audioFile := tmpAudioFile(t)

	code, out, errOut := runCapturing(t, []string{"-f", audioFile, "-backend", "openai"})

	if code != 1 {
		t.Fatalf("expected exit 1 without a key, got %d (stderr %q)", code, errOut)
	}
	if out != "" {
		t.Errorf("stdout should stay empty on failure, got %q", out)
	}
	for _, want := range []string{"vox transcribe: no API key", "OPENAI_API_KEY", "-api-key"} {
		if !strings.Contains(errOut, want) {
			t.Errorf("stderr should contain %q, got %q", want, errOut)
		}
	}
	if strings.Contains(errOut, "exec") {
		t.Errorf("stderr should not leak an exec error, got %q", errOut)
	}
}

// Local STT alone does not make the run keyless: the default cleanup is
// OpenAI, and without the check the transcript would be posted there
// unauthenticated. The desktop pipeline applies the same rule.
func TestTranscribe_LocalSTT_OpenAICleanupNeedsKey(t *testing.T) {
	withNoKey(t)
	sttSrv := fakeSTTServer("hallo welt")
	defer sttSrv.Close()
	audioFile := tmpAudioFile(t)

	code, _, errOut := runCapturing(t, []string{
		"-f", audioFile,
		"-backend", "local",
		"-stt-url", sttSrv.URL,
	})

	if code != 1 {
		t.Fatalf("expected exit 1: local STT with OpenAI cleanup and no key, got %d (stderr %q)", code, errOut)
	}
	if !strings.Contains(errOut, "no API key") {
		t.Errorf("stderr should name the missing key, got %q", errOut)
	}
}

// A run that needs no key — local STT, no OpenAI cleanup — must not consult
// the OS keychain at all: on a server that would shell out to secret-tool for
// nothing, on a desktop it may raise an unlock prompt for a key nobody uses.
// The fake helper on PATH records whether it was called.
func TestTranscribe_KeylessRunDoesNotTouchKeychain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Windows keychain is a file, not a helper")
	}
	isolateConfig(t)
	t.Setenv("OPENAI_API_KEY", "")
	helper := map[string]string{"linux": "secret-tool", "darwin": "security"}[runtime.GOOS]
	if helper == "" {
		t.Skipf("no keychain helper known for %s", runtime.GOOS)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "called")
	script := "#!/bin/sh\n: > " + marker + "\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, helper), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	sttSrv := fakeSTTServer("hallo welt")
	defer sttSrv.Close()
	audioFile := tmpAudioFile(t)

	code, out, errOut := runCapturing(t, []string{
		"-f", audioFile,
		"-backend", "local",
		"-stt-url", sttSrv.URL,
		"-llm-backend", "none",
	})

	if code != 0 {
		t.Fatalf("expected exit 0 for a keyless run, got %d (stderr %q)", code, errOut)
	}
	if out != "hallo welt" {
		t.Errorf("expected the raw transcript, got %q", out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the keychain helper was called although no backend needs the key")
	}
}

// A config.yaml that exists but cannot be read must stop the run — falling
// back to the defaults would turn a local setup into an upload to OpenAI.
func TestTranscribe_UnreadableConfig(t *testing.T) {
	withTestKey(t)
	home := os.Getenv("HOME")
	// A directory in place of the file: ReadFile fails on every platform.
	if err := os.MkdirAll(filepath.Join(home, ".config", "vox", "config.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}
	sttSrv := fakeSTTServer("hallo welt")
	defer sttSrv.Close()
	audioFile := tmpAudioFile(t)

	code, _, errOut := runCapturing(t, []string{
		"-f", audioFile,
		"-backend", "local",
		"-stt-url", sttSrv.URL,
		"--raw",
	})

	if code != 1 {
		t.Fatalf("expected exit 1 for an unreadable config, got %d (stderr %q)", code, errOut)
	}
	if !strings.Contains(errOut, "config.yaml") {
		t.Errorf("stderr should name the config file, got %q", errOut)
	}
}
