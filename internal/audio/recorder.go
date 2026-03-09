package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Recording struct {
	cmd     *exec.Cmd
	file    string
	started time.Time
}

// Start begins recording audio via pw-record to a temporary WAV file.
// Returns a Recording that must be stopped with Stop().
func Start() (*Recording, error) {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("vox-%d.wav", time.Now().UnixNano()))

	cmd := exec.Command("pw-record",
		"--rate=16000",
		"--channels=1",
		"--format=s16",
		tmpFile,
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("pw-record starten fehlgeschlagen (ist PipeWire installiert?): %w", err)
	}

	return &Recording{
		cmd:     cmd,
		file:    tmpFile,
		started: time.Now(),
	}, nil
}

// Stop ends the recording and returns the path to the audio file and its duration.
func (r *Recording) Stop() (string, time.Duration, error) {
	duration := time.Since(r.started)

	// pw-record handles SIGINT gracefully and finalizes the WAV header
	if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
		r.cmd.Process.Kill()
	}
	r.cmd.Wait()

	// Verify the file exists and has content
	info, err := os.Stat(r.file)
	if err != nil || info.Size() < 100 {
		os.Remove(r.file)
		return "", 0, fmt.Errorf("Aufnahme fehlgeschlagen — kein Mikrofon erkannt?")
	}

	return r.file, duration, nil
}

// File returns the path to the recording file (for cleanup).
func (r *Recording) File() string {
	return r.file
}
