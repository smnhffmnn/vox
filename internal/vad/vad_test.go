package vad

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

const testRate = 16000

// silence returns seconds of faint room tone — not digital zero, because a
// real microphone never delivers that. Amplitude ~0.0005 ≈ -66 dBFS.
func silence(seconds float64) []int16 {
	rng := rand.New(rand.NewSource(1))
	n := int(seconds * testRate)
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(rng.Intn(33) - 16) // ±16 of 32768
	}
	return out
}

// speech returns seconds of a 220 Hz tone at the given amplitude (full scale
// 1.0) — a stand-in with speech-like energy for an energy-based check.
func speech(seconds, amplitude float64) []int16 {
	n := int(seconds * testRate)
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amplitude * 32767 * math.Sin(2*math.Pi*220*float64(i)/testRate))
	}
	return out
}

func concat(parts ...[]int16) []int16 {
	var out []int16
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func TestAnalyze_SilenceOnlyHasNoSpeech(t *testing.T) {
	an := Analyze(silence(5), testRate)
	if an.HasSpeech() {
		t.Errorf("5s of room tone: HasSpeech = true, want false (analysis: %+v)", an)
	}
	if !an.FirstWindowSilent() {
		t.Errorf("5s of room tone: FirstWindowSilent = false, want true")
	}
	if an.LeadingSilenceSeconds < 4.9 {
		t.Errorf("LeadingSilenceSeconds = %v, want the full duration", an.LeadingSilenceSeconds)
	}
}

func TestAnalyze_NormalDictationIsSpeech(t *testing.T) {
	// Half a second of lead-in, two spoken bursts with a pause.
	an := Analyze(concat(silence(0.5), speech(2, 0.1), silence(1), speech(2, 0.1)), testRate)
	if !an.HasSpeech() {
		t.Fatalf("dictation: HasSpeech = false, want true (analysis: %+v)", an)
	}
	if an.FirstWindowSilent() {
		t.Errorf("dictation: FirstWindowSilent = true, want false")
	}
	if an.LeadingSilenceSeconds < 0.4 || an.LeadingSilenceSeconds > 0.6 {
		t.Errorf("LeadingSilenceSeconds = %v, want ~0.5", an.LeadingSilenceSeconds)
	}
	if an.SpeechSeconds < 3.5 || an.SpeechSeconds > 4.5 {
		t.Errorf("SpeechSeconds = %v, want ~4", an.SpeechSeconds)
	}
}

// TestAnalyze_QuietSpeechStillCounts pins the safety property: the gate must
// never call quiet-but-real dictation silence. -40 dBFS is far below any
// normal recording level.
func TestAnalyze_QuietSpeechStillCounts(t *testing.T) {
	an := Analyze(concat(silence(1), speech(3, 0.01)), testRate)
	if !an.HasSpeech() {
		t.Fatalf("quiet speech (-40 dBFS): HasSpeech = false, want true (analysis: %+v)", an)
	}
}

// TestAnalyze_SilentFirstWindow reproduces the VOX-12 incident shape: ~36s of
// near-silence, then real speech. The recording has speech (so it uploads),
// but the first Whisper window is silent (so the prompt must be dropped).
func TestAnalyze_SilentFirstWindow(t *testing.T) {
	an := Analyze(concat(silence(36), speech(10, 0.1)), testRate)
	if !an.HasSpeech() {
		t.Fatalf("36s silence + 10s speech: HasSpeech = false, want true (analysis: %+v)", an)
	}
	if !an.FirstWindowSilent() {
		t.Errorf("36s leading silence: FirstWindowSilent = false, want true")
	}
	if an.LeadingSilenceSeconds < 35 || an.LeadingSilenceSeconds > 37 {
		t.Errorf("LeadingSilenceSeconds = %v, want ~36", an.LeadingSilenceSeconds)
	}
}

// TestAnalyze_NoisyRoomAdaptsThreshold: with a noticeable constant noise
// floor the adaptive threshold must sit above it, so noise alone is not
// speech but speech on top of it still is.
func TestAnalyze_NoisyRoomAdaptsThreshold(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	noisy := func(seconds float64) []int16 {
		n := int(seconds * testRate)
		out := make([]int16, n)
		for i := range out {
			out[i] = int16(rng.Intn(261) - 130) // ~0.004 full scale, above absoluteFloor
		}
		return out
	}

	onlyNoise := Analyze(noisy(5), testRate)
	if onlyNoise.HasSpeech() {
		t.Errorf("constant noise floor: HasSpeech = true, want false (analysis: %+v)", onlyNoise)
	}

	withSpeech := Analyze(concat(noisy(2), speech(3, 0.1), noisy(1)), testRate)
	if !withSpeech.HasSpeech() {
		t.Errorf("speech over noise floor: HasSpeech = false, want true (analysis: %+v)", withSpeech)
	}
}

func TestAnalyze_EmptyInput(t *testing.T) {
	an := Analyze(nil, testRate)
	if an.HasSpeech() {
		t.Error("empty input: HasSpeech = true, want false")
	}
}

// writeTestWAV writes samples in the exact layout vox's recorder produces.
func writeTestWAV(t *testing.T, samples []int16) string {
	t.Helper()

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
		uint32(16), uint16(1), uint16(1), uint32(testRate),
		uint32(testRate * 2), uint16(2), uint16(16),
	} {
		binary.Write(&buf, binary.LittleEndian, v)
	}
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(len(raw)))
	buf.Write(raw)

	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnalyzeFile_RoundTrip(t *testing.T) {
	path := writeTestWAV(t, concat(silence(1), speech(2, 0.1)))
	an, err := AnalyzeFile(path)
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}
	if !an.HasSpeech() {
		t.Errorf("HasSpeech = false, want true (analysis: %+v)", an)
	}
	if an.DurationSeconds < 2.9 || an.DurationSeconds > 3.1 {
		t.Errorf("DurationSeconds = %v, want ~3", an.DurationSeconds)
	}
}

func TestAnalyzeFile_RejectsNonWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.wav")
	if err := os.WriteFile(path, []byte("RIFF fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AnalyzeFile(path); err == nil {
		t.Error("expected error for a non-WAV file, got nil")
	}
}
