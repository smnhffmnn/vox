//go:build linux

package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Start begins recording audio via pw-record to a temporary WAV file.
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
