package stt

import "testing"

func TestNewTranscriber_LocalBackend(t *testing.T) {
	got := NewTranscriber("local", "irrelevant", "http://example.invalid", "")
	if _, ok := got.(*Local); !ok {
		t.Fatalf("NewTranscriber(\"local\", ...) = %T, want *Local", got)
	}
}

func TestNewTranscriber_DefaultsToOpenAI(t *testing.T) {
	tests := []struct {
		name    string
		backend string
	}{
		{"openai explicit", "openai"},
		{"empty string", ""},
		{"unknown backend", "something_else"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTranscriber(tt.backend, "k", "http://ignored.invalid", "")
			if _, ok := got.(*OpenAI); !ok {
				t.Fatalf("NewTranscriber(%q, ...) = %T, want *OpenAI", tt.backend, got)
			}
		})
	}
}

func TestNewTranscriber_PassesAPIKeyToOpenAI(t *testing.T) {
	got := NewTranscriber("openai", "secret-key", "", "")
	o, ok := got.(*OpenAI)
	if !ok {
		t.Fatalf("got %T, want *OpenAI", got)
	}
	if o.apiKey != "secret-key" {
		t.Errorf("apiKey = %q, want %q", o.apiKey, "secret-key")
	}
}

func TestNewTranscriber_PassesURLToLocal(t *testing.T) {
	got := NewTranscriber("local", "ignored", "http://custom:9000", "")
	l, ok := got.(*Local)
	if !ok {
		t.Fatalf("got %T, want *Local", got)
	}
	if l.url != "http://custom:9000" {
		t.Errorf("url = %q, want %q", l.url, "http://custom:9000")
	}
}

func TestNewTranscriber_PassesModelToOpenAI(t *testing.T) {
	got := NewTranscriber("openai", "k", "", "gpt-4o-transcribe")
	o, ok := got.(*OpenAI)
	if !ok {
		t.Fatalf("got %T, want *OpenAI", got)
	}
	if o.model != "gpt-4o-transcribe" {
		t.Errorf("model = %q, want gpt-4o-transcribe", o.model)
	}
}

func TestNewTranscriber_EmptyModelGivesWhisperDefault(t *testing.T) {
	got := NewTranscriber("openai", "k", "", "")
	o, ok := got.(*OpenAI)
	if !ok {
		t.Fatalf("got %T, want *OpenAI", got)
	}
	if o.model != "whisper-1" {
		t.Errorf("model = %q, want whisper-1", o.model)
	}
}
