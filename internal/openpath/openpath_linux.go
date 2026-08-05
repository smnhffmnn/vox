//go:build linux

package openpath

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// reveal opens the containing directory. xdg-open has no "select this file"
// equivalent, and opening the file itself would launch an editor instead.
func reveal(path string) error {
	if err := exec.Command("xdg-open", filepath.Dir(path)).Run(); err != nil {
		return fmt.Errorf("xdg-open failed: %w", err)
	}
	return nil
}
