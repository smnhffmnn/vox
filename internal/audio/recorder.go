package audio

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

// Recording represents an in-progress audio recording using native audio APIs.
type Recording struct {
	ctx     *malgo.AllocatedContext
	device  *malgo.Device
	file    string
	started time.Time

	mu      sync.Mutex
	samples []byte
}

const (
	sampleRate    = 16000
	channels      = 1
	bitsPerSample = 16
)

// Start begins recording audio from the default capture device.
// Uses native audio APIs via miniaudio: CoreAudio (macOS), WASAPI (Windows), PulseAudio/ALSA (Linux).
func Start() (*Recording, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("audio context init: %w", err)
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("vox-%d.wav", time.Now().UnixNano()))

	rec := &Recording{
		ctx:     ctx,
		file:    tmpFile,
		started: time.Now(),
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = channels
	deviceConfig.SampleRate = sampleRate
	deviceConfig.Capture.ShareMode = malgo.Shared
	// Don't change the playback device — prevents macOS from switching
	// audio output codec (e.g. AAC→SCO on Bluetooth) when capture starts.
	deviceConfig.Playback.Format = malgo.FormatUnknown
	deviceConfig.NoFixedSizedCallback = 1

	callbacks := malgo.DeviceCallbacks{
		Data: func(pOutputSamples, pInputSamples []byte, frameCount uint32) {
			rec.mu.Lock()
			rec.samples = append(rec.samples, pInputSamples...)
			rec.mu.Unlock()
		},
	}

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, callbacks)
	if err != nil {
		ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("audio device init: %w", err)
	}

	if err := device.Start(); err != nil {
		device.Uninit()
		ctx.Uninit()
		ctx.Free()
		return nil, fmt.Errorf("audio device start: %w", err)
	}

	rec.device = device
	return rec, nil
}

// Stop ends the recording and returns the path to the WAV file and its duration.
func (r *Recording) Stop() (string, time.Duration, error) {
	duration := time.Since(r.started)

	r.device.Stop()
	r.device.Uninit()
	r.ctx.Uninit()
	r.ctx.Free()

	r.mu.Lock()
	samples := r.samples
	r.mu.Unlock()

	if len(samples) < 100 {
		return "", 0, fmt.Errorf("recording failed — no microphone detected?")
	}

	if err := writeWAV(r.file, samples); err != nil {
		return "", 0, fmt.Errorf("writing audio file: %w", err)
	}

	return r.file, duration, nil
}

// File returns the path to the recording file.
func (r *Recording) File() string {
	return r.file
}

func writeWAV(path string, samples []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dataSize := uint32(len(samples))
	byteRate := uint32(sampleRate * channels * bitsPerSample / 8)
	blockAlign := uint16(channels * bitsPerSample / 8)

	w := func(data any) error {
		return binary.Write(f, binary.LittleEndian, data)
	}
	wb := func(b []byte) error {
		_, err := f.Write(b)
		return err
	}

	// RIFF header
	if err := wb([]byte("RIFF")); err != nil {
		return err
	}
	if err := w(uint32(36 + dataSize)); err != nil {
		return err
	}
	if err := wb([]byte("WAVE")); err != nil {
		return err
	}

	// fmt chunk
	if err := wb([]byte("fmt ")); err != nil {
		return err
	}
	for _, v := range []any{
		uint32(16), uint16(1), uint16(channels), uint32(sampleRate),
		byteRate, blockAlign, uint16(bitsPerSample),
	} {
		if err := w(v); err != nil {
			return err
		}
	}

	// data chunk
	if err := wb([]byte("data")); err != nil {
		return err
	}
	if err := w(dataSize); err != nil {
		return err
	}
	return wb(samples)
}
