package audio

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Recording represents an in-progress audio recording.
type Recording struct {
	cmd     *exec.Cmd
	file    string
	started time.Time
}

// Stop ends the recording and returns the path to the audio file and its duration.
func (r *Recording) Stop() (string, time.Duration, error) {
	duration := time.Since(r.started)

	if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
		r.cmd.Process.Kill()
	}
	r.cmd.Wait()

	info, err := os.Stat(r.file)
	if err != nil || info.Size() < 100 {
		os.Remove(r.file)
		return "", 0, fmt.Errorf("Aufnahme fehlgeschlagen — kein Mikrofon erkannt?")
	}

	return r.file, duration, nil
}

// File returns the path to the recording file.
func (r *Recording) File() string {
	return r.file
}
