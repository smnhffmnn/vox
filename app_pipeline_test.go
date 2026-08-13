package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/smnhffmnn/vox/internal/config"
	"github.com/smnhffmnn/vox/internal/stt"
)

// TestTranscribeAndCleanup_MarksInsteadOfDropping pins the VOX-10/VOX-11
// contract at the pipeline level: a transcript that matches a hallucination
// pattern is returned verbatim with the suspected mark — not turned into an
// error, which was the path that used to delete the recording. Only genuinely
// empty text remains a failure (errNoSpeech), and even that no longer costs
// the audio.
func TestTranscribeAndCleanup_MarksInsteadOfDropping(t *testing.T) {
	// Keeps resolveAPIKey out of the OS keychain; the key itself is never used
	// because both backends below run keyless.
	t.Setenv("OPENAI_API_KEY", "test-key")

	cases := []struct {
		name          string
		text          string
		wantSuspected bool
		wantErr       error
	}{
		{"hallucination pattern is delivered and marked", "Vielen Dank fürs Zuschauen", true, nil},
		{"fuzzy match on real dictation is delivered and marked", "Vielen Dank für die Info, ich schaue mir das nachher an", true, nil},
		{"normal dictation passes unmarked", "Das ist ein normaler Satz.", false, nil},
		{"empty text is the one remaining failure", "   ", false, errNoSpeech},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]string{"text": tc.text})
			}))
			defer srv.Close()

			cfg := config.DefaultConfig()
			cfg.STTBackend = "local"
			cfg.STTURL = srv.URL
			cfg.Raw = true // the cleanup step is not under test
			cfg.LLMBackend = "none"

			audioFile := filepath.Join(t.TempDir(), "in.wav")
			if err := os.WriteFile(audioFile, []byte("RIFF fake"), 0o600); err != nil {
				t.Fatal(err)
			}

			a := &App{cfg: cfg}
			tr, err := a.transcribeAndCleanup(audioFile, nil)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tr.raw != tc.text {
				t.Errorf("raw = %q, want %q — a suspected transcript must come through verbatim", tr.raw, tc.text)
			}
			if tr.suspectedHallucination != tc.wantSuspected {
				t.Errorf("suspectedHallucination = %v, want %v", tr.suspectedHallucination, tc.wantSuspected)
			}
		})
	}
}

// TestTranscribeAndCleanup_CarriesSegments pins VOX-13 at the pipeline level:
// timestamped segments from the STT backend land on the transcription result.
// They describe the raw transcript, so the cleanup step must not touch them.
func TestTranscribeAndCleanup_CarriesSegments(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"text":"Guten Morgen. Bis später.","segments":[` +
			`{"start":0,"end":1.8,"text":"Guten Morgen."},` +
			`{"start":2.6,"end":4.2,"text":" Bis später."}]}`))
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.STTBackend = "local"
	cfg.STTURL = srv.URL
	cfg.Raw = true // the cleanup step is not under test
	cfg.LLMBackend = "none"

	audioFile := filepath.Join(t.TempDir(), "in.wav")
	if err := os.WriteFile(audioFile, []byte("RIFF fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &App{cfg: cfg}
	tr, err := a.transcribeAndCleanup(audioFile, nil)
	if err != nil {
		t.Fatalf("transcribeAndCleanup: %v", err)
	}

	want := []stt.Segment{
		{Start: 0, End: 1.8, Text: "Guten Morgen."},
		{Start: 2.6, End: 4.2, Text: " Bis später."},
	}
	if len(tr.segments) != len(want) {
		t.Fatalf("segments = %v, want %v", tr.segments, want)
	}
	for i := range want {
		if tr.segments[i] != want[i] {
			t.Fatalf("segment[%d] = %v, want %v", i, tr.segments[i], want[i])
		}
	}

	// And their history form keeps the values as stored on the entry.
	hs := historySegments(tr.segments)
	if len(hs) != 2 || hs[0].End != 1.8 || hs[1].Text != " Bis später." {
		t.Errorf("historySegments = %v", hs)
	}
}
