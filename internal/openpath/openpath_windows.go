//go:build windows

package openpath

import (
	"fmt"
	"os/exec"
)

func reveal(path string) error {
	// explorer returns a non-zero exit code even on success, so the error
	// from Run is deliberately ignored here.
	cmd := exec.Command("explorer", "/select,"+path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("explorer failed: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
