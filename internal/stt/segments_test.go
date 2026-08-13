package stt

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// verboseJSONBody is a trimmed verbose_json response: the real one carries
// more per-segment fields (tokens, logprobs, …), which decoding must ignore.
const verboseJSONBody = `{
	"task": "transcribe",
	"language": "german",
	"duration": 4.2,
	"text": "Guten Morgen. Bis später.",
	"segments": [
		{"id": 0, "seek": 0, "start": 0.0, "end": 1.8, "text": "Guten Morgen.", "temperature": 0.0, "avg_logprob": -0.2, "no_speech_prob": 0.01},
		{"id": 1, "seek": 0, "start": 2.6, "end": 4.2, "text": " Bis später.", "temperature": 0.0, "avg_logprob": -0.3, "no_speech_prob": 0.02}
	]
}`

func assertSegments(t *testing.T, got []Segment) {
	t.Helper()
	want := []Segment{
		{Start: 0.0, End: 1.8, Text: "Guten Morgen."},
		{Start: 2.6, End: 4.2, Text: " Bis später."},
	}
	if len(got) != len(want) {
		t.Fatalf("segments = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("segment[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestOpenAI_Transcribe_Whisper1RequestsSegments pins the VOX-13 contract for
// the cloud backend: whisper-1 asks for verbose_json and decodes the
// timestamped segments alongside the text.
func TestOpenAI_Transcribe_Whisper1RequestsSegments(t *testing.T) {
	audioPath := writeDummyAudio(t, "voice.wav")

	var gotFormat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		gotFormat = r.FormValue("response_format")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(verboseJSONBody))
	}))
	defer srv.Close()

	o := newTestOpenAI("sk-abc", srv.URL) // defaults to whisper-1
	res, err := o.Transcribe(audioPath, "de", "")
	if err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}
	if gotFormat != "verbose_json" {
		t.Errorf("response_format = %q, want verbose_json", gotFormat)
	}
	if res.Text != "Guten Morgen. Bis später." {
		t.Errorf("text = %q, want the full transcript", res.Text)
	}
	assertSegments(t, res.Segments)
}

// TestOpenAI_Transcribe_GPT4oDoesNotRequestVerboseJSON: the gpt-4o transcribe
// models reject verbose_json, so the request must not ask for it — the result
// is text without segments, which callers treat as normal.
func TestOpenAI_Transcribe_GPT4oDoesNotRequestVerboseJSON(t *testing.T) {
	audioPath := writeDummyAudio(t, "voice.wav")

	var hasFormat bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		_, hasFormat = r.MultipartForm.Value["response_format"]
		_, _ = w.Write([]byte(`{"text":"plain"}`))
	}))
	defer srv.Close()

	o := NewOpenAI("sk-abc", "gpt-4o-transcribe")
	o.baseURL = srv.URL
	res, err := o.Transcribe(audioPath, "", "")
	if err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}
	if hasFormat {
		t.Error("response_format should not be sent for gpt-4o-transcribe")
	}
	if res.Text != "plain" || len(res.Segments) != 0 {
		t.Errorf("result = %+v, want plain text without segments", res)
	}
}

// TestLocal_Transcribe_ParsesSegments: a local server that honours
// verbose_json delivers segments like the cloud backend does.
func TestLocal_Transcribe_ParsesSegments(t *testing.T) {
	audioPath := writeDummyAudio(t, "voice.wav")

	var gotFormat string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		gotFormat = r.FormValue("response_format")
		_, _ = w.Write([]byte(verboseJSONBody))
	}))
	defer srv.Close()

	l := NewLocal(srv.URL)
	res, err := l.Transcribe(audioPath, "de", "")
	if err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}
	if gotFormat != "verbose_json" {
		t.Errorf("response_format = %q, want verbose_json", gotFormat)
	}
	assertSegments(t, res.Segments)
}

// TestLocal_Transcribe_FallsBackWhenVerboseJSONRejected pins the degradation
// path: a server that errors on the unknown response_format gets one retry
// without it, and the dictation succeeds with plain text. Segments are
// diagnostic detail — never worth failing a dictation over.
func TestLocal_Transcribe_FallsBackWhenVerboseJSONRejected(t *testing.T) {
	audioPath := writeDummyAudio(t, "voice.wav")

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		if r.FormValue("response_format") != "" {
			http.Error(w, "unknown response_format", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"text":"hallo welt"}`))
	}))
	defer srv.Close()

	l := NewLocal(srv.URL)
	res, err := l.Transcribe(audioPath, "de", "")
	if err != nil {
		t.Fatalf("Transcribe() err = %v", err)
	}
	if requests != 2 {
		t.Errorf("requests = %d, want 2 (verbose_json attempt + plain retry)", requests)
	}
	if res.Text != "hallo welt" || len(res.Segments) != 0 {
		t.Errorf("result = %+v, want plain text without segments", res)
	}
}
