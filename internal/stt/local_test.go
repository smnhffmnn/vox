package stt

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLocal_DefaultsURLWhenEmpty(t *testing.T) {
	l := NewLocal("")
	if l.url != defaultLocalURL {
		t.Errorf("url = %q, want %q", l.url, defaultLocalURL)
	}
}

func TestNewLocal_KeepsProvidedURL(t *testing.T) {
	l := NewLocal("http://custom:9000")
	if l.url != "http://custom:9000" {
		t.Errorf("url = %q, want %q", l.url, "http://custom:9000")
	}
}

// writeDummyAudio writes a small placeholder file into a temp dir and returns
// its path. The Whisper upload doesn't care about the actual content for the
// purposes of these multipart tests — we just need something on disk.
func writeDummyAudio(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	// A handful of bytes; enough to show up in the multipart body.
	if err := os.WriteFile(path, []byte("RIFF....WAVEfmt "), 0o644); err != nil {
		t.Fatalf("write dummy audio: %v", err)
	}
	return path
}

func TestLocal_Transcribe_SendsCorrectRequest(t *testing.T) {
	audioPath := writeDummyAudio(t, "sample.wav")

	var (
		gotPath          string
		gotAuth          string
		gotContentType   string
		gotFileField     string
		gotFileContents  []byte
		gotModel         string
		gotLanguage      string
		gotPrompt        string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}

		f, fh, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile(\"file\"): %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		defer f.Close()
		gotFileField = fh.Filename
		gotFileContents, _ = io.ReadAll(f)

		gotModel = r.FormValue("model")
		gotLanguage = r.FormValue("language")
		gotPrompt = r.FormValue("prompt")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hallo welt"}`))
	}))
	defer srv.Close()

	l := NewLocal(srv.URL)
	text, err := l.Transcribe(audioPath, "de", "system prompt")
	if err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}
	if text != "hallo welt" {
		t.Errorf("text = %q, want %q", text, "hallo welt")
	}

	if gotPath != "/v1/audio/transcriptions" {
		t.Errorf("path = %q, want /v1/audio/transcriptions", gotPath)
	}
	if gotAuth != "" {
		t.Errorf("Authorization = %q, want empty for local server", gotAuth)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data prefix", gotContentType)
	}
	if gotFileField != "sample.wav" {
		t.Errorf("file filename = %q, want sample.wav", gotFileField)
	}
	if string(gotFileContents) != "RIFF....WAVEfmt " {
		t.Errorf("file contents = %q, want the dummy bytes", string(gotFileContents))
	}
	if gotModel != "whisper-1" {
		t.Errorf("model = %q, want whisper-1", gotModel)
	}
	if gotLanguage != "de" {
		t.Errorf("language = %q, want de", gotLanguage)
	}
	if gotPrompt != "system prompt" {
		t.Errorf("prompt = %q, want \"system prompt\"", gotPrompt)
	}
}

func TestLocal_Transcribe_OmitsEmptyOptionalFields(t *testing.T) {
	audioPath := writeDummyAudio(t, "voice.wav")

	var hasLanguage, hasPrompt bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		_, hasLanguage = r.MultipartForm.Value["language"]
		_, hasPrompt = r.MultipartForm.Value["prompt"]
		_, _ = w.Write([]byte(`{"text":""}`))
	}))
	defer srv.Close()

	l := NewLocal(srv.URL)
	if _, err := l.Transcribe(audioPath, "", ""); err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}
	if hasLanguage {
		t.Error("language field should not be sent when empty")
	}
	if hasPrompt {
		t.Error("prompt field should not be sent when empty")
	}
}

func TestLocal_Transcribe_NonOKStatusReturnsError(t *testing.T) {
	audioPath := writeDummyAudio(t, "err.wav")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	l := NewLocal(srv.URL)
	_, err := l.Transcribe(audioPath, "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention status 503, got: %v", err)
	}
	if !strings.Contains(err.Error(), "model not loaded") {
		t.Errorf("error should include body, got: %v", err)
	}
}

func TestLocal_Transcribe_InvalidJSONReturnsError(t *testing.T) {
	audioPath := writeDummyAudio(t, "bad.wav")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	l := NewLocal(srv.URL)
	_, err := l.Transcribe(audioPath, "", "")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLocal_Transcribe_MissingAudioFileReturnsError(t *testing.T) {
	l := NewLocal("http://unused.invalid")
	_, err := l.Transcribe("/does/not/exist.wav", "", "")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
