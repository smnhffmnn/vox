package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestTranscribe_DefaultRunsCleanup(t *testing.T) {
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
