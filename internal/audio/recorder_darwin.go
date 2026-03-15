//go:build darwin

package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Start begins recording audio via sox to a temporary WAV file.
func Start() (*Recording, error) {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("vox-%d.wav", time.Now().UnixNano()))

	cmd := exec.Command("sox",
		"-d",
		"-r", "16000",
		"-c", "1",
		"-b", "16",
		"-e", "signed-integer",
		tmpFile,
	)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sox starten fehlgeschlagen (ist sox installiert? brew install sox): %w", err)
	}

	return &Recording{
		cmd:     cmd,
		file:    tmpFile,
		started: time.Now(),
	}, nil
}
