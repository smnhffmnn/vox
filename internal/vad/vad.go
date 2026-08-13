// Package vad implements a lightweight energy-based voice-activity check for
// vox's own recordings (16 kHz mono PCM16 WAV).
//
// It exists to answer two questions before an upload (VOX-4):
//
//   - Is there any speech at all? Whisper invents text over silence, so a
//     recording without speech is better failed locally than transcribed.
//   - Is the first transcription window effectively silent? Then the Whisper
//     prompt must not be sent: it is decoder context the model continues, and
//     over silence that continuation IS the output (the VOX-12 incident).
//
// Deliberately simple: frame RMS against a threshold adapted to the
// recording's own noise floor. That errs on the side of "speech" — the only
// dangerous mistake here is calling real dictation silence, because the gate
// turns it into a (retryable) failure. A neural VAD would be more precise and
// is not worth the dependency for this decision.
package vad

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
)

const (
	// frameDuration is the analysis granularity. 20ms is the conventional
	// VAD frame size: short enough to localize speech onsets, long enough
	// for a stable RMS.
	frameDuration = 0.020

	// absoluteFloor is the RMS (full scale = 1.0) below which a frame is
	// never speech, whatever the adaptive threshold says. ~-52 dBFS: even
	// quiet dictation into a laptop microphone sits well above it.
	absoluteFloor = 0.0025

	// noiseFactor scales the recording's own noise floor (the 10th
	// percentile frame RMS) into the speech threshold. Speech is many times
	// louder than the pauses between words; a factor of 3 keeps breathing
	// and room tone below the line without pushing quiet speech under it.
	noiseFactor = 3.0

	// minSpeechSeconds is the cumulative speech required before the
	// recording counts as containing speech at all. Guards against a key
	// click or a pop being taken for a word; the shortest real utterance
	// ("ja") still clears 250ms of voiced frames... barely — which is the
	// point: the gate must only ever catch genuinely empty recordings.
	minSpeechSeconds = 0.25

	// firstWindow is Whisper's transcription window: the span whose content
	// decides whether the prompt has real audio to attach to.
	firstWindow = 30.0
)

// Analysis is the result of measuring a recording.
type Analysis struct {
	// DurationSeconds is the total length of the audio.
	DurationSeconds float64
	// SpeechSeconds is the cumulative duration of frames classified as speech.
	SpeechSeconds float64
	// LeadingSilenceSeconds is the time before the first speech frame. Equals
	// DurationSeconds when no speech was found.
	LeadingSilenceSeconds float64
	// FirstWindowSpeechSeconds is the cumulative speech within the first 30s —
	// the span Whisper decodes with the prompt as its context.
	FirstWindowSpeechSeconds float64
}

// HasSpeech reports whether the recording contains enough voiced audio to be
// worth uploading.
func (a Analysis) HasSpeech() bool { return a.SpeechSeconds >= minSpeechSeconds }

// FirstWindowSilent reports whether the first transcription window is
// effectively silent — the condition under which a Whisper prompt becomes a
// self-continuation seed instead of vocabulary context (VOX-12).
func (a Analysis) FirstWindowSilent() bool { return a.FirstWindowSpeechSeconds < minSpeechSeconds }

// AnalyzeFile reads a WAV file as vox writes it (PCM16) and measures it.
func AnalyzeFile(path string) (Analysis, error) {
	f, err := os.Open(path)
	if err != nil {
		return Analysis{}, err
	}
	defer f.Close()

	samples, sampleRate, err := readWAV(f)
	if err != nil {
		return Analysis{}, err
	}
	return Analyze(samples, sampleRate), nil
}

