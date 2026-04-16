package stt

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewOpenAI_DefaultsBaseURLAndModel(t *testing.T) {
	o := NewOpenAI("sk-test")
	if o.apiKey != "sk-test" {
		t.Errorf("apiKey = %q, want sk-test", o.apiKey)
	}
	if o.model != "whisper-1" {
		t.Errorf("model = %q, want whisper-1", o.model)
	}
	if o.baseURL != defaultOpenAIBaseURL {
		t.Errorf("baseURL = %q, want %q", o.baseURL, defaultOpenAIBaseURL)
	}
}

// newTestOpenAI returns an *OpenAI whose requests go to the given test server.
// The baseURL field is unexported on purpose — only tests (in the same package)
// should override it. This is the hook we introduced to keep OpenAI testable
// without exposing a public WithBaseURL option.
func newTestOpenAI(apiKey, baseURL string) *OpenAI {
	o := NewOpenAI(apiKey)
	o.baseURL = baseURL
	return o
}

func TestOpenAI_Transcribe_SendsCorrectRequest(t *testing.T) {
	audioPath := writeDummyAudio(t, "voice.wav")

	var (
		gotPath         string
		gotAuth         string
		gotContentType  string
		gotFileField    string
		gotFileContents []byte
		gotModel        string
		gotLanguage     string
		gotPrompt       string
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
		_, _ = w.Write([]byte(`{"text":"hello world"}`))
	}))
	defer srv.Close()

	o := newTestOpenAI("sk-abc123", srv.URL)
	text, err := o.Transcribe(audioPath, "en", "be concise")
	if err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}
	if text != "hello world" {
		t.Errorf("text = %q, want %q", text, "hello world")
	}

	if gotPath != "/v1/audio/transcriptions" {
		t.Errorf("path = %q, want /v1/audio/transcriptions", gotPath)
	}
	if gotAuth != "Bearer sk-abc123" {
		t.Errorf("Authorization = %q, want Bearer sk-abc123", gotAuth)
	}
	if !strings.HasPrefix(gotContentType, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data prefix", gotContentType)
	}
	if gotFileField != "voice.wav" {
		t.Errorf("file filename = %q, want voice.wav", gotFileField)
	}
	if string(gotFileContents) != "RIFF....WAVEfmt " {
		t.Errorf("file contents = %q, want the dummy bytes", string(gotFileContents))
	}
	if gotModel != "whisper-1" {
		t.Errorf("model = %q, want whisper-1", gotModel)
	}
	if gotLanguage != "en" {
		t.Errorf("language = %q, want en", gotLanguage)
	}
	if gotPrompt != "be concise" {
		t.Errorf("prompt = %q, want \"be concise\"", gotPrompt)
	}
}

func TestOpenAI_Transcribe_OmitsEmptyOptionalFields(t *testing.T) {
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

	o := newTestOpenAI("sk-xyz", srv.URL)
	if _, err := o.Transcribe(audioPath, "", ""); err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}
	if hasLanguage {
		t.Error("language field should not be sent when empty")
	}
	if hasPrompt {
		t.Error("prompt field should not be sent when empty")
	}
}

func TestOpenAI_Transcribe_NonOKStatusReturnsError(t *testing.T) {
	audioPath := writeDummyAudio(t, "err.wav")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"invalid api key"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	o := newTestOpenAI("sk-bad", srv.URL)
	_, err := o.Transcribe(audioPath, "", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention status 401, got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid api key") {
		t.Errorf("error should include body, got: %v", err)
	}
}

func TestOpenAI_Transcribe_InvalidJSONReturnsError(t *testing.T) {
	audioPath := writeDummyAudio(t, "bad.wav")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	o := newTestOpenAI("sk", srv.URL)
	_, err := o.Transcribe(audioPath, "", "")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestOpenAI_Transcribe_MissingAudioFileReturnsError(t *testing.T) {
	o := NewOpenAI("sk")
	_, err := o.Transcribe("/does/not/exist.wav", "", "")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
