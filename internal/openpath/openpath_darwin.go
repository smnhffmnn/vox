//go:build darwin

package openpath

import (
	"fmt"
	"os/exec"
)

func reveal(path string) error {
	if err := exec.Command("open", "-R", path).Run(); err != nil {
		return fmt.Errorf("open -R failed: %w", err)
	}
	return nil
}