// Analyze measures mono PCM16 samples.
func Analyze(samples []int16, sampleRate int) Analysis {
	if sampleRate <= 0 || len(samples) == 0 {
		return Analysis{}
	}

	frameLen := int(float64(sampleRate) * frameDuration)
	if frameLen <= 0 {
		return Analysis{}
	}

	// RMS per frame, normalized to full scale 1.0. The trailing partial
	// frame is dropped — 20ms of audio decide nothing here.
	frames := make([]float64, 0, len(samples)/frameLen)
	for off := 0; off+frameLen <= len(samples); off += frameLen {
		var sum float64
		for _, s := range samples[off : off+frameLen] {
			v := float64(s) / 32768.0
			sum += v * v
		}
		frames = append(frames, math.Sqrt(sum/float64(frameLen)))
	}
	if len(frames) == 0 {
		return Analysis{DurationSeconds: float64(len(samples)) / float64(sampleRate)}
	}

	// The noise floor is the recording's own quiet level: the 10th
	// percentile frame. Using the recording itself makes the threshold
	// independent of microphone gain.
	sorted := append([]float64(nil), frames...)
	sort.Float64s(sorted)
	noiseFloor := sorted[len(sorted)/10]
	threshold := math.Max(absoluteFloor, noiseFloor*noiseFactor)

	an := Analysis{
		DurationSeconds:       float64(len(samples)) / float64(sampleRate),
		LeadingSilenceSeconds: float64(len(frames)) * frameDuration,
	}
	firstSpeech := -1
	for i, rms := range frames {
		if rms <= threshold {
			continue
		}
		an.SpeechSeconds += frameDuration
		if firstSpeech < 0 {
			firstSpeech = i
			an.LeadingSilenceSeconds = float64(i) * frameDuration
		}
		if float64(i)*frameDuration < firstWindow {
			an.FirstWindowSpeechSeconds += frameDuration
		}
	}
	if firstSpeech < 0 {
		an.LeadingSilenceSeconds = an.DurationSeconds
	}
	return an
}

// readWAV decodes a PCM16 WAV stream into mono samples. Multi-channel audio
// is not merged — vox records mono, and the first channel carries the voice
// either way — but the fmt chunk is parsed rather than assumed, so a foreign
// file fails loudly instead of being misread.
func readWAV(r io.Reader) ([]int16, int, error) {
	var riff [12]byte
	if _, err := io.ReadFull(r, riff[:]); err != nil {
		return nil, 0, fmt.Errorf("read RIFF header: %w", err)
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		return nil, 0, errors.New("not a WAV file")
	}

	var (
		sampleRate    int
		channels      int
		bitsPerSample int
		haveFmt       bool
	)

	for {
		var chunk [8]byte
		if _, err := io.ReadFull(r, chunk[:]); err != nil {
			if err == io.EOF {
				return nil, 0, errors.New("WAV has no data chunk")
			}
			return nil, 0, fmt.Errorf("read chunk header: %w", err)
		}
		id := string(chunk[0:4])
		size := binary.LittleEndian.Uint32(chunk[4:8])

		switch id {
		case "fmt ":
			if size < 16 {
				return nil, 0, errors.New("fmt chunk too short")
			}
			body := make([]byte, size)
			if _, err := io.ReadFull(r, body); err != nil {
				return nil, 0, fmt.Errorf("read fmt chunk: %w", err)
			}
			format := binary.LittleEndian.Uint16(body[0:2])
			channels = int(binary.LittleEndian.Uint16(body[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(body[4:8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(body[14:16]))
			if format != 1 || bitsPerSample != 16 {
				return nil, 0, fmt.Errorf("unsupported WAV format (format=%d, bits=%d) — expected PCM16", format, bitsPerSample)
			}
			if channels < 1 || sampleRate <= 0 {
				return nil, 0, fmt.Errorf("implausible WAV format (channels=%d, rate=%d)", channels, sampleRate)
			}
			haveFmt = true
		case "data":
			if !haveFmt {
				return nil, 0, errors.New("WAV data chunk before fmt chunk")
			}
			raw := make([]byte, size)
			if _, err := io.ReadFull(r, raw); err != nil {
				return nil, 0, fmt.Errorf("read data chunk: %w", err)
			}
			frameBytes := 2 * channels
			n := len(raw) / frameBytes
			samples := make([]int16, n)
			for i := 0; i < n; i++ {
				// First channel only; see the function comment.
				samples[i] = int16(binary.LittleEndian.Uint16(raw[i*frameBytes : i*frameBytes+2]))
			}
			return samples, sampleRate, nil
		default:
			// Skip unknown chunks (LIST, fact, …), including the pad byte
			// odd-sized chunks carry.
			skip := int64(size)
			if size%2 == 1 {
				skip++
			}
			if _, err := io.CopyN(io.Discard, r, skip); err != nil {
				return nil, 0, fmt.Errorf("skip %s chunk: %w", id, err)
			}
		}
	}
}
