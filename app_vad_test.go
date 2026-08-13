package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/smnhffmnn/vox/internal/config"
)

const testWAVRate = 16000

// synthWAV writes a PCM16 WAV in the recorder's layout: faint room tone with
// an optional speech-energy burst (220 Hz tone) starting at speechAt.
func synthWAV(t *testing.T, totalSec, speechAt, speechSec float64) string {
	t.Helper()

	rng := rand.New(rand.NewSource(1))
	n := int(totalSec * testWAVRate)
	samples := make([]int16, n)
	for i := range samples {
		samples[i] = int16(rng.Intn(33) - 16) // ~-66 dBFS room tone
	}
	if speechSec > 0 {
		start := int(speechAt * testWAVRate)
		end := start + int(speechSec*testWAVRate)
		for i := start; i < end && i < n; i++ {
			samples[i] = int16(0.1 * 32767 * math.Sin(2*math.Pi*220*float64(i)/testWAVRate))
		}
	}

	raw := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(s))
	}
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+len(raw)))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	for _, v := range []any{
		uint32(16), uint16(1), uint16(1), uint32(testWAVRate),
		uint32(testWAVRate * 2), uint16(2), uint16(16),
	} {
		binary.Write(&buf, binary.LittleEndian, v)
	}
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(len(raw)))
	buf.Write(raw)

	path := filepath.Join(t.TempDir(), "synth.wav")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// sttStub records how the pipeline talked to the backend.
type sttStub struct {
	srv      *httptest.Server
	requests int
	prompt   string
}

func newSTTStub(t *testing.T) *sttStub {
	t.Helper()
	s := &sttStub{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests++
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		s.prompt = r.FormValue("prompt")
		_, _ = w.Write([]byte(`{"text":"hallo welt"}`))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func vadTestApp(stub *sttStub, dictionary []string) *App {
	cfg := config.DefaultConfig() // VAD defaults to on
	cfg.STTBackend = "local"
	cfg.STTURL = stub.srv.URL
	cfg.Raw = true
	cfg.LLMBackend = "none"
	return &App{cfg: cfg, dictionary: dictionary}
}

// TestVADGate_SilentRecordingFailsBeforeUpload pins the VOX-4 core: a
// recording without any detected speech becomes errNoSpeech WITHOUT the audio
// leaving the machine. The failure keeps the recording, so nothing is lost.
func TestVADGate_SilentRecordingFailsBeforeUpload(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	stub := newSTTStub(t)
	a := vadTestApp(stub, nil)

	audio := synthWAV(t, 5, 0, 0) // 5s of room tone, no speech
	_, err := a.transcribeAndCleanup(audio, nil, transcribeOpts{vadGate: true})

	if !errors.Is(err, errNoSpeech) {
		t.Fatalf("err = %v, want errNoSpeech", err)
	}
	if stub.requests != 0 {
		t.Errorf("backend received %d requests, want 0 — silence must not be uploaded", stub.requests)
	}
}

// TestVADGate_RetrySkipsTheGate: without vadGate (the retry and CLI paths) the
// same silent recording IS uploaded — the gate must never stand between the
// user and an explicit "transcribe this" request.
func TestVADGate_RetrySkipsTheGate(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	stub := newSTTStub(t)
	a := vadTestApp(stub, nil)

	audio := synthWAV(t, 5, 0, 0)
	tr, err := a.transcribeAndCleanup(audio, nil, transcribeOpts{})
	if err != nil {
		t.Fatalf("transcribeAndCleanup: %v", err)
	}
	if stub.requests != 1 {
		t.Errorf("backend received %d requests, want 1", stub.requests)
	}
	if tr.raw != "hallo welt" {
		t.Errorf("raw = %q, want the stub transcript", tr.raw)
	}
}

// TestVADGate_ConfigOffDisablesTheCheck: vad: false in the config switches the
// gate off even for fresh dictations.
func TestVADGate_ConfigOffDisablesTheCheck(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	stub := newSTTStub(t)
	a := vadTestApp(stub, nil)
	a.cfg.VAD = false

	audio := synthWAV(t, 5, 0, 0)
	if _, err := a.transcribeAndCleanup(audio, nil, transcribeOpts{vadGate: true}); err != nil {
		t.Fatalf("transcribeAndCleanup: %v", err)
	}
	if stub.requests != 1 {
		t.Errorf("backend received %d requests, want 1 — the check must be off", stub.requests)
	}
}

// TestVADPromptDrop_SilentFirstWindow reproduces the VOX-12 incident shape:
// ~36s of silence, then real speech. The recording uploads (it has speech),
// but the prompt must be dropped — over a silent first window the model
// continues the prompt instead of transcribing.
func TestVADPromptDrop_SilentFirstWindow(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	stub := newSTTStub(t)
	a := vadTestApp(stub, []string{"SEPA-Lastschrift"})

	audio := synthWAV(t, 40, 36, 3)
	if _, err := a.transcribeAndCleanup(audio, nil, transcribeOpts{vadGate: true}); err != nil {
		t.Fatalf("transcribeAndCleanup: %v", err)
	}
	if stub.requests != 1 {
		t.Fatalf("backend received %d requests, want 1", stub.requests)
	}
	if stub.prompt != "" {
		t.Errorf("prompt = %q, want empty — a silent first window must drop the prompt", stub.prompt)
	}
}

// TestVADPromptDrop_NormalDictationKeepsPrompt: speech from the start keeps
// the shaped dictionary prompt.
func TestVADPromptDrop_NormalDictationKeepsPrompt(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	stub := newSTTStub(t)
	a := vadTestApp(stub, []string{"SEPA-Lastschrift"})

	audio := synthWAV(t, 6, 0.5, 4)
	if _, err := a.transcribeAndCleanup(audio, nil, transcribeOpts{vadGate: true}); err != nil {
		t.Fatalf("transcribeAndCleanup: %v", err)
	}
	if stub.prompt != "Fachbegriffe: SEPA-Lastschrift." {
		t.Errorf("prompt = %q, want the shaped dictionary prompt", stub.prompt)
	}
}

// TestVAD_UnreadableAudioUploadsAnyway: the level check is an optimization —
// when it cannot read the file, the upload happens as before. The pipeline
// tests feed "RIFF fake" bytes through this exact path.
func TestVAD_UnreadableAudioUploadsAnyway(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	stub := newSTTStub(t)
	a := vadTestApp(stub, nil)

	audio := filepath.Join(t.TempDir(), "in.wav")
	if err := os.WriteFile(audio, []byte("RIFF fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.transcribeAndCleanup(audio, nil, transcribeOpts{vadGate: true}); err != nil {
		t.Fatalf("transcribeAndCleanup: %v", err)
	}
	if stub.requests != 1 {
		t.Errorf("backend received %d requests, want 1 — an unreadable file must not block the dictation", stub.requests)
	}
}
